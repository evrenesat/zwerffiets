package main

import (
	"context"
	"database/sql"
	"fmt"
	"sort"
	"time"
)

type PaginatedOperatorReports struct {
	Reports     []OperatorReportView
	TotalCount  int
	TotalPages  int
	CurrentPage int
	PageSize    int
}

func (a *App) listOperatorReportsPaginated(ctx context.Context, filters map[string]any, page, pageSize int) (*PaginatedOperatorReports, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 50
	}

	query, args := buildOperatorReportsQuery(filters, page, pageSize)

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	views := []OperatorReportView{}
	totalCount := 0

	for rows.Next() {
		var r Report
		var bg BikeGroup
		var note sql.NullString
		var dedupeGroupID sql.NullInt64
		var addr, city, post, muni sql.NullString
		var userID sql.NullInt64
		var tagsRaw []byte
		var rCreatedAt, rUpdatedAt time.Time
		var bgCreatedAt, bgUpdatedAt time.Time
		var bgLastReportAt time.Time
		var bgFirstQual, bgLastQual sql.NullTime

		err := rows.Scan(
			&r.ID, &r.PublicID, &rCreatedAt, &rUpdatedAt, &r.Status,
			&r.Location.Lat, &r.Location.Lng, &r.Location.AccuracyM, &tagsRaw, &note,
			&r.Source, &dedupeGroupID, &r.BikeGroupID, &r.FingerprintHash, &r.ReporterHash,
			&r.FlaggedForReview, &addr, &city, &post, &muni, &userID,
			&bg.ID, &bg.AnchorLat, &bg.AnchorLng, &bgLastReportAt, &bg.TotalReports,
			&bg.UniqueReporters, &bg.SameReporterReconfirmations, &bg.DistinctReporterReconfirmations,
			&bgFirstQual, &bgLastQual, &bg.SignalStrength, &bgCreatedAt, &bgUpdatedAt,
			&totalCount,
		)
		if err != nil {
			return nil, err
		}

		// Hydrate Report
		if note.Valid {
			r.Note = &note.String
		}
		if dedupeGroupID.Valid {
			val := int(dedupeGroupID.Int64)
			r.DedupeGroupID = &val
		}
		if addr.Valid {
			r.Address = &addr.String
		}
		if city.Valid {
			r.City = &city.String
		}
		if post.Valid {
			r.PostalCode = &post.String
		}
		if muni.Valid {
			r.Municipality = &muni.String
		}
		if userID.Valid {
			val := int(userID.Int64)
			r.UserID = &val
		}
		r.CreatedAt = rCreatedAt.UTC().Format(time.RFC3339)
		r.UpdatedAt = rUpdatedAt.UTC().Format(time.RFC3339)
		r.Tags, _ = parseTagsJSON(tagsRaw)

		// Hydrate BikeGroup
		bg.CreatedAt = bgCreatedAt.UTC().Format(time.RFC3339)
		bg.UpdatedAt = bgUpdatedAt.UTC().Format(time.RFC3339)
		bg.LastReportAt = bgLastReportAt.UTC().Format(time.RFC3339)
		if bgFirstQual.Valid {
			val := bgFirstQual.Time.UTC().Format(time.RFC3339)
			bg.FirstQualifyingReconfirmationAt = &val
		}
		if bgLastQual.Valid {
			val := bgLastQual.Time.UTC().Format(time.RFC3339)
			bg.LastQualifyingReconfirmationAt = &val
		}

		// Fetch photos (N+1 query, but N is small (page size) and optimized local query)
		photos, _ := a.listReportPhotos(ctx, r.ID)
		var previewPhotoURL *string
		if len(photos) > 0 {
			url := a.buildOperatorReportPhotoURL(r.ID, photos[0].ID)
			previewPhotoURL = &url
		}

		views = append(views, OperatorReportView{
			Report:          r,
			BikeGroupID:     bg.ID,
			SignalSummary:   bikeGroupToSignalSummary(bg),
			SignalStrength:  bg.SignalStrength,
			PreviewPhotoURL: previewPhotoURL,
		})
	}

	totalPages := 0
	if totalCount > 0 {
		totalPages = (totalCount + pageSize - 1) / pageSize
	}

	return &PaginatedOperatorReports{
		Reports:     views,
		TotalCount:  totalCount,
		TotalPages:  totalPages,
		CurrentPage: page,
		PageSize:    pageSize,
	}, nil
}

func buildOperatorReportsQuery(filters map[string]any, page, pageSize int) (string, []any) {
	// Base query
	query := `
		SELECT
			reports.id, reports.public_id, reports.created_at, reports.updated_at, reports.status,
			reports.lat, reports.lng, reports.accuracy_m, reports.tags, reports.note,
			reports.source, reports.dedupe_group_id, reports.bike_group_id, reports.fingerprint_hash, reports.reporter_hash,
			reports.flagged_for_review, reports.address, reports.city, reports.postcode, reports.municipality, reports.user_id,
			bg.id, bg.anchor_lat, bg.anchor_lng, bg.last_report_at, bg.total_reports,
			bg.unique_reporters, bg.same_reporter_reconfirmations, bg.distinct_reporter_reconfirmations,
			bg.first_qualifying_reconfirmation_at, bg.last_qualifying_reconfirmation_at, bg.signal_strength, bg.created_at, bg.updated_at,
			COUNT(*) OVER() as total_count
		FROM reports
		JOIN bike_groups bg ON reports.bike_group_id = bg.id
		WHERE 1=1
	`
	whereClause, args := buildReportFilters(filters)
	query += whereClause
	argIndex := len(args) + 1

	if signalStrength, ok := filters["signal_strength"].(string); ok && signalStrength != "" {
		query += fmt.Sprintf(" AND bg.signal_strength = $%d", argIndex)
		args = append(args, signalStrength)
		argIndex++
	}
	if hasQual, ok := filters["has_qualifying_reconfirmation"].(bool); ok && hasQual {
		query += " AND (bg.same_reporter_reconfirmations > 0 OR bg.distinct_reporter_reconfirmations > 0)"
	} else if hasQual, ok := filters["has_qualifying_reconfirmation"].(bool); ok && !hasQual {
		query += " AND (bg.same_reporter_reconfirmations = 0 AND bg.distinct_reporter_reconfirmations = 0)"
	}
	if strongOnly, ok := filters["strong_only"].(bool); ok && strongOnly {
		query += " AND bg.signal_strength = 'strong_distinct_reporters'"
	}

	// Apply Sorting
	sortBy, _ := filters["sort"].(string)
	if sortBy == "signal" {
		// Custom sort for signal strength priority
		// We can't easily map string enum to integer in pure SQL without a CASE statement or lookup table
		// Priority: strong_distinct_reporters (2) > weak_same_reporter (1) > none (0)
		query += `
			ORDER BY
			CASE bg.signal_strength
				WHEN 'strong_distinct_reporters' THEN 2
				WHEN 'weak_same_reporter' THEN 1
				ELSE 0
			END DESC,
			reports.created_at DESC
		`
	} else {
		// Default: newest first
		query += " ORDER BY reports.created_at DESC"
	}

	// Apply Pagination
	offset := (page - 1) * pageSize
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", argIndex, argIndex+1)
	args = append(args, pageSize, offset)

	return query, args
}

// sortReports is typically unused if SQL does the sorting, but keeping signature for compile compat if needed elsewhere
func (a *App) sortReports(reports []OperatorReportView, by string) {
	if by == "signal" {
		sort.Slice(reports, func(i, j int) bool {
			left := signalStrengthPriority[reports[i].SignalStrength]
			right := signalStrengthPriority[reports[j].SignalStrength]
			if left != right {
				return left > right
			}
			return reports[i].CreatedAt > reports[j].CreatedAt
		})
	} else {
		// Default newest
		sort.Slice(reports, func(i, j int) bool {
			return reports[i].CreatedAt > reports[j].CreatedAt
		})
	}
}

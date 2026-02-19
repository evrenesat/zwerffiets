package main

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

const photoStorageNameRandomBytes = 16

func (a *App) saveReportPhotosTx(ctx context.Context, tx *sql.Tx, reportID int, photos []PhotoUpload) error {
	reportDir := filepath.Join(a.cfg.DataRoot, "uploads", "reports", strconv.Itoa(reportID))
	if err := os.MkdirAll(reportDir, 0o755); err != nil {
		return err
	}
	for _, photo := range photos {
		ext := extensionFromMime(photo.MimeType, photo.Name)
		var photoID int
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO report_photos (report_id, storage_path, mime_type, filename, size_bytes)
			VALUES ($1, '', $2, $3, $4)
			RETURNING id
		`, reportID, photo.MimeType, photo.Name, len(photo.Bytes)).Scan(&photoID); err != nil {
			return err
		}

		fileName, err := generatePhotoStorageFileName(ext)
		if err != nil {
			return err
		}
		fullPath := filepath.Join(reportDir, fileName)
		relPath, _ := filepath.Rel(a.cfg.DataRoot, fullPath)

		if err := os.WriteFile(fullPath, photo.Bytes, 0o644); err != nil {
			return err
		}

		if _, err := tx.ExecContext(ctx, `
			UPDATE report_photos SET storage_path = $1 WHERE id = $2
		`, relPath, photoID); err != nil {
			return err
		}
	}
	return nil
}

func (a *App) listReportPhotos(ctx context.Context, reportID int) ([]ReportPhoto, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, report_id, storage_path, mime_type, filename, size_bytes, created_at
		FROM report_photos
		WHERE report_id = $1
		ORDER BY created_at ASC
	`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	photos := make([]ReportPhoto, 0)
	for rows.Next() {
		var photo ReportPhoto
		var createdAt time.Time
		if err := rows.Scan(
			&photo.ID,
			&photo.ReportID,
			&photo.StoragePath,
			&photo.MimeType,
			&photo.Filename,
			&photo.SizeBytes,
			&createdAt,
		); err != nil {
			return nil, err
		}

		photo.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		photos = append(photos, photo)
	}

	return photos, rows.Err()
}

func (a *App) getReportPhotoByID(ctx context.Context, reportID int, photoID int) (*ReportPhoto, error) {
	var photo ReportPhoto
	var createdAt time.Time
	err := a.db.QueryRowContext(ctx, `
		SELECT id, report_id, storage_path, mime_type, filename, size_bytes, created_at
		FROM report_photos
		WHERE id = $1 AND report_id = $2
	`, photoID, reportID).Scan(
		&photo.ID,
		&photo.ReportID,
		&photo.StoragePath,
		&photo.MimeType,
		&photo.Filename,
		&photo.SizeBytes,
		&createdAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}

	photo.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	return &photo, nil
}

func (a *App) buildOperatorReportPhotoURL(reportID int, photoID int) string {
	return fmt.Sprintf("/api/v1/operator/reports/%d/photos/%d", reportID, photoID)
}

func (a *App) toOperatorReportPhotoViews(reportID int, photos []ReportPhoto) []OperatorReportPhotoView {
	views := make([]OperatorReportPhotoView, 0, len(photos))
	for _, photo := range photos {
		views = append(views, OperatorReportPhotoView{
			ID:        photo.ID,
			URL:       a.buildOperatorReportPhotoURL(reportID, photo.ID),
			MimeType:  photo.MimeType,
			Filename:  photo.Filename,
			SizeBytes: photo.SizeBytes,
			CreatedAt: photo.CreatedAt,
		})
	}
	return views
}

func extensionFromMime(mimeType string, fallbackName string) string {
	extensions, _ := mime.ExtensionsByType(mimeType)
	if len(extensions) > 0 {
		return extensions[0]
	}
	ext := filepath.Ext(fallbackName)
	if ext == "" {
		return ".jpg"
	}
	return ext
}

func generatePhotoStorageFileName(ext string) (string, error) {
	buffer := make([]byte, photoStorageNameRandomBytes)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer) + ext, nil
}

func (a *App) addEvent(ctx context.Context, reportID int, eventType, actor string, metadata map[string]any) error {
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO report_events (report_id, type, actor, metadata)
		VALUES ($1, $2, $3, $4)
	`, reportID, eventType, actor, anyMapToJSON(metadata))
	return err
}

func (a *App) addEventTx(ctx context.Context, tx *sql.Tx, reportID int, eventType, actor string, metadata map[string]any) error {
	_, err := tx.ExecContext(ctx, `
		INSERT INTO report_events (report_id, type, actor, metadata)
		VALUES ($1, $2, $3, $4)
	`, reportID, eventType, actor, anyMapToJSON(metadata))
	return err
}

func (a *App) listEvents(ctx context.Context, reportID int) ([]ReportEvent, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, report_id, type, actor, metadata, created_at
		FROM report_events
		WHERE report_id = $1
		ORDER BY created_at ASC
	`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	events := make([]ReportEvent, 0)
	for rows.Next() {
		var event ReportEvent
		var metadataRaw []byte
		var createdAt time.Time
		if err := rows.Scan(&event.ID, &event.ReportID, &event.Type, &event.Actor, &metadataRaw, &createdAt); err != nil {
			return nil, err
		}
		event.Metadata = jsonToAnyMap(metadataRaw)
		event.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		events = append(events, event)
	}
	return events, rows.Err()
}

func (a *App) selectBikeGroupForReport(ctx context.Context, payload ReportCreatePayload, now time.Time) (*BikeGroup, error) {
	since := now.AddDate(0, 0, -signalLookbackDays).Format(time.RFC3339)
	reports, err := a.listReportsSince(ctx, since)
	if err != nil {
		return nil, err
	}
	bestScoreByGroup := make(map[int]float64)

	incoming := Report{Location: payload.Location, Tags: payload.Tags}
	for _, candidate := range reports {
		if candidate.Status == "invalid" {
			continue
		}
		score := scoreSignalGroupCandidate(incoming, candidate, now)
		if score == nil {
			continue
		}
		if currentBest, ok := bestScoreByGroup[candidate.BikeGroupID]; !ok || *score > currentBest {
			bestScoreByGroup[candidate.BikeGroupID] = *score
		}
	}
	if len(bestScoreByGroup) == 0 {
		return nil, nil
	}

	type candidate struct {
		id    int
		score float64
	}
	candidates := make([]candidate, 0, len(bestScoreByGroup))
	for id, score := range bestScoreByGroup {
		candidates = append(candidates, candidate{id: id, score: score})
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].score > candidates[j].score })
	return a.getBikeGroupByID(ctx, candidates[0].id)
}

func (a *App) createBikeGroup(ctx context.Context, anchor ReportLocation) (BikeGroup, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	var groupID int
	err := a.db.QueryRowContext(ctx, `
		INSERT INTO bike_groups (
			anchor_lat, anchor_lng, last_report_at, total_reports,
			unique_reporters, same_reporter_reconfirmations,
			distinct_reporter_reconfirmations,
			first_qualifying_reconfirmation_at,
			last_qualifying_reconfirmation_at,
			signal_strength
		)
		VALUES ($1, $2, $3, 0, 0, 0, 0, NULL, NULL, 'none')
		RETURNING id
	`, anchor.Lat, anchor.Lng, now).Scan(&groupID)
	if err != nil {
		return BikeGroup{}, err
	}
	group, err := a.getBikeGroupByID(ctx, groupID)
	if err != nil {
		return BikeGroup{}, err
	}
	if group == nil {
		return BikeGroup{}, fmt.Errorf("created bike group not found")
	}
	return *group, nil
}

func (a *App) getBikeGroupByID(ctx context.Context, groupID int) (*BikeGroup, error) {
	var group BikeGroup
	var createdAt time.Time
	var updatedAt time.Time
	var lastReportAt time.Time
	var firstQual sql.NullTime
	var lastQual sql.NullTime
	err := a.db.QueryRowContext(ctx, `
		SELECT
			id, anchor_lat, anchor_lng,
			last_report_at,
			total_reports, unique_reporters,
			same_reporter_reconfirmations,
			distinct_reporter_reconfirmations,
			first_qualifying_reconfirmation_at,
			last_qualifying_reconfirmation_at,
			signal_strength,
			created_at,
			updated_at
		FROM bike_groups
		WHERE id = $1
	`, groupID).Scan(
		&group.ID,
		&group.AnchorLat,
		&group.AnchorLng,
		&lastReportAt,
		&group.TotalReports,
		&group.UniqueReporters,
		&group.SameReporterReconfirmations,
		&group.DistinctReporterReconfirmations,
		&firstQual,
		&lastQual,
		&group.SignalStrength,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	group.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	group.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	group.LastReportAt = lastReportAt.UTC().Format(time.RFC3339)
	if firstQual.Valid {
		value := firstQual.Time.UTC().Format(time.RFC3339)
		group.FirstQualifyingReconfirmationAt = &value
	}
	if lastQual.Valid {
		value := lastQual.Time.UTC().Format(time.RFC3339)
		group.LastQualifyingReconfirmationAt = &value
	}
	return &group, nil
}

func (a *App) updateBikeGroup(ctx context.Context, group BikeGroup) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE bike_groups
		SET
			anchor_lat = $1,
			anchor_lng = $2,
			last_report_at = $3,
			total_reports = $4,
			unique_reporters = $5,
			same_reporter_reconfirmations = $6,
			distinct_reporter_reconfirmations = $7,
			first_qualifying_reconfirmation_at = $8,
			last_qualifying_reconfirmation_at = $9,
			signal_strength = $10,
			updated_at = NOW()
		WHERE id = $11
	`, group.AnchorLat, group.AnchorLng, group.LastReportAt, group.TotalReports, group.UniqueReporters, group.SameReporterReconfirmations, group.DistinctReporterReconfirmations, group.FirstQualifyingReconfirmationAt, group.LastQualifyingReconfirmationAt, group.SignalStrength, group.ID)
	return err
}

func (a *App) getReportByPublicID(ctx context.Context, publicID string) (*Report, error) {
	rows, err := a.db.QueryContext(ctx, reportSelect+` WHERE public_id = $1 LIMIT 1`, publicID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		return &report, nil
	}
	return nil, rows.Err()
}

func (a *App) getReportByID(ctx context.Context, reportID int) (*Report, error) {
	rows, err := a.db.QueryContext(ctx, reportSelect+` WHERE id = $1 LIMIT 1`, reportID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	if rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		return &report, nil
	}
	return nil, rows.Err()
}

const reportSelect = `
	SELECT
		id,
		public_id,
		created_at,
		updated_at,
		status,
		lat,
		lng,
		accuracy_m,
		tags,
		note,
		source,
		dedupe_group_id,
		bike_group_id,
		fingerprint_hash,
		reporter_hash,
		flagged_for_review,
		address,
		city,
		postcode,
		municipality,
		user_id
	FROM reports
`

type rowScanner interface {
	Scan(dest ...any) error
}

func scanReport(scanner rowScanner) (Report, error) {
	var report Report
	var createdAt time.Time
	var updatedAt time.Time
	var tagsRaw []byte
	var note sql.NullString
	var dedupeGroupID sql.NullInt64
	var addr, city, post, muni sql.NullString
	var userID sql.NullInt64
	if err := scanner.Scan(
		&report.ID,
		&report.PublicID,
		&createdAt,
		&updatedAt,
		&report.Status,
		&report.Location.Lat,
		&report.Location.Lng,
		&report.Location.AccuracyM,
		&tagsRaw,
		&note,
		&report.Source,
		&dedupeGroupID,
		&report.BikeGroupID,
		&report.FingerprintHash,
		&report.ReporterHash,
		&report.FlaggedForReview,
		&addr,
		&city,
		&post,
		&muni,
		&userID,
	); err != nil {
		return Report{}, err
	}
	if note.Valid {
		report.Note = &note.String
	}
	if dedupeGroupID.Valid {
		val := int(dedupeGroupID.Int64)
		report.DedupeGroupID = &val
	}
	if addr.Valid {
		report.Address = &addr.String
	}
	if city.Valid {
		report.City = &city.String
	}
	if post.Valid {
		report.PostalCode = &post.String
	}
	if muni.Valid {
		report.Municipality = &muni.String
	}
	if userID.Valid {
		val := int(userID.Int64)
		report.UserID = &val
	}
	report.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	report.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	tags, err := parseTagsJSON(tagsRaw)
	if err != nil {
		return Report{}, err
	}
	report.Tags = tags
	return report, nil
}

func (a *App) listReports(ctx context.Context, filters map[string]any) ([]Report, error) {
	query := reportSelect + ` WHERE 1=1`
	whereClause, args := buildReportFilters(filters)
	query += whereClause

	query += " ORDER BY reports.created_at DESC"
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := make([]Report, 0)
	for rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (a *App) listReportsSince(ctx context.Context, sinceISO string) ([]Report, error) {
	rows, err := a.db.QueryContext(ctx, reportSelect+` WHERE created_at >= $1 ORDER BY created_at ASC`, sinceISO)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := make([]Report, 0)
	for rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (a *App) listReportsByBikeGroupID(ctx context.Context, bikeGroupID int) ([]Report, error) {
	rows, err := a.db.QueryContext(ctx, reportSelect+` WHERE bike_group_id = $1 ORDER BY created_at ASC`, bikeGroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	reports := make([]Report, 0)
	for rows.Next() {
		report, err := scanReport(rows)
		if err != nil {
			return nil, err
		}
		reports = append(reports, report)
	}
	return reports, rows.Err()
}

func (a *App) listOpenReportsSince(ctx context.Context, sinceISO string) ([]Report, error) {
	reports, err := a.listReportsSince(ctx, sinceISO)
	if err != nil {
		return nil, err
	}
	filtered := make([]Report, 0)
	for _, report := range reports {
		if containsString(openReportStatuses, report.Status) {
			filtered = append(filtered, report)
		}
	}
	return filtered, nil
}

func (a *App) operatorReportsQueryFromURL(rawURL string) map[string]any {
	_ = rawURL
	return map[string]any{}
}

func (a *App) createReportQueryForGroups(ctx context.Context, since string) ([]Report, error) {
	return a.listReportsSince(ctx, since)
}

func (a *App) buildStatusFilter(status string) bool {
	return containsString(reportStatuses, status)
}

func (a *App) isProduction() bool {
	return strings.EqualFold(a.cfg.Env, "production")
}

func (a *App) parseOperatorReportsFilters(c *gin.Context) map[string]any {
	_ = c
	return map[string]any{}
}

func (a *App) operatorReportsSortSignal(reports []OperatorReportView) {
	sort.Slice(reports, func(i, j int) bool {
		left := signalStrengthPriority[reports[i].SignalStrength]
		right := signalStrengthPriority[reports[j].SignalStrength]
		if left != right {
			return left > right
		}
		return reports[i].CreatedAt > reports[j].CreatedAt
	})
}

func (a *App) parseOperatorReportID(c *gin.Context) int {
	id, _ := strconv.Atoi(c.Param("id"))
	return id
}

func (a *App) parseExportID(c *gin.Context) int {
	id, _ := strconv.Atoi(c.Param("id"))
	return id
}

func (a *App) parseReportPublicID(c *gin.Context) string {
	return c.Param("public_id")
}

func (a *App) cleanMimeType(input string) string {
	value := strings.TrimSpace(strings.ToLower(input))
	if strings.Contains(value, ";") {
		value = strings.SplitN(value, ";", 2)[0]
	}
	return value
}

func (a *App) detectMimeType(data []byte, fallback string) string {
	if fallback != "" {
		mimeType := a.cleanMimeType(fallback)
		if _, ok := allowedImageTypes[mimeType]; ok {
			return mimeType
		}
	}
	mimeType := a.cleanMimeType(http.DetectContentType(data))
	if _, ok := allowedImageTypes[mimeType]; ok {
		return mimeType
	}
	return ""
}

func (a *App) ensureUniquePublicID(ctx context.Context) (string, error) {
	for range 5 {
		candidate := generatePublicID()
		report, err := a.getReportByPublicID(ctx, candidate)
		if err != nil {
			return "", err
		}
		if report == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("unable to generate unique public id")
}

func (a *App) parsePeriodType(value string) string {
	if value == "monthly" {
		return "monthly"
	}
	if value == "all" {
		return "all"
	}
	return "weekly"
}

func (a *App) writeJSON(c *gin.Context, status int, payload any) {
	c.JSON(status, payload)
}

func (a *App) parseBoolQuery(c *gin.Context, key string) (bool, bool) {
	raw := strings.TrimSpace(c.Query(key))
	if raw == "" {
		return false, false
	}
	return raw == "true", true
}

func (a *App) parseStringQuery(c *gin.Context, key string) (string, bool) {
	value := strings.TrimSpace(c.Query(key))
	if value == "" {
		return "", false
	}
	return value, true
}

func (a *App) parseFloat(input string) (float64, error) {
	return strconv.ParseFloat(strings.TrimSpace(input), 64)
}

func (a *App) parseJSONTags(raw string) ([]string, error) {
	var tags []string
	if err := json.Unmarshal([]byte(raw), &tags); err != nil {
		return nil, err
	}
	return tags, nil
}

func (a *App) parseClientTimestamp(raw string) *string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return &raw
}

func (a *App) noop() {}

func (a *App) updateReportDetails(ctx context.Context, reportID int, municipality, address, postcode string, lat, lng float64) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE reports
		SET municipality = $1, address = $2, postcode = $3, lat = $4, lng = $5, updated_at = NOW()
		WHERE id = $6
	`, municipality, address, postcode, lat, lng, reportID)
	return err
}

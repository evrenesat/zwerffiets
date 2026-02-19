package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/go-pdf/fpdf"
)

func (a *App) operatorReportEventsHandler(c *gin.Context) {
	reportID := a.parseOperatorReportID(c)
	if reportID == 0 {
		writeAPIError(c, &apiError{Status: http.StatusBadRequest, Code: "invalid_id", Message: "Invalid report ID"})
		return
	}
	events, err := a.listEvents(c.Request.Context(), reportID)
	if err != nil {
		writeAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, events)
}

func (a *App) operatorUpdateStatusHandler(c *gin.Context) {
	session, err := getOperatorSession(c)
	if err != nil {
		writeAPIError(c, &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Operator session required"})
		return
	}
	var body struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeAPIError(c, &apiError{Status: http.StatusBadRequest, Code: "invalid_payload", Message: "Invalid payload"})
		return
	}

	reportID := a.parseOperatorReportID(c)
	if reportID == 0 {
		writeAPIError(c, &apiError{Status: http.StatusBadRequest, Code: "invalid_id", Message: "Invalid report ID"})
		return
	}

	updated, err := a.updateReportStatus(c.Request.Context(), reportID, body.Status, session)
	if err != nil {
		writeAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, updated)
}

func (a *App) updateReportStatus(ctx context.Context, reportID int, nextStatus string, session OperatorSession) (*Report, error) {
	if !containsString(reportStatuses, nextStatus) {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "invalid_status_transition", Message: "Invalid status"}
	}

	current, err := a.getReportByID(ctx, reportID)
	if err != nil {
		return nil, err
	}
	if current == nil {
		return nil, &apiError{Status: http.StatusNotFound, Code: "report_not_found", Message: "Report not found"}
	}
	allowed := false
	for _, status := range statusTransitions[current.Status] {
		if status == nextStatus {
			allowed = true
			break
		}
	}
	if !allowed {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "invalid_status_transition", Message: fmt.Sprintf("Cannot transition from %s to %s", current.Status, nextStatus)}
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE reports SET status = $1, updated_at = NOW() WHERE id = $2`, nextStatus, reportID); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := a.addEventTx(ctx, tx, reportID, "status_changed", session.Email, map[string]any{"status": nextStatus}); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return a.getReportByID(ctx, reportID)
}

func (a *App) operatorMergeHandler(c *gin.Context) {
	session, err := getOperatorSession(c)
	if err != nil {
		writeAPIError(c, &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Operator session required"})
		return
	}
	var body struct {
		CanonicalReportID  int   `json:"canonical_report_id"`
		DuplicateReportIDs []int `json:"duplicate_report_ids"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		writeAPIError(c, &apiError{Status: http.StatusBadRequest, Code: "invalid_payload", Message: "Invalid payload"})
		return
	}
	if body.CanonicalReportID == 0 || len(body.DuplicateReportIDs) == 0 {
		writeAPIError(c, &apiError{Status: http.StatusBadRequest, Code: "invalid_payload", Message: "canonical_report_id and duplicate_report_ids are required"})
		return
	}

	group, err := a.mergeDuplicateReports(c.Request.Context(), body.CanonicalReportID, body.DuplicateReportIDs, session)
	if err != nil {
		writeAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, group)
}

func (a *App) mergeDuplicateReports(ctx context.Context, canonicalReportID int, duplicateReportIDs []int, session OperatorSession) (*DedupeGroup, error) {
	canonical, err := a.getReportByID(ctx, canonicalReportID)
	if err != nil {
		return nil, err
	}
	if canonical == nil {
		return nil, &apiError{Status: http.StatusNotFound, Code: "canonical_not_found", Message: "Canonical report not found"}
	}
	for _, id := range duplicateReportIDs {
		report, err := a.getReportByID(ctx, id)
		if err != nil {
			return nil, err
		}
		if report == nil {
			return nil, &apiError{Status: http.StatusNotFound, Code: "duplicate_not_found", Message: fmt.Sprintf("Duplicate report not found: %d", id)}
		}
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	var existingID int
	var mergedRaw []byte
	err = tx.QueryRowContext(ctx, `SELECT id, merged_report_ids FROM dedupe_groups WHERE canonical_report_id = $1 LIMIT 1`, canonicalReportID).Scan(&existingID, &mergedRaw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		_ = tx.Rollback()
		return nil, err
	}

	mergedSet := make(map[int]struct{})
	if err == nil {
		var current []int
		_ = json.Unmarshal(mergedRaw, &current)
		for _, value := range current {
			mergedSet[value] = struct{}{}
		}
	}
	for _, value := range duplicateReportIDs {
		mergedSet[value] = struct{}{}
	}
	merged := make([]int, 0, len(mergedSet))
	for value := range mergedSet {
		merged = append(merged, value)
	}
	sort.Ints(merged)

	groupID := existingID
	if groupID == 0 {
		// New dedupe group
		mergedJSON, _ := json.Marshal(merged)
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO dedupe_groups (canonical_report_id, merged_report_ids, created_by)
			VALUES ($1, $2, $3)
			RETURNING id
		`, canonicalReportID, mergedJSON, session.Email).Scan(&groupID); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	} else {
		mergedJSON, _ := json.Marshal(merged)
		_, err = tx.ExecContext(ctx, `
			UPDATE dedupe_groups
			SET merged_report_ids = $1
			WHERE id = $2
		`, mergedJSON, groupID)
		if err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}

	idsToUpdate := append([]int{canonicalReportID}, merged...)
	for _, id := range idsToUpdate {
		if _, err := tx.ExecContext(ctx, `UPDATE reports SET dedupe_group_id = $1, updated_at = NOW() WHERE id = $2`, groupID, id); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}

	for _, mergedID := range duplicateReportIDs {
		if err := a.addEventTx(ctx, tx, mergedID, "merged", session.Email, map[string]any{"canonicalReportID": canonicalReportID, "dedupeGroupId": groupID}); err != nil {
			_ = tx.Rollback()
			return nil, err
		}
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	var createdAt time.Time
	var createdBy string
	if err := a.db.QueryRowContext(ctx, `SELECT created_at, created_by FROM dedupe_groups WHERE id = $1`, groupID).Scan(&createdAt, &createdBy); err != nil {
		return nil, err
	}

	return &DedupeGroup{
		ID:                groupID,
		CanonicalReportID: canonicalReportID,
		MergedReportIDs:   merged,
		CreatedAt:         createdAt.UTC().Format(time.RFC3339),
		CreatedBy:         createdBy,
	}, nil
}

func (a *App) operatorExportsHandler(c *gin.Context) {
	session, err := getOperatorSession(c)
	if err != nil {
		writeAPIError(c, &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Operator session required"})
		return
	}
	exports, err := a.listExportBatches(c.Request.Context(), session)
	if err != nil {
		writeAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, exports)
}

func (a *App) operatorGenerateExportHandler(c *gin.Context) {
	session, err := getOperatorSession(c)
	if err != nil {
		writeAPIError(c, &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Operator session required"})
		return
	}

	var body map[string]any
	if err := c.ShouldBindJSON(&body); err != nil {
		writeAPIError(c, &apiError{Status: http.StatusBadRequest, Code: "invalid_payload", Message: "Invalid payload"})
		return
	}

	batch, err := a.generateExportBatch(c.Request.Context(), body, session)
	if err != nil {
		writeAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, batch)
}

func (a *App) generateExportBatch(ctx context.Context, input map[string]any, session OperatorSession) (*ExportBatch, error) {
	periodType, _ := input["period_type"].(string)
	if periodType == "" {
		periodType = "weekly"
	}
	if periodType != "weekly" && periodType != "monthly" && periodType != "all" {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "invalid_period", Message: "Invalid period type"}
	}

	var requestedStart *string
	if value, ok := input["period_start"].(string); ok && strings.TrimSpace(value) != "" {
		trimmed := strings.TrimSpace(value)
		requestedStart = &trimmed
	}
	var requestedEnd *string
	if value, ok := input["period_end"].(string); ok && strings.TrimSpace(value) != "" {
		trimmed := strings.TrimSpace(value)
		requestedEnd = &trimmed
	}

	periodStart, periodEnd, err := getReportWindow(periodType, requestedStart, requestedEnd)
	if err != nil {
		return nil, err
	}

	status, _ := input["status"].(string)
	municipality, _ := input["municipality"].(string)

	municipality, err = exportMunicipalityForSession(session, municipality)
	if err != nil {
		return nil, err
	}

	filters := map[string]any{
		"from": periodStart,
		"to":   periodEnd,
	}
	if status != "" {
		filters["status"] = status
	}
	if municipality != "" {
		filters["city"] = municipality // listReports uses "city" filter key for municipality column
	}

	filteredReports, err := a.listReports(ctx, filters)
	if err != nil {
		return nil, err
	}

	titleParts := []string{"ZwerfFiets Export"}
	if municipality != "" {
		titleParts = append(titleParts, municipality)
	}
	if status != "" {
		titleParts = append(titleParts, fmt.Sprintf("Status: %s", status))
	}
	title := strings.Join(titleParts, " - ")

	artifacts, err := buildExportArtifacts(filteredReports, periodStart, periodEnd, title)
	if err != nil {
		return nil, err
	}

	var exportID int
	// We'll insert and get the ID.
	// But we need the ID to name files?
	// We can use a temp ID or just insert first.
	// We need 2 steps: insert to get ID, then save files, then update paths.

	var filterStatus, filterMunicipality sql.NullString
	if status != "" {
		filterStatus = sql.NullString{String: status, Valid: true}
	}
	if municipality != "" {
		filterMunicipality = sql.NullString{String: municipality, Valid: true}
	}

	if err := a.db.QueryRowContext(ctx, `
		INSERT INTO exports (
			period_type, period_start, period_end, generated_by, row_count,
			csv_path, geojson_path, pdf_path, filter_status, filter_municipality
		)
		VALUES ($1, $2, $3, $4, $5, '', '', '', $6, $7)
		RETURNING id
	`, periodType, periodStart, periodEnd, session.Email, len(filteredReports), filterStatus, filterMunicipality).Scan(&exportID); err != nil {
		return nil, err
	}

	exportDir := filepath.Join(a.cfg.DataRoot, "exports", strconv.Itoa(exportID))
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return nil, err
	}

	periodFilePart := sanitizeFileNamePart(periodStart)
	baseName := fmt.Sprintf("zwerffiets-%s-%s", periodType, periodFilePart)
	if municipality != "" {
		baseName = fmt.Sprintf("zwerffiets-%s-%s-%s", strings.ToLower(sanitizeFileNamePart(municipality)), periodType, periodFilePart)
	}

	csvFile := filepath.Join(exportDir, baseName+".csv")
	geoFile := filepath.Join(exportDir, baseName+".geojson")
	pdfFile := filepath.Join(exportDir, baseName+".pdf")

	if err := os.WriteFile(csvFile, []byte(artifacts.CSV), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(geoFile, []byte(artifacts.GeoJSON), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(pdfFile, artifacts.PDF, 0o644); err != nil {
		return nil, err
	}

	relCSV, _ := filepath.Rel(a.cfg.DataRoot, csvFile)
	relGeo, _ := filepath.Rel(a.cfg.DataRoot, geoFile)
	relPDF, _ := filepath.Rel(a.cfg.DataRoot, pdfFile)

	if _, err := a.db.ExecContext(ctx, `
		UPDATE exports
		SET csv_path = $1, geojson_path = $2, pdf_path = $3
		WHERE id = $4
	`, relCSV, relGeo, relPDF, exportID); err != nil {
		return nil, err
	}

	a.log.Info("export email queued",
		"email", map[string]any{
			"recipients": []string{a.cfg.ExportEmailTo},
			"subject":    fmt.Sprintf("[ZwerfFiets] %s export generated", periodType),
			"body":       fmt.Sprintf("Export %d generated for %s - %s. Open %s to download artifacts.", exportID, periodStart, periodEnd, buildPublicURL(a.cfg.PublicBaseURL, "/bikeadmin/exports")),
		},
	)

	return &ExportBatch{
		ID:          exportID,
		PeriodType:  periodType,
		PeriodStart: periodStart,
		PeriodEnd:   periodEnd,
		GeneratedAt: time.Now().UTC().Format(time.RFC3339),
		GeneratedBy: session.Email,
		RowCount:    len(filteredReports),
		Artifacts: ExportArtifacts{
			CSV:     "",
			GeoJSON: "",
			PDF:     []byte{},
		},
	}, nil
}

func getReportWindow(periodType string, requestedStart, requestedEnd *string) (string, string, error) {
	if requestedStart != nil && requestedEnd != nil {
		return *requestedStart, *requestedEnd, nil
	}

	location, err := time.LoadLocation("Europe/Amsterdam")
	if err != nil {
		return "", "", err
	}
	now := time.Now().In(location)

	if periodType == "all" {
		start := time.Date(2020, 1, 1, 0, 0, 0, 0, location)
		end := now
		return start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339), nil
	}

	if periodType == "weekly" {
		previousWeek := now.AddDate(0, 0, -7)
		start := startOfWeek(previousWeek)
		end := start.AddDate(0, 0, 7).Add(-time.Nanosecond)
		return start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339), nil
	}

	previousMonth := now.AddDate(0, -1, 0)
	start := time.Date(previousMonth.Year(), previousMonth.Month(), 1, 0, 0, 0, 0, location)
	end := start.AddDate(0, 1, 0).Add(-time.Nanosecond)
	return start.UTC().Format(time.RFC3339), end.UTC().Format(time.RFC3339), nil
}

func startOfWeek(value time.Time) time.Time {
	weekday := int(value.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	start := value.AddDate(0, 0, -(weekday - 1))
	return time.Date(start.Year(), start.Month(), start.Day(), 0, 0, 0, 0, value.Location())
}

func sanitizeFileNamePart(value string) string {
	value = strings.ReplaceAll(value, ":", "-")
	value = strings.ReplaceAll(value, ".", "-")
	return value
}

func buildExportArtifacts(reports []Report, periodStart, periodEnd, title string) (ExportArtifacts, error) {
	sortedReports := append([]Report{}, reports...)
	sort.Slice(sortedReports, func(i, j int) bool {
		if sortedReports[i].CreatedAt != sortedReports[j].CreatedAt {
			return sortedReports[i].CreatedAt < sortedReports[j].CreatedAt
		}
		return sortedReports[i].ID < sortedReports[j].ID
	})

	csvData, err := buildCSV(sortedReports)
	if err != nil {
		return ExportArtifacts{}, err
	}
	geoJSON, err := buildGeoJSON(sortedReports)
	if err != nil {
		return ExportArtifacts{}, err
	}
	pdfData, err := buildPDF(sortedReports, periodStart, periodEnd, title)
	if err != nil {
		return ExportArtifacts{}, err
	}

	return ExportArtifacts{CSV: csvData, GeoJSON: geoJSON, PDF: pdfData}, nil
}

func buildCSV(reports []Report) (string, error) {
	buffer := bytes.NewBuffer(nil)
	writer := csv.NewWriter(buffer)
	headers := []string{"report_id", "public_id", "created_at", "status", "lat", "lng", "accuracy_m", "tags", "note", "dedupe_group_id"}
	if err := writer.Write(headers); err != nil {
		return "", err
	}
	for _, report := range reports {
		note := ""
		if report.Note != nil {
			note = *report.Note
		}
		dedupe := ""
		if report.DedupeGroupID != nil {
			dedupe = strconv.Itoa(*report.DedupeGroupID)
		}
		row := []string{
			strconv.Itoa(report.ID),
			report.PublicID,
			report.CreatedAt,
			report.Status,
			fmt.Sprintf("%f", report.Location.Lat),
			fmt.Sprintf("%f", report.Location.Lng),
			fmt.Sprintf("%f", report.Location.AccuracyM),
			strings.Join(report.Tags, "|"),
			note,
			dedupe,
		}
		if err := writer.Write(row); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return buffer.String(), nil
}

func buildGeoJSON(reports []Report) (string, error) {
	features := make([]map[string]any, 0, len(reports))
	for _, report := range reports {
		features = append(features, map[string]any{
			"type": "Feature",
			"geometry": map[string]any{
				"type":        "Point",
				"coordinates": []float64{report.Location.Lng, report.Location.Lat},
			},
			"properties": map[string]any{
				"report_id":       report.ID,
				"public_id":       report.PublicID,
				"created_at":      report.CreatedAt,
				"status":          report.Status,
				"tags":            report.Tags,
				"dedupe_group_id": report.DedupeGroupID,
			},
		})
	}
	payload := map[string]any{"type": "FeatureCollection", "features": features}
	encoded, err := json.MarshalIndent(payload, "", "  ")
	if err != nil {
		return "", err
	}
	return string(encoded), nil
}

func buildPDF(reports []Report, periodStart, periodEnd, title string) ([]byte, error) {
	pdf := fpdf.New("P", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "", 16)
	pdf.Cell(0, 10, title)

	pdf.Ln(12)

	pdf.SetFont("Helvetica", "", 11)
	pdf.Cell(0, 8, fmt.Sprintf("Period: %s - %s", periodStart, periodEnd))
	pdf.Ln(7)
	pdf.Cell(0, 8, fmt.Sprintf("Total reports: %d", len(reports)))
	pdf.Ln(10)

	statusCounts := map[string]int{}
	tagCounts := map[string]int{}
	for _, report := range reports {
		statusCounts[report.Status]++
		for _, tag := range report.Tags {
			tagCounts[tag]++
		}
	}

	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(0, 8, "Status distribution")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 10)
	statusKeys := make([]string, 0, len(statusCounts))
	for key := range statusCounts {
		statusKeys = append(statusKeys, key)
	}
	sort.Slice(statusKeys, func(i, j int) bool { return statusCounts[statusKeys[i]] > statusCounts[statusKeys[j]] })
	for _, key := range statusKeys {
		pdf.Cell(0, 6, fmt.Sprintf("- %s: %d", key, statusCounts[key]))
		pdf.Ln(6)
	}

	pdf.Ln(4)
	pdf.SetFont("Helvetica", "B", 11)
	pdf.Cell(0, 8, "Top tags")
	pdf.Ln(8)
	pdf.SetFont("Helvetica", "", 10)
	type tagCount struct {
		Tag   string
		Count int
	}
	tags := make([]tagCount, 0, len(tagCounts))
	for tag, count := range tagCounts {
		tags = append(tags, tagCount{Tag: tag, Count: count})
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Count > tags[j].Count })
	limit := len(tags)
	if limit > 10 {
		limit = 10
	}
	for i := 0; i < limit; i++ {
		pdf.Cell(0, 6, fmt.Sprintf("- %s: %d", tags[i].Tag, tags[i].Count))
		pdf.Ln(6)
	}

	buffer := bytes.NewBuffer(nil)
	if err := pdf.Output(buffer); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

func (a *App) operatorExportDownloadHandler(c *gin.Context) {
	exportID := a.parseExportID(c)
	if exportID == 0 {
		writeAPIError(c, &apiError{Status: http.StatusBadRequest, Code: "invalid_id", Message: "Invalid export ID"})
		return
	}
	format := strings.TrimSpace(c.Query("format"))
	if format == "" {
		format = "csv"
	}
	if format != "csv" && format != "geojson" && format != "pdf" {
		format = "csv"
	}

	session, err := getOperatorSession(c)
	if err != nil {
		writeAPIError(c, &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Operator session required"})
		return
	}

	contentType, body, fileName, err := a.getExportAsset(c.Request.Context(), exportID, format, session)
	if err != nil {
		writeAPIError(c, err)
		return
	}
	c.Header("Content-Type", contentType)
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", fileName))
	_, _ = c.Writer.Write(body)
}

func (a *App) getExportAsset(ctx context.Context, exportID int, format string, session OperatorSession) (string, []byte, string, error) {
	var periodType string
	var periodStart time.Time
	var csvPath sql.NullString
	var geojsonPath sql.NullString
	var pdfPath sql.NullString
	var filterMunicipality sql.NullString
	err := a.db.QueryRowContext(ctx, `
		SELECT period_type, period_start, csv_path, geojson_path, pdf_path, filter_municipality
		FROM exports
		WHERE id = $1
	`, exportID).Scan(&periodType, &periodStart, &csvPath, &geojsonPath, &pdfPath, &filterMunicipality)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, "", &apiError{Status: http.StatusNotFound, Code: "export_not_found", Message: "Export batch not found"}
		}
		return "", nil, "", err
	}

	// Scope check
	if session.Role != "admin" {
		if session.Municipality == nil {
			return "", nil, "", &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Access restricted: invalid operator scope"}
		}
		// If the export has a specific municipality filter, it MUST match the operator's municipality.
		if filterMunicipality.Valid && !strings.EqualFold(filterMunicipality.String, *session.Municipality) {
			return "", nil, "", &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Access denied to this export"}
		}
		// If the export is "global" (nil municipality filter), should a municipality operator see it?
		// Perhaps global exports contain all data -> LEAK.
		// So non-admins cannot download global exports unless we are sure they are safe.
		// Safe default: deny global exports to non-admins.
		if !filterMunicipality.Valid {
			return "", nil, "", &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Access denied to global export"}
		}
	}

	base := fmt.Sprintf("zwerffiets-%s-%s", periodType, sanitizeFileNamePart(periodStart.UTC().Format(time.RFC3339)))
	var selectedPath string
	switch format {
	case "geojson":
		if !geojsonPath.Valid {
			return "", nil, "", &apiError{Status: http.StatusNotFound, Code: "export_not_found", Message: "GeoJSON artifact not found"}
		}
		selectedPath = geojsonPath.String
	case "pdf":
		if !pdfPath.Valid {
			return "", nil, "", &apiError{Status: http.StatusNotFound, Code: "export_not_found", Message: "PDF artifact not found"}
		}
		selectedPath = pdfPath.String
	default:
		if !csvPath.Valid {
			return "", nil, "", &apiError{Status: http.StatusNotFound, Code: "export_not_found", Message: "CSV artifact not found"}
		}
		selectedPath = csvPath.String
	}

	fullPath := filepath.Join(a.cfg.DataRoot, selectedPath)
	bytes, err := os.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", nil, "", &apiError{Status: http.StatusNotFound, Code: "export_not_found", Message: "Export artifact not found"}
		}
		return "", nil, "", err
	}

	switch format {
	case "geojson":
		return "application/geo+json; charset=utf-8", bytes, base + ".geojson", nil
	case "pdf":
		return "application/pdf", bytes, base + ".pdf", nil
	default:
		return "text/csv; charset=utf-8", bytes, base + ".csv", nil
	}
}

func (a *App) listExportBatches(ctx context.Context, session OperatorSession) ([]ExportBatch, error) {
	query := `
		SELECT id, period_type, period_start, period_end, generated_by, row_count, created_at, filter_status, filter_municipality
		FROM exports
		WHERE 1=1
	`
	args := []any{}
	argIndex := 1

	if session.Role != "admin" {
		if session.Municipality == nil {
			// Security fail, return empty or error
			return []ExportBatch{}, nil
		}
		// Show exports that are filtered to this user's municipality
		// OR exports generated by this user?
		// For now, let's restrict to exports matching the municipality filter.
		query += fmt.Sprintf(" AND LOWER(filter_municipality) = LOWER($%d)", argIndex)
		args = append(args, *session.Municipality)
		argIndex++
	}

	query += " ORDER BY created_at DESC"

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	batches := make([]ExportBatch, 0)
	for rows.Next() {
		var batch ExportBatch
		var periodStart time.Time
		var periodEnd time.Time
		var createdAt time.Time
		var filterStatus, filterMunicipality sql.NullString
		if err := rows.Scan(&batch.ID, &batch.PeriodType, &periodStart, &periodEnd, &batch.GeneratedBy, &batch.RowCount, &createdAt, &filterStatus, &filterMunicipality); err != nil {
			return nil, err
		}
		if filterStatus.Valid {
			batch.FilterStatus = &filterStatus.String
		}
		if filterMunicipality.Valid {
			batch.FilterMunicipality = &filterMunicipality.String
		}
		batch.PeriodStart = periodStart.UTC().Format(time.RFC3339)
		batch.PeriodEnd = periodEnd.UTC().Format(time.RFC3339)
		batch.GeneratedAt = createdAt.UTC().Format(time.RFC3339)
		batch.Artifacts = ExportArtifacts{CSV: "", GeoJSON: "", PDF: []byte{}}
		batches = append(batches, batch)
	}
	return batches, rows.Err()
}

func exportMunicipalityForSession(session OperatorSession, requestedMunicipality string) (string, error) {
	if session.Role == "admin" {
		return requestedMunicipality, nil
	}
	if session.Municipality == nil {
		return "", &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Access restricted: invalid operator scope"}
	}
	return *session.Municipality, nil
}

func (b ExportBatch) PeriodRange() string {
	start, err := time.Parse(time.RFC3339, b.PeriodStart)
	if err != nil {
		return b.PeriodStart + " - " + b.PeriodEnd
	}
	end, err := time.Parse(time.RFC3339, b.PeriodEnd)
	if err != nil {
		return b.PeriodStart + " - " + b.PeriodEnd
	}

	// Format: "Jan 02 2006 - Jan 02 2006"
	return start.Format("2006-01-02") + " tot " + end.Format("2006-01-02")
}

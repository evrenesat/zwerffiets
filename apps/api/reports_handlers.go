package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"zwerffiets/libs/mailer"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

const operatorPhotoCacheMaxAgeSeconds = 3600
const operatorMediaInternalPathPrefix = "/_protected_media/"

func (a *App) operatorLoginHandler(c *gin.Context) {
	var payload struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		writeAPIError(c, &apiError{Status: http.StatusBadRequest, Code: "invalid_payload", Message: "Invalid login payload"})
		return
	}
	payload.Email = strings.TrimSpace(payload.Email)

	role, municipality, err := a.authenticateOperatorCredentials(c.Request.Context(), payload.Email, payload.Password)
	if err != nil {
		writeAPIError(c, err)
		return
	}

	if err := a.startOperatorSession(c, OperatorSession{Email: payload.Email, Role: role, Municipality: municipality}); err != nil {
		writeAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"email": payload.Email, "role": role})
}

func (a *App) operatorLogoutHandler(c *gin.Context) {
	a.clearOperatorSession(c)
	c.JSON(http.StatusOK, gin.H{"ok": true})
}

func (a *App) operatorSessionHandler(c *gin.Context) {
	token, err := c.Cookie(operatorCookieName)
	if err != nil {
		writeAPIError(c, &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Operator session required"})
		return
	}
	session, err := a.verifyOperatorSessionToken(token)
	if err != nil {
		writeAPIError(c, &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "Operator session required"})
		return
	}
	c.JSON(http.StatusOK, session)
}

func (a *App) authenticateOperatorCredentials(ctx context.Context, email string, password string) (string, *string, error) {
	var passwordHash sql.NullString
	var role string
	var isActive bool
	var municipality sql.NullString
	err := a.db.QueryRowContext(ctx, `
		SELECT password_hash, role, is_active, municipality
		FROM operators
		WHERE email = $1
	`, email).Scan(&passwordHash, &role, &isActive, &municipality)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", nil, &apiError{Status: http.StatusUnauthorized, Code: "invalid_credentials", Message: "Invalid credentials"}
		}
		return "", nil, err
	}
	if !passwordHash.Valid || !isActive || bcrypt.CompareHashAndPassword([]byte(passwordHash.String), []byte(password)) != nil {
		return "", nil, &apiError{Status: http.StatusUnauthorized, Code: "invalid_credentials", Message: "Invalid credentials"}
	}
	if municipality.Valid && municipality.String != "" {
		return role, &municipality.String, nil
	}
	return role, nil, nil
}

func (a *App) startOperatorSession(c *gin.Context, session OperatorSession) error {
	token, err := a.createOperatorSessionToken(session)
	if err != nil {
		return err
	}
	secure := strings.EqualFold(a.cfg.Env, "production")
	c.SetCookie(operatorCookieName, token, int(operatorSessionDuration.Seconds()), "/", "", secure, true)
	return nil
}

func (a *App) clearOperatorSession(c *gin.Context) {
	secure := strings.EqualFold(a.cfg.Env, "production")
	c.SetCookie(operatorCookieName, "", -1, "/", "", secure, true)
}

func (a *App) ensureAnonymousReporterIdentity(c *gin.Context) (string, string) {
	anonymousID, err := c.Cookie(anonReporterCookieName)
	if err != nil || strings.TrimSpace(anonymousID) == "" {
		// Just a random string for cookie, not used as DB ID anymore
		anonymousID = createMagicLinkToken()
		secure := strings.EqualFold(a.cfg.Env, "production")
		c.SetCookie(anonReporterCookieName, anonymousID, int(anonReporterCookieMaxAge.Seconds()), "/", "", secure, true)
	}
	return anonymousID, a.deriveReporterHash(anonymousID)
}

func (a *App) tagsHandler(c *gin.Context) {
	tags, err := a.getTags(c.Request.Context())
	if err != nil {
		writeAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, tags)
}

func (a *App) getTags(ctx context.Context) ([]Tag, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, code, label, is_active
		FROM tags
		ORDER BY code ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tags []Tag
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.ID, &tag.Code, &tag.Label, &tag.IsActive); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(tags) > 0 {
		return tags, nil
	}

	for _, seed := range defaultTagDictionary {
		_, err := a.db.ExecContext(ctx, `
			INSERT INTO tags (code, label, is_active)
			VALUES ($1, $2, $3)
			ON CONFLICT (code) DO NOTHING
		`, seed.Code, seed.Label, seed.IsActive)
		if err != nil {
			return nil, err
		}
	}

	rows, err = a.db.QueryContext(ctx, `SELECT id, code, label, is_active FROM tags ORDER BY code ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var tag Tag
		if err := rows.Scan(&tag.ID, &tag.Code, &tag.Label, &tag.IsActive); err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

func sanitizeAndValidatePhotos(photos []PhotoUpload) ([]PhotoUpload, error) {
	if len(photos) < minPhotoCount || len(photos) > maxPhotoCount {
		return nil, &apiError{Status: http.StatusBadRequest, Code: "invalid_photo_count", Message: "Photos must contain 1 to 3 images"}
	}

	out := make([]PhotoUpload, 0, len(photos))
	for _, photo := range photos {
		if len(photo.Bytes) > maxUploadBytes {
			return nil, &apiError{Status: http.StatusBadRequest, Code: "photo_too_large", Message: "Photo exceeds upload size limit"}
		}
		if _, ok := allowedImageTypes[photo.MimeType]; !ok {
			return nil, &apiError{Status: http.StatusBadRequest, Code: "invalid_photo_type", Message: "Photo mime type is not supported"}
		}

		if photo.MimeType == "image/jpeg" {
			decoded, _, err := image.Decode(bytes.NewReader(photo.Bytes))
			if err == nil {
				buffer := bytes.NewBuffer(nil)
				if encodeErr := jpeg.Encode(buffer, decoded, &jpeg.Options{Quality: 88}); encodeErr == nil {
					photo.Bytes = buffer.Bytes()
				}
			}
		}

		out = append(out, photo)
	}

	return out, nil
}

func parseDataURLPhoto(dataURL string, fallbackName string) (PhotoUpload, error) {
	parts := strings.SplitN(dataURL, ",", 2)
	if len(parts) != 2 {
		return PhotoUpload{}, fmt.Errorf("invalid data URL")
	}
	meta := parts[0]
	payload := parts[1]
	if !strings.HasPrefix(meta, "data:image/") || !strings.Contains(meta, ";base64") {
		return PhotoUpload{}, fmt.Errorf("invalid data URL mime")
	}
	mimeType := strings.TrimPrefix(strings.SplitN(meta, ";", 2)[0], "data:")
	if _, ok := allowedImageTypes[mimeType]; !ok {
		return PhotoUpload{}, fmt.Errorf("unsupported mime type")
	}
	decoded, err := base64.StdEncoding.DecodeString(payload)
	if err != nil {
		return PhotoUpload{}, err
	}
	return PhotoUpload{Name: fallbackName, MimeType: mimeType, Bytes: decoded}, nil
}

func normalizeNote(value string) *string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return nil
	}
	if len(trimmed) > maxNoteLength {
		cut := trimmed[:maxNoteLength]
		return &cut
	}
	return &trimmed
}

func normalizeUILanguage(raw string) string {
	switch strings.TrimSpace(raw) {
	case "en":
		return "en"
	default:
		return "nl"
	}
}

func normalizeReporterEmail(raw string) *string {
	v := strings.ToLower(strings.TrimSpace(raw))
	if v == "" || !strings.Contains(v, "@") {
		return nil
	}
	return &v
}

func parseReportCreatePayload(c *gin.Context) (ReportCreatePayload, error) {
	contentType := strings.ToLower(c.GetHeader("Content-Type"))
	payload := ReportCreatePayload{Source: "web"}

	if strings.Contains(contentType, "application/json") {
		var body struct {
			Photos   []string `json:"photos"`
			Location struct {
				Lat       float64 `json:"lat"`
				Lng       float64 `json:"lng"`
				AccuracyM float64 `json:"accuracy_m"`
			} `json:"location"`
			Tags          []string `json:"tags"`
			Note          *string  `json:"note"`
			ClientTS      *string  `json:"client_ts"`
			ReporterEmail string   `json:"reporter_email"`
			UILanguage    string   `json:"ui_language"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			return payload, &apiError{Status: http.StatusBadRequest, Code: "invalid_payload", Message: "Invalid JSON body"}
		}
		photos := make([]PhotoUpload, 0, len(body.Photos))
		for idx, raw := range body.Photos {
			photo, err := parseDataURLPhoto(raw, fmt.Sprintf("offline-%d.jpg", idx+1))
			if err != nil {
				return payload, &apiError{Status: http.StatusBadRequest, Code: "invalid_photo_data", Message: "Invalid photo payload"}
			}
			photos = append(photos, photo)
		}
		payload.Photos = photos
		payload.Location = ReportLocation{Lat: body.Location.Lat, Lng: body.Location.Lng, AccuracyM: body.Location.AccuracyM}
		payload.Tags = body.Tags
		if body.Note != nil {
			payload.Note = normalizeNote(*body.Note)
		}
		payload.ClientTS = body.ClientTS
		payload.ReporterEmail = normalizeReporterEmail(body.ReporterEmail)
		payload.UILanguage = normalizeUILanguage(body.UILanguage)
		return payload, nil
	}

	if err := c.Request.ParseMultipartForm(32 << 20); err != nil {
		return payload, &apiError{Status: http.StatusBadRequest, Code: "invalid_multipart", Message: "Invalid multipart form"}
	}

	lat, err := strconv.ParseFloat(c.PostForm("lat"), 64)
	if err != nil {
		return payload, &apiError{Status: http.StatusBadRequest, Code: "invalid_location", Message: "Invalid latitude"}
	}
	lng, err := strconv.ParseFloat(c.PostForm("lng"), 64)
	if err != nil {
		return payload, &apiError{Status: http.StatusBadRequest, Code: "invalid_location", Message: "Invalid longitude"}
	}
	accuracy, err := strconv.ParseFloat(c.PostForm("accuracy_m"), 64)
	if err != nil {
		return payload, &apiError{Status: http.StatusBadRequest, Code: "invalid_location", Message: "Invalid accuracy"}
	}

	tagsRaw := c.PostForm("tags")
	var tags []string
	if err := json.Unmarshal([]byte(tagsRaw), &tags); err != nil {
		return payload, &apiError{Status: http.StatusBadRequest, Code: "invalid_tags", Message: "Invalid tags payload"}
	}

	clientTS := strings.TrimSpace(c.PostForm("client_ts"))
	if clientTS != "" {
		payload.ClientTS = &clientTS
	}

	files := c.Request.MultipartForm.File["photos"]
	photos := make([]PhotoUpload, 0, len(files))
	for idx, fileHeader := range files {
		opened, err := fileHeader.Open()
		if err != nil {
			return payload, err
		}
		data, readErr := io.ReadAll(io.LimitReader(opened, maxUploadBytes+1))
		_ = opened.Close()
		if readErr != nil {
			return payload, readErr
		}
		if len(data) > maxUploadBytes {
			return payload, &apiError{Status: http.StatusBadRequest, Code: "photo_too_large", Message: "Photo exceeds upload size limit"}
		}

		mimeType := fileHeader.Header.Get("Content-Type")
		if mimeType == "" {
			mimeType = http.DetectContentType(data)
		}
		mimeType = strings.ToLower(strings.TrimSpace(strings.Split(mimeType, ";")[0]))
		if _, ok := allowedImageTypes[mimeType]; !ok {
			return payload, &apiError{Status: http.StatusBadRequest, Code: "invalid_photo_type", Message: "Photo mime type is not supported"}
		}

		name := strings.TrimSpace(fileHeader.Filename)
		if name == "" {
			name = fmt.Sprintf("photo-%d.jpg", idx+1)
		}
		photos = append(photos, PhotoUpload{Name: name, MimeType: mimeType, Bytes: data})
	}

	payload.Photos = photos
	payload.Location = ReportLocation{Lat: lat, Lng: lng, AccuracyM: accuracy}
	payload.Tags = tags
	payload.Note = normalizeNote(c.PostForm("note"))
	payload.ReporterEmail = normalizeReporterEmail(c.PostForm("reporter_email"))
	payload.UILanguage = normalizeUILanguage(c.PostForm("ui_language"))
	return payload, nil
}

func validateReportCreatePayload(payload ReportCreatePayload, maxLocationAccuracyM float64) error {
	if payload.Location.Lat < -90 || payload.Location.Lat > 90 || payload.Location.Lng < -180 || payload.Location.Lng > 180 {
		return &apiError{Status: http.StatusBadRequest, Code: "invalid_location", Message: "Location is invalid"}
	}
	if payload.Location.AccuracyM < 0 || payload.Location.AccuracyM > maxLocationAccuracyM {
		return &apiError{Status: http.StatusBadRequest, Code: "invalid_location", Message: "Location accuracy is invalid"}
	}
	if len(payload.Tags) < minTagCount || len(payload.Tags) > maxTagCount {
		return &apiError{Status: http.StatusBadRequest, Code: "invalid_tags", Message: "Tags count is invalid"}
	}
	for _, tag := range payload.Tags {
		if strings.TrimSpace(tag) == "" || len(tag) > 64 {
			return &apiError{Status: http.StatusBadRequest, Code: "invalid_tags", Message: "Tag value is invalid"}
		}
	}
	if payload.Note != nil && len(*payload.Note) > maxNoteLength {
		return &apiError{Status: http.StatusBadRequest, Code: "invalid_note", Message: "Note exceeds max length"}
	}
	return nil
}

func (a *App) createReportHandler(c *gin.Context) {
	payload, err := parseReportCreatePayload(c)
	if err != nil {
		writeAPIError(c, err)
		return
	}

	reporterID, reporterHash := a.ensureAnonymousReporterIdentity(c)
	_ = reporterID

	payload.IP = c.ClientIP()
	payload.FingerprintHash = buildFingerprint(c.ClientIP(), c.GetHeader("User-Agent"), c.GetHeader("Accept-Language"))
	payload.ReporterHash = reporterHash

	if token, cookieErr := c.Cookie(userCookieName); cookieErr == nil {
		if userSession, sessionErr := a.verifyUserSessionToken(token); sessionErr == nil {
			payload.UserID = &userSession.UserID
			payload.ReporterEmail = nil
		}
	}

	created, err := a.createReport(c.Request.Context(), payload)
	if err != nil {
		writeAPIError(c, err)
		return
	}

	go func(id int) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := a.geocodeReport(ctx, id); err != nil {
			a.log.Error("background geocoding failed", "id", id, "err", err)
		}
	}(created.ID)

	if payload.ReporterEmail != nil && payload.UserID == nil {
		go func(email, publicID, lang string) {
			ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
			defer cancel()
			if err := a.sendReportMagicLinkEmail(ctx, email, publicID, lang); err != nil {
				a.log.Error("failed to send reporter magic link", "email", email, "err", err)
			}
		}(*payload.ReporterEmail, created.PublicID, payload.UILanguage)
	}

	c.JSON(http.StatusCreated, created)
}

func (a *App) createReport(ctx context.Context, payload ReportCreatePayload) (ReportCreateResponse, error) {
	if err := validateReportCreatePayload(payload, a.cfg.MaxLocationAccuracyM); err != nil {
		return ReportCreateResponse{}, err
	}

	now := time.Now().UTC()
	allowed := a.checkRateLimit("report:"+payload.IP, reportRateLimitRequests, reportRateLimitWindow, now)
	if !allowed {
		return ReportCreateResponse{}, &apiError{Status: http.StatusTooManyRequests, Code: "rate_limited", Message: "Too many reports from this IP. Please retry later."}
	}

	sanitizedPhotos, err := sanitizeAndValidatePhotos(payload.Photos)
	if err != nil {
		return ReportCreateResponse{}, err
	}

	tags, err := a.getTags(ctx)
	if err != nil {
		return ReportCreateResponse{}, err
	}
	active := make(map[string]struct{}, len(tags))
	for _, tag := range tags {
		if tag.IsActive {
			active[tag.Code] = struct{}{}
		}
	}
	for _, tag := range payload.Tags {
		if _, ok := active[tag]; !ok {
			return ReportCreateResponse{}, &apiError{Status: http.StatusBadRequest, Code: "invalid_tag", Message: fmt.Sprintf("Unknown or inactive tag: %s", tag)}
		}
	}

	matchedGroup, err := a.selectBikeGroupForReport(ctx, payload, now)
	if err != nil {
		return ReportCreateResponse{}, err
	}
	bikeGroup := matchedGroup
	if bikeGroup == nil {
		created, err := a.createBikeGroup(ctx, payload.Location)
		if err != nil {
			return ReportCreateResponse{}, err
		}
		bikeGroup = &created
	}

	publicID := generatePublicID()

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return ReportCreateResponse{}, err
	}

	actor := "citizen_anonymous"
	if payload.UserID != nil {
		actor = "citizen_authenticated"
	}

	var reportID int
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO reports (
			public_id, status, lat, lng, accuracy_m, tags, note,
			dedupe_group_id, source, fingerprint_hash, reporter_hash,
			flagged_for_review, bike_group_id, user_id, reporter_email, reporter_email_confirmed,
			created_at, updated_at
		) VALUES (
			$1, 'new', $2, $3, $4, $5, $6,
			NULL, $7, $8, $9,
			FALSE, $10, $11, $12, FALSE,
			NOW(), NOW()
		)
		RETURNING id
	`, publicID, payload.Location.Lat, payload.Location.Lng, payload.Location.AccuracyM, tagsToJSON(payload.Tags), payload.Note, payload.Source, payload.FingerprintHash, payload.ReporterHash, bikeGroup.ID, payload.UserID, payload.ReporterEmail).Scan(&reportID); err != nil {
		_ = tx.Rollback()
		return ReportCreateResponse{}, err
	}

	if err := a.saveReportPhotosTx(ctx, tx, reportID, sanitizedPhotos); err != nil {
		_ = tx.Rollback()
		return ReportCreateResponse{}, err
	}

	if err := a.addEventTx(ctx, tx, reportID, "created", actor, map[string]any{
		"source":         payload.Source,
		"retention_days": 365,
		"bike_group_id":  bikeGroup.ID,
	}); err != nil {
		_ = tx.Rollback()
		return ReportCreateResponse{}, err
	}

	if err := tx.Commit(); err != nil {
		return ReportCreateResponse{}, err
	}

	report, err := a.getReportByID(ctx, reportID)
	if err != nil {
		return ReportCreateResponse{}, err
	}
	if report == nil {
		return ReportCreateResponse{}, fmt.Errorf("report not found after insert")
	}

	groupReports, err := a.listReportsByBikeGroupID(ctx, bikeGroup.ID)
	if err != nil {
		return ReportCreateResponse{}, err
	}
	recomputation := computeReconfirmation(groupReports)
	previousStrength := bikeGroup.SignalStrength
	updatedGroup := applySummaryToBikeGroup(*bikeGroup, recomputation.Summary, recomputation.SignalStrength)
	if err := a.updateBikeGroup(ctx, updatedGroup); err != nil {
		return ReportCreateResponse{}, err
	}

	classification := recomputation.ClassificationByReportID[report.ID]
	if classification == "ignored_same_day" {
		_ = a.addEvent(ctx, report.ID, "signal_reconfirmation_ignored_same_day", "system", map[string]any{"bike_group_id": bikeGroup.ID})
	}
	if classification == "counted_same_reporter" || classification == "counted_distinct_reporter" {
		kind := "same_reporter"
		if classification == "counted_distinct_reporter" {
			kind = "distinct_reporter"
		}
		_ = a.addEvent(ctx, report.ID, "signal_reconfirmation_counted", "system", map[string]any{
			"bike_group_id":       bikeGroup.ID,
			"reporter_match_kind": kind,
		})
	}
	if previousStrength != recomputation.SignalStrength {
		_ = a.addEvent(ctx, report.ID, "signal_strength_changed", "system", map[string]any{
			"previous_signal_strength": previousStrength,
			"signal_strength":          recomputation.SignalStrength,
			"bike_group_id":            bikeGroup.ID,
		})
	}

	openSince := now.AddDate(0, 0, -dedupeLookbackDays).Format(time.RFC3339)
	openReports, err := a.listOpenReportsSince(ctx, openSince)
	if err != nil {
		return ReportCreateResponse{}, err
	}
	candidates := make([]dedupeCandidate, 0)
	for _, candidate := range openReports {
		if candidate.ID == report.ID {
			continue
		}
		scored := scoreDuplicateCandidate(*report, candidate, now)
		if scored != nil {
			candidates = append(candidates, *scored)
		}
	}
	sort.Slice(candidates, func(i, j int) bool { return candidates[i].Score > candidates[j].Score })
	if len(candidates) > 5 {
		candidates = candidates[:5]
	}
	dedupeIDs := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		// ReportID in dedupeCandidate is already int
		dedupeIDs = append(dedupeIDs, strconv.Itoa(candidate.ReportID))
	}

	flagged := a.applyFingerprintHeuristic(payload.FingerprintHash, now)
	if flagged {
		if _, err := a.db.ExecContext(ctx, `UPDATE reports SET flagged_for_review = TRUE, updated_at = NOW() WHERE id = $1`, report.ID); err != nil {
			return ReportCreateResponse{}, err
		}
	}

	token, err := a.createTrackingToken(report.PublicID, trackingTokenDays*24*time.Hour)
	if err != nil {
		return ReportCreateResponse{}, err
	}

	return ReportCreateResponse{
		ID:               report.ID,
		PublicID:         report.PublicID,
		CreatedAt:        report.CreatedAt,
		Status:           report.Status,
		TrackingURL:      buildPublicURL(a.cfg.PublicBaseURL, fmt.Sprintf("/report/status/%s?token=%s", report.PublicID, token)),
		DedupeCandidates: dedupeIDs,
		FlaggedForReview: flagged,
		BikeGroupID:      updatedGroup.ID,
		SignalStrength:   updatedGroup.SignalStrength,
		SignalSummary:    bikeGroupToSignalSummary(updatedGroup),
	}, nil
}

func (a *App) reportStatusHandler(c *gin.Context) {
	publicID := c.Param("public_id")
	token := trackingTokenFromRequest(c)
	report, err := a.getReportByPublicID(c.Request.Context(), publicID)
	if err != nil {
		writeAPIError(c, err)
		return
	}
	if report == nil {
		writeAPIError(c, &apiError{Status: http.StatusNotFound, Code: "report_not_found", Message: "Report not found"})
		return
	}

	if token != "" {
		claimPublicID, tokenErr := a.verifyTrackingToken(token)
		if tokenErr != nil || claimPublicID != publicID {
			writeAPIError(c, &apiError{Status: http.StatusForbidden, Code: "token_mismatch", Message: "Tracking token does not match report id"})
			return
		}
	} else {
		sessionToken, cookieErr := c.Cookie(userCookieName)
		if cookieErr != nil {
			writeAPIError(c, &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "User session required"})
			return
		}
		session, sessionErr := a.verifyUserSessionToken(sessionToken)
		if sessionErr != nil {
			writeAPIError(c, &apiError{Status: http.StatusUnauthorized, Code: "unauthorized", Message: "User session required"})
			return
		}
		if report.UserID == nil || *report.UserID != session.UserID {
			writeAPIError(c, &apiError{Status: http.StatusNotFound, Code: "report_not_found", Message: "Report not found"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"publicId":  report.PublicID,
		"status":    report.Status,
		"createdAt": report.CreatedAt,
		"updatedAt": report.UpdatedAt,
		"address":   report.Address,
		"city":      report.City,
	})
}

func trackingTokenFromRequest(c *gin.Context) string {
	queryToken := strings.TrimSpace(c.Query("token"))
	if queryToken != "" {
		return queryToken
	}

	authHeader := strings.TrimSpace(c.GetHeader("Authorization"))
	if authHeader == "" {
		return ""
	}

	const bearerPrefix = "Bearer "
	if len(authHeader) < len(bearerPrefix) || !strings.EqualFold(authHeader[:len(bearerPrefix)], bearerPrefix) {
		return ""
	}
	return strings.TrimSpace(authHeader[len(bearerPrefix):])
}

func (a *App) operatorReportsHandler(c *gin.Context) {
	filters := map[string]any{}
	if status := strings.TrimSpace(c.Query("status")); status != "" {
		filters["status"] = status
	}
	if tag := strings.TrimSpace(c.Query("tag")); tag != "" {
		filters["tag"] = tag
	}
	if from := strings.TrimSpace(c.Query("from")); from != "" {
		filters["from"] = from
	}
	if to := strings.TrimSpace(c.Query("to")); to != "" {
		filters["to"] = to
	}
	if signalStrength := strings.TrimSpace(c.Query("signal_strength")); signalStrength != "" {
		filters["signal_strength"] = signalStrength
	}
	if hasQual := strings.TrimSpace(c.Query("has_qualifying_reconfirmation")); hasQual != "" {
		filters["has_qualifying_reconfirmation"] = hasQual == "true"
	}
	if strongOnly := strings.TrimSpace(c.Query("strong_only")); strongOnly != "" {
		filters["strong_only"] = strongOnly == "true"
	}
	if input := strings.TrimSpace(c.Query("sort")); input == "signal" || input == "newest" {
		filters["sort"] = input
	}

	session, err := getOperatorSession(c)
	if err != nil {
		writeAPIError(c, err)
		return
	}
	if session.Municipality != nil {
		filters["city"] = *session.Municipality
	}

	reports, err := a.listOperatorReports(c.Request.Context(), filters)
	if err != nil {
		writeAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, reports)
}

func (a *App) listOperatorReports(ctx context.Context, filters map[string]any) ([]OperatorReportView, error) {
	reports, err := a.listReports(ctx, filters)
	if err != nil {
		return nil, err
	}

	groupIDs := make(map[int]struct{})
	for _, report := range reports {
		groupIDs[report.BikeGroupID] = struct{}{}
	}
	groups := make(map[int]BikeGroup, len(groupIDs))
	for groupID := range groupIDs {
		group, err := a.getBikeGroupByID(ctx, groupID)
		if err != nil {
			return nil, err
		}
		if group != nil {
			groups[group.ID] = *group
		}
	}

	views := make([]OperatorReportView, 0, len(reports))
	for _, report := range reports {
		group, ok := groups[report.BikeGroupID]
		if !ok {
			return nil, &apiError{Status: http.StatusInternalServerError, Code: "bike_group_missing", Message: fmt.Sprintf("Bike group not found for report %d", report.ID)}
		}

		photos, err := a.listReportPhotos(ctx, report.ID)
		if err != nil {
			return nil, err
		}

		var previewPhotoURL *string
		if len(photos) > 0 {
			previewURL := a.buildOperatorReportPhotoURL(report.ID, photos[0].ID)
			previewPhotoURL = &previewURL
		}

		view := OperatorReportView{
			Report:          report,
			BikeGroupID:     group.ID,
			SignalSummary:   bikeGroupToSignalSummary(group),
			SignalStrength:  group.SignalStrength,
			PreviewPhotoURL: previewPhotoURL,
		}

		if strength, ok := filters["signal_strength"].(string); ok && strength != "" && view.SignalStrength != strength {
			continue
		}
		if strongOnly, ok := filters["strong_only"].(bool); ok && strongOnly && view.SignalStrength != "strong_distinct_reporters" {
			continue
		}
		if hasQual, ok := filters["has_qualifying_reconfirmation"].(bool); ok {
			if view.SignalSummary.HasQualifyingReconfirmation != hasQual {
				continue
			}
		}

		views = append(views, view)
	}

	if sortBy, ok := filters["sort"].(string); ok && sortBy == "signal" {
		sort.Slice(views, func(i, j int) bool {
			left := signalStrengthPriority[views[i].SignalStrength]
			right := signalStrengthPriority[views[j].SignalStrength]
			if left != right {
				return left > right
			}
			return views[i].CreatedAt > views[j].CreatedAt
		})
	}

	return views, nil
}

func (a *App) operatorReportDetailsHandler(c *gin.Context) {
	reportID := a.parseOperatorReportID(c)
	if reportID == 0 {
		writeAPIError(c, &apiError{Status: http.StatusBadRequest, Code: "invalid_id", Message: "Invalid report ID"})
		return
	}
	details, err := a.getReportDetails(c.Request.Context(), reportID)
	if err != nil {
		writeAPIError(c, err)
		return
	}

	if err := a.checkMunicipalityScope(c, details.Report.Municipality); err != nil {
		writeAPIError(c, err)
		return
	}
	c.JSON(http.StatusOK, details)
}

func (a *App) checkMunicipalityScope(c *gin.Context, reportMunicipality *string) error {
	session, err := getOperatorSession(c)
	if err != nil {
		return err
	}

	// Admins have full access to all reports
	if session.Role == "admin" {
		return nil
	}

	// For non-admins (municipality_operator), a municipality MUST be assigned
	if session.Municipality == nil {
		return &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Access restricted to valid municipality"}
	}

	// If the report has no municipality (e.g. not yet geocoded or outside known areas),
	// municipality operators cannot see it.
	if reportMunicipality == nil {
		return &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Access restricted to municipality"}
	}

	// Finally, check if the report belongs to the operator's municipality
	if !strings.EqualFold(*reportMunicipality, *session.Municipality) {
		return &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Access restricted to municipality"}
	}
	return nil
}

func (a *App) operatorReportPhotoHandler(c *gin.Context) {
	reportID := a.parseOperatorReportID(c)
	photoID, _ := strconv.Atoi(c.Param("photoID"))

	if reportID == 0 || photoID == 0 {
		writeAPIError(c, &apiError{Status: http.StatusBadRequest, Code: "invalid_id", Message: "Invalid ID"})
		return
	}

	photo, err := a.getReportPhotoByID(c.Request.Context(), reportID, photoID)
	if err != nil {
		writeAPIError(c, err)
		return
	}
	if photo == nil {
		writeAPIError(c, &apiError{Status: http.StatusNotFound, Code: "photo_not_found", Message: "Photo not found"})
		return
	}

	report, err := a.getReportByID(c.Request.Context(), reportID)
	if err != nil {
		writeAPIError(c, err)
		return
	}
	if report != nil {
		if err := a.checkMunicipalityScope(c, report.Municipality); err != nil {
			writeAPIError(c, err)
			return
		}
	}

	relativePath, err := a.resolveExistingPhotoStoragePath(photo.StoragePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeAPIError(c, &apiError{Status: http.StatusNotFound, Code: "photo_not_found", Message: "Photo not found"})
			return
		}
		writeAPIError(c, &apiError{Status: http.StatusInternalServerError, Code: "invalid_photo_path", Message: "Photo path is invalid"})
		return
	}

	if photo.MimeType != "" {
		c.Header("Content-Type", photo.MimeType)
	}
	c.Header("Content-Disposition", fmt.Sprintf("inline; filename=%q", photo.Filename))
	c.Header("Cache-Control", fmt.Sprintf("private, max-age=%d", operatorPhotoCacheMaxAgeSeconds))
	if a.shouldUseInternalMediaRedirect() {
		c.Header("X-Accel-Redirect", buildOperatorMediaInternalPath(relativePath))
		c.Status(http.StatusOK)
		return
	}

	fullPath, err := a.resolveDataRootStoragePath(relativePath)
	if err != nil {
		writeAPIError(c, err)
		return
	}
	contents, err := os.ReadFile(fullPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			writeAPIError(c, &apiError{Status: http.StatusNotFound, Code: "photo_not_found", Message: "Photo not found"})
			return
		}
		writeAPIError(c, err)
		return
	}
	c.Data(http.StatusOK, photo.MimeType, contents)
}

func (a *App) getReportDetails(ctx context.Context, reportID int) (*OperatorReportDetails, error) {
	report, err := a.getReportByID(ctx, reportID)
	if err != nil {
		return nil, err
	}
	if report == nil {
		return nil, &apiError{Status: http.StatusNotFound, Code: "not_found", Message: "Report not found"}
	}

	photos, err := a.listReportPhotos(ctx, reportID)
	if err != nil {
		return nil, err
	}

	group, err := a.getBikeGroupByID(ctx, report.BikeGroupID)
	if err != nil {
		return nil, err
	}
	if group == nil {
		return nil, &apiError{Status: http.StatusInternalServerError, Code: "group_missing", Message: "Bike group missing"}
	}

	groupReports, err := a.listReportsByBikeGroupID(ctx, group.ID)
	if err != nil {
		return nil, err
	}

	events, err := a.listEvents(ctx, reportID)
	if err != nil {
		return nil, err
	}

	signalDetails := buildSignalDetails(groupReports, *group)
	return &OperatorReportDetails{
		Report:        *report,
		Events:        events,
		Photos:        a.toOperatorReportPhotoViews(reportID, photos),
		SignalDetails: signalDetails,
	}, nil
}

func (a *App) resolveDataRootStoragePath(storagePath string) (string, error) {
	cleanStoragePath := filepath.Clean(strings.TrimSpace(storagePath))
	if cleanStoragePath == "" || cleanStoragePath == "." || filepath.IsAbs(cleanStoragePath) {
		return "", fmt.Errorf("invalid storage path")
	}

	root := filepath.Clean(a.cfg.DataRoot)
	resolved := filepath.Clean(filepath.Join(root, cleanStoragePath))
	relative, err := filepath.Rel(root, resolved)
	if err != nil {
		return "", err
	}
	if relative == ".." || strings.HasPrefix(relative, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("resolved path escapes data root")
	}

	return resolved, nil
}

func (a *App) resolveExistingPhotoStoragePath(storagePath string) (string, error) {
	resolvedPath, err := a.resolveDataRootStoragePath(storagePath)
	if err != nil {
		return "", err
	}
	if fileExists(resolvedPath) {
		return a.relativeDataRootPath(resolvedPath)
	}
	if filepath.Ext(resolvedPath) != "" {
		return "", os.ErrNotExist
	}

	matches, err := filepath.Glob(resolvedPath + ".*")
	if err != nil {
		return "", err
	}
	sort.Strings(matches)
	for _, candidatePath := range matches {
		if !fileExists(candidatePath) {
			continue
		}
		return a.relativeDataRootPath(candidatePath)
	}
	return "", os.ErrNotExist
}

func (a *App) relativeDataRootPath(fullPath string) (string, error) {
	root := filepath.Clean(a.cfg.DataRoot)
	relativePath, err := filepath.Rel(root, filepath.Clean(fullPath))
	if err != nil {
		return "", err
	}
	if relativePath == "." || relativePath == ".." || strings.HasPrefix(relativePath, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes data root")
	}
	return filepath.ToSlash(relativePath), nil
}

func buildOperatorMediaInternalPath(relativeStoragePath string) string {
	normalized := filepath.ToSlash(filepath.Clean(strings.TrimSpace(relativeStoragePath)))
	normalized = strings.TrimPrefix(normalized, "/")
	return operatorMediaInternalPathPrefix + normalized
}

func (a *App) shouldUseInternalMediaRedirect() bool {
	return strings.EqualFold(a.cfg.Env, "production")
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func buildSignalDetails(reports []Report, group BikeGroup) SignalDetails {
	sortedReports := append([]Report{}, reports...)
	sort.Slice(sortedReports, func(i, j int) bool {
		return sortedReports[i].CreatedAt < sortedReports[j].CreatedAt
	})

	recomputation := computeReconfirmation(sortedReports)
	reporterOrder := make([]string, 0)
	reporterSeen := make(map[string]struct{})
	for _, report := range sortedReports {
		rid := effectiveReporterID(report)
		if _, ok := reporterSeen[rid]; ok {
			continue
		}
		reporterSeen[rid] = struct{}{}
		reporterOrder = append(reporterOrder, rid)
	}

	labelByReporter := make(map[string]string, len(reporterOrder))
	for idx, reporterID := range reporterOrder {
		alphabetLabel := string(rune('A' + (idx % 26)))
		suffix := ""
		if idx >= 26 {
			suffix = strconv.Itoa(idx/26 + 1)
		}
		labelByReporter[reporterID] = "Reporter " + alphabetLabel + suffix
	}

	timeline := make([]SignalTimelineEntry, 0, len(sortedReports))
	for _, report := range sortedReports {
		classification := recomputation.ClassificationByReportID[report.ID]
		var reporterMatchKind *string
		if classification == "counted_same_reporter" {
			value := "same_reporter"
			reporterMatchKind = &value
		}
		if classification == "counted_distinct_reporter" {
			value := "distinct_reporter"
			reporterMatchKind = &value
		}

		timeline = append(timeline, SignalTimelineEntry{
			ReportID:          report.ID,
			PublicID:          report.PublicID,
			CreatedAt:         report.CreatedAt,
			ReporterLabel:     labelByReporter[effectiveReporterID(report)],
			ReporterMatchKind: reporterMatchKind,
			Qualified:         classification == "counted_same_reporter" || classification == "counted_distinct_reporter",
			IgnoredSameDay:    classification == "ignored_same_day",
		})
	}

	return SignalDetails{
		BikeGroup:      group,
		SignalSummary:  bikeGroupToSignalSummary(group),
		SignalStrength: group.SignalStrength,
		Timeline:       timeline,
	}
}

func (a *App) geocodeReport(ctx context.Context, reportID int) error {
	report, err := a.getReportByID(ctx, reportID)
	if err != nil {
		return err
	}
	if report == nil {
		return fmt.Errorf("report not found")
	}

	res, err := a.geocoder.Geocode(ctx, report.Location.Lat, report.Location.Lng)
	if err != nil {
		return err
	}
	if res == nil {
		return nil // No address found
	}

	municipality := lookupMunicipality(res.City)
	a.log.Info("geocoded report", "id", reportID, "address", res.Address, "city", res.City, "municipality", municipality)
	return a.updateReportAddress(ctx, reportID, res.Address, res.City, res.PostalCode, municipality)
}

func (a *App) updateReportAddress(ctx context.Context, reportID int, address, city, postcode, municipality string) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE reports
		SET address = $1, city = $2, postcode = $3, municipality = $4, updated_at = NOW()
		WHERE id = $5
	`, address, city, postcode, municipality, reportID)
	return err
}

// reportMagicLinkContent holds per-language email content for the anonymous reporter magic link email.
var reportMagicLinkContent = map[string]struct {
	Subject  string
	BodyHTML string // format args: publicID, magicLinkURL
	BodyText string // format args: publicID, magicLinkURL
}{
	"nl": {
		Subject:  "Jouw ZwerfFiets melding",
		BodyHTML: `<p>We hebben je melding (#%s) ontvangen. Klik op de onderstaande link om in te loggen en je meldingen te bekijken. De link is 15 minuten geldig. We gebruiken je e-mailadres nergens anders voor.</p><p><a href="%s">Bekijk mijn meldingen</a></p>`,
		BodyText: "We hebben je melding (#%s) ontvangen. Log in via: %s\n\nDe link is 15 minuten geldig.",
	},
	"en": {
		Subject:  "Your ZwerfFiets report",
		BodyHTML: `<p>We received your report (#%s). Click the link below to log in and view your reports. This link is valid for 15 minutes. Your email address will not be used for any other purpose.</p><p><a href="%s">View my reports</a></p>`,
		BodyText: "We received your report (#%s). Log in at: %s\n\nThis link expires in 15 minutes.",
	},
}

func (a *App) sendReportMagicLinkEmail(ctx context.Context, email, publicID, uiLanguage string) error {
	content, ok := reportMagicLinkContent[uiLanguage]
	if !ok {
		content = reportMagicLinkContent["nl"]
	}

	user, err := a.findOrCreateUser(ctx, email)
	if err != nil {
		return err
	}

	token := createMagicLinkToken()
	tokenHash := hashMagicLinkToken(token)
	expiresAt := time.Now().UTC().Add(magicLinkTokenExpiry)

	_, err = a.db.ExecContext(ctx, `
		INSERT INTO magic_link_tokens (user_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, user.ID, tokenHash, expiresAt)
	if err != nil {
		return err
	}

	magicLinkURL := fmt.Sprintf("%s/auth/verify?token=%s", a.cfg.PublicBaseURL, token)

	msg := mailer.Message{
		To:      []string{email},
		Subject: content.Subject,
		HTML:    fmt.Sprintf(content.BodyHTML, publicID, magicLinkURL),
		Text:    fmt.Sprintf(content.BodyText, publicID, magicLinkURL),
	}

	result, err := a.mailer.Send(msg)
	if err != nil {
		return err
	}

	a.log.Info("reporter magic link sent", "email", email, "public_id", publicID, "language", uiLanguage, "message_id", result.ProviderMessageID)
	return nil
}

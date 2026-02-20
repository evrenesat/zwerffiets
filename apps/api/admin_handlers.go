package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

func (a *App) registerAdminRoutes(r *gin.Engine) {
	staticFS, err := adminStaticFileSystem(a.cfg.Env)
	if err != nil {
		panic(err)
	}
	r.StaticFS("/bikeadmin/static", staticFS)

	r.GET("/bikeadmin/login", a.adminLoginPageHandler)
	r.POST("/bikeadmin/login", a.adminLoginSubmitHandler)
	r.POST("/bikeadmin/logout", a.adminLogoutSubmitHandler)
	r.POST("/bikeadmin/language", a.adminLanguageSubmitHandler)

	admin := r.Group("/bikeadmin")
	admin.Use(a.requireOperatorSessionHTML())
	{
		admin.GET("", a.adminTriagePageHandler)
		admin.GET("/", a.adminTriagePageHandler)
		admin.POST("/reports/bulk-status", a.adminBulkStatusSubmitHandler)
		admin.GET("/reports/:id", a.adminReportDetailsPageHandler)
		admin.POST("/reports/:id/status", a.adminReportStatusSubmitHandler)
		admin.POST("/reports/:id/merge", a.adminMergeSubmitHandler)
		admin.GET("/map", a.adminMapPageHandler)
		admin.GET("/exports", a.adminExportsPageHandler)
		admin.POST("/exports/generate", a.adminGenerateExportSubmitHandler)
		admin.GET("/showcase/editor", a.requireRole("admin"), a.adminShowcaseEditorPageHandler)
		admin.POST("/showcase/editor", a.requireRole("admin"), a.adminShowcaseEditorSubmitHandler)

		admin.GET("/content", a.requireRole("admin"), a.adminContentPageHandler)
		admin.POST("/content", a.requireRole("admin"), a.adminContentSubmitHandler)

		admin.GET("/reports/:id/edit", a.requireRole("admin"), a.adminReportEditPageHandler)
		admin.POST("/reports/:id/edit", a.requireRole("admin"), a.adminReportEditSubmitHandler)

		admin.GET("/operators", a.requireRole("admin"), a.adminOperatorsPageHandler)
		admin.GET("/operators/new", a.requireRole("admin"), a.adminOperatorCreatePageHandler)
		admin.POST("/operators", a.requireRole("admin"), a.adminOperatorCreateSubmitHandler)
		admin.POST("/operators/:id/toggle", a.requireRole("admin"), a.adminOperatorToggleSubmitHandler)
		admin.GET("/operators/:id/edit", a.requireRole("admin"), a.adminOperatorEditPageHandler)
		admin.POST("/operators/:id/edit", a.requireRole("admin"), a.adminOperatorEditSubmitHandler)
		admin.POST("/operators/:id/toggle-reports", a.requireRole("admin"), a.adminOperatorToggleReportsSubmitHandler)

		admin.GET("/users", a.requireRole("admin"), a.adminUsersPageHandler)
		admin.GET("/users/:id/edit", a.requireRole("admin"), a.adminUserEditPageHandler)
		admin.POST("/users/:id/edit", a.requireRole("admin"), a.adminUserEditSubmitHandler)
		admin.POST("/users/bulk", a.requireRole("admin"), a.adminUsersBulkSubmitHandler)

		admin.GET("/blog", a.requireRole("admin"), a.adminBlogListPageHandler)
		admin.GET("/blog/new", a.requireRole("admin"), a.adminBlogCreatePageHandler)
		admin.POST("/blog", a.requireRole("admin"), a.adminBlogSubmitHandler)
		admin.GET("/blog/:id", a.requireRole("admin"), a.adminBlogEditPageHandler)
		admin.POST("/blog/:id", a.requireRole("admin"), a.adminBlogSubmitHandler)
		admin.POST("/blog/media", a.requireRole("admin"), a.adminBlogMediaUploadHandler)
	}
}

func (a *App) adminLoginPageHandler(c *gin.Context) {
	if token, err := c.Cookie(operatorCookieName); err == nil {
		if _, verifyErr := a.verifyOperatorSessionToken(token); verifyErr == nil {
			c.Redirect(http.StatusSeeOther, "/bikeadmin")
			return
		}
	}

	next := sanitizeAdminRedirectTarget(c.Query("next"))
	base := a.adminBaseData(c, "page_title_login", "")
	data := adminLoginViewData{
		adminBaseViewData: base,
		Email:             "",
		Next:              next,
	}
	a.renderAdminTemplate(c, http.StatusOK, adminTemplateLoginPath, data)
}

func (a *App) adminLoginSubmitHandler(c *gin.Context) {
	email := strings.TrimSpace(c.PostForm("email"))
	password := c.PostForm("password")
	next := sanitizeAdminRedirectTarget(c.PostForm("next"))
	lang := normalizeAdminLanguage(c.PostForm("language"))
	a.setAdminLanguageCookie(c, lang)

	role, municipality, err := a.adminAuthenticate(c.Request.Context(), email, password)
	if err != nil {
		status := http.StatusInternalServerError
		errorMessage := adminText(lang, "error_login_failed")
		var apiErr *apiError
		if errors.As(err, &apiErr) {
			if apiErr.Status == http.StatusUnauthorized {
				status = http.StatusUnauthorized
				errorMessage = adminText(lang, "error_invalid_credentials")
			} else {
				status = apiErr.Status
				errorMessage = apiErr.Message
			}
		}

		base := a.adminBaseData(c, "page_title_login", "")
		base.Lang = lang
		base.Text = adminTexts(lang)
		base.ErrorMessage = errorMessage

		data := adminLoginViewData{
			adminBaseViewData: base,
			Email:             email,
			Next:              next,
		}
		a.renderAdminTemplate(c, status, adminTemplateLoginPath, data)
		return
	}

	if err := a.startOperatorSession(c, OperatorSession{Email: email, Role: role, Municipality: municipality}); err != nil {
		writeAPIError(c, err)
		return
	}

	c.Redirect(http.StatusSeeOther, next)
}

func (a *App) adminShowcaseEditorPageHandler(c *gin.Context) {
	photoIDStr := c.Query("photo_id")
	photoID := 0
	if photoIDStr != "" {
		parsed, err := strconv.Atoi(photoIDStr)
		if err == nil {
			photoID = parsed
		}
	}
	next := sanitizeAdminRedirectTarget(c.Query("next"))

	showcaseItems, err := a.storeGetShowcaseItems(c.Request.Context())
	if err != nil {
		c.String(http.StatusInternalServerError, "Failed to load showcase items")
		return
	}

	// Make sure we have exactly 4 items mapped (1 to 4) for the UI
	// Pad with empty defaults if missing
	itemsMap := make(map[int]ShowcaseItem)
	for _, item := range showcaseItems {
		itemsMap[item.Slot] = item
	}
	var paddedItems []ShowcaseItem
	for slot := 1; slot <= 4; slot++ {
		if item, exists := itemsMap[slot]; exists {
			paddedItems = append(paddedItems, item)
		} else {
			paddedItems = append(paddedItems, ShowcaseItem{
				Slot:         slot,
				Subtitle:     "",
				FocalX:       50,
				FocalY:       50,
				ScalePercent: 100,
			})
		}
	}

	// Note: We need the ReportID for the photo so we can construct a URL or just fetch the photo metadata using photoID directly
	// Actually, operator photo URL is `/api/v1/operator/reports/:id/photos/:photoID`
	// Wait, we need the report ID to construct that URL natively.
	// But `storeGetShowcaseItems` doesn't fetch ReportID.
	// We can add it, or we can provide a generic way to fetch the photo locally for the editor:
	// A new endpoint maybe? No, let's just lookup the ReportID from the photoID so we can use existing URL logic.
	// Wait, we have `getReportPhotoByID` but we only have photoID!
	// Okay, I'll update store_utils to lookup photo by ID without report ID, or just pass ReportID in query params.
	// Let's pass ReportID in query params: ?photo_id=123&report_id=456
	reportIDStr := c.Query("report_id")
	reportID := 0
	if reportIDStr != "" {
		reportID, _ = strconv.Atoi(reportIDStr)
	}

	newPhotoURL := ""
	if reportID > 0 && photoID > 0 {
		newPhotoURL = a.buildOperatorReportPhotoURL(reportID, photoID)
	}

	for i := range paddedItems {
		if paddedItems[i].ReportPhotoID > 0 {
			// Instead of needing report_id for all existing showcase items, we can just use the public endpoint we just created
			paddedItems[i].StoragePath = fmt.Sprintf("/api/v1/showcase/%d/photo", paddedItems[i].Slot)
		}
	}

	showcaseJSON, _ := json.Marshal(paddedItems)

	base := a.adminBaseData(c, "showcase_editor_title", "showcase")
	data := adminShowcaseEditorViewData{
		adminBaseViewData: base,
		BackURL:           next,
		PhotoID:           photoID,
		PhotoURL:          newPhotoURL,
		ShowcaseItems:     paddedItems,
		ShowcaseJSON:      string(showcaseJSON),
	}

	a.renderAdminTemplate(c, http.StatusOK, "templates/admin/showcase_editor.tmpl", data)
}

func (a *App) adminShowcaseEditorSubmitHandler(c *gin.Context) {
	lang := a.adminLanguageFromRequest(c)
	next := sanitizeAdminRedirectTarget(c.PostForm("next"))

	photoIDStr := c.PostForm("photo_id")
	photoID, err := strconv.Atoi(photoIDStr)
	if err != nil {
		redirectAdminWithMessage(c, next, "error", adminText(lang, "error_showcase_update_failed"))
		return
	}

	slotStr := c.PostForm("slot")
	slot, err := strconv.Atoi(slotStr)
	if err != nil || slot < 1 || slot > 4 {
		redirectAdminWithMessage(c, next, "error", adminText(lang, "error_showcase_update_failed"))
		return
	}

	subtitle := strings.TrimSpace(c.PostForm("subtitle"))
	if subtitle == "" {
		redirectAdminWithMessage(c, next, "error", adminText(lang, "error_showcase_update_failed"))
		return
	}

	focalXStr := c.PostForm("focal_x")
	focalX, err := strconv.Atoi(focalXStr)
	if err != nil {
		focalX = 50
	}
	focalYStr := c.PostForm("focal_y")
	focalY, err := strconv.Atoi(focalYStr)
	if err != nil {
		focalY = 50
	}

	scaleStr := c.PostForm("scale_percent")
	scalePercent, err := strconv.Atoi(scaleStr)
	if err != nil {
		scalePercent = 100
	}

	if err := a.storeUpsertShowcaseItem(c.Request.Context(), slot, photoID, focalX, focalY, scalePercent, subtitle); err != nil {
		a.log.Error("failed to update showcase item", "error", err)
		redirectAdminWithMessage(c, next, "error", adminText(lang, "error_showcase_update_failed"))
		return
	}

	baseNext := strings.Split(next, "?")[0]
	redirectAdminWithMessage(c, baseNext, "notice", adminText(lang, "notice_showcase_updated"))
}
func (a *App) adminLogoutSubmitHandler(c *gin.Context) {
	a.clearOperatorSession(c)
	c.Redirect(http.StatusSeeOther, "/bikeadmin/login")
}

func (a *App) adminLanguageSubmitHandler(c *gin.Context) {
	language := normalizeAdminLanguage(c.PostForm("language"))
	a.setAdminLanguageCookie(c, language)
	next := sanitizeAdminRedirectTarget(c.PostForm("next"))
	c.Redirect(http.StatusSeeOther, next)
}

func (a *App) adminTriagePageHandler(c *gin.Context) {
	lang := a.adminLanguageFromRequest(c)
	filters := parseAdminReportFilters(c)
	storeFilters := filters.toStoreFilters()

	page := parseAdminPage(c.Query("page"))
	pageSize := adminDefaultPerPage

	session, _ := getOperatorSession(c)
	if err := applySessionMunicipalityScope(storeFilters, session); err != nil {
		base := a.adminBaseData(c, "page_title_triage", "triage")
		base.ErrorMessage = "Access restricted: invalid operator scope"
		a.renderAdminTemplate(c, http.StatusForbidden, adminTemplateTriagePath, adminTriageViewData{
			adminBaseViewData: base,
			Filters:           filters.toView(),
			CityOptions:       []string{},
		})
		return
	}

	cityOptions, err := a.adminListCachedReportCities(c.Request.Context(), session)
	if err != nil {
		a.log.Error("list triage city options failed", "error", err)
		cityOptions = []string{}
	}

	paginatedResult, err := a.adminListReportsPaginated(c.Request.Context(), storeFilters, page, pageSize)
	if err != nil {
		base := a.adminBaseData(c, "page_title_triage", "triage")
		base.ErrorMessage = adminText(lang, "error_reports_load_failed")
		a.renderAdminTemplate(c, http.StatusInternalServerError, adminTemplateTriagePath, adminTriageViewData{
			adminBaseViewData: base,
			Filters:           filters.toView(),
			CityOptions:       cityOptions,
			Reports:           []adminReportRowView{},
		})
		return
	}

	reports := paginatedResult.Reports
	currentURL := filters.currentURL()
	rows := make([]adminReportRowView, 0, len(reports))
	for _, report := range reports {
		tags := make([]string, 0, len(report.Tags))
		for _, tag := range report.Tags {
			tags = append(tags, adminTagLabel(lang, tag))
		}
		lastQual := adminText(lang, "common_dash")
		if report.SignalSummary.LastQualifyingReconfirmationAt != nil {
			lastQual = formatAdminTimestamp(*report.SignalSummary.LastQualifyingReconfirmationAt)
		}
		preview := ""
		if report.PreviewPhotoURL != nil {
			preview = *report.PreviewPhotoURL
		}
		detailURL := fmt.Sprintf("/bikeadmin/reports/%d?next=%s", report.ID, url.QueryEscape(currentURL))

		rows = append(rows, adminReportRowView{
			ID:                   report.ID,
			PublicID:             report.PublicID,
			DetailURL:            detailURL,
			PreviewPhotoURL:      preview,
			StatusLabel:          adminStatusLabel(lang, report.Status),
			City:                 valueOrDash(report.City),
			SignalLabel:          adminSignalLabel(lang, report.SignalStrength),
			SignalClass:          adminSignalClass(report.SignalStrength),
			UniqueReporters:      report.SignalSummary.UniqueReporters,
			LastReconfirmationAt: lastQual,
			CreatedAt:            formatAdminTimestamp(report.CreatedAt),
			TagsLabel:            strings.Join(tags, ", "),
			StatusActions:        buildAdminStatusActions(lang, report.Status, currentURL),
		})
	}

	base := a.adminBaseData(c, "page_title_triage", "triage")
	pagination := buildAdminPaginationView(
		paginatedResult.TotalCount,
		paginatedResult.CurrentPage,
		paginatedResult.PageSize,
		currentURL,
	)
	data := adminTriageViewData{
		adminBaseViewData: base,
		Filters:           filters.toView(),
		CityOptions:       cityOptions,
		Reports:           rows,
		Pagination:        pagination,
	}
	a.renderAdminTemplate(c, http.StatusOK, adminTemplateTriagePath, data)
}

func (a *App) adminReportDetailsPageHandler(c *gin.Context) {
	lang := a.adminLanguageFromRequest(c)
	reportID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ID")
		return
	}
	next := sanitizeAdminRedirectTarget(c.Query("next"))

	details, err := a.adminLoadReportDetails(c.Request.Context(), reportID)
	if err == nil {
		if scopeErr := a.checkMunicipalityScope(c, details.Report.Municipality); scopeErr != nil {
			err = scopeErr
		}
	}

	if err != nil {
		status := http.StatusInternalServerError
		errorMessage := adminText(lang, "error_report_load_failed")
		var apiErr *apiError
		if errors.As(err, &apiErr) {
			status = apiErr.Status
			errorMessage = apiErr.Message
		}
		base := a.adminBaseData(c, "page_title_report", "triage")
		base.ErrorMessage = errorMessage
		a.renderAdminTemplate(c, status, adminTemplateReportPath, adminReportDetailViewData{adminBaseViewData: base, BackURL: next, ActionNext: next})
		return
	}

	tags := make([]string, 0, len(details.Report.Tags))
	for _, tag := range details.Report.Tags {
		tags = append(tags, adminTagLabel(lang, tag))
	}
	note := adminText(lang, "common_dash")
	if details.Report.Note != nil && strings.TrimSpace(*details.Report.Note) != "" {
		note = *details.Report.Note
	}

	firstQual := adminText(lang, "common_dash")
	if details.SignalDetails.SignalSummary.FirstQualifyingReconfirmationAt != nil {
		firstQual = formatAdminTimestamp(*details.SignalDetails.SignalSummary.FirstQualifyingReconfirmationAt)
	}
	lastQual := adminText(lang, "common_dash")
	if details.SignalDetails.SignalSummary.LastQualifyingReconfirmationAt != nil {
		lastQual = formatAdminTimestamp(*details.SignalDetails.SignalSummary.LastQualifyingReconfirmationAt)
	}

	timeline := make([]adminTimelineRowView, 0, len(details.SignalDetails.Timeline))
	for _, entry := range details.SignalDetails.Timeline {
		timeline = append(timeline, adminTimelineRowView{
			ReporterLabel: entry.ReporterLabel,
			CreatedAt:     formatAdminTimestamp(entry.CreatedAt),
			Description:   adminTimelineLabel(lang, entry),
		})
	}

	events := make([]adminEventRowView, 0, len(details.Events))
	for _, event := range details.Events {
		events = append(events, adminEventRowView{
			TypeLabel: adminEventLabel(lang, event.Type),
			Actor:     event.Actor,
			CreatedAt: formatAdminTimestamp(event.CreatedAt),
		})
	}

	isAdmin := false
	if sessionVal, _ := c.Get("operatorSession"); sessionVal != nil {
		if s, ok := sessionVal.(OperatorSession); ok && s.Role == "admin" {
			isAdmin = true
		}
	}

	detailSelf := fmt.Sprintf("/bikeadmin/reports/%d?next=%s", details.Report.ID, url.QueryEscape(next))
	base := a.adminBaseData(c, "page_title_report", "triage")
	base.IncludeMapLibre = true
	data := adminReportDetailViewData{
		adminBaseViewData:   base,
		ReportID:            details.Report.ID,
		PublicID:            details.Report.PublicID,
		BackURL:             next,
		ActionNext:          detailSelf,
		StatusLabel:         adminStatusLabel(lang, details.Report.Status),
		StatusActions:       buildAdminStatusActions(lang, details.Report.Status, detailSelf),
		Location:            fmt.Sprintf("%.6f, %.6f", details.Report.Location.Lat, details.Report.Location.Lng),
		Lat:                 details.Report.Location.Lat,
		Lng:                 details.Report.Location.Lng,
		TagsLabel:           strings.Join(tags, ", "),
		NoteLabel:           note,
		Photos:              details.Photos,
		MergeInput:          "",
		SignalStrengthLabel: adminSignalLabel(lang, details.SignalDetails.SignalStrength),
		BikeGroupID:         details.SignalDetails.BikeGroup.ID,
		SignalSummary: adminSignalSummaryView{
			TotalReports:                    details.SignalDetails.SignalSummary.TotalReports,
			UniqueReporters:                 details.SignalDetails.SignalSummary.UniqueReporters,
			SameReporterReconfirmations:     details.SignalDetails.SignalSummary.SameReporterReconfirmations,
			DistinctReporterReconfirmations: details.SignalDetails.SignalSummary.DistinctReporterReconfirmations,
			FirstQualifying:                 firstQual,
			LastQualifying:                  lastQual,
		},
		Timeline:     timeline,
		Events:       events,
		Address:      valueOrDash(details.Report.Address),
		City:         valueOrDash(details.Report.City),
		Municipality: valueOrDash(details.Report.Municipality),
		IsAdmin:      isAdmin,
	}
	a.renderAdminTemplate(c, http.StatusOK, adminTemplateReportPath, data)
}

func (a *App) adminReportEditPageHandler(c *gin.Context) {
	lang := a.adminLanguageFromRequest(c)
	reportID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ID")
		return
	}
	next := sanitizeAdminRedirectTarget(c.Query("next"))

	details, err := a.adminLoadReportDetails(c.Request.Context(), reportID)
	if err != nil {
		base := a.adminBaseData(c, "page_title_report_edit", "triage")
		base.ErrorMessage = adminText(lang, "error_report_load_failed")
		a.renderAdminTemplate(c, http.StatusInternalServerError, "templates/admin/report_edit.tmpl", adminReportEditViewData{
			adminBaseViewData: base,
			BackURL:           next,
		})
		return
	}

	muni := ""
	if details.Report.Municipality != nil {
		muni = *details.Report.Municipality
	}
	addr := ""
	if details.Report.Address != nil {
		addr = *details.Report.Address
	}
	postcode := ""
	if details.Report.PostalCode != nil {
		postcode = *details.Report.PostalCode
	}

	base := a.adminBaseData(c, "page_title_report_edit", "triage")
	data := adminReportEditViewData{
		adminBaseViewData: base,
		ReportID:          reportID,
		BackURL:           next,
		Municipality:      muni,
		Address:           addr,
		Postcode:          postcode,
		Lat:               details.Report.Location.Lat,
		Lng:               details.Report.Location.Lng,
		Municipalities:    municipalityList(),
	}
	a.renderAdminTemplate(c, http.StatusOK, "templates/admin/report_edit.tmpl", data)
}

func (a *App) adminReportEditSubmitHandler(c *gin.Context) {
	lang := a.adminLanguageFromRequest(c)
	reportID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ID")
		return
	}
	next := sanitizeAdminRedirectTarget(c.PostForm("next"))
	if next == "/bikeadmin" {
		next = fmt.Sprintf("/bikeadmin/reports/%d", reportID)
	}

	municipality := strings.TrimSpace(c.PostForm("municipality"))
	address := strings.TrimSpace(c.PostForm("address"))
	postcode := strings.TrimSpace(c.PostForm("postcode"))
	latStr := strings.TrimSpace(c.PostForm("lat"))
	lngStr := strings.TrimSpace(c.PostForm("lng"))

	editURL := fmt.Sprintf("/bikeadmin/reports/%d/edit?next=%s", reportID, url.QueryEscape(next))

	if municipality != "" && !isValidMunicipality(municipality) {
		redirectAdminWithMessage(c, editURL, "error", adminText(lang, "error_invalid_municipality"))
		return
	}

	lat, err := strconv.ParseFloat(latStr, 64)
	if err != nil {
		redirectAdminWithMessage(c, editURL, "error", adminText(lang, "error_report_update_failed"))
		return
	}
	lng, err := strconv.ParseFloat(lngStr, 64)
	if err != nil {
		redirectAdminWithMessage(c, editURL, "error", adminText(lang, "error_report_update_failed"))
		return
	}

	if err := a.updateReportDetails(c.Request.Context(), reportID, municipality, address, postcode, lat, lng); err != nil {
		redirectAdminWithMessage(c, editURL, "error", adminText(lang, "error_report_update_failed"))
		return
	}

	redirectAdminWithMessage(c, next, "notice", adminText(lang, "notice_report_updated"))
}

func (a *App) adminReportStatusSubmitHandler(c *gin.Context) {
	session, err := getOperatorSession(c)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/bikeadmin/login")
		return
	}
	next := sanitizeAdminRedirectTarget(c.PostForm("next"))
	status := strings.TrimSpace(c.PostForm("status"))
	reportID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		redirectAdminWithMessage(c, next, "error", "Invalid ID")
		return
	}

	if session.Role == "municipality_operator" && status == "forwarded" {
		redirectAdminWithMessage(c, next, "error", adminText(a.adminLanguageFromRequest(c), "error_status_update_failed"))
		return
	}

	if err := a.ensureReportStatusScope(c.Request.Context(), session, reportID); err != nil {
		redirectAdminWithMessage(c, next, "error", normalizeAdminErrorMessage(err, a.adminLanguageFromRequest(c), "error_status_update_failed"))
		return
	}

	if _, err := a.adminUpdateStatus(c.Request.Context(), reportID, status, session); err != nil {
		redirectAdminWithMessage(c, next, "error", normalizeAdminErrorMessage(err, a.adminLanguageFromRequest(c), "error_status_update_failed"))
		return
	}
	redirectAdminWithMessage(c, next, "notice", adminText(a.adminLanguageFromRequest(c), "notice_status_updated"))
}

func (a *App) adminBulkStatusSubmitHandler(c *gin.Context) {
	session, err := getOperatorSession(c)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/bikeadmin/login")
		return
	}

	lang := a.adminLanguageFromRequest(c)
	next := sanitizeAdminRedirectTarget(c.PostForm("next"))
	status := strings.TrimSpace(c.PostForm("status"))
	reportIDStrs := c.PostFormArray("report_ids")

	if len(reportIDStrs) == 0 || status == "" {
		redirectAdminWithMessage(c, next, "error", adminText(lang, "error_bulk_no_selection"))
		return
	}

	var failed int
	for _, idStr := range reportIDStrs {
		idStr = strings.TrimSpace(idStr)
		if idStr == "" {
			continue
		}
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		if err := a.ensureReportStatusScope(c.Request.Context(), session, id); err != nil {
			a.log.Error("bulk status scope check failed", "report_id", id, "status", status, "error", err)
			failed++
			continue
		}
		if _, err := a.adminUpdateStatus(c.Request.Context(), id, status, session); err != nil {
			a.log.Error("bulk status update failed", "report_id", id, "status", status, "error", err)
			failed++
		}
	}

	if failed > 0 {
		redirectAdminWithMessage(c, next, "error", fmt.Sprintf(adminText(lang, "error_bulk_partial"), len(reportIDStrs)-failed, len(reportIDStrs)))
		return
	}

	redirectAdminWithMessage(c, next, "notice", fmt.Sprintf(adminText(lang, "notice_bulk_updated"), len(reportIDStrs)))
}

func (a *App) adminMergeSubmitHandler(c *gin.Context) {
	session, err := getOperatorSession(c)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/bikeadmin/login")
		return
	}
	next := sanitizeAdminRedirectTarget(c.PostForm("next"))
	duplicateIDs := parseDuplicateReportIDs(c.PostForm("duplicate_report_ids"))
	if len(duplicateIDs) == 0 {
		redirectAdminWithMessage(c, next, "error", adminText(a.adminLanguageFromRequest(c), "error_merge_input_required"))
		return
	}
	reportID, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		redirectAdminWithMessage(c, next, "error", "Invalid ID")
		return
	}

	if _, err := a.adminMerge(c.Request.Context(), reportID, duplicateIDs, session); err != nil {
		redirectAdminWithMessage(c, next, "error", normalizeAdminErrorMessage(err, a.adminLanguageFromRequest(c), "error_merge_failed"))
		return
	}
	redirectAdminWithMessage(c, next, "notice", adminText(a.adminLanguageFromRequest(c), "notice_merge_completed"))
}

func (a *App) adminMapPageHandler(c *gin.Context) {
	lang := a.adminLanguageFromRequest(c)

	filters := map[string]any{}
	session, _ := getOperatorSession(c)
	if err := applySessionMunicipalityScope(filters, session); err != nil {
		base := a.adminBaseData(c, "page_title_map", "map")
		base.ErrorMessage = "Access restricted: invalid operator scope"
		a.renderAdminTemplate(c, http.StatusForbidden, adminTemplateMapPath, adminMapViewData{adminBaseViewData: base, MapData: template.JS("[]")})
		return
	}

	reports, err := a.adminListReports(c.Request.Context(), filters)
	if err != nil {
		base := a.adminBaseData(c, "page_title_map", "map")
		base.ErrorMessage = adminText(lang, "error_reports_load_failed")
		a.renderAdminTemplate(c, http.StatusInternalServerError, adminTemplateMapPath, adminMapViewData{adminBaseViewData: base, MapData: template.JS("[]")})
		return
	}

	points := make([]adminMapPoint, 0, len(reports))
	for _, report := range reports {
		tags := make([]string, 0, len(report.Tags))
		for _, tag := range report.Tags {
			tags = append(tags, adminTagLabel(lang, tag))
		}
		points = append(points, adminMapPoint{
			ID:          report.ID,
			PublicID:    report.PublicID,
			Lat:         report.Location.Lat,
			Lng:         report.Location.Lng,
			Status:      report.Status,
			StatusLabel: adminStatusLabel(lang, report.Status),
			Tags:        strings.Join(tags, ", "),
			Address:     valueOrDash(report.Address),
			City:        valueOrDash(report.City),
		})
	}
	encoded, marshalErr := json.Marshal(points)
	if marshalErr != nil {
		base := a.adminBaseData(c, "page_title_map", "map")
		base.ErrorMessage = adminText(lang, "error_reports_load_failed")
		a.renderAdminTemplate(c, http.StatusInternalServerError, adminTemplateMapPath, adminMapViewData{adminBaseViewData: base, MapData: template.JS("[]")})
		return
	}

	base := a.adminBaseData(c, "page_title_map", "map")
	base.IncludeMapLibre = true
	data := adminMapViewData{
		adminBaseViewData: base,
		MapData:           template.JS(string(encoded)),
	}
	a.renderAdminTemplate(c, http.StatusOK, adminTemplateMapPath, data)
}

func (a *App) adminExportsPageHandler(c *gin.Context) {
	session, _ := getOperatorSession(c)
	lang := a.adminLanguageFromRequest(c)

	// Allow all operators to access exports page, scope handling is in generation/listing
	exports, err := a.adminListAllExports(c.Request.Context(), session)
	if err != nil {
		base := a.adminBaseData(c, "page_title_exports", "exports")
		base.ErrorMessage = adminText(lang, "error_export_load_failed")
		a.renderAdminTemplate(c, http.StatusInternalServerError, adminTemplateExportsPath, adminExportsViewData{adminBaseViewData: base, SelectedPeriod: "weekly", Exports: []adminExportRowView{}})
		return
	}

	rows := make([]adminExportRowView, 0, len(exports))
	for _, batch := range exports {
		filterStatus := ""
		if batch.FilterStatus != nil {
			filterStatus = *batch.FilterStatus
		}
		filterMunicipality := ""
		if batch.FilterMunicipality != nil {
			filterMunicipality = *batch.FilterMunicipality
		}

		rows = append(rows, adminExportRowView{
			ID:                 batch.ID,
			GeneratedAt:        formatAdminTimestamp(batch.GeneratedAt),
			PeriodType:         adminPeriodTypeLabel(lang, batch.PeriodType),
			PeriodRange:        batch.PeriodRange(),
			RowCount:           batch.RowCount,
			FilterStatus:       filterStatus,
			FilterMunicipality: filterMunicipality,
		})
	}

	municipalities := []string{}
	if session.Role == "admin" {
		municipalities = municipalityList()
	} else {
		if session.Municipality == nil {
			base := a.adminBaseData(c, "page_title_exports", "exports")
			base.ErrorMessage = "Access restricted: invalid operator scope"
			a.renderAdminTemplate(c, http.StatusForbidden, adminTemplateExportsPath, adminExportsViewData{adminBaseViewData: base})
			return
		}
		// Filter exports list to only specific municipality if needed.
		// Note: adminListAllExports currently returns all. We rely on valid session.
	}

	base := a.adminBaseData(c, "page_title_exports", "exports")
	data := adminExportsViewData{
		adminBaseViewData: base,
		SelectedPeriod:    a.parsePeriodType(strings.TrimSpace(c.Query("period_type"))),
		Exports:           rows,
		Municipalities:    municipalities,
		Statuses:          reportStatuses,
	}
	a.renderAdminTemplate(c, http.StatusOK, adminTemplateExportsPath, data)
}

func (a *App) adminGenerateExportSubmitHandler(c *gin.Context) {
	session, err := getOperatorSession(c)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/bikeadmin/login")
		return
	}

	periodType := a.parsePeriodType(strings.TrimSpace(c.PostForm("period_type")))
	status := strings.TrimSpace(c.PostForm("status"))
	municipality := strings.TrimSpace(c.PostForm("municipality"))

	input := map[string]any{
		"period_type":  periodType,
		"status":       status,
		"municipality": municipality,
	}

	if _, err := a.adminGenerate(c.Request.Context(), input, session); err != nil {
		redirectAdminWithMessage(c, "/bikeadmin/exports", "error", normalizeAdminErrorMessage(err, a.adminLanguageFromRequest(c), "error_export_generate_failed"))
		return
	}
	redirectAdminWithMessage(c, "/bikeadmin/exports", "notice", adminText(a.adminLanguageFromRequest(c), "notice_export_generated"))
}

func (a *App) adminAuthenticate(ctx context.Context, email, password string) (string, *string, error) {
	if a.adminAuthenticateOperator != nil {
		return a.adminAuthenticateOperator(ctx, email, password)
	}
	return a.authenticateOperatorCredentials(ctx, email, password)
}

func (a *App) adminListReports(ctx context.Context, filters map[string]any) ([]OperatorReportView, error) {
	if a.adminListOperatorReports != nil {
		return a.adminListOperatorReports(ctx, filters)
	}
	return a.listOperatorReports(ctx, filters)
}

func (a *App) adminListReportsPaginated(ctx context.Context, filters map[string]any, page, pageSize int) (*PaginatedOperatorReports, error) {
	if a.adminListPaginatedReports != nil {
		return a.adminListPaginatedReports(ctx, filters, page, pageSize)
	}
	return a.listOperatorReportsPaginated(ctx, filters, page, pageSize)
}

func (a *App) adminLoadReportDetails(ctx context.Context, reportID int) (*OperatorReportDetails, error) {
	if a.adminGetReportDetails != nil {
		return a.adminGetReportDetails(ctx, reportID)
	}
	return a.getReportDetails(ctx, reportID)
}

func (a *App) adminLoadReportByID(ctx context.Context, reportID int) (*Report, error) {
	if a.adminGetReportByID != nil {
		return a.adminGetReportByID(ctx, reportID)
	}
	return a.getReportByID(ctx, reportID)
}

func (a *App) adminUpdateStatus(ctx context.Context, reportID int, status string, session OperatorSession) (*Report, error) {
	if a.adminUpdateReportStatus != nil {
		return a.adminUpdateReportStatus(ctx, reportID, status, session)
	}
	return a.updateReportStatus(ctx, reportID, status, session)
}

func (a *App) adminMerge(ctx context.Context, canonicalReportID int, duplicateReportIDs []int, session OperatorSession) (*DedupeGroup, error) {
	if a.adminMergeDuplicates != nil {
		return a.adminMergeDuplicates(ctx, canonicalReportID, duplicateReportIDs, session)
	}
	return a.mergeDuplicateReports(ctx, canonicalReportID, duplicateReportIDs, session)
}

func (a *App) adminListAllExports(ctx context.Context, session OperatorSession) ([]ExportBatch, error) {
	if a.adminListExports != nil {
		return a.adminListExports(ctx, session)
	}
	return a.listExportBatches(ctx, session)
}

func (a *App) adminGenerate(ctx context.Context, input map[string]any, session OperatorSession) (*ExportBatch, error) {
	if a.adminGenerateExport != nil {
		return a.adminGenerateExport(ctx, input, session)
	}
	return a.generateExportBatch(ctx, input, session)
}

func (a *App) renderAdminTemplate(c *gin.Context, status int, contentTemplatePath string, data any) {
	templates, err := a.adminTemplates.templatesForRender(contentTemplatePath)
	if err != nil {
		c.String(http.StatusInternalServerError, "admin template error: %v", err)
		return
	}

	c.Status(status)
	if executeErr := templates.ExecuteTemplate(c.Writer, "layout", data); executeErr != nil {
		a.log.Error("render admin template failed", "error", executeErr)
		if !c.Writer.Written() {
			c.String(http.StatusInternalServerError, "render failure")
		}
	}
}

func (a *App) adminBaseData(c *gin.Context, titleKey, activeNav string) adminBaseViewData {
	lang := a.adminLanguageFromRequest(c)
	var session *OperatorSession
	if value, ok := c.Get("operatorSession"); ok {
		if stored, castOK := value.(OperatorSession); castOK {
			session = &stored
		}
	}

	return adminBaseViewData{
		Title:           adminText(lang, titleKey),
		Lang:            lang,
		Text:            adminTexts(lang),
		Session:         session,
		CurrentPath:     sanitizeAdminRedirectTarget(c.Request.URL.RequestURI()),
		ActiveNav:       activeNav,
		ErrorMessage:    strings.TrimSpace(c.Query("error")),
		NoticeMessage:   strings.TrimSpace(c.Query("notice")),
		IncludeMapLibre: false,
		ShowExports:     session != nil && session.Role == "admin",
	}
}

func (a *App) setAdminLanguageCookie(c *gin.Context, language string) {
	secure := strings.EqualFold(a.cfg.Env, "production")
	c.SetCookie(adminLanguageCookieName, normalizeAdminLanguage(language), int(adminLanguageCookieMaxAge.Seconds()), "/", "", secure, true)
}

func (a *App) adminLanguageFromRequest(c *gin.Context) string {
	cookieValue, err := c.Cookie(adminLanguageCookieName)
	if err != nil {
		return adminDefaultLanguage
	}
	return normalizeAdminLanguage(cookieValue)
}

func normalizeAdminLanguage(language string) string {
	switch strings.ToLower(strings.TrimSpace(language)) {
	case "en":
		return "en"
	default:
		return adminDefaultLanguage
	}
}

func adminTexts(lang string) map[string]string {
	language := normalizeAdminLanguage(lang)
	if translations, ok := adminTranslations[language]; ok {
		return translations
	}
	return adminTranslations[adminDefaultLanguage]
}

func adminText(lang, key string) string {
	translations := adminTexts(lang)
	if value, ok := translations[key]; ok {
		return value
	}
	fallback := adminTranslations[adminDefaultLanguage]
	if value, ok := fallback[key]; ok {
		return value
	}
	return key
}

func adminStatusLabel(lang, status string) string {
	return adminText(lang, "status_"+status)
}

func adminSignalLabel(lang, signal string) string {
	switch signal {
	case "weak_same_reporter":
		return adminText(lang, "signal_weak_same_reporter")
	case "strong_distinct_reporters":
		return adminText(lang, "signal_strong_distinct_reporters")
	default:
		return adminText(lang, "signal_none")
	}
}

func adminEventLabel(lang, eventType string) string {
	return adminText(lang, "event_"+eventType)
}

func adminTimelineLabel(lang string, entry SignalTimelineEntry) string {
	if entry.IgnoredSameDay {
		return adminText(lang, "timeline_ignored")
	}
	if entry.Qualified && entry.ReporterMatchKind != nil && *entry.ReporterMatchKind == "same_reporter" {
		return adminText(lang, "timeline_same")
	}
	if entry.Qualified && entry.ReporterMatchKind != nil && *entry.ReporterMatchKind == "distinct_reporter" {
		return adminText(lang, "timeline_distinct")
	}
	return adminText(lang, "timeline_recorded")
}

func adminPeriodTypeLabel(lang, periodType string) string {
	if periodType == "monthly" {
		return adminText(lang, "exports_monthly")
	}
	return adminText(lang, "exports_weekly")
}

func adminTagLabel(lang, tag string) string {
	if translatedLanguage, ok := adminTagLabels[normalizeAdminLanguage(lang)]; ok {
		if translated, found := translatedLanguage[tag]; found {
			return translated
		}
	}
	if translated, found := adminTagLabels[adminDefaultLanguage][tag]; found {
		return translated
	}
	return tag
}

func formatAdminTimestamp(raw string) string {
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return raw
	}
	return parsed.In(adminTimeLocation()).Format(adminDisplayTimestampLayout)
}

func adminTimeLocation() *time.Location {
	adminTimeZoneOnce.Do(func() {
		location, err := time.LoadLocation("Europe/Amsterdam")
		if err != nil {
			adminTimeZone = time.UTC
			return
		}
		adminTimeZone = location
	})
	return adminTimeZone
}

func buildAdminStatusActions(lang, currentStatus, next string) []adminStatusActionView {
	allowed := statusTransitions[currentStatus]
	actions := make([]adminStatusActionView, 0, len(allowed))
	for _, candidate := range allowed {
		actions = append(actions, adminStatusActionView{
			Status: candidate,
			Label:  adminText(lang, fmt.Sprintf(adminStatusActionLabelTemplate, candidate)),
			Next:   next,
		})
	}
	return actions
}

func adminSignalClass(strength string) string {
	switch strength {
	case "strong_distinct_reporters":
		return adminSignalClassStrong
	case "weak_same_reporter":
		return adminSignalClassWeak
	default:
		return adminSignalClassNone
	}
}

func sanitizeAdminRedirectTarget(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "/bikeadmin"
	}

	parsed, err := url.Parse(trimmed)
	if err != nil {
		return "/bikeadmin"
	}
	if parsed.IsAbs() || parsed.Host != "" {
		return "/bikeadmin"
	}
	if strings.HasPrefix(parsed.Path, "//") {
		return "/bikeadmin"
	}
	if !strings.HasPrefix(parsed.Path, "/bikeadmin") {
		return "/bikeadmin"
	}
	if parsed.Path == "/bikeadmin/login" || parsed.Path == "/bikeadmin/logout" || parsed.Path == "/bikeadmin/language" {
		return "/bikeadmin"
	}

	target := parsed.Path
	if parsed.RawQuery != "" {
		target += "?" + parsed.RawQuery
	}
	return target
}

func redirectAdminWithMessage(c *gin.Context, target, key, value string) {
	parsed, err := url.Parse(sanitizeAdminRedirectTarget(target))
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/bikeadmin")
		return
	}
	query := parsed.Query()
	query.Del("error")
	query.Del("notice")
	query.Set(key, value)
	parsed.RawQuery = query.Encode()

	redirectURL := parsed.Path
	if parsed.RawQuery != "" {
		redirectURL += "?" + parsed.RawQuery
	}
	c.Redirect(http.StatusSeeOther, redirectURL)
}

func parseDuplicateReportIDs(raw string) []int {
	values := strings.FieldsFunc(raw, func(r rune) bool {
		switch r {
		case ',', '\n', '\r', '\t', ';', ' ':
			return true
		default:
			return false
		}
	})
	seen := make(map[int]struct{}, len(values))
	ids := make([]int, 0, len(values))
	for _, value := range values {
		clean := strings.TrimSpace(value)
		if clean == "" {
			continue
		}
		id, err := strconv.Atoi(clean)
		if err != nil {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	sort.Ints(ids)
	return ids
}

func normalizeAdminErrorMessage(err error, lang string, fallbackKey string) string {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		if strings.TrimSpace(apiErr.Message) != "" {
			return apiErr.Message
		}
	}
	return adminText(lang, fallbackKey)
}

func applySessionMunicipalityScope(filters map[string]any, session OperatorSession) error {
	if session.Role == "admin" {
		return nil
	}
	if session.Municipality == nil {
		return &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Access restricted: invalid operator scope"}
	}
	filters["city"] = *session.Municipality
	return nil
}

func (a *App) ensureReportStatusScope(ctx context.Context, session OperatorSession, reportID int) error {
	if session.Role == "admin" {
		return nil
	}
	if session.Municipality == nil {
		return &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Access restricted to valid municipality"}
	}

	report, err := a.adminLoadReportByID(ctx, reportID)
	if err != nil {
		return err
	}
	if report == nil {
		return &apiError{Status: http.StatusNotFound, Code: "report_not_found", Message: "Report not found"}
	}
	if report.Municipality == nil || !strings.EqualFold(*report.Municipality, *session.Municipality) {
		return &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Access restricted to municipality"}
	}
	return nil
}

func parseAdminReportFilters(c *gin.Context) adminReportFilters {
	filters := adminReportFilters{
		City:                        strings.TrimSpace(c.Query("city")),
		Status:                      strings.TrimSpace(c.Query("status")),
		SignalStrength:              strings.TrimSpace(c.Query("signal_strength")),
		HasQualifyingReconfirmation: strings.TrimSpace(c.Query("has_qualifying_reconfirmation")),
		StrongOnly:                  strings.TrimSpace(c.Query("strong_only")),
		Sort:                        strings.TrimSpace(c.Query("sort")),
	}
	if filters.Sort != "signal" {
		filters.Sort = "newest"
	}
	return filters
}

func (f adminReportFilters) toStoreFilters() map[string]any {
	filters := map[string]any{}
	if f.City != "" {
		filters["report_city"] = f.City
	}
	if containsString(reportStatuses, f.Status) {
		filters["status"] = f.Status
	}
	if f.SignalStrength == "none" || f.SignalStrength == "weak_same_reporter" || f.SignalStrength == "strong_distinct_reporters" {
		filters["signal_strength"] = f.SignalStrength
	}
	if f.HasQualifyingReconfirmation == "true" {
		filters["has_qualifying_reconfirmation"] = true
	}
	if f.HasQualifyingReconfirmation == "false" {
		filters["has_qualifying_reconfirmation"] = false
	}
	if f.StrongOnly == "true" {
		filters["strong_only"] = true
	}
	if f.StrongOnly == "false" {
		filters["strong_only"] = false
	}
	filters["sort"] = f.Sort
	return filters
}

func (f adminReportFilters) queryString() string {
	params := url.Values{}
	if f.City != "" {
		params.Set("city", f.City)
	}
	if f.Status != "" {
		params.Set("status", f.Status)
	}
	if f.SignalStrength != "" {
		params.Set("signal_strength", f.SignalStrength)
	}
	if f.HasQualifyingReconfirmation != "" {
		params.Set("has_qualifying_reconfirmation", f.HasQualifyingReconfirmation)
	}
	if f.StrongOnly != "" {
		params.Set("strong_only", f.StrongOnly)
	}
	params.Set("sort", f.Sort)
	return params.Encode()
}

func (f adminReportFilters) currentURL() string {
	query := f.queryString()
	if query == "" {
		return "/bikeadmin"
	}
	return "/bikeadmin?" + query
}

func (f adminReportFilters) toView() adminReportFiltersView {
	return adminReportFiltersView{
		City:                        f.City,
		Status:                      f.Status,
		SignalStrength:              f.SignalStrength,
		HasQualifyingReconfirmation: f.HasQualifyingReconfirmation,
		StrongOnly:                  f.StrongOnly,
		Sort:                        f.Sort,
		CurrentURL:                  f.currentURL(),
	}
}

func (a *App) adminListCachedReportCities(ctx context.Context, session OperatorSession) ([]string, error) {
	cacheKey, municipality, err := cityFilterScope(session)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	a.cityFilterMu.Lock()
	entry, ok := a.cityFilterCache[cacheKey]
	if ok && now.Before(entry.expiresAt) {
		cached := append([]string(nil), entry.values...)
		a.cityFilterMu.Unlock()
		return cached, nil
	}
	a.cityFilterMu.Unlock()

	cities, err := a.adminLoadReportCities(ctx, municipality)
	if err != nil {
		return nil, err
	}
	normalized := normalizeCityOptions(cities)

	a.cityFilterMu.Lock()
	if a.cityFilterCache == nil {
		a.cityFilterCache = make(map[string]cityFilterCacheEntry)
	}
	a.cityFilterCache[cacheKey] = cityFilterCacheEntry{
		values:    append([]string(nil), normalized...),
		expiresAt: now.Add(adminCityFilterCacheTTL),
	}
	a.cityFilterMu.Unlock()

	return normalized, nil
}

func (a *App) adminLoadReportCities(ctx context.Context, municipality *string) ([]string, error) {
	if a.adminListReportCities != nil {
		return a.adminListReportCities(ctx, municipality)
	}
	return a.storeListReportCities(ctx, municipality)
}

func cityFilterScope(session OperatorSession) (string, *string, error) {
	if session.Role == "admin" {
		return "admin", nil, nil
	}
	if session.Municipality == nil {
		return "", nil, &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Access restricted: invalid operator scope"}
	}
	city := strings.TrimSpace(*session.Municipality)
	if city == "" {
		return "", nil, &apiError{Status: http.StatusForbidden, Code: "forbidden", Message: "Access restricted: invalid operator scope"}
	}
	cacheKey := "municipality:" + strings.ToLower(city)
	return cacheKey, &city, nil
}

func normalizeCityOptions(input []string) []string {
	seen := make(map[string]struct{}, len(input))
	values := make([]string, 0, len(input))
	for _, raw := range input {
		city := strings.TrimSpace(raw)
		if city == "" {
			continue
		}
		key := strings.ToLower(city)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		values = append(values, city)
	}
	sort.Slice(values, func(i, j int) bool {
		left := strings.ToLower(values[i])
		right := strings.ToLower(values[j])
		if left == right {
			return values[i] < values[j]
		}
		return left < right
	})
	return values
}

func valueOrDash(s *string) string {
	if s == nil || *s == "" {
		return "-"
	}
	return *s
}

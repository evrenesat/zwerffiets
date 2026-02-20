package main

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

type adminContentRowView struct {
	Key     string
	NlText  string
	EnText  string
	Updated string
}

type adminContentViewData struct {
	adminBaseViewData
	Contents []adminContentRowView
}

func (a *App) adminContentPageHandler(c *gin.Context) {
	list := GetAllSiteContents()

	rows := make([]adminContentRowView, 0, len(list))
	for _, item := range list {
		rows = append(rows, adminContentRowView{
			Key:     item.Key,
			NlText:  item.NlText,
			EnText:  item.EnText,
			Updated: formatAdminTimestamp(item.UpdatedAt.Format(time.RFC3339)),
		})
	}

	base := a.adminBaseData(c, "page_title_content", "content")
	data := adminContentViewData{
		adminBaseViewData: base,
		Contents:          rows,
	}
	a.renderAdminTemplate(c, http.StatusOK, "templates/admin/content_manager.tmpl", data)
}

func (a *App) adminContentSubmitHandler(c *gin.Context) {
	lang := a.adminLanguageFromRequest(c)
	session, err := getOperatorSession(c)
	if err != nil {
		c.Redirect(http.StatusSeeOther, "/bikeadmin/login")
		return
	}

	key := strings.TrimSpace(c.PostForm("key"))
	nlText := strings.TrimSpace(c.PostForm("nl_text"))
	enText := strings.TrimSpace(c.PostForm("en_text"))

	if key == "" {
		redirectAdminWithMessage(c, "/bikeadmin/content", "error", adminText(lang, "error_content_update_failed"))
		return
	}

	content := SiteContent{
		Key:       key,
		NlText:    nlText,
		EnText:    enText,
		UpdatedBy: session.Email,
	}

	if err := SaveSiteContent(c.Request.Context(), a.db, content); err != nil {
		a.log.Error("failed to save site content", "error", err, "key", key)
		redirectAdminWithMessage(c, "/bikeadmin/content", "error", adminText(lang, "error_content_update_failed"))
		return
	}

	redirectAdminWithMessage(c, fmt.Sprintf("/bikeadmin/content#row-%s", key), "notice", adminText(lang, "notice_content_updated"))
}

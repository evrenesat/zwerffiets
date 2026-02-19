package main

import (
	"context"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type adminUserListViewData struct {
	adminBaseViewData
	Users      []User
	Filters    adminUserFiltersView
	Pagination adminPaginationViewData
}

type adminUserEditViewData struct {
	adminBaseViewData
	User User
}

type adminUserFilters struct {
	Q      string
	Status string
}

type adminUserFiltersView struct {
	Q          string
	Status     string
	CurrentURL string
}

func (a *App) adminUsersPageHandler(c *gin.Context) {
	lang := a.adminLanguageFromRequest(c)
	filters := parseAdminUserFilters(c)
	storeFilters := filters.toStoreFilters()
	page := parseAdminPage(c.Query("page"))

	paginatedUsers, err := a.adminListUsersPaginated(c.Request.Context(), storeFilters, page, adminDefaultPerPage)
	if err != nil {
		base := a.adminBaseData(c, "page_title_users", "users")
		base.ErrorMessage = adminText(lang, "error_users_load_failed")
		a.renderAdminTemplate(c, http.StatusInternalServerError, "templates/admin/users.tmpl", adminUserListViewData{
			adminBaseViewData: base,
			Filters:           filters.toView(),
		})
		return
	}

	currentURL := filters.currentURL()
	base := a.adminBaseData(c, "page_title_users", "users")
	a.renderAdminTemplate(c, http.StatusOK, "templates/admin/users.tmpl", adminUserListViewData{
		adminBaseViewData: base,
		Users:             paginatedUsers.Users,
		Filters:           filters.toView(),
		Pagination: buildAdminPaginationView(
			paginatedUsers.TotalCount,
			paginatedUsers.CurrentPage,
			paginatedUsers.PageSize,
			currentURL,
		),
	})
}

func (a *App) adminUserEditPageHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	lang := a.adminLanguageFromRequest(c)
	if err != nil {
		redirectAdminWithMessage(c, "/bikeadmin/users", "error", "Invalid ID")
		return
	}

	user, err := a.adminGetUserByID(c.Request.Context(), id)
	if err != nil || user == nil {
		redirectAdminWithMessage(c, "/bikeadmin/users", "error", adminText(lang, "error_user_not_found"))
		return
	}

	base := a.adminBaseData(c, "page_title_user_edit", "users")
	a.renderAdminTemplate(c, http.StatusOK, "templates/admin/user_edit.tmpl", adminUserEditViewData{
		adminBaseViewData: base,
		User:              *user,
	})
}

func (a *App) adminUserEditSubmitHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	lang := a.adminLanguageFromRequest(c)
	if err != nil {
		redirectAdminWithMessage(c, "/bikeadmin/users", "error", "Invalid ID")
		return
	}

	email := strings.TrimSpace(c.PostForm("email"))
	displayName := strings.TrimSpace(c.PostForm("display_name"))
	isActive := c.PostForm("is_active") == "on"

	var dnPtr *string
	if displayName != "" {
		dnPtr = &displayName
	}

	if err := a.adminUpdateUser(c.Request.Context(), id, email, dnPtr, isActive); err != nil {
		redirectAdminWithMessage(c, "/bikeadmin/users/"+strconv.Itoa(id)+"/edit", "error", adminText(lang, "error_user_update_failed"))
		return
	}

	redirectAdminWithMessage(c, "/bikeadmin/users", "notice", adminText(lang, "notice_user_updated"))
}

func (a *App) adminUsersBulkSubmitHandler(c *gin.Context) {
	lang := a.adminLanguageFromRequest(c)
	action := c.PostForm("action")
	userIDs := c.PostFormArray("user_ids")
	next := sanitizeAdminRedirectTarget(c.PostForm("next"))
	if next == "/bikeadmin" {
		next = "/bikeadmin/users"
	}

	if len(userIDs) == 0 {
		redirectAdminWithMessage(c, next, "error", adminText(lang, "error_bulk_no_selection"))
		return
	}

	var ids []int
	for _, idStr := range userIDs {
		if id, err := strconv.Atoi(idStr); err == nil {
			ids = append(ids, id)
		}
	}

	var err error
	var message string
	switch action {
	case "activate":
		err = a.adminBulkUpdateUserStatus(c.Request.Context(), ids, true)
		message = adminText(lang, "notice_bulk_updated")
	case "deactivate":
		err = a.adminBulkUpdateUserStatus(c.Request.Context(), ids, false)
		message = adminText(lang, "notice_bulk_updated")
	case "delete":
		err = a.adminBulkDeleteUsers(c.Request.Context(), ids)
		message = adminText(lang, "notice_bulk_deleted")
	default:
		redirectAdminWithMessage(c, next, "error", "Invalid action")
		return
	}

	if err != nil {
		redirectAdminWithMessage(c, next, "error", adminText(lang, "error_bulk_operation_failed"))
		return
	}

	redirectAdminWithMessage(c, next, "notice", message)
}

func parseAdminUserFilters(c *gin.Context) adminUserFilters {
	filters := adminUserFilters{
		Q:      strings.TrimSpace(c.Query("q")),
		Status: strings.TrimSpace(c.Query("status")),
	}
	if filters.Status != "active" && filters.Status != "inactive" {
		filters.Status = ""
	}
	return filters
}

func (f adminUserFilters) toStoreFilters() map[string]any {
	filters := map[string]any{}
	if f.Q != "" {
		filters["q"] = f.Q
	}
	if f.Status != "" {
		filters["status"] = f.Status
	}
	return filters
}

func (f adminUserFilters) queryString() string {
	params := url.Values{}
	if f.Q != "" {
		params.Set("q", f.Q)
	}
	if f.Status != "" {
		params.Set("status", f.Status)
	}
	return params.Encode()
}

func (f adminUserFilters) currentURL() string {
	query := f.queryString()
	if query == "" {
		return "/bikeadmin/users"
	}
	return "/bikeadmin/users?" + query
}

func (f adminUserFilters) toView() adminUserFiltersView {
	return adminUserFiltersView{
		Q:          f.Q,
		Status:     f.Status,
		CurrentURL: f.currentURL(),
	}
}

func (a *App) adminListUsersPaginated(ctx context.Context, filters map[string]any, page, pageSize int) (*PaginatedUsers, error) {
	if a.adminListPaginatedUsers != nil {
		return a.adminListPaginatedUsers(ctx, filters, page, pageSize)
	}
	return a.storeListUsersPaginated(ctx, filters, page, pageSize)
}

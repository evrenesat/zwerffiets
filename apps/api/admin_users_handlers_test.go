package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
)

func TestAdminUsersPageHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	app, router := newAdminTestServer(t)

	capturedFilters := map[string]any{}
	capturedPage := 0
	capturedPageSize := 0
	app.adminListPaginatedUsers = func(ctx context.Context, filters map[string]any, page, pageSize int) (*PaginatedUsers, error) {
		for key, value := range filters {
			capturedFilters[key] = value
		}
		capturedPage = page
		capturedPageSize = pageSize
		return &PaginatedUsers{
			Users: []User{
				{ID: 1, Email: "test@example.com", IsActive: true},
			},
			TotalCount:  120,
			TotalPages:  3,
			CurrentPage: page,
			PageSize:    pageSize,
		}, nil
	}

	w := httptest.NewRecorder()
	req := authenticatedRequest(t, app, http.MethodGet, "/bikeadmin/users?q=test&status=active&page=2", "")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Contains(t, w.Body.String(), "test@example.com")
	assert.Equal(t, 2, capturedPage)
	assert.Equal(t, adminDefaultPerPage, capturedPageSize)
	assert.Equal(t, "test", capturedFilters["q"])
	assert.Equal(t, "active", capturedFilters["status"])
	assert.True(t, strings.Contains(w.Body.String(), "page=1"))
	assert.True(t, strings.Contains(w.Body.String(), "page=3"))
}

func TestAdminUsersPageHandler_InvalidPageDefaultsToFirstPage(t *testing.T) {
	app, router := newAdminTestServer(t)
	capturedPage := 0

	app.adminListPaginatedUsers = func(ctx context.Context, filters map[string]any, page, pageSize int) (*PaginatedUsers, error) {
		capturedPage = page
		return &PaginatedUsers{
			Users:       []User{},
			TotalCount:  0,
			TotalPages:  0,
			CurrentPage: page,
			PageSize:    pageSize,
		}, nil
	}

	w := httptest.NewRecorder()
	req := authenticatedRequest(t, app, http.MethodGet, "/bikeadmin/users?page=0", "")
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, capturedPage)
}

func TestAdminUserEditSubmitHandler(t *testing.T) {
	app, router := newAdminTestServer(t)

	var updatedID int
	var updatedEmail string
	var updatedActive bool

	app.adminUpdateUser = func(ctx context.Context, id int, email string, displayName *string, isActive bool) error {
		updatedID = id
		updatedEmail = email
		updatedActive = isActive
		return nil
	}

	form := url.Values{}
	form.Set("email", "new@example.com")
	form.Set("is_active", "on")
	form.Set("display_name", "")

	w := httptest.NewRecorder()
	req := authenticatedRequest(t, app, http.MethodPost, "/bikeadmin/users/1/edit", form.Encode())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.Equal(t, 1, updatedID)
	assert.Equal(t, "new@example.com", updatedEmail)
	assert.True(t, updatedActive)
}

func TestAdminUsersBulkSubmitHandler_Delete(t *testing.T) {
	app, router := newAdminTestServer(t)

	var deletedIDs []int
	app.adminBulkDeleteUsers = func(ctx context.Context, ids []int) error {
		deletedIDs = ids
		return nil
	}

	form := url.Values{}
	form.Set("action", "delete")
	form.Add("user_ids", "1")
	form.Add("user_ids", "2")

	w := httptest.NewRecorder()
	req := authenticatedRequest(t, app, http.MethodPost, "/bikeadmin/users/bulk", form.Encode())
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusSeeOther, w.Code)
	assert.ElementsMatch(t, []int{1, 2}, deletedIDs)
}

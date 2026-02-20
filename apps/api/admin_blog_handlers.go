package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

func (a *App) adminBlogListPageHandler(c *gin.Context) {
	posts, err := a.storeAdminListBlogPosts(c.Request.Context())
	if err != nil {
		a.log.Error("failed to list blog posts", "error", err)
		base := a.adminBaseData(c, "page_title_blog", "blog")
		base.ErrorMessage = adminText(a.adminLanguageFromRequest(c), "error_blog_load_failed")
		a.renderAdminTemplate(c, http.StatusInternalServerError, "templates/admin/blog_list.tmpl", adminBlogListViewData{adminBaseViewData: base})
		return
	}

	base := a.adminBaseData(c, "page_title_blog", "blog")
	a.renderAdminTemplate(c, http.StatusOK, "templates/admin/blog_list.tmpl", adminBlogListViewData{
		adminBaseViewData: base,
		Posts:             posts,
	})
}

func (a *App) adminBlogCreatePageHandler(c *gin.Context) {
	base := a.adminBaseData(c, "page_title_blog_new", "blog")
	a.renderAdminTemplate(c, http.StatusOK, "templates/admin/blog_edit.tmpl", adminBlogEditViewData{
		adminBaseViewData: base,
		Post:              &BlogPost{IsPublished: false},
		IsNew:             true,
		BackURL:           "/bikeadmin/blog",
	})
}

func (a *App) adminBlogEditPageHandler(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil {
		c.String(http.StatusBadRequest, "Invalid ID")
		return
	}

	post, err := a.storeAdminGetBlogPost(c.Request.Context(), id)
	if err != nil {
		a.log.Error("failed to get blog post", "id", id, "error", err)
		base := a.adminBaseData(c, "page_title_blog_edit", "blog")
		base.ErrorMessage = adminText(a.adminLanguageFromRequest(c), "error_blog_load_failed")
		a.renderAdminTemplate(c, http.StatusNotFound, "templates/admin/blog_edit.tmpl", adminBlogEditViewData{adminBaseViewData: base})
		return
	}

	base := a.adminBaseData(c, "page_title_blog_edit", "blog")
	a.renderAdminTemplate(c, http.StatusOK, "templates/admin/blog_edit.tmpl", adminBlogEditViewData{
		adminBaseViewData: base,
		Post:              post,
		IsNew:             false,
		BackURL:           "/bikeadmin/blog",
	})
}

func (a *App) adminBlogSubmitHandler(c *gin.Context) {
	idStr := c.Param("id")
	isNew := idStr == ""
	lang := a.adminLanguageFromRequest(c)

	title := strings.TrimSpace(c.PostForm("title"))
	slug := strings.TrimSpace(c.PostForm("slug"))
	content := strings.TrimSpace(c.PostForm("content_html"))
	isPublished := c.PostForm("is_published") == "true"
	publishedAtStr := c.PostForm("published_at")

	if title == "" || slug == "" {
		redirectURL := "/bikeadmin/blog/new"
		if !isNew {
			redirectURL = "/bikeadmin/blog/" + idStr
		}
		redirectAdminWithMessage(c, redirectURL, "error", adminText(lang, "error_blog_update_failed"))
		return
	}

	var pubAt *time.Time
	if publishedAtStr != "" {
		t, err := time.Parse("2006-01-02T15:04", publishedAtStr)
		if err == nil {
			pubAt = &t
		}
	}
	if isPublished && pubAt == nil {
		now := time.Now()
		pubAt = &now
	}

	session, _ := getOperatorSession(c)
	var authorID *int
	op, err := a.storeAdminGetOperatorByEmail(c.Request.Context(), session.Email)
	if err == nil {
		authorID = &op.ID
	}

	post := &BlogPost{
		Title:       title,
		Slug:        slug,
		ContentHTML: content,
		IsPublished: isPublished,
		PublishedAt: pubAt,
		AuthorID:    authorID,
	}

	if isNew {
		if err := a.storeAdminCreateBlogPost(c.Request.Context(), post); err != nil {
			a.log.Error("failed to create blog post", "error", err)
			redirectAdminWithMessage(c, "/bikeadmin/blog/new", "error", adminText(lang, "error_blog_update_failed"))
			return
		}
		redirectAdminWithMessage(c, "/bikeadmin/blog", "notice", adminText(lang, "notice_blog_created"))
	} else {
		id, _ := strconv.Atoi(idStr)
		post.ID = id
		if err := a.storeAdminUpdateBlogPost(c.Request.Context(), post); err != nil {
			a.log.Error("failed to update blog post", "id", id, "error", err)
			redirectAdminWithMessage(c, "/bikeadmin/blog/"+idStr, "error", adminText(lang, "error_blog_update_failed"))
			return
		}
		redirectAdminWithMessage(c, "/bikeadmin/blog", "notice", adminText(lang, "notice_blog_updated"))
	}
}

func (a *App) adminBlogMediaUploadHandler(c *gin.Context) {
	file, err := c.FormFile("image")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No image uploaded"})
		return
	}

	ext := filepath.Ext(file.Filename)
	newFilename := uuid.New().String() + ext
	storagePath := filepath.Join(a.cfg.DataRoot, "blog", newFilename)

	if err := c.SaveUploadedFile(file, storagePath); err != nil {
		a.log.Error("failed to save blog media", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to save image"})
		return
	}

	m := &BlogMedia{
		Filename:    newFilename,
		StoragePath: storagePath,
		MimeType:    file.Header.Get("Content-Type"),
		SizeBytes:   file.Size,
	}

	if err := a.storeSaveBlogMedia(c.Request.Context(), m); err != nil {
		a.log.Error("failed to record blog media", "error", err)
		// Try to cleanup file
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to record media"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"url": fmt.Sprintf("/api/v1/blog/media/%s", newFilename),
	})
}

func (a *App) blogMediaServeHandler(c *gin.Context) {
	filename := c.Param("filename")
	m, err := a.storeGetBlogMedia(c.Request.Context(), filename)
	if err != nil {
		c.String(http.StatusNotFound, "Media not found")
		return
	}

	c.File(m.StoragePath)
}

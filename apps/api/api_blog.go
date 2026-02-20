package main

import (
	"net/http"
	"strconv" // Added strconv import

	"github.com/gin-gonic/gin"
)

func (a *App) publicBlogListHandler(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "10")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 10
	}

	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	posts, total, err := a.storePublicListBlogPosts(c.Request.Context(), limit, offset)
	if err != nil {
		a.log.Error("failed to fetch public blog posts", "error", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal server error"})
		return
	}

	if posts == nil {
		posts = []BlogPost{}
	}

	c.JSON(http.StatusOK, gin.H{
		"posts": posts,
		"total": total,
	})
}

func (a *App) publicBlogPostHandler(c *gin.Context) {
	slug := c.Param("slug")
	post, err := a.storePublicGetBlogPostBySlug(c.Request.Context(), slug)
	if err != nil {
		a.log.Error("failed to fetch public blog post", "slug", slug, "error", err)
		c.JSON(http.StatusNotFound, gin.H{"error": "Blog post not found"})
		return
	}

	c.JSON(http.StatusOK, post)
}

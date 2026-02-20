package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// handleGetDynamicContent returns the full, pre-loaded dynamic content cache.
// Method: GET /api/v1/content
// Access: Public
func handleGetDynamicContent(c *gin.Context) {
	cache := GetContentCache()

	// The frontend will merge this directly over its static translations.nl and translations.en dictionaries.
	c.JSON(http.StatusOK, cache)
}

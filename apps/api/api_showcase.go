package main

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strconv"

	"github.com/gin-gonic/gin"
)

type ShowcaseItemPublic struct {
	Slot         int    `json:"slot"`
	Subtitle     string `json:"subtitle"`
	FocalX       int    `json:"focalX"`
	FocalY       int    `json:"focalY"`
	ScalePercent int    `json:"scalePercent"`
	PhotoURL     string `json:"photoUrl"`
}

func (a *App) publicShowcaseItemsHandler(c *gin.Context) {
	items, err := a.storeGetShowcaseItems(c.Request.Context())
	if err != nil {
		// Use empty array on failure, frontend will gracefully degraded
		a.writeJSON(c, http.StatusOK, gin.H{"items": []ShowcaseItemPublic{}})
		return
	}

	var publicItems []ShowcaseItemPublic
	for _, item := range items {
		publicItems = append(publicItems, ShowcaseItemPublic{
			Slot:         item.Slot,
			Subtitle:     item.Subtitle,
			FocalX:       item.FocalX,
			FocalY:       item.FocalY,
			ScalePercent: item.ScalePercent,
			PhotoURL:     fmt.Sprintf("%s/api/v1/showcase/%d/photo", a.cfg.PublicBaseURL, item.Slot),
		})
	}
	a.writeJSON(c, http.StatusOK, gin.H{"items": publicItems})
}

func (a *App) publicShowcasePhotoHandler(c *gin.Context) {
	slotStr := c.Param("slot")
	slot, err := strconv.Atoi(slotStr)
	if err != nil {
		c.Status(http.StatusBadRequest)
		return
	}

	items, err := a.storeGetShowcaseItems(c.Request.Context())
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	var targetItem *ShowcaseItem
	for _, item := range items {
		if item.Slot == slot {
			targetItem = &item
			break
		}
	}

	if targetItem == nil {
		c.Status(http.StatusNotFound)
		return
	}

	fullPath := filepath.Join(a.cfg.DataRoot, targetItem.StoragePath)

	if a.isProduction() {
		c.Header("X-Accel-Redirect", "/_protected_media/"+targetItem.StoragePath)
		c.Status(http.StatusOK)
		return
	}

	ext := filepath.Ext(fullPath)
	if ext == "" {
		// Legacy uploaded images might be extensionless. They are mostly JPEGs.
		c.Header("Content-Type", "image/jpeg")
	}

	c.File(fullPath)
}

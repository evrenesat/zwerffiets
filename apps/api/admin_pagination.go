package main

import (
	"strconv"
	"strings"
)

const (
	adminDefaultPage    = 1
	adminDefaultPerPage = 50
)

func parseAdminPage(rawPage string) int {
	page, err := strconv.Atoi(strings.TrimSpace(rawPage))
	if err != nil || page < adminDefaultPage {
		return adminDefaultPage
	}
	return page
}

func buildAdminPaginationView(totalCount, currentPage, pageSize int, pageURL string) adminPaginationViewData {
	if pageSize < 1 {
		pageSize = adminDefaultPerPage
	}
	if currentPage < adminDefaultPage {
		currentPage = adminDefaultPage
	}

	totalPages := 0
	if totalCount > 0 {
		totalPages = (totalCount + pageSize - 1) / pageSize
	}

	pageSeparator := "?"
	if strings.Contains(pageURL, "?") {
		pageSeparator = "&"
	}

	return adminPaginationViewData{
		CurrentPage:   currentPage,
		TotalPages:    totalPages,
		TotalCount:    totalCount,
		NextPage:      currentPage + 1,
		PrevPage:      currentPage - 1,
		HasNext:       currentPage < totalPages,
		HasPrev:       currentPage > adminDefaultPage,
		PageURL:       pageURL,
		PageSeparator: pageSeparator,
	}
}

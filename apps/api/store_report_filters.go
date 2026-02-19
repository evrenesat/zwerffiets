package main

import "fmt"

func buildReportFilters(filters map[string]any) (string, []any) {
	whereClause := ""
	args := make([]any, 0)
	argIndex := 1

	if status, ok := filters["status"].(string); ok && status != "" {
		whereClause += fmt.Sprintf(" AND reports.status = $%d", argIndex)
		args = append(args, status)
		argIndex++
	}
	if tag, ok := filters["tag"].(string); ok && tag != "" {
		whereClause += fmt.Sprintf(" AND reports.tags::jsonb ? $%d", argIndex)
		args = append(args, tag)
		argIndex++
	}
	if from, ok := filters["from"].(string); ok && from != "" {
		whereClause += fmt.Sprintf(" AND reports.created_at >= $%d", argIndex)
		args = append(args, from)
		argIndex++
	}
	if to, ok := filters["to"].(string); ok && to != "" {
		whereClause += fmt.Sprintf(" AND reports.created_at <= $%d", argIndex)
		args = append(args, to)
		argIndex++
	}
	if municipality, ok := filters["city"].(string); ok && municipality != "" {
		whereClause += fmt.Sprintf(" AND LOWER(reports.municipality) = LOWER($%d)", argIndex)
		args = append(args, municipality)
		argIndex++
	}
	if reportCity, ok := filters["report_city"].(string); ok && reportCity != "" {
		whereClause += fmt.Sprintf(" AND LOWER(reports.city) = LOWER($%d)", argIndex)
		args = append(args, reportCity)
		argIndex++
	}

	return whereClause, args
}

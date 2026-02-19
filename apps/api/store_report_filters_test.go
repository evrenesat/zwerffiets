package main

import (
	"strings"
	"testing"
)

func TestBuildReportFilters(t *testing.T) {
	tests := []struct {
		name      string
		filters   map[string]any
		wantParts []string
		wantArgs  []any
	}{
		{
			name:      "No filters",
			filters:   map[string]any{},
			wantParts: []string{},
			wantArgs:  []any{},
		},
		{
			name: "Common filters",
			filters: map[string]any{
				"status": "new",
				"tag":    "flat_tires",
				"from":   "2026-01-01T00:00:00Z",
				"to":     "2026-01-31T00:00:00Z",
				"city":   "Amsterdam",
			},
			wantParts: []string{
				"reports.status = $1",
				"reports.tags::jsonb ? $2",
				"reports.created_at >= $3",
				"reports.created_at <= $4",
				"LOWER(reports.municipality) = LOWER($5)",
			},
			wantArgs: []any{"new", "flat_tires", "2026-01-01T00:00:00Z", "2026-01-31T00:00:00Z", "Amsterdam"},
		},
		{
			name: "Report city filter",
			filters: map[string]any{
				"report_city": "Utrecht",
			},
			wantParts: []string{
				"LOWER(reports.city) = LOWER($1)",
			},
			wantArgs: []any{"Utrecht"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			whereClause, args := buildReportFilters(tt.filters)
			for _, part := range tt.wantParts {
				if !strings.Contains(whereClause, part) {
					t.Fatalf("where clause missing %q in %q", part, whereClause)
				}
			}
			if len(args) != len(tt.wantArgs) {
				t.Fatalf("args length mismatch: got %d want %d", len(args), len(tt.wantArgs))
			}
			for i := range tt.wantArgs {
				if args[i] != tt.wantArgs[i] {
					t.Fatalf("arg %d mismatch: got %v want %v", i, args[i], tt.wantArgs[i])
				}
			}
		})
	}
}

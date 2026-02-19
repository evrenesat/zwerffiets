package main

import (
	"strings"
	"testing"
)

func TestBuildOperatorReportsQuery(t *testing.T) {
	tests := []struct {
		name     string
		filters  map[string]any
		page     int
		pageSize int
		wantSql  []string // Substrings expected in SQL
		wantArgs []any    // Expected args in order (approximate check)
	}{
		{
			name:     "Default",
			filters:  map[string]any{},
			page:     1,
			pageSize: 10,
			wantSql: []string{
				"SELECT",
				"FROM reports",
				"JOIN bike_groups bg",
				"ORDER BY reports.created_at DESC",
				"LIMIT $1 OFFSET $2",
			},
			wantArgs: []any{10, 0},
		},
		{
			name:     "Status Filter",
			filters:  map[string]any{"status": "new"},
			page:     1,
			pageSize: 10,
			wantSql: []string{
				"AND reports.status = $1",
				"LIMIT $2 OFFSET $3",
			},
			wantArgs: []any{"new", 10, 0},
		},
		{
			name:     "City Filter",
			filters:  map[string]any{"city": "Amsterdam"},
			page:     1,
			pageSize: 10,
			wantSql: []string{
				"AND LOWER(reports.municipality) = LOWER($1)",
				"LIMIT $2 OFFSET $3",
			},
			wantArgs: []any{"Amsterdam", 10, 0},
		},
		{
			name:     "Report City Filter",
			filters:  map[string]any{"report_city": "IJmuiden"},
			page:     1,
			pageSize: 10,
			wantSql: []string{
				"AND LOWER(reports.city) = LOWER($1)",
				"LIMIT $2 OFFSET $3",
			},
			wantArgs: []any{"IJmuiden", 10, 0},
		},
		{
			name:     "Sort Signal",
			filters:  map[string]any{"sort": "signal"},
			page:     1,
			pageSize: 10,
			wantSql: []string{
				"CASE bg.signal_strength",
				"WHEN 'strong_distinct_reporters' THEN 2",
				"LIMIT $1 OFFSET $2",
			},
			wantArgs: []any{10, 0},
		},
		{
			name:     "Pagination Page 2",
			filters:  map[string]any{},
			page:     2,
			pageSize: 20,
			wantSql: []string{
				"LIMIT $1 OFFSET $2",
			},
			wantArgs: []any{20, 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSql, gotArgs := buildOperatorReportsQuery(tt.filters, tt.page, tt.pageSize)

			for _, substr := range tt.wantSql {
				if !strings.Contains(gotSql, substr) {
					t.Errorf("SQL missing substring %q\nGot: %s", substr, gotSql)
				}
			}

			if len(gotArgs) != len(tt.wantArgs) {
				t.Errorf("Args length mismatch. Got %d, want %d", len(gotArgs), len(tt.wantArgs))
			}
			// Basic args check
			for i, arg := range tt.wantArgs {
				if i < len(gotArgs) && gotArgs[i] != arg {
					t.Errorf("Arg %d mismatch. Got %v, want %v", i, gotArgs[i], arg)
				}
			}
		})
	}
}

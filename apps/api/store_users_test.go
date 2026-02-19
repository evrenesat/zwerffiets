package main

import (
	"strings"
	"testing"
)

func TestBuildPaginatedUsersQuery(t *testing.T) {
	tests := []struct {
		name     string
		filters  map[string]any
		page     int
		pageSize int
		wantSQL  []string
		wantArgs []any
	}{
		{
			name:     "default",
			filters:  map[string]any{},
			page:     1,
			pageSize: 50,
			wantSQL: []string{
				"COUNT(*) OVER() as total_count",
				"ORDER BY created_at DESC",
				"LIMIT $1 OFFSET $2",
			},
			wantArgs: []any{50, 0},
		},
		{
			name: "with search and status",
			filters: map[string]any{
				"q":      "bob",
				"status": "inactive",
			},
			page:     2,
			pageSize: 20,
			wantSQL: []string{
				"email ILIKE $1",
				"is_active = $2",
				"LIMIT $3 OFFSET $4",
			},
			wantArgs: []any{"%bob%", false, 20, 20},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSQL, gotArgs := buildPaginatedUsersQuery(tt.filters, tt.page, tt.pageSize)
			for _, piece := range tt.wantSQL {
				if !strings.Contains(gotSQL, piece) {
					t.Fatalf("expected SQL to contain %q, got: %s", piece, gotSQL)
				}
			}
			if len(gotArgs) != len(tt.wantArgs) {
				t.Fatalf("unexpected arg count: got %d want %d", len(gotArgs), len(tt.wantArgs))
			}
			for i := range tt.wantArgs {
				if gotArgs[i] != tt.wantArgs[i] {
					t.Fatalf("arg %d mismatch: got %#v want %#v", i, gotArgs[i], tt.wantArgs[i])
				}
			}
		})
	}
}

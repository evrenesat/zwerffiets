package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

type PaginatedUsers struct {
	Users       []User
	TotalCount  int
	TotalPages  int
	CurrentPage int
	PageSize    int
}

func (a *App) storeListUsers(ctx context.Context, filters map[string]any) ([]User, error) {
	query := `SELECT id, email, display_name, is_active, created_at, updated_at FROM users`
	whereClause, args := buildUsersWhereClause(filters)

	if whereClause != "" {
		query += " WHERE " + whereClause
	}

	query += " ORDER BY created_at DESC"

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

func (a *App) storeListUsersPaginated(ctx context.Context, filters map[string]any, page, pageSize int) (*PaginatedUsers, error) {
	if page < adminDefaultPage {
		page = adminDefaultPage
	}
	if pageSize < 1 {
		pageSize = adminDefaultPerPage
	}

	query, args := buildPaginatedUsersQuery(filters, page, pageSize)
	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	users := []User{}
	totalCount := 0

	for rows.Next() {
		var u User
		if err := rows.Scan(&u.ID, &u.Email, &u.DisplayName, &u.IsActive, &u.CreatedAt, &u.UpdatedAt, &totalCount); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	totalPages := 0
	if totalCount > 0 {
		totalPages = (totalCount + pageSize - 1) / pageSize
	}

	return &PaginatedUsers{
		Users:       users,
		TotalCount:  totalCount,
		TotalPages:  totalPages,
		CurrentPage: page,
		PageSize:    pageSize,
	}, nil
}

func buildPaginatedUsersQuery(filters map[string]any, page, pageSize int) (string, []any) {
	query := `
		SELECT
			id, email, display_name, is_active, created_at, updated_at,
			COUNT(*) OVER() as total_count
		FROM users
	`
	whereClause, args := buildUsersWhereClause(filters)
	if whereClause != "" {
		query += " WHERE " + whereClause
	}
	query += " ORDER BY created_at DESC"

	offset := (page - 1) * pageSize
	query += fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)+1, len(args)+2)
	args = append(args, pageSize, offset)

	return query, args
}

func buildUsersWhereClause(filters map[string]any) (string, []any) {
	where := []string{}
	args := []any{}

	if q, ok := filters["q"].(string); ok && q != "" {
		where = append(where, fmt.Sprintf("email ILIKE $%d", len(args)+1))
		args = append(args, "%"+q+"%")
	}

	if status, ok := filters["status"].(string); ok && status != "" {
		isActive := status == "active"
		where = append(where, fmt.Sprintf("is_active = $%d", len(args)+1))
		args = append(args, isActive)
	}

	return strings.Join(where, " AND "), args
}

func (a *App) storeGetUserByID(ctx context.Context, id int) (*User, error) {
	var u User
	err := a.db.QueryRowContext(ctx, `
		SELECT id, email, display_name, is_active, created_at, updated_at 
		FROM users WHERE id = $1
	`, id).Scan(&u.ID, &u.Email, &u.DisplayName, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, err
	}
	return &u, nil
}

func (a *App) storeUpdateUser(ctx context.Context, id int, email string, displayName *string, isActive bool) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE users 
		SET email = $1, display_name = $2, is_active = $3, updated_at = NOW() 
		WHERE id = $4
	`, email, displayName, isActive, id)
	return err
}

func (a *App) storeDeleteUser(ctx context.Context, id int) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Unlink reports
	if _, err := tx.ExecContext(ctx, `UPDATE reports SET user_id = NULL WHERE user_id = $1`, id); err != nil {
		return err
	}

	// Delete user (magic_link_tokens will cascade)
	if _, err := tx.ExecContext(ctx, `DELETE FROM users WHERE id = $1`, id); err != nil {
		return err
	}

	return tx.Commit()
}

func (a *App) storeBulkUpdateUserStatus(ctx context.Context, ids []int, isActive bool) error {
	if len(ids) == 0 {
		return nil
	}

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids)+1)
	args[0] = isActive
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+2)
		args[i+1] = id
	}

	query := fmt.Sprintf("UPDATE users SET is_active = $1, updated_at = NOW() WHERE id IN (%s)", strings.Join(placeholders, ","))
	_, err := a.db.ExecContext(ctx, query, args...)
	return err
}

func (a *App) storeBulkDeleteUsers(ctx context.Context, ids []int) error {
	if len(ids) == 0 {
		return nil
	}

	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	placeholders := make([]string, len(ids))
	args := make([]any, len(ids))
	for i, id := range ids {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
		args[i] = id
	}

	inClause := strings.Join(placeholders, ",")

	// Unlink reports
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("UPDATE reports SET user_id = NULL WHERE user_id IN (%s)", inClause), args...); err != nil {
		return err
	}

	// Delete users
	if _, err := tx.ExecContext(ctx, fmt.Sprintf("DELETE FROM users WHERE id IN (%s)", inClause), args...); err != nil {
		return err
	}

	return tx.Commit()
}

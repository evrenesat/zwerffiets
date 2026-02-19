package main

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

type Operator struct {
	ID                   int     `json:"id"`
	Email                string  `json:"email"`
	Role                 string  `json:"role"`
	Municipality         *string `json:"municipality"`
	IsActive             bool    `json:"is_active"`
	ReceivesReports      bool    `json:"receives_reports"`
	UnsubscribeRequested bool    `json:"unsubscribe_requested"`
	CreatedAt            string  `json:"created_at"`
	UpdatedAt            string  `json:"updated_at"`
}

func (a *App) storeAdminListOperators(ctx context.Context) ([]Operator, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, email, role, municipality, is_active, receives_reports, unsubscribe_requested, created_at, updated_at
		FROM operators
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var operators []Operator
	for rows.Next() {
		var op Operator
		var createdAt, updatedAt time.Time
		var mun sql.NullString
		if err := rows.Scan(
			&op.ID,
			&op.Email,
			&op.Role,
			&mun,
			&op.IsActive,
			&op.ReceivesReports,
			&op.UnsubscribeRequested,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		if mun.Valid {
			val := mun.String
			op.Municipality = &val
		}
		op.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		op.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		operators = append(operators, op)
	}
	return operators, rows.Err()
}

func (a *App) storeAdminCreateOperator(ctx context.Context, email, password string, municipality *string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	role := "municipality_operator"
	if municipality == nil || strings.TrimSpace(*municipality) == "" {
		return fmt.Errorf("municipality is required")
	}

	_, err = a.db.ExecContext(ctx, `
		INSERT INTO operators (email, password_hash, role, municipality, is_active)
		VALUES ($1, $2, $3, $4, true)
	`, email, string(hash), role, municipality)
	return err
}

func (a *App) storeAdminToggleOperatorStatus(ctx context.Context, id int) (bool, error) {
	var isActive bool
	err := a.db.QueryRowContext(ctx, `
		UPDATE operators
		SET is_active = NOT is_active, updated_at = NOW()
		WHERE id = $1
		RETURNING is_active
	`, id).Scan(&isActive)
	return isActive, err
}

func (a *App) storeAdminUpdateOperator(ctx context.Context, id int, email, role string, municipality *string, password string) error {
	if password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return err
		}
		_, err = a.db.ExecContext(ctx, `
			UPDATE operators
			SET email = $2, role = $3, municipality = $4, password_hash = $5, updated_at = NOW()
			WHERE id = $1
		`, id, email, role, municipality, string(hash))
		return err
	}
	_, err := a.db.ExecContext(ctx, `
		UPDATE operators
		SET email = $2, role = $3, municipality = $4, updated_at = NOW()
		WHERE id = $1
	`, id, email, role, municipality)
	return err
}

func (a *App) storeAdminGetOperatorByID(ctx context.Context, id int) (*Operator, error) {
	var op Operator
	var createdAt, updatedAt time.Time
	var mun sql.NullString
	err := a.db.QueryRowContext(ctx, `
		SELECT id, email, role, municipality, is_active, receives_reports, unsubscribe_requested, created_at, updated_at
		FROM operators WHERE id = $1
	`, id).Scan(
		&op.ID,
		&op.Email,
		&op.Role,
		&mun,
		&op.IsActive,
		&op.ReceivesReports,
		&op.UnsubscribeRequested,
		&createdAt,
		&updatedAt,
	)
	if err != nil {
		return nil, err
	}
	if mun.Valid {
		val := mun.String
		op.Municipality = &val
	}
	op.CreatedAt = createdAt.UTC().Format(time.RFC3339)
	op.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
	return &op, nil
}

func (a *App) storeListMunicipalitiesWithReports(ctx context.Context) ([]string, error) {
	rows, err := a.db.QueryContext(ctx, `SELECT DISTINCT municipality FROM reports WHERE municipality IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var municipalities []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		municipalities = append(municipalities, m)
	}
	return municipalities, nil
}

func (a *App) storeListReportCities(ctx context.Context, municipality *string) ([]string, error) {
	query := `SELECT DISTINCT BTRIM(city) AS city FROM reports WHERE city IS NOT NULL AND BTRIM(city) <> ''`
	args := []any{}
	if municipality != nil && strings.TrimSpace(*municipality) != "" {
		query += ` AND LOWER(municipality) = LOWER($1)`
		args = append(args, *municipality)
	}
	query += ` ORDER BY city`

	rows, err := a.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cities := make([]string, 0)
	for rows.Next() {
		var city string
		if err := rows.Scan(&city); err != nil {
			return nil, err
		}
		cities = append(cities, city)
	}
	return cities, rows.Err()
}

func (a *App) storeHasReportRecipientForMunicipality(ctx context.Context, municipality string) (bool, error) {
	var exists bool
	err := a.db.QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM operators 
			WHERE municipality = $1 AND receives_reports = true AND is_active = true
		)
	`, municipality).Scan(&exists)
	return exists, err
}

func (a *App) storeCreateSeedOperator(ctx context.Context, municipality string) error {
	slug := strings.ToLower(strings.ReplaceAll(municipality, " ", "-"))
	email := fmt.Sprintf("gemeente-%s@zwerffiets.org", slug)
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO operators (email, role, municipality, is_active, receives_reports)
		VALUES ($1, 'municipality_operator', $2, true, true)
	`, email, municipality)
	return err
}

func (a *App) storeListReportRecipientOperators(ctx context.Context) ([]Operator, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT id, email, role, municipality, is_active, receives_reports, unsubscribe_requested, created_at, updated_at
		FROM operators
		WHERE receives_reports = true AND is_active = true AND email NOT LIKE 'gemeente-%'
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var operators []Operator
	for rows.Next() {
		var op Operator
		var createdAt, updatedAt time.Time
		var mun sql.NullString
		if err := rows.Scan(
			&op.ID,
			&op.Email,
			&op.Role,
			&mun,
			&op.IsActive,
			&op.ReceivesReports,
			&op.UnsubscribeRequested,
			&createdAt,
			&updatedAt,
		); err != nil {
			return nil, err
		}
		if mun.Valid {
			val := mun.String
			op.Municipality = &val
		}
		op.CreatedAt = createdAt.UTC().Format(time.RFC3339)
		op.UpdatedAt = updatedAt.UTC().Format(time.RFC3339)
		operators = append(operators, op)
	}
	return operators, rows.Err()
}

func (a *App) storeCountTriagedReportsByMunicipality(ctx context.Context, municipality string) (int, error) {
	var count int
	err := a.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM reports 
		WHERE LOWER(municipality) = LOWER($1) AND status = 'triaged'
	`, municipality).Scan(&count)
	return count, err
}

func (a *App) storeSetUnsubscribeRequested(ctx context.Context, operatorID int) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE operators SET unsubscribe_requested = true, updated_at = NOW() WHERE id = $1
	`, operatorID)
	return err
}

func (a *App) storeCreateOperatorMagicLinkToken(ctx context.Context, operatorID int, tokenHash string, expiresAt time.Time) error {
	_, err := a.db.ExecContext(ctx, `
		INSERT INTO operator_magic_link_tokens (operator_id, token_hash, expires_at)
		VALUES ($1, $2, $3)
	`, operatorID, tokenHash, expiresAt)
	return err
}

func (a *App) storeVerifyOperatorMagicLinkToken(ctx context.Context, tokenHash string) (int, error) {
	var operatorID int
	err := a.db.QueryRowContext(ctx, `
		UPDATE operator_magic_link_tokens
		SET used_at = NOW()
		WHERE token_hash = $1 AND used_at IS NULL AND expires_at > NOW()
		RETURNING operator_id
	`, tokenHash).Scan(&operatorID)
	if err != nil {
		if err == sql.ErrNoRows {
			return 0, fmt.Errorf("invalid or expired token")
		}
		return 0, err
	}
	return operatorID, nil
}

func (a *App) storeToggleReceivesReports(ctx context.Context, id int) (bool, error) {
	var newValue bool
	err := a.db.QueryRowContext(ctx, `
		UPDATE operators
		SET receives_reports = NOT receives_reports, updated_at = NOW()
		WHERE id = $1
		RETURNING receives_reports
	`, id).Scan(&newValue)
	return newValue, err
}

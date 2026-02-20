package main

import (
	"context"
	"time"
)

type ShowcaseItem struct {
	Slot          int
	ReportPhotoID int
	Subtitle      string
	FocalX        int
	FocalY        int
	ScalePercent  int
	CreatedAt     time.Time
	UpdatedAt     time.Time
	StoragePath   string
}

func (a *App) storeGetShowcaseItems(ctx context.Context) ([]ShowcaseItem, error) {
	query := `
		SELECT s.slot, s.report_photo_id, s.subtitle, s.focal_x, s.focal_y, s.scale_percent, s.created_at, s.updated_at, p.storage_path
		FROM showcase_items s
		JOIN report_photos p ON s.report_photo_id = p.id
		ORDER BY s.slot ASC
	`
	rows, err := a.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []ShowcaseItem
	for rows.Next() {
		var item ShowcaseItem
		if err := rows.Scan(
			&item.Slot, &item.ReportPhotoID, &item.Subtitle,
			&item.FocalX, &item.FocalY, &item.ScalePercent, &item.CreatedAt, &item.UpdatedAt,
			&item.StoragePath,
		); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (a *App) storeUpsertShowcaseItem(ctx context.Context, slot, reportPhotoID, focalX, focalY, scalePercent int, subtitle string) error {
	query := `
		INSERT INTO showcase_items (slot, report_photo_id, subtitle, focal_x, focal_y, scale_percent, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, NOW())
		ON CONFLICT (slot) DO UPDATE SET
			report_photo_id = EXCLUDED.report_photo_id,
			subtitle = EXCLUDED.subtitle,
			focal_x = EXCLUDED.focal_x,
			focal_y = EXCLUDED.focal_y,
			scale_percent = EXCLUDED.scale_percent,
			updated_at = NOW();
	`
	_, err := a.db.ExecContext(ctx, query, slot, reportPhotoID, subtitle, focalX, focalY, scalePercent)
	return err
}

package main

import (
	"context"
	"database/sql"
	"sort"
	"sync"
	"time"
)

type SiteContent struct {
	Key       string    `json:"key"`
	NlText    string    `json:"nl_text"`
	EnText    string    `json:"en_text"`
	UpdatedAt time.Time `json:"updated_at"`
	UpdatedBy string    `json:"updated_by"`
}

var (
	contentCacheMu sync.RWMutex
	contentCache   map[string]SiteContent
)

// InitContentCache preloads all dynamic content from the database.
func InitContentCache(ctx context.Context, db *sql.DB) error {
	rows, err := db.QueryContext(ctx, "SELECT key, nl_text, en_text, updated_at, updated_by FROM site_contents")
	if err != nil {
		return err
	}
	defer rows.Close()

	newCache := make(map[string]SiteContent)
	for rows.Next() {
		var c SiteContent
		if err := rows.Scan(&c.Key, &c.NlText, &c.EnText, &c.UpdatedAt, &c.UpdatedBy); err != nil {
			return err
		}
		newCache[c.Key] = c
	}

	if err := rows.Err(); err != nil {
		return err
	}

	contentCacheMu.Lock()
	contentCache = newCache
	contentCacheMu.Unlock()

	return nil
}

// GetContentCache returns a copy of the currently cached content maps for frontends to consume.
// It structures the data as { "nl": { "key": "value" }, "en": { "key": "value" } }.
func GetContentCache() map[string]map[string]string {
	contentCacheMu.RLock()
	defer contentCacheMu.RUnlock()

	res := map[string]map[string]string{
		"nl": make(map[string]string),
		"en": make(map[string]string),
	}

	for key, content := range contentCache {
		if content.NlText != "" {
			res["nl"][key] = content.NlText
		}
		if content.EnText != "" {
			res["en"][key] = content.EnText
		}
	}

	return res
}

// GetAllSiteContents returns the raw struct list, typically used by the admin UI.
func GetAllSiteContents() []SiteContent {
	contentCacheMu.RLock()
	defer contentCacheMu.RUnlock()

	var list []SiteContent
	for _, c := range contentCache {
		list = append(list, c)
	}

	sort.Slice(list, func(i, j int) bool {
		return list[i].Key < list[j].Key
	})

	return list
}

// SaveSiteContent updates the database and immediately updates the cache on success.
func SaveSiteContent(ctx context.Context, db *sql.DB, content SiteContent) error {
	now := time.Now()
	_, err := db.ExecContext(ctx, `
		INSERT INTO site_contents (key, nl_text, en_text, updated_at, updated_by)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (key) DO UPDATE SET
			nl_text = EXCLUDED.nl_text,
			en_text = EXCLUDED.en_text,
			updated_at = EXCLUDED.updated_at,
			updated_by = EXCLUDED.updated_by
	`, content.Key, content.NlText, content.EnText, now, content.UpdatedBy)

	if err != nil {
		return err
	}

	// Update the cache immediately so it's globally visible.
	content.UpdatedAt = now
	contentCacheMu.Lock()
	if contentCache == nil {
		contentCache = make(map[string]SiteContent)
	}
	contentCache[content.Key] = content
	contentCacheMu.Unlock()

	return nil
}

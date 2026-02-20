package main

import (
	"context"
	"time"
)

type BlogPost struct {
	ID          int        `json:"id"`
	Slug        string     `json:"slug"`
	Title       string     `json:"title"`
	ContentHTML string     `json:"content_html"`
	AuthorID    *int       `json:"author_id"`
	AuthorName  string     `json:"author_name"`
	IsPublished bool       `json:"is_published"`
	PublishedAt *time.Time `json:"published_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

type BlogMedia struct {
	ID          int       `json:"id"`
	Filename    string    `json:"filename"`
	StoragePath string    `json:"storage_path"`
	MimeType    string    `json:"mime_type"`
	SizeBytes   int64     `json:"size_bytes"`
	CreatedAt   time.Time `json:"created_at"`
}

func (a *App) storeAdminListBlogPosts(ctx context.Context) ([]BlogPost, error) {
	rows, err := a.db.QueryContext(ctx, `
		SELECT b.id, b.slug, b.title, b.content_html, b.author_id, COALESCE(o.name, 'Deleted Operator'), 
		       b.is_published, b.published_at, b.created_at, b.updated_at
		FROM blog_posts b
		LEFT JOIN operators o ON b.author_id = o.id
		ORDER BY b.created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []BlogPost
	for rows.Next() {
		var p BlogPost
		if err := rows.Scan(
			&p.ID, &p.Slug, &p.Title, &p.ContentHTML, &p.AuthorID, &p.AuthorName,
			&p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, rows.Err()
}

func (a *App) storeAdminGetBlogPost(ctx context.Context, id int) (*BlogPost, error) {
	var p BlogPost
	err := a.db.QueryRowContext(ctx, `
		SELECT b.id, b.slug, b.title, b.content_html, b.author_id, COALESCE(o.name, 'Deleted Operator'), 
		       b.is_published, b.published_at, b.created_at, b.updated_at
		FROM blog_posts b
		LEFT JOIN operators o ON b.author_id = o.id
		WHERE b.id = $1
	`, id).Scan(
		&p.ID, &p.Slug, &p.Title, &p.ContentHTML, &p.AuthorID, &p.AuthorName,
		&p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (a *App) storeAdminCreateBlogPost(ctx context.Context, p *BlogPost) error {
	return a.db.QueryRowContext(ctx, `
		INSERT INTO blog_posts (slug, title, content_html, author_id, is_published, published_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at, updated_at
	`, p.Slug, p.Title, p.ContentHTML, p.AuthorID, p.IsPublished, p.PublishedAt).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
}

func (a *App) storeAdminUpdateBlogPost(ctx context.Context, p *BlogPost) error {
	_, err := a.db.ExecContext(ctx, `
		UPDATE blog_posts
		SET slug = $2, title = $3, content_html = $4, is_published = $5, published_at = $6, updated_at = NOW()
		WHERE id = $1
	`, p.ID, p.Slug, p.Title, p.ContentHTML, p.IsPublished, p.PublishedAt)
	return err
}

func (a *App) storeAdminDeleteBlogPost(ctx context.Context, id int) error {
	_, err := a.db.ExecContext(ctx, `DELETE FROM blog_posts WHERE id = $1`, id)
	return err
}

func (a *App) storePublicListBlogPosts(ctx context.Context, limit, offset int) ([]BlogPost, int, error) {
	var total int
	err := a.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM blog_posts WHERE is_published = TRUE`).Scan(&total)
	if err != nil {
		return nil, 0, err
	}

	rows, err := a.db.QueryContext(ctx, `
		SELECT b.id, b.slug, b.title, b.content_html, b.author_id, COALESCE(o.name, 'Deleted Operator'), 
		       b.is_published, b.published_at, b.created_at, b.updated_at
		FROM blog_posts b
		LEFT JOIN operators o ON b.author_id = o.id
		WHERE b.is_published = TRUE
		ORDER BY b.published_at DESC, b.created_at DESC
		LIMIT $1 OFFSET $2
	`, limit, offset)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()

	var posts []BlogPost
	for rows.Next() {
		var p BlogPost
		if err := rows.Scan(
			&p.ID, &p.Slug, &p.Title, &p.ContentHTML, &p.AuthorID, &p.AuthorName,
			&p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, 0, err
		}
		posts = append(posts, p)
	}
	return posts, total, rows.Err()
}

func (a *App) storePublicGetBlogPostBySlug(ctx context.Context, slug string) (*BlogPost, error) {
	var p BlogPost
	err := a.db.QueryRowContext(ctx, `
		SELECT b.id, b.slug, b.title, b.content_html, b.author_id, COALESCE(o.name, 'Deleted Operator'), 
		       b.is_published, b.published_at, b.created_at, b.updated_at
		FROM blog_posts b
		LEFT JOIN operators o ON b.author_id = o.id
		WHERE b.slug = $1 AND b.is_published = TRUE
	`, slug).Scan(
		&p.ID, &p.Slug, &p.Title, &p.ContentHTML, &p.AuthorID, &p.AuthorName,
		&p.IsPublished, &p.PublishedAt, &p.CreatedAt, &p.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &p, nil
}

func (a *App) storeSaveBlogMedia(ctx context.Context, m *BlogMedia) error {
	return a.db.QueryRowContext(ctx, `
		INSERT INTO blog_media (filename, storage_path, mime_type, size_bytes)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (filename) DO UPDATE SET updated_at = NOW() -- Should not happen if named uniquely
		RETURNING id, created_at
	`, m.Filename, m.StoragePath, m.MimeType, m.SizeBytes).Scan(&m.ID, &m.CreatedAt)
}

func (a *App) storeGetBlogMedia(ctx context.Context, filename string) (*BlogMedia, error) {
	var m BlogMedia
	err := a.db.QueryRowContext(ctx, `
		SELECT id, filename, storage_path, mime_type, size_bytes, created_at
		FROM blog_media
		WHERE filename = $1
	`, filename).Scan(&m.ID, &m.Filename, &m.StoragePath, &m.MimeType, &m.SizeBytes, &m.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &m, nil
}

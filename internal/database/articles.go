package database

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// CreateArticle inserts a new article into the database
func (db *DB) CreateArticle(ctx context.Context, article *Article) error {
	query := `
		INSERT INTO articles (source, url, title, author, published_at, content_html, content_text)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at
	`

	err := db.pool.QueryRow(ctx, query,
		article.Source,
		article.URL,
		article.Title,
		article.Author,
		article.PublishedAt,
		article.ContentHTML,
		article.ContentText,
	).Scan(&article.ID, &article.CreatedAt, &article.UpdatedAt)

	if err != nil {
		return fmt.Errorf("failed to create article: %w", err)
	}

	return nil
}

// GetArticleByID retrieves an article by its ID
func (db *DB) GetArticleByID(ctx context.Context, id int) (*Article, error) {
	query := `
		SELECT id, source, url, title, author, published_at, content_html, content_text, created_at, updated_at
		FROM articles
		WHERE id = $1
	`

	var article Article
	err := db.pool.QueryRow(ctx, query, id).Scan(
		&article.ID,
		&article.Source,
		&article.URL,
		&article.Title,
		&article.Author,
		&article.PublishedAt,
		&article.ContentHTML,
		&article.ContentText,
		&article.CreatedAt,
		&article.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, fmt.Errorf("article not found")
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get article: %w", err)
	}

	return &article, nil
}

// GetArticleByURL retrieves an article by its URL (for deduplication)
func (db *DB) GetArticleByURL(ctx context.Context, url string) (*Article, error) {
	query := `
		SELECT id, source, url, title, author, published_at, content_html, content_text, created_at, updated_at
		FROM articles
		WHERE url = $1
	`

	var article Article
	err := db.pool.QueryRow(ctx, query, url).Scan(
		&article.ID,
		&article.Source,
		&article.URL,
		&article.Title,
		&article.Author,
		&article.PublishedAt,
		&article.ContentHTML,
		&article.ContentText,
		&article.CreatedAt,
		&article.UpdatedAt,
	)

	if err == pgx.ErrNoRows {
		return nil, nil // Return nil if not found (not an error for deduplication check)
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get article by URL: %w", err)
	}

	return &article, nil
}

// GetAllArticles retrieves all articles, ordered by most recent first
func (db *DB) GetAllArticles(ctx context.Context, limit int) ([]*Article, error) {
	query := `
		SELECT id, source, url, title, author, published_at, content_html, content_text, created_at, updated_at
		FROM articles
		ORDER BY created_at DESC
		LIMIT $1
	`

	rows, err := db.pool.Query(ctx, query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query articles: %w", err)
	}
	defer rows.Close()

	var articles []*Article
	for rows.Next() {
		var article Article
		err := rows.Scan(
			&article.ID,
			&article.Source,
			&article.URL,
			&article.Title,
			&article.Author,
			&article.PublishedAt,
			&article.ContentHTML,
			&article.ContentText,
			&article.CreatedAt,
			&article.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan article: %w", err)
		}
		articles = append(articles, &article)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating articles: %w", err)
	}

	return articles, nil
}

// GetRecentArticles retrieves articles published within a time range
func (db *DB) GetRecentArticles(ctx context.Context, since time.Time, limit int) ([]*Article, error) {
	query := `
		SELECT id, source, url, title, author, published_at, content_html, content_text, created_at, updated_at
		FROM articles
		WHERE published_at >= $1
		ORDER BY published_at DESC
		LIMIT $2
	`

	rows, err := db.pool.Query(ctx, query, since, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query recent articles: %w", err)
	}
	defer rows.Close()

	var articles []*Article
	for rows.Next() {
		var article Article
		err := rows.Scan(
			&article.ID,
			&article.Source,
			&article.URL,
			&article.Title,
			&article.Author,
			&article.PublishedAt,
			&article.ContentHTML,
			&article.ContentText,
			&article.CreatedAt,
			&article.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan article: %w", err)
		}
		articles = append(articles, &article)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating recent articles: %w", err)
	}

	return articles, nil
}

// ArticleExists checks if an article with the given URL already exists
func (db *DB) ArticleExists(ctx context.Context, url string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM articles WHERE url = $1)`

	var exists bool
	err := db.pool.QueryRow(ctx, query, url).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check article existence: %w", err)
	}

	return exists, nil
}

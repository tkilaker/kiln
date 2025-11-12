package database

import "time"

// Article represents a scraped article from Gasetten or other sources
type Article struct {
	ID          int       `db:"id"`
	Source      string    `db:"source"`
	URL         string    `db:"url"`
	Title       *string   `db:"title"`
	Author      *string   `db:"author"`
	PublishedAt *time.Time `db:"published_at"`
	ContentHTML *string   `db:"content_html"`
	ContentText *string   `db:"content_text"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

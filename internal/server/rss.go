package server

import (
	"fmt"
	"time"

	"github.com/gorilla/feeds"
	"github.com/tim/kiln/internal/config"
	"github.com/tim/kiln/internal/database"
)

// GenerateRSSFeed creates an RSS feed from articles
func GenerateRSSFeed(articles []*database.Article, cfg *config.Config) (string, error) {
	now := time.Now()

	feed := &feeds.Feed{
		Title:       cfg.FeedTitle,
		Link:        &feeds.Link{Href: cfg.FeedLink},
		Description: cfg.FeedDescription,
		Author:      &feeds.Author{Name: cfg.FeedAuthor},
		Created:     now,
	}

	// Convert articles to feed items
	feed.Items = make([]*feeds.Item, 0, len(articles))
	for _, article := range articles {
		item := &feeds.Item{
			Title: getArticleTitle(article),
			Link:  &feeds.Link{Href: article.URL},
			Id:    fmt.Sprintf("%s/articles/%d", cfg.FeedLink, article.ID),
		}

		// Set description from content
		if article.ContentText != nil {
			// Truncate to reasonable length for RSS
			description := *article.ContentText
			if len(description) > 500 {
				description = description[:500] + "..."
			}
			item.Description = description
		} else if article.ContentHTML != nil {
			item.Description = *article.ContentHTML
		}

		// Set author
		if article.Author != nil {
			item.Author = &feeds.Author{Name: *article.Author}
		}

		// Set published date
		if article.PublishedAt != nil {
			item.Created = *article.PublishedAt
		} else {
			item.Created = article.CreatedAt
		}

		feed.Items = append(feed.Items, item)
	}

	// Generate RSS 2.0 format
	rss, err := feed.ToRss()
	if err != nil {
		return "", fmt.Errorf("failed to generate RSS: %w", err)
	}

	return rss, nil
}

func getArticleTitle(article *database.Article) string {
	if article.Title != nil {
		return *article.Title
	}
	return "Untitled Article"
}

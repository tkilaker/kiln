package server

import (
	"encoding/xml"
	"fmt"
	"time"

	"github.com/gorilla/feeds"
	"github.com/tkilaker/kiln/internal/config"
	"github.com/tkilaker/kiln/internal/database"
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

// Podcast RSS XML structures with iTunes namespace for Pocket Casts compatibility

type podcastRSS struct {
	XMLName xml.Name       `xml:"rss"`
	Version string         `xml:"version,attr"`
	ITunes  string         `xml:"xmlns:itunes,attr"`
	Channel podcastChannel `xml:"channel"`
}

type podcastChannel struct {
	Title          string              `xml:"title"`
	Link           string              `xml:"link"`
	Description    string              `xml:"description"`
	Language       string              `xml:"language"`
	LastBuildDate  string              `xml:"lastBuildDate"`
	ITunesAuthor   string              `xml:"itunes:author"`
	ITunesSummary  string              `xml:"itunes:summary"`
	ITunesExplicit string              `xml:"itunes:explicit"`
	ITunesType     string              `xml:"itunes:type"`
	ITunesImage    *podcastITunesImage `xml:"itunes:image,omitempty"`
	ITunesCategory podcastCategory     `xml:"itunes:category"`
	Items          []podcastItem       `xml:"item"`
}

type podcastITunesImage struct {
	Href string `xml:"href,attr"`
}

type podcastCategory struct {
	Text string `xml:"text,attr"`
}

type podcastItem struct {
	Title          string           `xml:"title"`
	Link           string           `xml:"link"`
	GUID           podcastGUID      `xml:"guid"`
	Description    string           `xml:"description"`
	Author         string           `xml:"itunes:author,omitempty"`
	PubDate        string           `xml:"pubDate"`
	Enclosure      podcastEnclosure `xml:"enclosure"`
	ITunesDuration string           `xml:"itunes:duration,omitempty"`
	ITunesExplicit string           `xml:"itunes:explicit"`
}

type podcastGUID struct {
	IsPermaLink string `xml:"isPermaLink,attr"`
	Value       string `xml:",chardata"`
}

type podcastEnclosure struct {
	URL    string `xml:"url,attr"`
	Length string `xml:"length,attr"`
	Type   string `xml:"type,attr"`
}

// GeneratePodcastFeed creates a podcast-compatible RSS feed with iTunes namespace
// tags for Pocket Casts and other podcast apps.
func GeneratePodcastFeed(articles []*database.Article, audioMap map[int]*database.AudioFile, cfg *config.Config) (string, error) {
	now := time.Now()

	channel := podcastChannel{
		Title:          cfg.FeedTitle + " (Podcast)",
		Link:           cfg.FeedLink + "/podcast.xml",
		Description:    cfg.FeedDescription + " - Audio versions of articles",
		Language:       cfg.PodcastLanguage,
		LastBuildDate:  now.Format(time.RFC1123Z),
		ITunesAuthor:   cfg.FeedAuthor,
		ITunesSummary:  cfg.FeedDescription + " - Audio versions of articles",
		ITunesExplicit: "false",
		ITunesType:     "episodic",
		ITunesCategory: podcastCategory{Text: "News"},
	}

	if cfg.PodcastImageURL != "" {
		channel.ITunesImage = &podcastITunesImage{Href: cfg.PodcastImageURL}
	}

	// Only include articles that have completed audio
	for _, article := range articles {
		audio, hasAudio := audioMap[article.ID]
		if !hasAudio || audio.Status != "completed" {
			continue
		}

		description := ""
		if article.ContentText != nil {
			description = *article.ContentText
			if len(description) > 500 {
				description = description[:500] + "..."
			}
		}

		author := cfg.FeedAuthor
		if article.Author != nil {
			author = *article.Author
		}

		pubDate := article.CreatedAt
		if article.PublishedAt != nil {
			pubDate = *article.PublishedAt
		}

		item := podcastItem{
			Title:       getArticleTitle(article),
			Link:        fmt.Sprintf("%s/articles/%d", cfg.FeedLink, article.ID),
			Description: description,
			Author:      author,
			PubDate:     pubDate.Format(time.RFC1123Z),
			GUID: podcastGUID{
				IsPermaLink: "false",
				Value:       fmt.Sprintf("%s/articles/%d/audio", cfg.FeedLink, article.ID),
			},
			Enclosure: podcastEnclosure{
				URL:    fmt.Sprintf("%s/articles/%d/audio?voice=%s", cfg.FeedLink, article.ID, audio.Voice),
				Length: fmt.Sprintf("%d", audio.FileSize),
				Type:   "audio/mpeg",
			},
			ITunesExplicit: "false",
		}

		channel.Items = append(channel.Items, item)
	}

	rss := podcastRSS{
		Version: "2.0",
		ITunes:  "http://www.itunes.com/dtds/podcast-1.0.dtd",
		Channel: channel,
	}

	output, err := xml.MarshalIndent(rss, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to generate podcast RSS: %w", err)
	}

	return xml.Header + string(output), nil
}

func getArticleTitle(article *database.Article) string {
	if article.Title != nil {
		return *article.Title
	}
	return "Untitled Article"
}

package scraper

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/tim/kiln/internal/database"
)

// Scraper handles web scraping for Gasetten
type Scraper struct {
	username   string
	password   string
	db         *database.DB
	sessionDir string
	browser    *rod.Browser
}

// New creates a new scraper instance
func New(username, password string, db *database.DB) (*Scraper, error) {
	// Create session directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	sessionDir := filepath.Join(homeDir, ".gasetten", "sessions")
	if err := os.MkdirAll(sessionDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create session directory: %w", err)
	}

	return &Scraper{
		username:   username,
		password:   password,
		db:         db,
		sessionDir: sessionDir,
	}, nil
}

// initBrowser initializes the Rod browser instance
func (s *Scraper) initBrowser() error {
	if s.browser != nil {
		return nil // Already initialized
	}

	// Launch browser
	path, _ := launcher.LookPath()
	u := launcher.New().
		Bin(path).
		Headless(true).
		UserDataDir(s.sessionDir).
		MustLaunch()

	browser := rod.New().ControlURL(u).MustConnect()
	s.browser = browser

	return nil
}

// Close closes the browser and cleans up resources
func (s *Scraper) Close() error {
	if s.browser != nil {
		return s.browser.Close()
	}
	return nil
}

// Login logs into Gasetten using username and password
func (s *Scraper) Login(ctx context.Context) error {
	if err := s.initBrowser(); err != nil {
		return err
	}

	page := s.browser.MustPage("https://gasetten.se/login")
	defer page.Close()

	// Wait for page to load
	page.MustWaitLoad()

	// Check if already logged in by looking for logout link or user menu
	if s.isLoggedIn(page) {
		log.Println("Already logged in to Gasetten")
		return nil
	}

	log.Println("Logging into Gasetten...")

	// Find and fill username field (adjust selectors based on actual Gasetten login page)
	usernameField := page.MustElement(`input[name="username"], input[type="email"], input[name="email"]`)
	usernameField.MustInput(s.username)

	// Find and fill password field
	passwordField := page.MustElement(`input[type="password"]`)
	passwordField.MustInput(s.password)

	// Find and click login button
	loginButton := page.MustElement(`button[type="submit"], input[type="submit"]`)
	loginButton.MustClick()

	// Wait for navigation after login
	page.MustWaitLoad()

	// Verify login was successful
	if !s.isLoggedIn(page) {
		return fmt.Errorf("login failed - could not verify successful authentication")
	}

	log.Println("Successfully logged into Gasetten")
	return nil
}

// isLoggedIn checks if the current page shows signs of being logged in
func (s *Scraper) isLoggedIn(page *rod.Page) bool {
	// Try to find common elements that indicate logged-in state
	// Adjust these selectors based on actual Gasetten structure
	has, _, _ := page.Has(`a[href*="logout"], .user-menu, .account-menu`)
	return has
}

// ScrapeArticles fetches and stores articles from Gasetten
func (s *Scraper) ScrapeArticles(ctx context.Context) (int, error) {
	if err := s.initBrowser(); err != nil {
		return 0, err
	}

	// Ensure we're logged in
	if err := s.Login(ctx); err != nil {
		return 0, fmt.Errorf("failed to login: %w", err)
	}

	page := s.browser.MustPage("https://gasetten.se")
	defer page.Close()

	page.MustWaitLoad()

	// Find all article links on the page
	// Adjust selector based on actual Gasetten HTML structure
	articleLinks := s.extractArticleLinks(page)

	log.Printf("Found %d articles to scrape", len(articleLinks))

	scrapedCount := 0
	for i, link := range articleLinks {
		log.Printf("Scraping article %d/%d: %s", i+1, len(articleLinks), link)

		// Check if article already exists
		exists, err := s.db.ArticleExists(ctx, link)
		if err != nil {
			log.Printf("Error checking article existence: %v", err)
			continue
		}
		if exists {
			log.Printf("Article already exists, skipping: %s", link)
			continue
		}

		// Scrape the article
		article, err := s.scrapeArticle(ctx, link)
		if err != nil {
			log.Printf("Error scraping article %s: %v", link, err)
			continue
		}

		// Save to database
		if err := s.db.CreateArticle(ctx, article); err != nil {
			log.Printf("Error saving article %s: %v", link, err)
			continue
		}

		scrapedCount++
		log.Printf("Successfully scraped and saved article: %s", article.URL)

		// Small delay to be respectful to the server
		time.Sleep(1 * time.Second)
	}

	return scrapedCount, nil
}

// extractArticleLinks extracts article URLs from a page
func (s *Scraper) extractArticleLinks(page *rod.Page) []string {
	var links []string

	// Try different common patterns for article links
	// Adjust these selectors based on actual Gasetten structure
	selectors := []string{
		`article a[href*="/articles/"]`,
		`article a[href*="/posts/"]`,
		`.article-list a`,
		`a.article-link`,
		`main a[href*="/20"]`, // Links containing year (like /2024/)
	}

	for _, selector := range selectors {
		elements, err := page.Elements(selector)
		if err != nil {
			continue
		}

		for _, el := range elements {
			href, err := el.Attribute("href")
			if err != nil || href == nil {
				continue
			}

			url := *href
			// Convert relative URLs to absolute
			if strings.HasPrefix(url, "/") {
				url = "https://gasetten.se" + url
			}

			// Deduplicate
			if !contains(links, url) {
				links = append(links, url)
			}
		}

		// If we found links with this selector, use them
		if len(links) > 0 {
			break
		}
	}

	return links
}

// scrapeArticle scrapes a single article page
func (s *Scraper) scrapeArticle(ctx context.Context, url string) (*database.Article, error) {
	page := s.browser.MustPage(url)
	defer page.Close()

	page.MustWaitLoad()

	article := &database.Article{
		Source: "gasetten",
		URL:    url,
	}

	// Extract title
	title := s.extractText(page, `h1, .article-title, article h1, [class*="title"]`)
	if title != "" {
		article.Title = &title
	}

	// Extract author
	author := s.extractText(page, `.author, .byline, [class*="author"], [rel="author"]`)
	if author != "" {
		article.Author = &author
	}

	// Extract published date
	publishedAt := s.extractDate(page)
	if publishedAt != nil {
		article.PublishedAt = publishedAt
	}

	// Extract article content HTML
	contentHTML := s.extractHTML(page, `article, .article-content, .post-content, main article, [class*="content"]`)
	if contentHTML != "" {
		article.ContentHTML = &contentHTML
	}

	// Extract plain text content
	contentText := s.extractText(page, `article, .article-content, .post-content, main article, [class*="content"]`)
	if contentText != "" {
		article.ContentText = &contentText
	}

	return article, nil
}

// extractText extracts text content using multiple selector fallbacks
func (s *Scraper) extractText(page *rod.Page, selectors string) string {
	for _, selector := range strings.Split(selectors, ",") {
		selector = strings.TrimSpace(selector)
		el, err := page.Element(selector)
		if err != nil {
			continue
		}
		text, err := el.Text()
		if err == nil && text != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}

// extractHTML extracts HTML content using multiple selector fallbacks
func (s *Scraper) extractHTML(page *rod.Page, selectors string) string {
	for _, selector := range strings.Split(selectors, ",") {
		selector = strings.TrimSpace(selector)
		el, err := page.Element(selector)
		if err != nil {
			continue
		}
		html, err := el.HTML()
		if err == nil && html != "" {
			return html
		}
	}
	return ""
}

// extractDate tries to extract and parse publication date
func (s *Scraper) extractDate(page *rod.Page) *time.Time {
	// Try to find date in various formats and selectors
	selectors := []string{
		`time[datetime]`,
		`.published-date`,
		`.post-date`,
		`[class*="date"]`,
		`meta[property="article:published_time"]`,
	}

	for _, selector := range selectors {
		el, err := page.Element(selector)
		if err != nil {
			continue
		}

		// Try datetime attribute first (for <time> elements)
		if datetime, err := el.Attribute("datetime"); err == nil && datetime != nil {
			if t, err := time.Parse(time.RFC3339, *datetime); err == nil {
				return &t
			}
		}

		// Try content attribute (for meta tags)
		if content, err := el.Attribute("content"); err == nil && content != nil {
			if t, err := time.Parse(time.RFC3339, *content); err == nil {
				return &t
			}
		}

		// Try parsing text content
		if text, err := el.Text(); err == nil {
			// Try common date formats
			formats := []string{
				"2006-01-02",
				"January 2, 2006",
				"2 January 2006",
				"02 Jan 2006",
			}
			for _, format := range formats {
				if t, err := time.Parse(format, strings.TrimSpace(text)); err == nil {
					return &t
				}
			}
		}
	}

	return nil
}

// contains checks if a string slice contains a value
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

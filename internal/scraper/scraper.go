package scraper

import (
	"context"
	"fmt"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	readability "github.com/go-shiori/go-readability"
	"github.com/tkilaker/kiln/internal/database"
)

const (
	// PageTimeout is the default timeout for page operations
	PageTimeout = 30 * time.Second
)

// Scraper handles web scraping for Gasetten
type Scraper struct {
	username   string
	password   string
	db         *database.DB
	sessionDir string
	browser    *rod.Browser
	headless   bool
	progress   *ProgressTracker
}

// New creates a new scraper instance
func New(username, password string, db *database.DB, headless bool) (*Scraper, error) {
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
		headless:   headless,
		progress:   NewProgressTracker(),
	}, nil
}

// initBrowser initializes the Rod browser instance
func (s *Scraper) initBrowser() error {
	// Check if browser exists and is still alive
	if s.browser != nil {
		if s.isBrowserAlive() {
			return nil
		}
		// Browser connection is dead, clean it up
		log.Println("Browser connection is stale, reinitializing...")
		s.browser.Close()
		s.browser = nil
	}

	// Launch browser
	path, _ := launcher.LookPath()
	l := launcher.New().
		Bin(path).
		Headless(s.headless).
		UserDataDir(s.sessionDir)

	u, err := l.Launch()
	if err != nil {
		return fmt.Errorf("failed to launch browser: %w", err)
	}

	browser := rod.New().ControlURL(u)
	if err := browser.Connect(); err != nil {
		return fmt.Errorf("failed to connect to browser: %w", err)
	}
	s.browser = browser

	if s.headless {
		log.Println("Browser launched in headless mode")
	} else {
		log.Println("Browser launched in visible mode for debugging")
	}

	return nil
}

// isBrowserAlive checks if the browser connection is still active
func (s *Scraper) isBrowserAlive() bool {
	if s.browser == nil {
		return false
	}

	// Try to get browser info to test the connection
	defer func() {
		if r := recover(); r != nil {
			log.Printf("Browser health check failed: %v", r)
		}
	}()

	// Attempt a simple operation that would fail if connection is dead
	_ = s.browser.GetContext()
	return true
}

// Close closes the browser and cleans up resources
func (s *Scraper) Close() error {
	if s.browser != nil {
		return s.browser.Close()
	}
	return nil
}

// GetProgressTracker returns the progress tracker
func (s *Scraper) GetProgressTracker() *ProgressTracker {
	return s.progress
}

// Login logs into Gasetten using username and password
func (s *Scraper) Login(ctx context.Context) error {
	if err := s.initBrowser(); err != nil {
		return err
	}

	// Create page with error handling instead of panicking
	page, err := s.browser.Page(proto.TargetCreateTarget{URL: "https://gasetten.se/min-profil/"})
	if err != nil {
		return fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// Set page timeout
	page = page.Timeout(PageTimeout)

	// Wait for page to load
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("timeout waiting for login page to load: %w", err)
	}

	// Check if already logged in by looking for logout link or user menu
	if s.isLoggedIn(page) {
		log.Println("Already logged in to Gasetten")
		return nil
	}

	log.Println("Logging into Gasetten...")

	// Find and fill username field (WordPress login form uses name="log")
	log.Println("Looking for username field...")
	usernameField, err := page.Element(`input[name="log"]`)
	if err != nil {
		return fmt.Errorf("could not find username field (input[name='log']): %w", err)
	}
	if err := usernameField.Input(s.username); err != nil {
		return fmt.Errorf("could not input username: %w", err)
	}
	log.Println("Entered username")

	// Find and fill password field (WordPress login form uses name="pwd")
	log.Println("Looking for password field...")
	passwordField, err := page.Element(`input[name="pwd"]`)
	if err != nil {
		return fmt.Errorf("could not find password field (input[name='pwd']): %w", err)
	}
	if err := passwordField.Input(s.password); err != nil {
		return fmt.Errorf("could not input password: %w", err)
	}
	log.Println("Entered password")

	// Find and click login button (WordPress uses id="wp-submit")
	log.Println("Looking for login button...")
	loginButton, err := page.Element(`input[id="wp-submit"]`)
	if err != nil {
		return fmt.Errorf("could not find login button (input[id='wp-submit']): %w", err)
	}
	if err := loginButton.Click(proto.InputMouseButtonLeft, 1); err != nil {
		return fmt.Errorf("could not click login button: %w", err)
	}
	log.Println("Clicked login button")

	// Wait for navigation after login
	log.Println("Waiting for navigation after login...")
	if err := page.WaitLoad(); err != nil {
		return fmt.Errorf("timeout waiting for page after login: %w", err)
	}

	// Log where we ended up
	pageInfo, err := page.Info()
	if err == nil {
		log.Printf("After login, redirected to: %s", pageInfo.URL)
	}

	// Verify login was successful
	if !s.isLoggedIn(page) {
		// Log additional debug info
		currentURL := "unknown"
		if pageInfo, err := page.Info(); err == nil {
			currentURL = pageInfo.URL
		}
		log.Printf("Login verification failed at URL: %s", currentURL)

		// Check if there's an error message on the page
		errorMsg := s.extractText(page, `.login-error, #login_error, .error`)
		if errorMsg != "" {
			return fmt.Errorf("login failed - error message: %s", errorMsg)
		}

		return fmt.Errorf("login failed - could not verify successful authentication at %s", currentURL)
	}

	log.Println("Successfully logged into Gasetten")
	return nil
}

// isLoggedIn checks if the current page shows signs of being logged in
func (s *Scraper) isLoggedIn(page *rod.Page) bool {
	// Primary check: if login form is present, we're NOT logged in
	hasLoginForm, _, _ := page.Has(`form#loginform`)
	if hasLoginForm {
		log.Println("Login form still present - not logged in")
		return false
	}

	// If no login form is present, we're logged in
	// This works because WordPress shows the login form when not authenticated,
	// and shows profile/account content when authenticated
	log.Println("No login form found - logged in successfully")
	return true
}

// ScrapeArticles fetches and stores articles from Gasetten
func (s *Scraper) ScrapeArticles(ctx context.Context) (int, error) {
	// Mark as active and reset progress
	s.progress.SetActive(true)
	s.progress.UpdateStatus(StatusStarting, "Initializing browser...")
	defer s.progress.SetActive(false)

	if err := s.initBrowser(); err != nil {
		s.progress.UpdateStatus(StatusFailed, fmt.Sprintf("Failed to initialize browser: %v", err))
		return 0, err
	}

	// Ensure we're logged in
	s.progress.UpdateStatus(StatusLoggingIn, "Logging into Gasetten...")
	if err := s.Login(ctx); err != nil {
		s.progress.UpdateStatus(StatusFailed, fmt.Sprintf("Login failed: %v", err))
		return 0, fmt.Errorf("failed to login: %w", err)
	}

	// Check if context was cancelled
	select {
	case <-ctx.Done():
		s.progress.UpdateStatus(StatusCancelled, "Operation cancelled by user")
		return 0, ctx.Err()
	default:
	}

	s.progress.UpdateStatus(StatusScraping, "Loading article category page...")

	// Scrape from the Malmö FF category page which has better article organization
	page, err := s.browser.Page(proto.TargetCreateTarget{URL: "https://gasetten.se/category/malmo-ff/"})
	if err != nil {
		s.progress.UpdateStatus(StatusFailed, "Failed to load category page")
		return 0, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// Set page timeout
	page = page.Timeout(PageTimeout)

	if err := page.WaitLoad(); err != nil {
		s.progress.UpdateStatus(StatusFailed, "Timeout waiting for category page")
		return 0, fmt.Errorf("timeout waiting for category page to load: %w", err)
	}

	log.Println("Loaded category page, extracting article links...")
	s.progress.UpdateStatus(StatusScraping, "Extracting article links...")

	// Find all article links on the page
	// Adjust selector based on actual Gasetten HTML structure
	articleLinks := s.extractArticleLinks(page)

	log.Printf("Found %d articles to scrape", len(articleLinks))
	s.progress.Update(ProgressUpdate{
		Status:     StatusScraping,
		Message:    fmt.Sprintf("Found %d articles, starting to scrape...", len(articleLinks)),
		TotalItems: len(articleLinks),
	})

	scrapedCount := 0
	for i, link := range articleLinks {
		// Check if context was cancelled
		select {
		case <-ctx.Done():
			s.progress.UpdateStatus(StatusCancelled, fmt.Sprintf("Operation cancelled. Scraped %d articles before cancellation.", scrapedCount))
			return scrapedCount, ctx.Err()
		default:
		}

		s.progress.UpdateProgress(i+1, len(articleLinks), fmt.Sprintf("Processing article %d/%d...", i+1, len(articleLinks)))
		log.Printf("Scraping article %d/%d: %s", i+1, len(articleLinks), link)

		// Check if article already exists
		exists, err := s.db.ArticleExists(ctx, link)
		if err != nil {
			log.Printf("Error checking article existence: %v", err)
			continue
		}
		if exists {
			log.Printf("Article already exists, skipping: %s", link)
			s.progress.UpdateProgress(i+1, len(articleLinks), fmt.Sprintf("Article %d/%d already exists, skipping...", i+1, len(articleLinks)))
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

		// Update progress with new article ID
		s.progress.Update(ProgressUpdate{
			Status:        StatusScraping,
			Message:       fmt.Sprintf("Saved article %d/%d (%d new)", i+1, len(articleLinks), scrapedCount),
			CurrentItem:   i + 1,
			TotalItems:    len(articleLinks),
			ArticlesAdded: scrapedCount,
			NewArticleID:  article.ID,
		})

		// Small delay to be respectful to the server
		time.Sleep(1 * time.Second)
	}

	s.progress.Update(ProgressUpdate{
		Status:        StatusCompleted,
		Message:       fmt.Sprintf("Completed! Added %d new articles.", scrapedCount),
		CurrentItem:   len(articleLinks),
		TotalItems:    len(articleLinks),
		ArticlesAdded: scrapedCount,
	})

	return scrapedCount, nil
}

// extractArticleLinks extracts article URLs from a page
func (s *Scraper) extractArticleLinks(page *rod.Page) []string {
	var links []string
	seen := make(map[string]bool)

	// Get all links on the page
	elements, err := page.Elements(`a[href]`)
	if err != nil {
		log.Printf("Error finding links: %v", err)
		return links
	}

	log.Printf("Found %d total links on page", len(elements))

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

		// Filter for article URLs - Gasetten articles are in categories like /malmo-ff/, /blogg/, etc.
		// Skip navigation links, author pages, tag pages, category pages, etc.
		if !strings.HasPrefix(url, "https://gasetten.se/") {
			continue
		}

		// Skip non-article pages
		if strings.Contains(url, "/author/") ||
			strings.Contains(url, "/tag/") ||
			strings.Contains(url, "/category/") ||
			strings.Contains(url, "/page/") ||
			strings.Contains(url, "/wp-content/") ||
			strings.Contains(url, "/wp-login") ||
			strings.Contains(url, "/min-profil") ||
			strings.Contains(url, "/about") ||
			strings.Contains(url, "/arkiv") ||
			strings.Contains(url, "/stotta-oss") ||
			strings.Contains(url, "/annonsera") ||
			strings.Contains(url, "/registrera") ||
			strings.Contains(url, "/kop-plus") ||
			strings.HasSuffix(url, "gasetten.se/") ||
			strings.HasSuffix(url, "gasetten.se/#") {
			continue
		}

		// Must have at least 2 path segments (e.g., /malmo-ff/article-slug/)
		parts := strings.Split(strings.TrimPrefix(strings.TrimSuffix(url, "/"), "https://gasetten.se/"), "/")
		if len(parts) < 2 {
			continue
		}

		// Deduplicate
		if !seen[url] {
			seen[url] = true
			links = append(links, url)
		}
	}

	log.Printf("Filtered to %d article links", len(links))
	return links
}

// scrapeArticle scrapes a single article page using Mozilla Readability
func (s *Scraper) scrapeArticle(ctx context.Context, articleURL string) (*database.Article, error) {
	page, err := s.browser.Page(proto.TargetCreateTarget{URL: articleURL})
	if err != nil {
		return nil, fmt.Errorf("failed to create page: %w", err)
	}
	defer page.Close()

	// Set page timeout
	page = page.Timeout(PageTimeout)

	if err := page.WaitLoad(); err != nil {
		return nil, fmt.Errorf("timeout waiting for article page to load: %w", err)
	}

	// Wait a bit for any lazy-loaded content
	time.Sleep(500 * time.Millisecond)

	// Get the full HTML of the page
	htmlContent, err := page.HTML()
	if err != nil {
		return nil, fmt.Errorf("failed to get page HTML: %w", err)
	}

	// Parse URL for readability
	parsedURL, err := url.Parse(articleURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse URL: %w", err)
	}

	// Use Mozilla Readability to extract article content
	readabilityArticle, err := readability.FromReader(strings.NewReader(htmlContent), parsedURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse article with readability: %w", err)
	}

	log.Printf("Readability extracted: title='%s', byline='%s', content=%d chars, text=%d chars",
		readabilityArticle.Title,
		readabilityArticle.Byline,
		len(readabilityArticle.Content),
		len(readabilityArticle.TextContent))

	// Create article from readability results
	article := &database.Article{
		Source:      "gasetten",
		URL:         articleURL,
		ContentHTML: &readabilityArticle.Content,
		ContentText: &readabilityArticle.TextContent,
	}

	// Set title
	if readabilityArticle.Title != "" {
		article.Title = &readabilityArticle.Title
	}

	// Set author (byline)
	if readabilityArticle.Byline != "" {
		article.Author = &readabilityArticle.Byline
	}

	// Try to extract published date from meta tags or readability
	if readabilityArticle.PublishedTime != nil && !readabilityArticle.PublishedTime.IsZero() {
		article.PublishedAt = readabilityArticle.PublishedTime
		log.Printf("Extracted date from readability: %v", readabilityArticle.PublishedTime)
	} else {
		// Fallback to manual date extraction
		publishedAt := s.extractDate(page)
		if publishedAt != nil {
			article.PublishedAt = publishedAt
			log.Printf("Extracted date manually: %v", publishedAt)
		}
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
		`time.post-date[datetime]`,
		`time.entry-date[datetime]`,
		`time[datetime]`,
		`meta[property="article:published_time"]`,
		`.post-date`,
		`[class*="date"]`,
	}

	for _, selector := range selectors {
		el, err := page.Element(selector)
		if err != nil {
			continue
		}

		// Try datetime attribute first (for <time> elements)
		if datetime, err := el.Attribute("datetime"); err == nil && datetime != nil {
			dateStr := *datetime

			// Try various date formats
			formats := []string{
				time.RFC3339,
				"2006-01-02",
				"2006-01-02T15:04:05",
			}

			for _, format := range formats {
				if t, err := time.Parse(format, dateStr); err == nil {
					return &t
				}
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
			// Try common Swedish/international date formats
			formats := []string{
				"2 January 2006", // "9 november, 2025"
				"2 January, 2006",
				"January 2, 2006",
				"2006-01-02",
				"02 Jan 2006",
			}

			// Clean up text (remove extra spaces, commas at weird places)
			cleanText := strings.TrimSpace(text)
			cleanText = strings.Replace(cleanText, ",", "", -1)

			for _, format := range formats {
				if t, err := time.Parse(format, cleanText); err == nil {
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

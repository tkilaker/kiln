package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tkilaker/kiln/internal/config"
	"github.com/tkilaker/kiln/internal/database"
	"github.com/tkilaker/kiln/internal/scraper"
)

// Server represents the HTTP server
type Server struct {
	router  *chi.Mux
	db      *database.DB
	scraper *scraper.Scraper
	config  *config.Config
}

// New creates a new server instance
func New(db *database.DB, scraper *scraper.Scraper, cfg *config.Config) *Server {
	s := &Server{
		router:  chi.NewRouter(),
		db:      db,
		scraper: scraper,
		config:  cfg,
	}

	s.setupRoutes()
	return s
}

// setupRoutes configures all HTTP routes
func (s *Server) setupRoutes() {
	// Middleware
	s.router.Use(middleware.Logger)
	s.router.Use(middleware.Recoverer)
	s.router.Use(middleware.RequestID)
	s.router.Use(middleware.RealIP)

	// Routes (no timeout middleware for SSE endpoint)
	s.router.Group(func(r chi.Router) {
		r.Use(middleware.Timeout(60 * time.Second))
		r.Get("/", s.handleIndex)
		r.Get("/articles", s.handleArticleList)
		r.Get("/articles/{id}", s.handleArticleDetail)
		r.Delete("/articles/{id}", s.handleDeleteArticle)
		r.Post("/scrape", s.handleScrape)
		r.Post("/articles/clear", s.handleClearArticles)
		r.Get("/rss.xml", s.handleRSS)
	})

	// SSE endpoint (no timeout)
	s.router.Get("/scrape/progress", s.handleScrapeProgress)

	// Health check
	s.router.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	})
}

// Router returns the Chi router
func (s *Server) Router() *chi.Mux {
	return s.router
}

// Start starts the HTTP server
func (s *Server) Start(addr string) error {
	log.Printf("Starting server on %s", addr)
	return http.ListenAndServe(addr, s.router)
}

// handleIndex renders the home page (redirects to articles list)
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/articles", http.StatusSeeOther)
}

// handleArticleList renders the list of all articles
func (s *Server) handleArticleList(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	articles, err := s.db.GetAllArticles(ctx, 100)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch articles: %v", err), http.StatusInternalServerError)
		return
	}

	// Render template
	ArticleListPage(articles).Render(ctx, w)
}

// handleArticleDetail renders a single article
func (s *Server) handleArticleDetail(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "Invalid article ID", http.StatusBadRequest)
		return
	}

	article, err := s.db.GetArticleByID(ctx, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Article not found: %v", err), http.StatusNotFound)
		return
	}

	// Render template
	ArticleDetailPage(article).Render(ctx, w)
}

// handleScrape triggers a manual scrape operation
func (s *Server) handleScrape(w http.ResponseWriter, r *http.Request) {
	// Check if scraping is already in progress
	if s.scraper.GetProgressTracker().IsActive() {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="p-4 bg-yellow-100 border border-yellow-400 text-yellow-700 rounded">
			Scraping is already in progress. Please wait for it to complete.
		</div>`)
		return
	}

	log.Println("Starting manual scrape in background...")

	// Start scraping in a background goroutine
	go func() {
		// Create a new context that won't be cancelled when the HTTP request ends
		ctx := context.Background()
		count, err := s.scraper.ScrapeArticles(ctx)
		if err != nil {
			log.Printf("Scrape failed: %v", err)
		} else {
			log.Printf("Scrape completed: %d new articles", count)
		}
	}()

	// Return immediate response with progress UI
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<div id="scrape-progress" class="p-4 bg-blue-100 border border-blue-400 text-blue-700 rounded">
		<div class="flex items-center gap-2 mb-2">
			<svg class="animate-spin h-4 w-4 text-blue-700" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
			</svg>
			<span class="font-semibold">Scraping in progress...</span>
		</div>
		<div id="progress-message">Initializing...</div>
		<div id="progress-bar-container" class="mt-2 w-full bg-gray-200 rounded-full h-2 hidden">
			<div id="progress-bar" class="bg-blue-600 h-2 rounded-full transition-all duration-300" style="width: 0%"></div>
		</div>
		<div id="progress-details" class="mt-2 text-sm text-blue-600"></div>
	</div>
	<script>
		const eventSource = new EventSource('/scrape/progress');
		eventSource.onmessage = function(e) {
			const data = JSON.parse(e.data);
			const message = document.getElementById('progress-message');
			const details = document.getElementById('progress-details');
			const barContainer = document.getElementById('progress-bar-container');
			const bar = document.getElementById('progress-bar');

			message.textContent = data.message;

			if (data.total_items > 0) {
				barContainer.classList.remove('hidden');
				const percent = (data.current_item / data.total_items) * 100;
				bar.style.width = percent + '%';
				details.textContent = 'Articles added: ' + data.articles_added;
			}

			if (data.article_html) {
				const emptyState = document.querySelector('.text-center.py-12');
				let articleList = document.querySelector('.space-y-4');

				if (emptyState) {
					const container = document.createElement('div');
					container.className = 'space-y-4';
					container.innerHTML = data.article_html;
					emptyState.replaceWith(container);
				} else if (articleList) {
					const temp = document.createElement('div');
					temp.innerHTML = data.article_html;
					if (temp.firstElementChild) {
						articleList.insertBefore(temp.firstElementChild, articleList.firstChild);
					}
				}
			}

			if (data.status === 'completed') {
				eventSource.close();
				document.getElementById('scrape-progress').innerHTML =
					'<div class="p-4 bg-green-100 border border-green-400 text-green-700 rounded">' +
					data.message +
					'</div>';
			} else if (data.status === 'failed' || data.status === 'cancelled') {
				eventSource.close();
				document.getElementById('scrape-progress').innerHTML =
					'<div class="p-4 bg-red-100 border border-red-400 text-red-700 rounded">' +
					data.message +
					'</div>';
			}
		};
		eventSource.onerror = function() {
			eventSource.close();
		};
	</script>`)
}

// handleScrapeProgress streams progress updates via Server-Sent Events
func (s *Server) handleScrapeProgress(w http.ResponseWriter, r *http.Request) {
	// Set headers for SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	// Get progress tracker
	tracker := s.scraper.GetProgressTracker()

	// Subscribe to progress updates
	updates := tracker.Subscribe()
	defer tracker.Unsubscribe(updates)

	// Stream updates to client
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			// Client disconnected
			return
		case update, ok := <-updates:
			if !ok {
				// Channel closed
				return
			}

			// Fetch and render article HTML if a new article was added
			articleHTML := ""
			if update.NewArticleID > 0 {
				article, err := s.db.GetArticleByID(ctx, update.NewArticleID)
				if err == nil {
					// Render article card to HTML
					var buf strings.Builder
					if err := ArticleCard(article).Render(ctx, &buf); err == nil {
						articleHTML = buf.String()
					}
				}
			}

			// Escape strings for JSON
			message := strings.ReplaceAll(update.Message, `"`, `\"`)
			message = strings.ReplaceAll(message, "\n", `\n`)
			articleHTML = strings.ReplaceAll(articleHTML, `"`, `\"`)
			articleHTML = strings.ReplaceAll(articleHTML, "\n", "")
			articleHTML = strings.ReplaceAll(articleHTML, "\t", "")

			// Format as JSON
			data := fmt.Sprintf(`{"status":"%s","message":"%s","current_item":%d,"total_items":%d,"articles_added":%d,"article_html":"%s"}`,
				update.Status,
				message,
				update.CurrentItem,
				update.TotalItems,
				update.ArticlesAdded,
				articleHTML,
			)

			// Send SSE message
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()

			// Close connection after completion/failure/cancellation
			if update.Status == "completed" || update.Status == "failed" || update.Status == "cancelled" {
				return
			}
		}
	}
}

// handleDeleteArticle deletes a specific article
func (s *Server) handleDeleteArticle(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "Invalid article ID", http.StatusBadRequest)
		return
	}

	log.Printf("Deleting article %d...", id)

	if err := s.db.DeleteArticle(ctx, id); err != nil {
		http.Error(w, fmt.Sprintf("Failed to delete article: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Deleted article %d", id)

	// Return empty response (article card will be removed by HTMX)
	w.WriteHeader(http.StatusOK)
}

// handleClearArticles deletes all articles from the database
func (s *Server) handleClearArticles(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	log.Println("Clearing all articles...")

	count, err := s.db.DeleteAllArticles(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to clear articles: %v", err), http.StatusInternalServerError)
		return
	}

	log.Printf("Deleted %d articles", count)

	// Return HTMX response with script to update the page
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div class="p-4 bg-yellow-100 border border-yellow-400 text-yellow-700 rounded">
		Deleted %d articles. Database is now empty.
	</div>
	<script>
		// Remove all article cards
		const articleList = document.querySelector('.space-y-4');
		if (articleList) {
			articleList.remove();
		}

		// Show empty state
		const main = document.querySelector('main');
		const emptyState = document.createElement('div');
		emptyState.className = 'text-center py-12';
		emptyState.innerHTML = '<p class="text-gray-500 text-lg">No articles yet. Click "Scrape New Articles" to get started!</p>';
		main.appendChild(emptyState);

		// Hide the Clear All button
		const clearButton = document.querySelector('button[hx-post="/articles/clear"]');
		if (clearButton) {
			clearButton.style.display = 'none';
		}
	</script>`, count)
}

// handleRSS generates and serves the RSS feed
func (s *Server) handleRSS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Get recent articles (last 30 days)
	since := time.Now().AddDate(0, 0, -30)
	articles, err := s.db.GetRecentArticles(ctx, since, 50)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch articles: %v", err), http.StatusInternalServerError)
		return
	}

	// Generate RSS feed
	feed, err := GenerateRSSFeed(articles, s.config)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate feed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write([]byte(feed))
}

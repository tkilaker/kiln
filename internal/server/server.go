package server

import (
	"fmt"
	"log"
	"net/http"
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
	s.router.Use(middleware.Timeout(60 * time.Second))

	// Routes
	s.router.Get("/", s.handleIndex)
	s.router.Get("/articles", s.handleArticleList)
	s.router.Get("/articles/{id}", s.handleArticleDetail)
	s.router.Post("/scrape", s.handleScrape)
	s.router.Get("/rss.xml", s.handleRSS)

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
	ctx := r.Context()

	log.Println("Starting manual scrape...")

	count, err := s.scraper.ScrapeArticles(ctx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Scrape failed: %v", err), http.StatusInternalServerError)
		return
	}

	// Return HTMX response
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div class="p-4 bg-green-100 border border-green-400 text-green-700 rounded">
		Successfully scraped %d new articles!
	</div>`, count)
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

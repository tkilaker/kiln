package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tkilaker/kiln/internal/config"
	"github.com/tkilaker/kiln/internal/database"
	"github.com/tkilaker/kiln/internal/scraper"
	"github.com/tkilaker/kiln/internal/tts"
)

// Server represents the HTTP server
type Server struct {
	router  *chi.Mux
	db      *database.DB
	scraper *scraper.Scraper
	tts     *tts.Service
	config  *config.Config
}

// New creates a new server instance
func New(db *database.DB, scraper *scraper.Scraper, ttsSvc *tts.Service, cfg *config.Config) *Server {
	s := &Server{
		router:  chi.NewRouter(),
		db:      db,
		scraper: scraper,
		tts:     ttsSvc,
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
		r.Get("/podcast.xml", s.handlePodcastRSS)
		r.Get("/articles/{id}/audio", s.handleServeAudio)
		r.Get("/tts/voices", s.handleTTSVoices)
	})

	// SSE endpoint (no timeout)
	s.router.Get("/scrape/progress", s.handleScrapeProgress)

	// TTS endpoint (no timeout - generation can take a while)
	s.router.Post("/articles/{id}/tts", s.handleGenerateTTS)

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

	// Fetch audio status for all articles
	var articleIDs []int
	for _, a := range articles {
		articleIDs = append(articleIDs, a.ID)
	}
	audioMap, _ := s.db.GetCompletedAudioForArticles(ctx, articleIDs)

	// Render template
	ArticleListPage(articles, audioMap, s.ttsEnabled()).Render(ctx, w)
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

	// Fetch audio files for this article
	audioFiles, _ := s.db.GetAudioFilesByArticle(ctx, id)

	// Render template
	ArticleDetailPage(article, audioFiles, s.ttsEnabled()).Render(ctx, w)
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

// ttsEnabled returns whether TTS is configured and available
func (s *Server) ttsEnabled() bool {
	return s.tts != nil
}

// handleGenerateTTS triggers TTS generation for an article
func (s *Server) handleGenerateTTS(w http.ResponseWriter, r *http.Request) {
	if !s.ttsEnabled() {
		http.Error(w, "TTS is not configured. Set OPENAI_API_KEY to enable.", http.StatusServiceUnavailable)
		return
	}

	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "Invalid article ID", http.StatusBadRequest)
		return
	}

	// Get the voice from form data or query param
	voice := r.FormValue("voice")
	if voice == "" {
		voice = s.config.TTSVoice
	}

	// Check if article exists
	article, err := s.db.GetArticleByID(ctx, id)
	if err != nil {
		http.Error(w, fmt.Sprintf("Article not found: %v", err), http.StatusNotFound)
		return
	}

	if article.ContentText == nil || *article.ContentText == "" {
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="p-3 bg-red-100 border border-red-400 text-red-700 rounded text-sm">No text content available for this article.</div>`)
		return
	}

	// Check if audio already exists and is completed
	existing, _ := s.db.GetAudioFileByArticle(ctx, id, voice)
	if existing != nil && existing.Status == "completed" {
		w.Header().Set("Content-Type", "text/html")
		var buf strings.Builder
		AudioPlayer(id, existing).Render(ctx, &buf)
		fmt.Fprint(w, buf.String())
		return
	}

	// Create a pending audio record
	audio := &database.AudioFile{
		ArticleID: id,
		Voice:     voice,
		FilePath:  "",
		Status:    "generating",
	}
	if err := s.db.CreateAudioFile(ctx, audio); err != nil {
		log.Printf("Failed to create audio record: %v", err)
		w.Header().Set("Content-Type", "text/html")
		fmt.Fprint(w, `<div class="p-3 bg-red-100 border border-red-400 text-red-700 rounded text-sm">Failed to start TTS generation.</div>`)
		return
	}

	// Return immediate progress UI
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `<div id="tts-status-%d" class="p-3 bg-blue-100 border border-blue-400 text-blue-700 rounded text-sm">
		<div class="flex items-center gap-2">
			<svg class="animate-spin h-4 w-4 text-blue-700" xmlns="http://www.w3.org/2000/svg" fill="none" viewBox="0 0 24 24">
				<circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
				<path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4zm2 5.291A7.962 7.962 0 014 12H0c0 3.042 1.135 5.824 3 7.938l3-2.647z"></path>
			</svg>
			<span>Generating audio with voice "%s"...</span>
		</div>
	</div>
	<script>
		(function() {
			const articleId = %d;
			const voice = "%s";
			const poll = setInterval(function() {
				fetch('/articles/' + articleId + '/audio?check=1&voice=' + voice)
					.then(r => r.json())
					.then(data => {
						if (data.status === 'completed') {
							clearInterval(poll);
							const el = document.getElementById('tts-status-' + articleId);
							if (el) {
								el.outerHTML = '<div id="tts-result-' + articleId + '">' +
									'<audio controls class="w-full mt-2" preload="metadata">' +
									'<source src="/articles/' + articleId + '/audio?voice=' + voice + '" type="audio/mpeg">' +
									'</audio></div>';
							}
						} else if (data.status === 'failed') {
							clearInterval(poll);
							const el = document.getElementById('tts-status-' + articleId);
							if (el) {
								el.innerHTML = '<div class="text-red-700">Audio generation failed: ' + (data.error || 'unknown error') + '</div>';
								el.className = 'p-3 bg-red-100 border border-red-400 text-red-700 rounded text-sm';
							}
						}
					})
					.catch(() => {});
			}, 2000);
		})();
	</script>`, id, voice, id, voice)

	// Generate audio in the background
	go func() {
		bgCtx := context.Background()
		filePath, fileSize, err := s.tts.GenerateAudio(bgCtx, id, *article.ContentText, voice)
		if err != nil {
			log.Printf("TTS generation failed for article %d: %v", id, err)
			errMsg := err.Error()
			s.db.UpdateAudioFileStatus(bgCtx, audio.ID, "failed", "", 0, &errMsg)
			return
		}
		s.db.UpdateAudioFileStatus(bgCtx, audio.ID, "completed", filePath, fileSize, nil)
		log.Printf("TTS generation completed for article %d: %s", id, filePath)
	}()
}

// handleServeAudio serves the generated audio file or checks its status
func (s *Server) handleServeAudio(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	idStr := chi.URLParam(r, "id")

	var id int
	if _, err := fmt.Sscanf(idStr, "%d", &id); err != nil {
		http.Error(w, "Invalid article ID", http.StatusBadRequest)
		return
	}

	voice := r.URL.Query().Get("voice")
	if voice == "" {
		voice = s.config.TTSVoice
	}

	audio, err := s.db.GetAudioFileByArticle(ctx, id, voice)
	if err != nil || audio == nil {
		// If this is a status check, return JSON
		if r.URL.Query().Get("check") == "1" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"status":"not_found"}`)
			return
		}
		http.Error(w, "Audio not found", http.StatusNotFound)
		return
	}

	// Status check endpoint
	if r.URL.Query().Get("check") == "1" {
		w.Header().Set("Content-Type", "application/json")
		if audio.Status == "failed" && audio.ErrorMessage != nil {
			errMsg := strings.ReplaceAll(*audio.ErrorMessage, `"`, `\"`)
			fmt.Fprintf(w, `{"status":"%s","error":"%s"}`, audio.Status, errMsg)
		} else {
			fmt.Fprintf(w, `{"status":"%s"}`, audio.Status)
		}
		return
	}

	if audio.Status != "completed" {
		http.Error(w, "Audio is not ready yet", http.StatusAccepted)
		return
	}

	// Serve the audio file
	if _, err := os.Stat(audio.FilePath); os.IsNotExist(err) {
		http.Error(w, "Audio file not found on disk", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "audio/mpeg")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`inline; filename="article-%d.mp3"`, id))
	http.ServeFile(w, r, audio.FilePath)
}

// handleTTSVoices returns the list of available TTS voices
func (s *Server) handleTTSVoices(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	voices := tts.AvailableVoices()
	var items []string
	for _, v := range voices {
		items = append(items, fmt.Sprintf(`"%s"`, v))
	}
	fmt.Fprintf(w, `{"voices":[%s],"default":"%s"}`, strings.Join(items, ","), s.config.TTSVoice)
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

// handlePodcastRSS generates a podcast-compatible RSS feed with audio enclosures
func (s *Server) handlePodcastRSS(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	since := time.Now().AddDate(0, 0, -30)
	articles, err := s.db.GetRecentArticles(ctx, since, 50)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch articles: %v", err), http.StatusInternalServerError)
		return
	}

	// Get audio files for these articles
	var articleIDs []int
	for _, a := range articles {
		articleIDs = append(articleIDs, a.ID)
	}
	audioMap, _ := s.db.GetCompletedAudioForArticles(ctx, articleIDs)

	feed, err := GeneratePodcastFeed(articles, audioMap, s.config)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate podcast feed: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	w.Write([]byte(feed))
}

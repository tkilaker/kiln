# 🔥 Kiln

**Kiln** is a personal "news kiln" that automatically scrapes articles from Gasetten.se and stores them in a structured format for later consumption. Think of it as your personal article archive with RSS feed support.

## 🎯 Features

- **Automated Login**: Automatically authenticates with Gasetten.se using your credentials
- **Smart Scraping**: Uses headless browser automation (Rod) to handle JavaScript-rendered content
- **Article Storage**: Stores full article content (HTML + text) in PostgreSQL
- **Web Interface**: Clean, responsive UI built with HTMX and TailwindCSS
- **RSS Feed**: Generate personal RSS feeds for consumption in podcast apps or readers
- **Session Persistence**: Maintains login sessions between runs
- **Deduplication**: Automatically skips articles that have already been scraped

## 🧩 Tech Stack

- **Backend**: Go 1.23+
- **Web Framework**: Chi (routing) + Templ (templates) + HTMX
- **Database**: PostgreSQL 17
- **Scraper**: Rod (headless browser automation)
- **Deployment**: Docker Compose

## 🚀 Quick Start

### Prerequisites

- Docker and Docker Compose installed
- Go 1.23+ (for local development)
- Gasetten.se account credentials

### 1. Clone and Configure

```bash
cd /Users/tim/dev/kiln

# Copy environment template
cp .env.example .env

# Edit .env with your credentials
nano .env
```

Update `.env` with your Gasetten credentials:

```env
DATABASE_URL=postgres://postgres:postgres@db:5432/kiln?sslmode=disable
GASETTEN_USER=your_username
GASETTEN_PASS=your_password
PORT=8080
FEED_TITLE=My Personal Kiln Feed
FEED_DESCRIPTION=Articles from Gasetten
FEED_LINK=http://localhost:8080
FEED_AUTHOR=Your Name
```

### 2. Start with Docker

```bash
# Build and start containers
make docker-build

# Or manually:
docker-compose up -d --build

# View logs
make docker-logs
```

### 3. Access the Application

- **Web UI**: http://localhost:8080
- **RSS Feed**: http://localhost:8080/rss.xml
- **Health Check**: http://localhost:8080/health

## 📖 Usage

### Scraping Articles

1. Open http://localhost:8080 in your browser
2. Click the "Scrape New Articles" button
3. The scraper will log in to Gasetten and fetch new articles
4. New articles appear in the list automatically (via HTMX)

### RSS Feed

Access your personal RSS feed at:

```
http://localhost:8080/rss.xml
```

Add this URL to your favorite RSS reader or podcast app.

## 🛠️ Development

### Local Development Setup

```bash
# Install dependencies
make deps

# Install templ CLI
make install-tools

# Generate templates
make templ

# Run locally (requires PostgreSQL running)
make run
```

### Project Structure

```
kiln/
├── cmd/kiln/              # Application entry point
│   └── main.go
├── internal/
│   ├── config/           # Configuration management
│   ├── database/         # Database models and queries
│   ├── scraper/          # Rod-based web scraper
│   ├── server/           # HTTP server and handlers
│   └── feed/             # RSS feed generation
├── migrations/           # SQL migrations
├── docker-compose.yml    # Docker orchestration
├── Dockerfile           # Application container
└── Makefile            # Development commands
```

### Database Schema

```sql
CREATE TABLE articles (
  id SERIAL PRIMARY KEY,
  source TEXT NOT NULL DEFAULT 'gasetten',
  url TEXT UNIQUE NOT NULL,
  title TEXT,
  author TEXT,
  published_at TIMESTAMP,
  content_html TEXT,
  content_text TEXT,
  created_at TIMESTAMP NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMP NOT NULL DEFAULT NOW()
);
```

### Useful Commands

```bash
# Build the application
make build

# Run tests
make test

# Format code
make fmt

# Tidy dependencies
make tidy

# Open database shell
make db-shell

# Stop containers
make docker-down

# Clean build artifacts
make clean
```

## 🔒 Security Notes

- **Never commit `.env`** - it contains your credentials
- Login sessions are stored in `~/.gasetten/sessions`
- All passwords are handled securely (never logged or exposed)
- When deploying remotely, use HTTPS and secure environment variable management

## 🐛 Troubleshooting

### Scraper Issues

**Problem**: Login fails or articles aren't found

**Solution**: Gasetten's HTML structure may have changed. Update the selectors in:
- `internal/scraper/scraper.go` (lines with `page.MustElement()`)

### Database Connection Issues

**Problem**: Can't connect to database

**Solution**:
```bash
# Check if database is running
docker-compose ps

# View database logs
docker-compose logs db

# Restart database
docker-compose restart db
```

### Port Already in Use

**Problem**: Port 8080 is already in use

**Solution**: Change `PORT` in `.env` to another port (e.g., 8081)

## 🗺️ Roadmap

### Current: MVP ✅
- [x] Automated login and scraping
- [x] PostgreSQL storage
- [x] Web UI with HTMX
- [x] RSS feed generation
- [x] Docker deployment

### Future Stages

#### Stage 2: Audio Generation
- Text-to-speech conversion (OpenAI TTS or ElevenLabs)
- Audio file management
- Podcast feed support

#### Stage 3: Multi-Source Support
- Additional website scrapers
- RSS feed aggregation
- Source prioritization

#### Stage 4: AI Enhancement
- Article summarization
- Auto-tagging and categorization
- Topic extraction

#### Stage 5: Mobile & Sync
- Progressive Web App (PWA)
- Mobile-friendly interface
- Cloud sync options

## 📝 License

This is a personal project. Use at your own discretion and respect Gasetten's terms of service.

## 🤝 Contributing

This is a personal tool, but suggestions and improvements are welcome! Open an issue or submit a pull request.

## 💡 Tips

1. **Scraping Frequency**: Start with manual scraping to avoid overwhelming the server
2. **RSS Readers**: Works great with Feedly, Reeder, or Apple Podcasts (for future audio support)
3. **Customization**: All HTML selectors can be adjusted in `scraper.go` if Gasetten changes their layout
4. **Backup**: Database is stored in Docker volume `db_data` - back it up regularly if needed

## 📧 Support

For issues or questions, check the troubleshooting section or review the code comments.

---

**Built with ❤️ using Go, Rod, Chi, and Templ**

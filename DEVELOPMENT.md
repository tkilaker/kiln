# 🛠️ Development Guide

## Two Development Workflows

### **Option 1: Local Development (⚡ FAST - Recommended)**

Run Go app locally, only PostgreSQL in Docker. **No rebuilds needed!**

```bash
# 1. Start only the database
docker-compose up -d db

# 2. Edit .env with your real Gasetten credentials
nano .env

# 3. Run the app locally
go run ./cmd/kiln

# OR use the Makefile
make run
```

**Benefits:**
- ✅ Instant code changes - just restart with Ctrl+C and run again
- ✅ Full IDE debugging support
- ✅ See logs directly in terminal
- ✅ Fast iteration cycle

**When you make changes:**
```bash
# For code changes - just restart
Ctrl+C
go run ./cmd/kiln

# For template changes - regenerate first
make templ
go run ./cmd/kiln
```

---

### **Option 2: Full Docker (🐋 SLOW - For production testing)**

Everything in Docker. **Requires rebuild on every code change.**

```bash
# 1. Edit .env with your real credentials
nano .env

# 2. Build and start everything
make docker-build

# 3. View logs
make docker-logs
```

**When you make changes:**
```bash
# Must rebuild the entire Docker image
docker-compose down
make docker-build

# OR just restart if only .env changed
docker-compose restart app
```

**Benefits:**
- ✅ Matches production exactly
- ✅ Tests the full containerized setup

**Drawbacks:**
- ❌ Slow - 30+ seconds per rebuild
- ❌ Can't use debugger easily
- ❌ More complex log viewing

---

## 🐛 Debugging

### Local Development Debugging

```bash
# Just add print statements and run
go run ./cmd/kiln

# Or use Delve debugger
go install github.com/go-delve/delve/cmd/dlv@latest
dlv debug ./cmd/kiln
```

### Docker Development Debugging

```bash
# View logs in real-time
docker-compose logs -f app

# Check if container is running
docker-compose ps

# Execute commands inside container
docker-compose exec app /bin/sh

# Check environment variables
docker-compose exec app env | grep GASETTEN
```

### Common Issues

**1. Scraper hangs after "Logging into Gasetten"**
- Check your credentials in `.env`
- The website selectors may need adjustment
- Try running locally to see detailed errors

**2. "Cannot connect to database"**
```bash
# Make sure database is running
docker-compose ps
docker-compose up -d db
```

**3. Port already in use**
```bash
# Find what's using port 8080
lsof -i :8080

# Or change PORT in .env
PORT=8081
```

---

## 📝 Typical Development Session

### Local Development (Recommended)

```bash
# Terminal 1 - Database
docker-compose up -d db

# Terminal 2 - Application
go run ./cmd/kiln

# Make changes to code...
# Press Ctrl+C in Terminal 2
# Run again
go run ./cmd/kiln

# If you changed templates
make templ
go run ./cmd/kiln
```

### Docker Development

```bash
# Start everything
make docker-build

# Terminal 1 - Logs
make docker-logs

# Make changes...
# Stop and rebuild
docker-compose down
make docker-build
```

---

## 🔧 Useful Commands

```bash
# Local Development
make run              # Run app locally
make build            # Build binary
make templ            # Regenerate templates
go test ./...         # Run tests

# Docker Development
make docker-build     # Build and start
make docker-logs      # View logs
make docker-down      # Stop containers
docker-compose restart app  # Restart just the app

# Database
make db-shell         # Open PostgreSQL shell
docker-compose exec db psql -U postgres -d kiln -c 'SELECT * FROM articles;'

# Cleanup
make clean            # Remove build artifacts
docker-compose down -v  # Remove volumes (⚠️ deletes data)
```

---

## 🎯 Recommended Setup

**For active development:**
1. Use **Local Development** (Option 1)
2. Run `docker-compose up -d db` once
3. Edit code and run `go run ./cmd/kiln` repeatedly
4. Only use full Docker to test the final build

**For testing scrapers:**
1. Edit `.env` with real credentials
2. Run locally: `go run ./cmd/kiln`
3. Watch logs directly in terminal
4. Add more `log.Println()` statements as needed
5. Adjust selectors in `internal/scraper/scraper.go`

---

## 🔍 Testing the Scraper

```bash
# 1. Make sure you have real credentials in .env
nano .env

# 2. Run locally to see detailed output
go run ./cmd/kiln

# 3. In browser, click "Scrape New Articles"

# 4. Watch the terminal for detailed logs
# You'll see:
# - Login attempts
# - Article discovery
# - Parsing attempts
# - Any errors

# 5. If selectors are wrong, edit:
nano internal/scraper/scraper.go
# Search for "MustElement" calls
# Update selectors to match Gasetten's HTML
```

---

## 🚀 Production Deployment

When ready to deploy:

```bash
# Test the Docker build works
make docker-build

# Push to your server
git push

# On server
docker-compose up -d --build
```

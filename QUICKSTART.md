# 🚀 Kiln Quick Start Guide

Get Kiln up and running in under 5 minutes!

## Step 1: Configure Environment

```bash
# Copy the environment template
cp .env.example .env

# Edit with your Gasetten credentials
nano .env
```

Replace these values in `.env`:
```env
GASETTEN_USER=your_actual_username
GASETTEN_PASS=your_actual_password
```

## Step 2: Start the Application

```bash
# Build and start with Docker Compose
docker-compose up -d --build

# Or use the Makefile
make docker-build
```

Wait for the containers to start (~30 seconds for first build).

## Step 3: Verify It's Running

```bash
# Check container status
docker-compose ps

# View logs
docker-compose logs -f app
```

You should see:
```
app_1  | Starting Kiln...
app_1  | Connected to database
app_1  | Initialized scraper
app_1  | Server starting on http://localhost:8080
```

## Step 4: Access the Web Interface

Open your browser to: **http://localhost:8080**

You should see the Kiln interface with:
- 🔥 Kiln logo/header
- "Scrape New Articles" button
- Empty article list (initially)

## Step 5: Scrape Your First Articles

1. Click the **"Scrape New Articles"** button
2. Watch the browser console or logs for scraping progress
3. Articles will appear in the list once scraping completes

```bash
# Monitor scraping in real-time
docker-compose logs -f app
```

## Step 6: Access Your RSS Feed

Your personal RSS feed is available at:

**http://localhost:8080/rss.xml**

Add this URL to your favorite RSS reader:
- Feedly
- Reeder
- Apple News
- Any RSS reader app

## Troubleshooting

### Can't Connect to Database

```bash
# Restart the database container
docker-compose restart db

# Or restart everything
docker-compose down && docker-compose up -d
```

### Port 8080 Already in Use

Edit `.env` and change:
```env
PORT=8081
```

Then restart:
```bash
docker-compose down && docker-compose up -d
```

### Scraper Not Finding Articles

The HTML selectors may need adjustment. Check:
- `internal/scraper/scraper.go` (lines 130-180)
- Update selectors based on Gasetten's current HTML structure

### Login Fails

Verify your credentials in `.env`:
```bash
cat .env | grep GASETTEN
```

Check logs for authentication errors:
```bash
docker-compose logs app | grep -i login
```

## Common Commands

```bash
# View logs
docker-compose logs -f

# Stop containers
docker-compose down

# Restart containers
docker-compose restart

# Rebuild after code changes
docker-compose up -d --build

# Access database shell
docker-compose exec db psql -U postgres -d kiln

# View database tables
docker-compose exec db psql -U postgres -d kiln -c '\dt'

# Count articles
docker-compose exec db psql -U postgres -d kiln -c 'SELECT COUNT(*) FROM articles;'
```

## Next Steps

1. **Set up automatic scraping**: Add a cron job or use a scheduler
2. **Customize the UI**: Edit `internal/server/templates.templ`
3. **Adjust scraping logic**: Modify `internal/scraper/scraper.go`
4. **Add more sources**: Extend the scraper for other websites

## Need Help?

Check the full documentation in `README.md` or review the troubleshooting section.

---

**That's it! You're ready to start building your personal article archive with Kiln.** 🔥

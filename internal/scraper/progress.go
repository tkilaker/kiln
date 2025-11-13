package scraper

import (
	"sync"
	"time"
)

// ProgressStatus represents the current status of a scraping operation
type ProgressStatus string

const (
	StatusStarting  ProgressStatus = "starting"
	StatusLoggingIn ProgressStatus = "logging_in"
	StatusScraping  ProgressStatus = "scraping"
	StatusCompleted ProgressStatus = "completed"
	StatusFailed    ProgressStatus = "failed"
	StatusCancelled ProgressStatus = "cancelled"
)

// ProgressUpdate represents a single progress update
type ProgressUpdate struct {
	Status        ProgressStatus
	Message       string
	CurrentItem   int
	TotalItems    int
	ArticlesAdded int
	NewArticleID  int // ID of newly added article (0 if none)
	Timestamp     time.Time
}

// ProgressTracker tracks the progress of a scraping operation
type ProgressTracker struct {
	mu        sync.RWMutex
	current   ProgressUpdate
	listeners []chan ProgressUpdate
	active    bool
}

// NewProgressTracker creates a new progress tracker
func NewProgressTracker() *ProgressTracker {
	return &ProgressTracker{
		current: ProgressUpdate{
			Status:    StatusStarting,
			Timestamp: time.Now(),
		},
		listeners: make([]chan ProgressUpdate, 0),
		active:    false,
	}
}

// Update updates the current progress
func (pt *ProgressTracker) Update(update ProgressUpdate) {
	pt.mu.Lock()
	update.Timestamp = time.Now()
	pt.current = update

	// Send to all listeners
	for _, listener := range pt.listeners {
		select {
		case listener <- update:
		default:
			// Skip if channel is full
		}
	}
	pt.mu.Unlock()
}

// UpdateStatus updates just the status and message
func (pt *ProgressTracker) UpdateStatus(status ProgressStatus, message string) {
	pt.mu.RLock()
	update := pt.current
	pt.mu.RUnlock()

	update.Status = status
	update.Message = message
	pt.Update(update)
}

// UpdateProgress updates the item counts
func (pt *ProgressTracker) UpdateProgress(current, total int, message string) {
	pt.mu.RLock()
	update := pt.current
	pt.mu.RUnlock()

	update.CurrentItem = current
	update.TotalItems = total
	update.Message = message
	pt.Update(update)
}

// IncrementArticlesAdded increments the count of articles successfully added
func (pt *ProgressTracker) IncrementArticlesAdded() {
	pt.mu.Lock()
	pt.current.ArticlesAdded++
	update := pt.current
	pt.mu.Unlock()

	pt.Update(update)
}

// GetCurrent returns the current progress
func (pt *ProgressTracker) GetCurrent() ProgressUpdate {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.current
}

// Subscribe creates a new listener channel for progress updates
func (pt *ProgressTracker) Subscribe() chan ProgressUpdate {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	ch := make(chan ProgressUpdate, 10)
	pt.listeners = append(pt.listeners, ch)

	// Send current state immediately
	ch <- pt.current

	return ch
}

// Unsubscribe removes a listener channel
func (pt *ProgressTracker) Unsubscribe(ch chan ProgressUpdate) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	for i, listener := range pt.listeners {
		if listener == ch {
			pt.listeners = append(pt.listeners[:i], pt.listeners[i+1:]...)
			close(ch)
			break
		}
	}
}

// SetActive marks the tracker as active or inactive
func (pt *ProgressTracker) SetActive(active bool) {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	pt.active = active
}

// IsActive returns whether the tracker is currently active
func (pt *ProgressTracker) IsActive() bool {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.active
}

// Reset resets the progress tracker to initial state
func (pt *ProgressTracker) Reset() {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	pt.current = ProgressUpdate{
		Status:    StatusStarting,
		Timestamp: time.Now(),
	}
	pt.active = false

	// Close all listener channels
	for _, listener := range pt.listeners {
		close(listener)
	}
	pt.listeners = make([]chan ProgressUpdate, 0)
}

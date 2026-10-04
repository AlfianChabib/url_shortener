package analytics

import (
	"context"
	"log"
	"sync"
	"time"
	"url_shortener/internal/model/domain"
	"url_shortener/internal/repository"
)

// Config defines the parameters for the asynchronous analytics worker pool.
type Config struct {
	BufferSize    int
	BatchSize     int
	FlushInterval time.Duration
}

// DefaultConfig provides sensible defaults matching PRD Section 3.3.
func DefaultConfig() Config {
	return Config{
		BufferSize:    10000,
		BatchSize:     100,
		FlushInterval: 200 * time.Millisecond,
	}
}

// WorkerPool defines the contract for processing click events asynchronously.
type WorkerPool interface {
	Enqueue(event *domain.ClickEvent) bool
	Start()
	Stop()
	Flush()
}

type workerPoolImpl struct {
	repo       repository.LinkRepository
	cfg        Config
	events     chan *domain.ClickEvent
	flushChan  chan chan struct{}
	stopChan   chan struct{}
	wg         sync.WaitGroup
	started    bool
	mu         sync.Mutex
}

// NewWorkerPool creates a new asynchronous analytics worker pool.
func NewWorkerPool(repo repository.LinkRepository, cfg ...Config) WorkerPool {
	c := DefaultConfig()
	if len(cfg) > 0 {
		c = cfg[0]
	}
	if c.BufferSize <= 0 {
		c.BufferSize = 10000
	}
	if c.BatchSize <= 0 {
		c.BatchSize = 100
	}
	if c.FlushInterval <= 0 {
		c.FlushInterval = 200 * time.Millisecond
	}

	return &workerPoolImpl{
		repo:      repo,
		cfg:       c,
		events:    make(chan *domain.ClickEvent, c.BufferSize),
		flushChan: make(chan chan struct{}),
		stopChan:  make(chan struct{}),
	}
}

// Enqueue adds a click event to the buffer.
// It is non-blocking: if the buffer is full, it drops the event and returns false
// to ensure zero performance degradation on the client redirection path.
func (w *workerPoolImpl) Enqueue(event *domain.ClickEvent) bool {
	if event == nil {
		return false
	}

	select {
	case w.events <- event:
		return true
	default:
		log.Printf("[WARN] Analytics event buffer is full (capacity %d), dropped event for %s", w.cfg.BufferSize, event.ShortCode)
		return false
	}
}

// Start begins the asynchronous batching worker goroutine.
func (w *workerPoolImpl) Start() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.started {
		return
	}
	w.started = true

	w.wg.Add(1)
	go w.workerLoop()
}

func (w *workerPoolImpl) workerLoop() {
	defer w.wg.Done()

	ticker := time.NewTicker(w.cfg.FlushInterval)
	defer ticker.Stop()

	batch := make([]*domain.ClickEvent, 0, w.cfg.BatchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		// Write batch to repository
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := w.repo.RecordClickEvents(ctx, batch); err != nil {
			log.Printf("[ERROR] Failed to record click events batch of size %d: %v", len(batch), err)
		}
		cancel()

		// Reset batch
		batch = make([]*domain.ClickEvent, 0, w.cfg.BatchSize)
	}

	for {
		select {
		case ev := <-w.events:
			batch = append(batch, ev)
			if len(batch) >= w.cfg.BatchSize {
				flush()
			}

		case <-ticker.C:
			flush()

		case ack := <-w.flushChan:
			// Drain all currently available events in the channel
			draining := true
			for draining {
				select {
				case ev := <-w.events:
					batch = append(batch, ev)
				default:
					draining = false
				}
			}
			flush()
			close(ack)

		case <-w.stopChan:
			// Drain remaining events before exit
			draining := true
			for draining {
				select {
				case ev := <-w.events:
					batch = append(batch, ev)
				default:
					draining = false
				}
			}
			flush()
			return
		}
	}
}

// Flush forces an immediate drain and flush of all pending events in the buffer.
func (w *workerPoolImpl) Flush() {
	ack := make(chan struct{})
	w.flushChan <- ack
	<-ack
}

// Stop signals the worker to finish processing and gracefully flushes all remaining items.
func (w *workerPoolImpl) Stop() {
	w.mu.Lock()
	if !w.started {
		w.mu.Unlock()
		return
	}
	w.mu.Unlock()

	close(w.stopChan)
	w.wg.Wait()
}

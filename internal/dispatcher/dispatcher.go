package dispatcher

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/cexll/swe/internal/executor"
	"github.com/cexll/swe/internal/webhook"
)

// TaskExecutor runs a webhook task
type TaskExecutor interface {
	Execute(ctx context.Context, task *webhook.Task) error
}

// Config controls dispatcher behaviour
type Config struct {
	Workers           int
	QueueSize         int
	MaxAttempts       int
	InitialBackoff    time.Duration
	BackoffMultiplier float64
	MaxBackoff        time.Duration
}

// Dispatcher serialises execution per PR and retries failed tasks with backoff
type Dispatcher struct {
	executor TaskExecutor
	cfg      Config

	queue chan *queueItem

	keyedLocks *keyedMutex

	stopCh chan struct{}
	wg     sync.WaitGroup

	once sync.Once
}

type queueItem struct {
	task    *webhook.Task
	attempt int
}

// New creates a dispatcher with the provided configuration
func New(executor TaskExecutor, cfg Config) *Dispatcher {
	normalized := normalizeConfig(cfg)
	d := &Dispatcher{
		executor:   executor,
		cfg:        normalized,
		queue:      make(chan *queueItem, normalized.QueueSize),
		keyedLocks: newKeyedMutex(),
		stopCh:     make(chan struct{}),
	}
	d.startWorkers()
	return d
}

func normalizeConfig(cfg Config) Config {
	if cfg.Workers <= 0 {
		cfg.Workers = 4
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = cfg.Workers * 4
	}
	if cfg.MaxAttempts <= 0 {
		cfg.MaxAttempts = 3
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = 15 * time.Second
	}
	if cfg.BackoffMultiplier <= 1 {
		cfg.BackoffMultiplier = 2
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = 5 * time.Minute
	}
	return cfg
}

func (d *Dispatcher) startWorkers() {
	for i := 0; i < d.cfg.Workers; i++ {
		d.wg.Add(1)
		go d.worker()
	}
}

// Enqueue queues a new task for execution
func (d *Dispatcher) Enqueue(task *webhook.Task) error {
	if task == nil {
		return errors.New("dispatcher enqueue: task is nil")
	}

	select {
	case <-d.stopCh:
		return webhook.ErrQueueClosed
	default:
	}

	select {
	case d.queue <- &queueItem{task: task, attempt: 1}:
		return nil
	default:
		return webhook.ErrQueueFull
	}
}

func (d *Dispatcher) worker() {
	defer d.wg.Done()

	for {
		select {
		case <-d.stopCh:
			return
		case item, ok := <-d.queue:
			if !ok {
				return
			}
			d.process(item)
		}
	}
}

func (d *Dispatcher) process(item *queueItem) {
	task := item.task
	task.Attempt = item.attempt

	key := fmt.Sprintf("%s#%d", task.Repo, task.Number)
	d.keyedLocks.Lock(key)

	ctx := context.Background()
	err := d.executor.Execute(ctx, task)

	d.keyedLocks.Unlock(key)

	if err != nil {
		log.Printf("Task %s attempt %d failed: %v", key, item.attempt, err)
		if executor.IsNonRetryable(err) {
			log.Printf("Task %s attempt %d marked non-retryable; no further attempts", key, item.attempt)
			return
		}
		d.handleRetry(item, err)
		return
	}

	log.Printf("Task %s attempt %d succeeded", key, item.attempt)
}

func (d *Dispatcher) handleRetry(item *queueItem, execErr error) {
	if item.attempt >= d.cfg.MaxAttempts {
		log.Printf("Task %s#%d exceeded max attempts (%d): %v", item.task.Repo, item.task.Number, d.cfg.MaxAttempts, execErr)
		return
	}

	nextAttempt := item.attempt + 1
	delay := d.backoffDuration(nextAttempt)
	log.Printf("Scheduling retry %d for %s#%d in %s", nextAttempt, item.task.Repo, item.task.Number, delay)

	go func() {
		timer := time.NewTimer(delay)
		defer timer.Stop()

		select {
		case <-timer.C:
			d.enqueueRetry(&queueItem{
				task:    item.task,
				attempt: nextAttempt,
			})
		case <-d.stopCh:
			return
		}
	}()
}

func (d *Dispatcher) enqueueRetry(item *queueItem) {
	for {
		select {
		case <-d.stopCh:
			return
		case d.queue <- item:
			return
		default:
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func (d *Dispatcher) backoffDuration(attempt int) time.Duration {
	backoff := float64(d.cfg.InitialBackoff)
	for i := 1; i < attempt; i++ {
		backoff *= d.cfg.BackoffMultiplier
		if backoff >= float64(d.cfg.MaxBackoff) {
			return d.cfg.MaxBackoff
		}
	}
	return time.Duration(backoff)
}

// Shutdown gracefully stops the dispatcher
func (d *Dispatcher) Shutdown(ctx context.Context) {
	d.once.Do(func() {
		close(d.stopCh)
	})

	done := make(chan struct{})
	go func() {
		defer close(done)
		d.wg.Wait()
	}()

	select {
	case <-ctx.Done():
		return
	case <-done:
		return
	}
}

type keyedMutex struct {
	mu    sync.Mutex
	locks map[string]*sync.Mutex
}

func newKeyedMutex() *keyedMutex {
	return &keyedMutex{
		locks: make(map[string]*sync.Mutex),
	}
}

func (k *keyedMutex) Lock(key string) {
	k.mu.Lock()
	m, ok := k.locks[key]
	if !ok {
		m = &sync.Mutex{}
		k.locks[key] = m
	}
	k.mu.Unlock()

	m.Lock()
}

func (k *keyedMutex) Unlock(key string) {
	k.mu.Lock()
	m, ok := k.locks[key]
	k.mu.Unlock()

	if !ok {
		return
	}

	m.Unlock()
}

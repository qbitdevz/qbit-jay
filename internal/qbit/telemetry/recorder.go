// Package telemetry is qbit-jay's local-only usage telemetry (spec 10.1).
//
// It records search, open, reformulation, context and save events in a
// separate SQLite file, <data dir>/telemetry.db. The file never leaves the
// machine, is not part of the main database, Git Sync or Cloud, and is
// written by a background goroutine so recording can never fail or slow the
// operation it observes. QBIT_JAY_TELEMETRY=off (or 0/false) disables it and
// then no file is created or written.
package telemetry

import (
	"crypto/rand"
	"encoding/hex"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/qbitdevz/qbit-jay/internal/qbit/env"
)

const (
	// DefaultMaxAge is the retention window.
	DefaultMaxAge = 90 * 24 * time.Hour
	// DefaultMaxBytes is the size cap of telemetry.db (assumption: 50 MB).
	DefaultMaxBytes int64 = 50 << 20

	reformulationWindow = 60 * time.Second
	openWindow          = 10 * time.Minute
	queueSize           = 512
	pruneEvery          = 500
	maxTrackedInjected  = 200
	maxText             = 500
)

// Enabled reports whether telemetry is on: it is, unless QBIT_JAY_TELEMETRY
// is off, 0 or false (ENGRAM_TELEMETRY is the upstream-name fallback).
func Enabled() bool {
	switch strings.ToLower(strings.TrimSpace(env.Get("TELEMETRY"))) {
	case "off", "0", "false":
		return false
	}
	return true
}

// Options configure a Recorder. Zero values take the defaults.
type Options struct {
	DataDir  string
	MaxAge   time.Duration
	MaxBytes int64
	Now      func() time.Time
}

type event struct {
	ts       time.Time
	kind     string
	project  string
	query    string
	mode     string
	results  int
	ids      string
	latency  time.Duration
	bytes    int
	tokens   int
	errText  string
	extra    string
	injected []injectedItem // context events only
	flush    chan struct{}  // non-nil: a flush marker, not an event
}

type injectedItem struct{ title, reason string }

type recentSearch struct {
	ts    time.Time
	query string
	ids   map[int64]bool
}

// Recorder implements the MCP tool observer and persists events.
type Recorder struct {
	opts    Options
	session string
	ch      chan event
	done    chan struct{}
	closed  atomic.Bool

	dropped atomic.Int64
	errs    atomic.Int64

	mu       sync.Mutex
	searches []recentSearch
	pending  map[string]string // injected title -> reason, awaiting a read/update
}

// New starts a Recorder. It returns nil when telemetry is disabled; in that
// case nothing is created on disk. The database is opened by the background
// goroutine, so New never blocks on the file system.
func New(opts Options) *Recorder {
	if !Enabled() || opts.DataDir == "" {
		return nil
	}
	if opts.MaxAge <= 0 {
		opts.MaxAge = DefaultMaxAge
	}
	if opts.MaxBytes == 0 {
		opts.MaxBytes = DefaultMaxBytes
	}
	if opts.Now == nil {
		opts.Now = time.Now
	}
	var b [8]byte
	_, _ = rand.Read(b[:])
	r := &Recorder{
		opts:    opts,
		session: hex.EncodeToString(b[:]),
		ch:      make(chan event, queueSize),
		done:    make(chan struct{}),
		pending: map[string]string{},
	}
	go r.run()
	return r
}

// Dropped counts events lost because the queue was full or the recorder was
// closed; Errors counts failed writes. Neither is ever shown to MCP clients.
func (r *Recorder) Dropped() int64 { return r.dropped.Load() }
func (r *Recorder) Errors() int64  { return r.errs.Load() }

// Flush waits until every event queued so far is written (or failed). It
// exists for tests and for Close.
func (r *Recorder) Flush() {
	if r == nil || r.closed.Load() {
		return
	}
	marker := make(chan struct{})
	select {
	case r.ch <- event{flush: marker}:
		select {
		case <-marker:
		case <-time.After(5 * time.Second):
		}
	case <-time.After(time.Second):
	}
}

// Close flushes and stops the writer.
func (r *Recorder) Close() {
	if r == nil {
		return
	}
	r.Flush()
	if r.closed.CompareAndSwap(false, true) {
		close(r.ch)
		select {
		case <-r.done:
		case <-time.After(5 * time.Second):
		}
	}
}

func (r *Recorder) enqueue(e event) {
	defer func() {
		if recover() != nil { // send on closed channel
			r.dropped.Add(1)
		}
	}()
	if r.closed.Load() {
		r.dropped.Add(1)
		return
	}
	select {
	case r.ch <- e:
	default:
		r.dropped.Add(1)
	}
}

func (r *Recorder) run() {
	defer close(r.done)
	path := Path(r.opts.DataDir)
	db, err := openDB(path, true)
	if err != nil {
		r.errs.Add(1)
		db = nil
	} else if err := prune(db, path, r.opts.Now(), r.opts.MaxAge, r.opts.MaxBytes); err != nil {
		r.errs.Add(1)
	}
	if db != nil {
		defer db.Close()
	}
	r.loop(db, path)
}

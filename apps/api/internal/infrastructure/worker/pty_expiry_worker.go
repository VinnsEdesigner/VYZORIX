// Package worker provides background workers.
package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/VinnsEdesigner/vyzorix/apps/api/internal/infrastructure/serverlock"
	hub "github.com/VinnsEdesigner/vyzorix/apps/api/internal/ws"
)

// DefaultPTYExpiryInterval is how often idle terminal sessions are swept.
const DefaultPTYExpiryInterval = 5 * time.Minute

// PTYExpiryWorker closes terminal sessions that have been idle past their TTL
// and prunes old cast recordings.
//
// Without it a forgotten browser tab holds a device's shell open indefinitely:
// the registry enforces the TTL, but nothing would call it. Sessions are also
// reaped on disconnect, so this worker covers the case where both sides stay
// connected but idle.
type PTYExpiryWorker struct {
	relay           *hub.PTYRelay
	lockSvc         *serverlock.Service
	logger          *slog.Logger
	stopCh          chan struct{}
	doneCh          chan struct{}
	recordingDir    string
	interval        time.Duration
	recordingMaxAge time.Duration
}

// NewPTYExpiryWorker creates the worker. recordingDir may be empty, in which
// case recording pruning is skipped.
func NewPTYExpiryWorker(
	relay *hub.PTYRelay,
	recordingDir string,
	lockSvc *serverlock.Service,
	logger *slog.Logger,
	interval time.Duration,
) *PTYExpiryWorker {
	if interval <= 0 {
		interval = DefaultPTYExpiryInterval
	}

	return &PTYExpiryWorker{
		relay:           relay,
		recordingDir:    recordingDir,
		lockSvc:         lockSvc,
		logger:          logger,
		interval:        interval,
		recordingMaxAge: hub.DefaultPTYRecordingMaxAge,
		stopCh:          make(chan struct{}),
		doneCh:          make(chan struct{}),
	}
}

// Start runs the worker in the background, recovering from a panic by
// restarting rather than leaving the fleet without session reaping.
func (w *PTYExpiryWorker) Start() {
	go func() {
		defer func() {
			if r := recover(); r != nil {
				if w.logger != nil {
					w.logger.Error("pty expiry worker panicked, restarting", "panic", r)
				}

				w.Start()
			}
		}()
		w.run()
	}()

	if w.logger != nil {
		w.logger.Info("pty expiry worker started", "interval", w.interval.String())
	}
}

// Stop signals the worker to exit and waits for it.
func (w *PTYExpiryWorker) Stop() {
	close(w.stopCh)
	<-w.doneCh

	if w.logger != nil {
		w.logger.Info("pty expiry worker stopped")
	}
}

// Healthy reports whether the worker is still running.
func (w *PTYExpiryWorker) Healthy() bool {
	select {
	case <-w.doneCh:
		return false
	default:
		return true
	}
}

func (w *PTYExpiryWorker) run() {
	defer close(w.doneCh)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-w.stopCh:
			return
		case <-ticker.C:
			w.sweep()
		}
	}
}

// sweep closes expired sessions and prunes old recordings. It takes the server
// lock so that, if the API runs as more than one replica, only one instance
// sweeps; the others would each close the same sessions otherwise.
func (w *PTYExpiryWorker) sweep() {
	if w.relay == nil {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if w.lockSvc != nil {
		acquired, err := w.lockSvc.Acquire(ctx, "pty-expiry-worker", "primary", w.interval)
		if err != nil || !acquired {
			return
		}
		defer func() { _ = w.lockSvc.Release(ctx, "pty-expiry-worker", "primary") }()
	}

	// CloseSession notifies the operator through the session's own endpoint, so
	// a nil operator here is correct: the relay already knows how to reach it.
	expired := w.relay.ExpireSessions(nil)
	if expired > 0 && w.logger != nil {
		w.logger.Info("expired idle pty sessions", "count", expired)
	}

	if w.recordingDir != "" {
		removed, err := hub.SweepRecordings(w.recordingDir, w.recordingMaxAge)
		if err != nil {
			if w.logger != nil {
				w.logger.Warn("failed to prune pty recordings", "err", err)
			}
		} else if removed > 0 && w.logger != nil {
			w.logger.Info("pruned old pty recordings", "count", removed)
		}
	}
}

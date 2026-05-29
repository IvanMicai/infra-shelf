// Package shelfcore is the library boundary between the CLI binary and the web
// server. Both invoke the same functions in this package — the CLI wraps them
// with cobra commands and pretty ANSI output, the web wraps them with HTTP
// handlers. Functions return tagged errors and structured results; user-facing
// progress goes through the Reporter interface so callers control presentation.
package shelfcore

import (
	"errors"

	"github.com/IvanMicai/infra-shelf/internal/registry"
)

// Reporter receives streamed progress messages while long-running operations
// (setup, add, remove, backup, restore) execute. A nil Reporter is safe — all
// emissions become no-ops. The CLI hooks Reporter into internal/output; the
// web server typically discards messages (handlers report via HTTP response).
type Reporter interface {
	Success(msg string)
	Error(msg string)
	Info(msg string)
	Warn(msg string)
	Title(msg string)
}

// Discard is a Reporter that drops all messages.
type Discard struct{}

func (Discard) Success(string) {}
func (Discard) Error(string)   {}
func (Discard) Info(string)    {}
func (Discard) Warn(string)    {}
func (Discard) Title(string)   {}

// Engine wires up the dependencies every shelfcore operation needs: the
// registry store, the backups directory and a reporter. Construct one per
// request (web) or per process (CLI) — methods are safe for sequential use
// only, mirroring the underlying TS CLI semantics.
type Engine struct {
	Store      *registry.Store
	BackupsDir string
	Reporter   Reporter
}

// New builds an Engine. A nil reporter becomes Discard so callers can omit it.
func New(store *registry.Store, backupsDir string, reporter Reporter) *Engine {
	if reporter == nil {
		reporter = Discard{}
	}
	return &Engine{Store: store, BackupsDir: backupsDir, Reporter: reporter}
}

// Sentinel errors — callers (especially the web HTTP layer) can map these to
// status codes (404 / 409 / 400) instead of regex-matching stdout.
var (
	ErrAppNotFound         = errors.New("app not found")
	ErrAppAlreadyExists    = errors.New("app already exists")
	ErrNoServices          = errors.New("at least one service is required")
	ErrServiceNotAttached  = errors.New("service is not attached to this app")
	ErrNotDetachable       = errors.New("service is not detachable")
	ErrContainerNotRunning = errors.New("container is not running")
	ErrNonBackupable       = errors.New("service has no backup support")
	ErrBackupNotFound      = errors.New("no backup found")
)

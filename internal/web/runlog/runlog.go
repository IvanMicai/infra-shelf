// Package runlog provides a shelfcore.Reporter that captures all emitted
// messages into a single ANSI-free string. The web UI uses it to surface a
// human-readable run summary in the schedule history without parsing stdout.
package runlog

import (
	"strings"
	"sync"
)

// Buffer is a Reporter implementation that appends every message (prefixed by
// kind) to an internal slice. String() joins them with newlines and trims
// trailing whitespace.
type Buffer struct {
	mu    sync.Mutex
	lines []string
}

func New() *Buffer { return &Buffer{} }

func (b *Buffer) Success(msg string) { b.add("✔", msg) }
func (b *Buffer) Error(msg string)   { b.add("✘", msg) }
func (b *Buffer) Info(msg string)    { b.add("ℹ", msg) }
func (b *Buffer) Warn(msg string)    { b.add("⚠", msg) }
func (b *Buffer) Title(msg string)   { b.add("›", msg) }

func (b *Buffer) add(prefix, msg string) {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, prefix+" "+msg)
}

// String returns the accumulated log, newline-separated, with no trailing
// whitespace. Safe to call from any goroutine.
func (b *Buffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return strings.TrimSpace(strings.Join(b.lines, "\n"))
}

// AppendLine adds a free-form line to the buffer. Used by callers that want
// to attach extra context (e.g. "Uploaded N files to s3://...") after a
// shelfcore call returns.
func (b *Buffer) AppendLine(line string) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	b.lines = append(b.lines, line)
}

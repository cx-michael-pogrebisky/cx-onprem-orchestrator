// Package exec runs child scanner processes: it streams their stdout/stderr with
// a per-engine prefix, mirrors output into capture buffers, scrubs secrets from
// everything that is displayed, captures the child exit code, and treats a
// non-zero exit as data (NOT a Go error) since a threshold breach is expected.
package exec

import (
	"bufio"
	"bytes"
	"io"
	"sync"
)

// Redactor replaces secret substrings in a line before display. A nil Redactor
// is a no-op.
type Redactor func([]byte) []byte

// prefixWriter writes each newline-delimited line to an underlying writer with a
// prefix, applying the redactor. It is safe for concurrent use by the stdout and
// stderr pumps.
type prefixWriter struct {
	mu     sync.Mutex
	w      io.Writer
	prefix []byte
	redact Redactor
	buf    bytes.Buffer
}

func newPrefixWriter(w io.Writer, prefix string, redact Redactor) *prefixWriter {
	return &prefixWriter{w: w, prefix: []byte(prefix), redact: redact}
}

func (p *prefixWriter) Write(b []byte) (int, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	n := len(b)
	p.buf.Write(b)
	for {
		line, err := p.buf.ReadBytes('\n')
		if err != nil {
			// No full line yet; stash the remainder back.
			p.buf.Reset()
			p.buf.Write(line)
			break
		}
		p.emit(line)
	}
	return n, nil
}

func (p *prefixWriter) emit(line []byte) {
	if p.redact != nil {
		line = p.redact(line)
	}
	if p.w == nil {
		return
	}
	_, _ = p.w.Write(p.prefix)
	_, _ = p.w.Write(line)
}

// flush writes any trailing partial line (no newline).
func (p *prefixWriter) flush() {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.buf.Len() == 0 {
		return
	}
	line := append(p.buf.Bytes(), '\n')
	p.emit(line)
	p.buf.Reset()
}

// teeCapture mirrors writes into both a capture buffer and a display writer.
type teeCapture struct {
	capture *bytes.Buffer
	display io.Writer
}

func (t *teeCapture) Write(b []byte) (int, error) {
	t.capture.Write(b)
	if t.display != nil {
		_, _ = t.display.Write(b)
	}
	return len(b), nil
}

// scanLines is a helper to read a reader line by line into a writer (used when a
// caller wants explicit line pumping); retained for completeness.
func scanLines(r io.Reader, w io.Writer) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		_, _ = w.Write(append(sc.Bytes(), '\n'))
	}
}

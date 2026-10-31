package agent

import (
	"io"
	"strings"
	"testing"
	"time"
)

// TestIdleTimeoutBodyCutsStalledStream proves the wrapper actually unblocks a
// Read that never returns data — the exact shape that froze the engine on
// 2026-10-14 (a gateway sent one SSE frame then held the connection open).
func TestIdleTimeoutBodyCutsStalledStream(t *testing.T) {
	pr, pw := io.Pipe() // never written to → Read blocks forever
	body := newIdleTimeoutBody(pr, 300*time.Millisecond)
	defer body.Close()

	buf := make([]byte, 8)
	done := make(chan error, 1)
	go func() {
		_, err := body.Read(buf)
		done <- err
	}()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("stalled read returned nil error; wrapper did not time out")
		}
		if !strings.Contains(err.Error(), "stream stalled") {
			t.Fatalf("want stall error, got %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("read never returned — wrapper failed to unblock a stalled stream")
	}
	_ = pw
}

// TestIdleTimeoutBodyKeepsStreaming verifies a stream that keeps producing data
// is not cut off, so a legitimate long generation survives.
func TestIdleTimeoutBodyKeepsStreaming(t *testing.T) {
	pr, pw := io.Pipe()
	body := newIdleTimeoutBody(pr, 250*time.Millisecond)
	defer body.Close()

	go func() {
		for i := 0; i < 8; i++ {
			_, _ = pw.Write([]byte("x"))
			time.Sleep(80 * time.Millisecond) // faster than the idle window
		}
		pw.Close()
	}()

	total := 0
	buf := make([]byte, 4)
	for {
		n, err := body.Read(buf)
		total += n
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("active stream was cut short after %d bytes: %v", total, err)
		}
	}
	if total != 8 {
		t.Fatalf("read %d bytes, want 8", total)
	}
}

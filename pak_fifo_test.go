//go:build unix

package larian

import (
	"errors"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestOpenFileRefusesAFIFOWithoutBlocking(t *testing.T) {
	t.Parallel()

	fifo := filepath.Join(t.TempDir(), "x.pak")
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Skipf("no FIFOs here: %v", err)
	}

	done := make(chan error, 1)

	go func() { _, err := OpenFile(fifo); done <- err }()

	select {
	case err := <-done:
		if !errors.Is(err, ErrInvalid) {
			t.Fatalf("err = %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OpenFile blocked on a FIFO")
	}
}

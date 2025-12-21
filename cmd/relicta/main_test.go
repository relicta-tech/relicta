package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
	"testing"
)

type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func (b *lockedBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Len()
}

func TestRun_Success(t *testing.T) {
	out := &lockedBuffer{}
	code := run(context.Background(), nil, func(context.Context) error { return nil }, func() {}, out, func(int) {})
	if code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if out.Len() != 0 {
		t.Fatalf("stderr not empty: %q", out.String())
	}
}

func TestRun_Error(t *testing.T) {
	out := &lockedBuffer{}
	code := run(context.Background(), nil, func(context.Context) error {
		return errors.New("boom")
	}, func() {}, out, func(int) {})
	if code != 1 {
		t.Fatalf("exit code = %d, want 1", code)
	}
	if !strings.Contains(out.String(), "Error: boom") {
		t.Fatalf("stderr = %q, want error output", out.String())
	}
}

func TestRun_ContextCanceled(t *testing.T) {
	out := &lockedBuffer{}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	code := run(ctx, nil, func(context.Context) error {
		return errors.New("interrupted")
	}, func() {}, out, func(int) {})
	if code != 130 {
		t.Fatalf("exit code = %d, want 130", code)
	}
	if !strings.Contains(out.String(), "Operation canceled") {
		t.Fatalf("stderr = %q, want cancel output", out.String())
	}
}

func TestRun_HandlesSignal(t *testing.T) {
	out := &lockedBuffer{}
	sigChan := make(chan os.Signal, 1)
	go func() {
		sigChan <- os.Interrupt
	}()

	code := run(context.Background(), sigChan, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}, func() {}, out, func(int) {})

	if code != 130 {
		t.Fatalf("exit code = %d, want 130", code)
	}
	if !strings.Contains(out.String(), "Received signal") {
		t.Fatalf("stderr = %q, want signal output", out.String())
	}
}

func TestRun_SecondSignalForcesExit(t *testing.T) {
	out := &lockedBuffer{}
	sigChan := make(chan os.Signal, 2)

	exitCalled := make(chan int, 1)
	exitFn := func(code int) {
		exitCalled <- code
	}

	go func() {
		sigChan <- os.Interrupt
		sigChan <- os.Interrupt
	}()

	code := run(context.Background(), sigChan, func(ctx context.Context) error {
		<-ctx.Done()
		return ctx.Err()
	}, func() {}, out, exitFn)

	if code != 130 {
		t.Fatalf("exit code = %d, want 130", code)
	}
	select {
	case <-exitCalled:
	default:
		t.Fatal("expected exit function to be called on second signal")
	}
}

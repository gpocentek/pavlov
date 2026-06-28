package tailer

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestReadAndEmitNilFileDescriptor(t *testing.T) {
	tailer := &Tailer{
		File:   "/tmp/missing.log",
		events: make(chan string, 1),
	}

	tailer.readAndEmit(context.Background())
}

func TestRunShutdown(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	if err := os.WriteFile(logFile, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	tailer, err := NewTailer(logFile)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- tailer.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(logFile, []byte("line one\nline two\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(100 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Fatalf("expected context canceled or nil, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("tailer did not stop after context cancel")
	}
}

func TestRunShutdownDuringReadBurst(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")

	var lines strings.Builder
	for i := range 200 {
		lines.WriteString("line ")
		lines.WriteString(strings.Repeat("x", 32))
		lines.WriteString(" ")
		lines.WriteString(string(rune('0' + i%10)))
		lines.WriteByte('\n')
	}
	if err := os.WriteFile(logFile, []byte(lines.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	tailer, err := NewTailer(logFile)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		done <- tailer.Run(ctx)
	}()

	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(logFile, []byte(lines.String()+lines.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err != nil && err != context.Canceled {
			t.Fatalf("expected context canceled or nil, got %v", err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("tailer did not stop during read burst")
	}
}

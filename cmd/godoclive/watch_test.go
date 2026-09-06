package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
)

func TestWatchNewDirectories(t *testing.T) {
	root := t.TempDir()
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = watcher.Close() }()
	if err := addWatchDirs(watcher, root); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	changes := make(chan struct{}, 10)
	done := make(chan error, 1)
	go func() { done <- watchChanges(ctx, watcher, 20*time.Millisecond, func() { changes <- struct{}{} }) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	waitChange := func() {
		t.Helper()
		select {
		case <-changes:
		case <-time.After(5 * time.Second):
			t.Fatal("missing regeneration")
		}
	}
	// Directory creation and files can arrive before the event loop adds watches.
	dir := filepath.Join(root, "admin", "handlers")
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(dir, "routes.go")
	if err := os.WriteFile(file, []byte("package handlers\n"), 0600); err != nil {
		t.Fatal(err)
	}
	waitChange()
	// Ensure the new tree is watched, then consume any remaining creation events.
	found := false
	for _, path := range watcher.WatchList() {
		if path == dir {
			found = true
		}
	}
	if !found {
		t.Fatalf("new nested directory not watched: %v", watcher.WatchList())
	}
	time.Sleep(60 * time.Millisecond)
	for len(changes) > 0 {
		<-changes
	}
	if err := os.WriteFile(file, []byte("package handlers\n// changed\n"), 0600); err != nil {
		t.Fatal(err)
	}
	waitChange()
}

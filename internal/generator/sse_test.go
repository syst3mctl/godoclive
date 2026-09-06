package generator

import (
	"bufio"
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestReloadBroadcastAndDisconnect(t *testing.T) {
	hub := &reloadHub{clients: make(map[chan struct{}]struct{})}
	server := httptest.NewServer(hub)
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	clients := make([]*http.Response, 2)
	scanners := make([]*bufio.Scanner, 2)
	for i := range clients {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL, nil)
		if err != nil {
			t.Fatal(err)
		}
		resp, err := server.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		clients[i] = resp
		defer func() { _ = resp.Body.Close() }()
		scanners[i] = bufio.NewScanner(resp.Body)
		if !scanners[i].Scan() || scanners[i].Text() != ": connected" {
			t.Fatal("missing connection acknowledgement")
		}
	}
	expectReload := func(i int) {
		t.Helper()
		for scanners[i].Scan() {
			if scanners[i].Text() == "event: reload" {
				return
			}
		}
		t.Fatalf("client %d missed reload: %v", i, scanners[i].Err())
	}
	hub.broadcast()
	expectReload(0)
	expectReload(1)
	if err := clients[0].Body.Close(); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(time.Second)
	for {
		hub.mu.Lock()
		n := len(hub.clients)
		hub.mu.Unlock()
		if n == 1 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("disconnected client retained")
		}
		time.Sleep(time.Millisecond)
	}
	// Repeated notifications are coalesced without blocking an unread client.
	for i := 0; i < 100; i++ {
		hub.broadcast()
	}
	expectReload(1)
}

func TestServeWithSSEReportsBindFailure(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = listener.Close() }()
	if _, err := ServeWithSSE(t.TempDir(), listener.Addr().String()); err == nil {
		t.Fatal("expected occupied port error")
	}
}

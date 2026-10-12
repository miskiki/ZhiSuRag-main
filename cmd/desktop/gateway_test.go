package main

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// TestGatewayStreamsSSE proves the desktop gateway does not buffer streamed
// responses: chunks pushed by an SSE upstream must reach the client while the
// stream is still being written, not as one lump at the end.
func TestGatewayStreamsSSE(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("upstream lacks flusher")
			return
		}
		for i := 0; i < 4; i++ {
			_, _ = w.Write([]byte("data: chunk-" + string(rune('0'+i)) + "\n\n"))
			flusher.Flush()
			time.Sleep(400 * time.Millisecond)
		}
	}))
	defer upstream.Close()

	app := NewApp()
	g, err := startDesktopGateway(app, upstream.URL)
	if err != nil {
		t.Fatalf("startDesktopGateway: %v", err)
	}

	start := time.Now()
	resp, err := http.Get(g.baseURL + "/api/v1/some/chat/stream")
	if err != nil {
		t.Fatalf("gateway request: %v", err)
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "text/event-stream") {
		t.Fatalf("unexpected content type %q", ct)
	}

	scanner := bufio.NewScanner(resp.Body)
	var arrivals []time.Duration
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: chunk-") {
			arrivals = append(arrivals, time.Since(start))
		}
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("read stream: %v", err)
	}
	if len(arrivals) != 4 {
		t.Fatalf("expected 4 chunks, got %d (%v)", len(arrivals), arrivals)
	}

	// With buffering, all four chunks would share (approximately) one
	// arrival timestamp around 1.6s. With streaming they spread out.
	firstArrival := arrivals[0]
	if firstArrival > 900*time.Millisecond {
		t.Fatalf("first chunk arrived at %v — stream looks buffered", firstArrival)
	}
	span := arrivals[len(arrivals)-1] - arrivals[0]
	if span < 800*time.Millisecond {
		t.Fatalf("chunks spanned only %v — stream looks buffered (arrivals=%v)", span, arrivals)
	}
	t.Logf("streaming ok: first=%v span=%v", firstArrival, span)
}

// TestGatewayInjectsDesktopScript checks the HTML injection for the app UI.
func TestGatewayInjectsDesktopScript(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte("<!DOCTYPE html><html><head><title>t</title></head><body>hi</body></html>"))
	}))
	defer upstream.Close()

	app := NewApp()
	g, err := startDesktopGateway(app, upstream.URL)
	if err != nil {
		t.Fatalf("startDesktopGateway: %v", err)
	}

	resp, err := http.Get(g.baseURL + "/login")
	if err != nil {
		t.Fatalf("gateway request: %v", err)
	}
	defer resp.Body.Close()
	buf := make([]byte, 64<<10)
	n, _ := resp.Body.Read(buf)
	html := string(buf[:n])
	if !strings.Contains(html, "__zhishuDesktopShell") {
		t.Fatal("desktop shell script was not injected into HTML")
	}
	if !strings.Contains(html, "__ZHISHU_API_BASE__") {
		t.Fatal("API base was not injected into HTML")
	}
}

// TestGatewayDesktopEndpoints checks the native-helper endpoints.
func TestGatewayDesktopEndpoints(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	defer upstream.Close()

	app := NewApp()
	g, err := startDesktopGateway(app, upstream.URL)
	if err != nil {
		t.Fatalf("startDesktopGateway: %v", err)
	}

	resp, err := http.Get(g.baseURL + "/__desktop__/ping")
	if err != nil {
		t.Fatalf("ping: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("ping status %d", resp.StatusCode)
	}

	// RPC whitelist: unknown fn rejected, valid shape accepted.
	resp2, err := http.Post(g.baseURL+"/__desktop__/rpc", "application/json",
		strings.NewReader(`{"fn":"Nope","args":[]}`))
	if err != nil {
		t.Fatalf("rpc: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("rpc status %d", resp2.StatusCode)
	}

	state := app.GetDesktopSetupState()
	if _, ok := state["mode"]; !ok {
		t.Fatal("GetDesktopSetupState missing mode")
	}
}

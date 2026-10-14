package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// clientGateway serves the webview UI over real loopback HTTP: WebView2's
// virtual-host channel buffers streamed bodies, which would break chat
// streaming. It also hosts /__desktop__/* native-helper endpoints and
// injects the desktop shell into HTML responses. The proxy target can be
// repointed at runtime (SetTarget) when the user switches servers.
type clientGateway struct {
	app       *clientApp
	baseURL   string
	mu        sync.RWMutex
	target    *url.URL
	targetStr string
}

// SetTarget repoints the proxy at a new server (used when the user switches
// servers from the setup page without restarting the app).
func (g *clientGateway) SetTarget(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return err
	}
	g.mu.Lock()
	g.target = u
	g.targetStr = raw
	g.mu.Unlock()
	return nil
}

func (g *clientGateway) currentTarget() *url.URL {
	g.mu.RLock()
	defer g.mu.RUnlock()
	return g.target
}

func startClientGateway(app *clientApp, targetURL string) (*clientGateway, error) {
	g := &clientGateway{app: app}
	if err := g.SetTarget(targetURL); err != nil {
		return nil, err
	}

	proxy := &httputil.ReverseProxy{
		FlushInterval: -1,
		Director: func(req *http.Request) {
			cur := g.currentTarget()
			req.URL.Scheme = cur.Scheme
			req.URL.Host = cur.Host
			req.Host = cur.Host
			// Disable upstream compression so HTML injection can see plain
			// bodies; loopback/LAN bandwidth makes identity encoding cheap.
			req.Header.Set("Accept-Encoding", "identity")
			if _, ok := req.Header["User-Agent"]; !ok {
				req.Header.Set("User-Agent", "ZhiShuClient-Desktop")
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			ct := resp.Header.Get("Content-Type")
			if !strings.Contains(strings.ToLower(ct), "text/html") {
				return nil
			}
			body, err := io.ReadAll(io.LimitReader(resp.Body, 16<<20))
			resp.Body.Close()
			if err != nil {
				return err
			}
			g.mu.RLock()
			tStr := g.targetStr
			g.mu.RUnlock()
			injected := injectDesktopScript(string(body), tStr)
			resp.Body = io.NopCloser(strings.NewReader(injected))
			resp.Header.Del("Content-Length")
			resp.Header.Del("Content-Encoding")
			return nil
		},
		ErrorHandler: func(w http.ResponseWriter, r *http.Request, err error) {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusBadGateway)
			_, _ = w.Write([]byte(`<html><head><meta charset="utf-8"><title>无法连接服务器</title></head>` +
				`<body style="font-family:'Microsoft YaHei',system-ui,sans-serif;background:#eef7f1;display:flex;` +
				`align-items:center;justify-content:center;height:96vh;color:#1f2d27;">` +
				`<div style="text-align:center;"><h2 style="font-weight:600;">暂时无法连接服务器</h2>` +
				`<p style="color:#6b7f74;">请确认服务器已启动、网络可达，或在菜单「服务器设置…」中检查地址。</p></div></body></html>`))
		},
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/__desktop__/", g.handleDesktopAPI)
	mux.HandleFunc("/__desktop__/save-blob", g.handleSaveBlob)
	mux.HandleFunc("/__desktop__/rpc", g.handleRPC)
	mux.HandleFunc(clientSetupPath, func(w http.ResponseWriter, r *http.Request) {
		serveClientSetup(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", "0"))
	if err != nil {
		return nil, err
	}
	g.baseURL = "http://" + listener.Addr().String()

	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()
	return g, nil
}

// handleDesktopAPI routes the simple GET-style desktop endpoints.
func (g *clientGateway) handleDesktopAPI(w http.ResponseWriter, r *http.Request) {
	switch strings.TrimSuffix(r.URL.Path, "/") {
	case "/__desktop__/ping":
		// Cross-origin probe from the bootstrap page (no sensitive data).
		w.Header().Set("Access-Control-Allow-Origin", "*")
		writeJSON(w, map[string]interface{}{"ok": true, "gateway": true})
	case "/__desktop__/probe-target":
		// Bootstrap 探测当前目标服务器是否可达：可达才把 webview 交给
		// 服务器页面；不可达则引导回设置页（携带 offline 标记）。
		// 跨域探测（bootstrap 页在 wails 源上），必须带 CORS 头。
		w.Header().Set("Access-Control-Allow-Origin", "*")
		target := g.currentTarget().String()
		client := &http.Client{Timeout: 3 * time.Second}
		resp, err := client.Get(target + "/config.js")
		if err != nil {
			writeJSON(w, map[string]interface{}{"ok": false})
			return
		}
		resp.Body.Close()
		writeJSON(w, map[string]interface{}{"ok": resp.StatusCode == http.StatusOK})
	case "/__desktop__/open-external":
		target := strings.TrimSpace(r.URL.Query().Get("url"))
		if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
			writeJSON(w, map[string]interface{}{"ok": false, "error": "invalid url"})
			return
		}
		if g.app.ctx != nil {
			wailsruntime.BrowserOpenURL(g.app.ctx, target)
		}
		writeJSON(w, map[string]interface{}{"ok": true})
	case "/__desktop__/open":
		writeJSON(w, map[string]interface{}{"ok": g.app.OpenFilePath(r.URL.Query().Get("path"))})
	case "/__desktop__/reveal":
		writeJSON(w, map[string]interface{}{"ok": g.app.RevealInFolder(r.URL.Query().Get("path"))})
	default:
		http.NotFound(w, r)
	}
}

// handleRPC serves setup-page calls (the setup page may run on the gateway
// origin where Wails bindings are unavailable).
func (g *clientGateway) handleRPC(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var req struct {
		Fn   string        `json:"fn"`
		Args []interface{} `json:"args"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "invalid body"})
		return
	}
	whitelist := map[string]int{
		"TestServerURL":       1,
		"ApplyClientSetup":    1,
		"GetClientSetupState": 0,
	}
	argc, ok := whitelist[req.Fn]
	if !ok || len(req.Args) != argc {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "unknown fn"})
		return
	}
	args := make([]string, len(req.Args))
	for i, v := range req.Args {
		s, isStr := v.(string)
		if !isStr {
			writeJSON(w, map[string]interface{}{"ok": false, "error": "bad arg"})
			return
		}
		args[i] = s
	}
	var result interface{}
	switch req.Fn {
	case "TestServerURL":
		result = g.app.TestServerURL(args[0])
	case "ApplyClientSetup":
		result = g.app.ApplyClientSetup(args[0])
	case "GetClientSetupState":
		result = g.app.GetClientSetupState()
	}
	writeJSON(w, map[string]interface{}{"ok": true, "result": result})
}

// handleSaveBlob persists a base64 payload through the native save dialog.
func (g *clientGateway) handleSaveBlob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var req struct {
		Name    string `json:"name"`
		DataB64 string `json:"dataB64"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, int64(maxClientBridgePayloadBytes+1<<20))).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "invalid body"})
		return
	}
	path := g.app.SaveBlobFile(req.Name, req.DataB64)
	if path == "" {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "cancelled or failed"})
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "path": path})
}

// bootstrapHandler serves the tiny startup page: wait for the gateway, then
// probe the target server. Reachable → hand the webview to the server UI;
// unreachable → fall back to the setup page (offline banner shown there).
func (g *clientGateway) bootstrapHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, clientBootstrapPageHTML, g.baseURL)
	})
}

const clientBootstrapPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>ZhiShu Client</title>
<style>
  html, body { margin:0; height:100%%; }
  body { background:#eef7f1; display:flex; align-items:center; justify-content:center;
         font-family:"Segoe UI","Microsoft YaHei",system-ui,sans-serif; color:#1f2d27; }
  .box { text-align:center; }
  .ring { width:34px; height:34px; margin:0 auto 16px; border-radius:50%%;
          border:3px solid #cfe6d8; border-top-color:#21a35a; animation:spin .9s linear infinite; }
  @keyframes spin { to { transform:rotate(360deg); } }
  p { color:#6b7f74; font-size:13px; }
</style>
</head>
<body>
<div class="box"><div class="ring"></div><p>正在连接服务器…</p></div>
<script>
(function(){
  var gw = %q;
  var tries = 0;
  function probe(){
    tries++;
    fetch(gw + '/__desktop__/ping', {cache:'no-store'})
      .then(function(r){ return r.json(); })
      .then(function(j){
        if (!(j && j.ok)) { retry(); return; }
        // 网关就绪，再探测目标服务器是否可达
        return fetch(gw + '/__desktop__/probe-target', {cache:'no-store'})
          .then(function(r){ return r.json(); })
          .then(function(t){
            if (t && t.ok) { location.replace(gw + '/'); return; }
            // 上次的服务器不可达：回设置页，附带 offline 标记显示提示
            location.replace(gw + '/__client_setup__?offline=1');
          });
      })
      .catch(retry);
  }
  function retry(){
    if (tries > 60) {
      document.querySelector('p').textContent = '连接超时，请关闭窗口后在菜单「服务器设置…」中检查地址。';
      return;
    }
    setTimeout(probe, 250);
  }
  probe();
})();
</script>
</body>
</html>
`

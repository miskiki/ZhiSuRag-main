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

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// desktopGateway serves the webview UI over real loopback HTTP.
//
// Why this exists (Windows): WebView2's virtual-host channel
// (http://wails.localhost) buffers streamed response bodies before handing
// them to the page, which turns the backend's SSE chat stream into a single
// lump — no incremental answer, no retrieval/thinking progress. Serving the
// same UI from a real 127.0.0.1 HTTP listener gives the webview normal
// browser-grade networking: SSE streams, downloads and cookies behave
// exactly like the web client.
//
// The gateway also hosts /__desktop__/* endpoints that replace the Wails JS
// bindings lost after the webview leaves the wails:// origin (native save
// dialog, open/reveal file, open external browser, setup RPC).
type desktopGateway struct {
	app     *App
	baseURL string
}

// startDesktopGateway binds 127.0.0.1:<random> and begins proxying to
// targetURL. The listener is up when this call returns.
func startDesktopGateway(app *App, targetURL string) (*desktopGateway, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid gateway target %q: %w", targetURL, err)
	}

	g := &desktopGateway{app: app}

	proxy := &httputil.ReverseProxy{
		// Flush every write: the chat endpoint streams SSE events and the
		// webview must see them as they arrive.
		FlushInterval: -1,
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
			// Disable upstream compression so HTML injection can see plain
			// bodies; loopback/LAN bandwidth makes identity encoding cheap.
			req.Header.Set("Accept-Encoding", "identity")
			if _, ok := req.Header["User-Agent"]; !ok {
				req.Header.Set("User-Agent", "ZhiShuLite-Desktop")
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
			injected := injectDesktopScript(string(body), targetURL)
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
	mux.HandleFunc(desktopSetupPath, func(w http.ResponseWriter, r *http.Request) {
		serveDesktopSetup(w, r)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		proxy.ServeHTTP(w, r)
	})

	listener, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", "0"))
	if err != nil {
		return nil, err
	}
	g.baseURL = "http://" + listener.Addr().String()
	g.app.gatewayURL = g.baseURL

	server := &http.Server{Handler: mux}
	go func() {
		_ = server.Serve(listener)
	}()
	return g, nil
}

// injectDesktopScript embeds the desktop shell plus origin-specific API base
// into a text/html document, right before the closing </head> when present.
func injectDesktopScript(html, targetURL string) string {
	if strings.Contains(html, "__zhishuDesktopShell") {
		return html // already injected (proxied SPA re-serve safety)
	}
	script := "<script>window.__ZHISHU_API_BASE__=" +
		jsonStringLiteral(strings.TrimRight(targetURL, "/")+"/api/v1") + ";" +
		desktopShellJS + "</script>"
	if idx := strings.LastIndex(strings.ToLower(html), "</head>"); idx >= 0 {
		return html[:idx] + script + html[idx:]
	}
	return script + html
}

func jsonStringLiteral(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

// handleDesktopAPI routes the simple GET-style desktop endpoints.
func (g *desktopGateway) handleDesktopAPI(w http.ResponseWriter, r *http.Request) {
	switch strings.TrimSuffix(r.URL.Path, "/") {
	case "/__desktop__/ping":
		// The bootstrap page lives on the wails:// origin and probes this
		// endpoint cross-origin before handing the webview over; the response
		// carries no sensitive data, so a permissive CORS header is fine.
		w.Header().Set("Access-Control-Allow-Origin", "*")
		writeJSON(w, map[string]interface{}{"ok": true, "gateway": true})
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
		path := r.URL.Query().Get("path")
		writeJSON(w, map[string]interface{}{"ok": g.app.OpenFilePath(path)})
	case "/__desktop__/reveal":
		path := r.URL.Query().Get("path")
		writeJSON(w, map[string]interface{}{"ok": g.app.RevealInFolder(path)})
	default:
		http.NotFound(w, r)
	}
}

// handleSaveBlob persists a base64 payload from the webview through the
// native "Save as" dialog (the exe-side download experience).
func (g *desktopGateway) handleSaveBlob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	var req struct {
		Name    string `json:"name"`
		DataB64 string `json:"dataB64"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, int64(maxBridgePayloadBytes+1<<20))).Decode(&req); err != nil {
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

// desktopRPCFns whitelists the App methods callable from the setup page when
// it runs on the gateway origin (no Wails bindings there).
var desktopRPCFns = map[string]int{
	"TestServerURL":        1,
	"ApplyDesktopSetup":    2,
	"GetDesktopSetupState": 0,
}

func (g *desktopGateway) handleRPC(w http.ResponseWriter, r *http.Request) {
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
	argc, ok := desktopRPCFns[req.Fn]
	if !ok || len(req.Args) != argc {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "unknown fn"})
		return
	}
	args := make([]string, len(req.Args))
	for i, a := range req.Args {
		s, isStr := a.(string)
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
	case "ApplyDesktopSetup":
		result = g.app.ApplyDesktopSetup(args[0], args[1])
	case "GetDesktopSetupState":
		result = g.app.GetDesktopSetupState()
	}
	writeJSON(w, map[string]interface{}{"ok": true, "result": result})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

// bootstrapHandler serves the tiny page the webview first loads on the
// wails:// origin: it waits until the gateway answers, then hands the
// webview over to the real HTTP origin.
func (g *desktopGateway) bootstrapHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = fmt.Fprintf(w, bootstrapPageHTML, g.baseURL)
	})
}

const bootstrapPageHTML = `<!DOCTYPE html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<title>ZhiShu Lite</title>
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
<div class="box"><div class="ring"></div><p>正在启动智述 Lite…</p></div>
<script>
(function(){
  var gw = %q;
  var tries = 0;
  function probe(){
    tries++;
    fetch(gw + '/__desktop__/ping', {cache:'no-store'})
      .then(function(r){ return r.json(); })
      .then(function(j){
        if (j && j.ok) { location.replace(gw + '/'); return; }
        retry();
      })
      .catch(retry);
  }
  function retry(){
    if (tries > 60) {
      document.querySelector('p').textContent = '启动超时，请关闭窗口后重新打开应用。';
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

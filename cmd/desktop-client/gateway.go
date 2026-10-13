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

// desktopSetupPath is served by the asset server (wails:// origin) so the
// user can reconfigure the server address from the menu at any time.
const desktopSetupPath = "/__client_setup__"

// clientGateway serves the webview UI over real loopback HTTP: WebView2's
// virtual-host channel buffers streamed bodies, which would break chat
// streaming. It also hosts /__desktop__/* native-helper endpoints and
// injects the desktop shell into HTML responses. See cmd/desktop/gateway.go
// in the full client for the full rationale.
type clientGateway struct {
	app     *clientApp
	baseURL string
}

func startClientGateway(app *clientApp, targetURL string) (*clientGateway, error) {
	target, err := url.Parse(targetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid gateway target %q: %w", targetURL, err)
	}

	g := &clientGateway{app: app}

	proxy := &httputil.ReverseProxy{
		FlushInterval: -1,
		Director: func(req *http.Request) {
			req.URL.Scheme = target.Scheme
			req.URL.Host = target.Host
			req.Host = target.Host
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

// clientRPCFns whitelists setup-page RPC calls on the gateway origin.
var clientRPCFns = map[string]int{
	"TestServerURL":       1,
	"ApplyClientSetup":    1,
	"GetClientSetupState": 0,
}

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
	argc, ok := clientRPCFns[req.Fn]
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
	case "ApplyClientSetup":
		result = g.app.ApplyClientSetup(args[0])
	case "GetClientSetupState":
		result = g.app.GetClientSetupState()
	}
	writeJSON(w, map[string]interface{}{"ok": true, "result": result})
}

// bootstrapHandler serves the tiny startup page: wait for the gateway, then
// hand the webview over to the real HTTP origin (the server UI).
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
        if (j && j.ok) { location.replace(gw + '/'); return; }
        retry();
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

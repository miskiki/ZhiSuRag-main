package main

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// clientVersion is overridden at build time via -ldflags -X.
var clientVersion = "0.7.2"

// defaultServerURL is baked into the client so first launch needs no typing.
// Override at build time via -ldflags -X, or per-deployment via the setup UI.
var defaultServerURL = "http://192.168.10.131"

const productGitHubRepoURL = "https://github.com/miskiki/ZhiSuRag-main"

// maxClientBridgePayloadBytes caps base64 payloads through the JS bridge;
// larger downloads fall back to the webview's default download behaviour.
const maxClientBridgePayloadBytes = 48 << 20

// injectDesktopScript embeds the desktop shell plus API base into a
// text/html document, right before the closing </head> when present.
func injectDesktopScript(html, targetURL string) string {
	if strings.Contains(html, "__zhishuDesktopShell") {
		return html
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
	q := strings.ReplaceAll(s, `\`, `\`)
	q = strings.ReplaceAll(q, `"`, `\"`)
	return `"` + q + `"`
}

// NormalizeServerURL cleans a user-entered server address: adds http:// when
// the scheme is missing, strips any path (users often paste /login etc.) so
// only the origin remains, and drops query/fragment. Returns "" when invalid.
func NormalizeServerURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	if !strings.Contains(trimmed, "://") {
		trimmed = "http://" + trimmed
	}
	u, err := url.Parse(trimmed)
	if err != nil || u.Host == "" {
		return ""
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return ""
	}
	u.Path = ""
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

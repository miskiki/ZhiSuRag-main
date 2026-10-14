package main

import (
	"strings"
)

// clientVersion is overridden at build time via -ldflags -X.
var clientVersion = "0.7.2"

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

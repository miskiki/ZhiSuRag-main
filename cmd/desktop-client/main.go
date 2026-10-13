package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"
	"strings"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// clientVersion is overridden at build time via -ldflags -X.
var clientVersion = "0.7.2"

const productGitHubRepoURL = "https://github.com/miskiki/ZhiSuRag-main"

// wailsThemeSyncJS keeps the window background in sync with the SPA theme
// (harmless no-op on remote origins where window.runtime is absent).
const wailsThemeSyncJS = `(function(){try{var t=localStorage.getItem('ZhiShu_theme')||localStorage.getItem('WeKnora_theme')||'light';if(t==='system')t=window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';var bg=t==='dark'?'#181818':'#eee';document.documentElement.setAttribute('theme-mode',t);document.documentElement.style.background=bg;var w=window.runtime;if(!w)return;if(t==='dark'){if(w.WindowSetDarkTheme)w.WindowSetDarkTheme();if(w.WindowSetBackgroundColour)w.WindowSetBackgroundColour(24,24,24,255);}else{if(w.WindowSetLightTheme)w.WindowSetLightTheme();if(w.WindowSetBackgroundColour)w.WindowSetBackgroundColour(238,238,238,255);}}catch(e){}})()`

// maxClientBridgePayloadBytes caps base64 payloads through the JS bridge;
// larger downloads fall back to the webview's default download behaviour.
const maxClientBridgePayloadBytes = 48 << 20

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

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
	q := strings.ReplaceAll(s, `\`, `\\`)
	q = strings.ReplaceAll(q, `"`, `\"`)
	return `"` + q + `"`
}

func main() {
	// Startup black box: any silent failure on colleague machines leaves a
	// startup-error.log next to the exe for diagnosis.
	startupLogInit()
	defer startupCrashGuard()
	defer startupLogClose()
	startupLogf("boot: client version=%s os=%s", clientVersion, runtime.GOOS)

	serverURL := loadClientServerURL()
	startupLogf("server URL: %q", serverURL)

	app := newClientApp()

	var assetHandler http.Handler
	if serverURL == "" {
		// First run / reconfiguration: server address setup page.
		assetHandler = http.HandlerFunc(serveClientSetup)
	} else {
		gateway, err := startClientGateway(app, serverURL)
		if err != nil {
			startupLogf("gateway failed: %v", err)
			fmt.Fprintln(os.Stderr, "gateway:", err)
			os.Exit(1)
		}
		app.gatewayURL = gateway.baseURL
		startupLogf("gateway ready: %s -> %s", gateway.baseURL, serverURL)
		assetHandler = gateway.bootstrapHandler()
	}

	// Windows: verify the WebView2 runtime before creating the window.
	if runtime.GOOS == "windows" {
		startupLogf("webview2 runtime installed: %v", webview2RuntimeInstalled())
		if !ensureWebView2Runtime() {
			startupLogf("exit: webview2 runtime missing")
			startupLogClose()
			os.Exit(1)
		}
	}

	AppMenu := buildAppMenu(app)
	_ = AppMenu

	err := wails.Run(&options.App{
		Title:  "ZhiShu Client · 智枢文渊",
		Width:  1280,
		Height: 800,
		Menu:   AppMenu,
		AssetServer: &assetserver.Options{
			Handler: assetHandler,
		},
		StartHidden: false,
		OnStartup:   app.startup,
		OnDomReady: func(ctx context.Context) {
			wailsruntime.WindowExecJS(ctx, wailsThemeSyncJS)
		},
		OnShutdown: func(ctx context.Context) {
			startupLogClose()
		},
		Bind: []interface{}{
			app,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
	})

	if err != nil {
		startupLogf("wails.Run failed: %v", err)
		startupLogClose()
		fmt.Fprintln(os.Stderr, "Error:", err.Error())
	}
}

// buildAppMenu wires the (small) client menu bar.

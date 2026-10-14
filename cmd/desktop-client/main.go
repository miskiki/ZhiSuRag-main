//go:build !bindings

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"runtime"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// wailsThemeSyncJS keeps the window background in sync with the SPA theme
// (harmless no-op on remote origins where window.runtime is absent).
const wailsThemeSyncJS = `(function(){try{var t=localStorage.getItem('ZhiShu_theme')||localStorage.getItem('WeKnora_theme')||'light';if(t==='system')t=window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';var bg=t==='dark'?'#181818':'#eee';document.documentElement.setAttribute('theme-mode',t);document.documentElement.style.background=bg;var w=window.runtime;if(!w)return;if(t==='dark'){if(w.WindowSetDarkTheme)w.WindowSetDarkTheme();if(w.WindowSetBackgroundColour)w.WindowSetBackgroundColour(24,24,24,255);}else{if(w.WindowSetLightTheme)w.WindowSetLightTheme();if(w.WindowSetBackgroundColour)w.WindowSetBackgroundColour(238,238,238,255);}}catch(e){}})()`

func main() {
	// Startup black box: any silent failure on colleague machines leaves a
	// startup-error.log next to the exe for diagnosis.
	startupLogInit()
	defer startupCrashGuard()
	defer startupLogClose()
	startupLogf("boot: client version=%s os=%s", clientVersion, runtime.GOOS)

	prefs := loadClientPrefs()
	startupLogf("saved server: %q history=%v", prefs.ServerURL, prefs.History)

	app := newClientApp()

	var assetHandler http.Handler
	if prefs.ServerURL == "" {
		// First run / reconfiguration: setup page plus desktop RPC routes so
		// the page works even when Wails bindings are unavailable.
		mux := http.NewServeMux()
		mux.HandleFunc("/__desktop__/rpc", app.handleRPC)
		mux.HandleFunc("/__desktop__/save-blob", app.handleSaveBlob)
		mux.HandleFunc("/__desktop__/open", app.handleOpen)
		mux.HandleFunc("/__desktop__/reveal", app.handleReveal)
		mux.HandleFunc("/__desktop__/open-external", app.handleOpenExternal)
		mux.HandleFunc("/__desktop__/ping", func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"ok": true})
		})
		mux.HandleFunc("/", serveClientSetup)
		assetHandler = mux
		startupLogf("mode: setup page (no saved server)")
	} else {
		gwURL, err := app.ensureGateway(prefs.ServerURL)
		if err != nil {
			startupLogf("gateway failed: %v", err)
			fmt.Fprintln(os.Stderr, "gateway:", err)
			os.Exit(1)
		}
		startupLogf("gateway ready: %s -> %s", gwURL, prefs.ServerURL)
		assetHandler = app.gateway.bootstrapHandler()
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

	wailsErr := wails.Run(&options.App{
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

	if wailsErr != nil {
		startupLogf("wails.Run failed: %v", wailsErr)
		startupLogClose()
		fmt.Fprintln(os.Stderr, "Error:", wailsErr.Error())
	}
}

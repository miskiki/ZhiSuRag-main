//go:build !bindings

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	goruntime "runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/container"
	"github.com/Tencent/WeKnora/internal/logger"
	"github.com/Tencent/WeKnora/internal/runtime"
	"github.com/Tencent/WeKnora/internal/types/interfaces"
	"github.com/joho/godotenv"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// dragHandlerJS is injected into the webview on DomReady (macOS only).
// It bypasses Wails' built-in CSS-variable-based drag detection (which uses
// getComputedStyle and has timing/inheritance issues with dynamic SPA content)
// and instead uses robust DOM-traversal via el.closest() plus a Y-position
// fallback for the macOS title-bar region on layout containers. The "drag"
// message is sent directly through the WKWebView script-message bridge,
// which the native Objective-C handler in WailsContext.m converts to
// [NSWindow performWindowDragWithEvent:].
const dragHandlerJS = `(function(){
if(window.__wkDragBound)return;
window.__wkDragBound=true;
document.documentElement.classList.add('wails-desktop');

// Disable rubber-band overscroll that reveals the dark window background
document.documentElement.style.overscrollBehavior='none';
document.body.style.overscrollBehavior='none';

if(window.wails&&window.wails.flags){
  window.wails.flags.cssDragProperty='__disabled__';
  window.wails.flags.cssDragValue='__never__';
}

// Prevent native text/image drag-out to fix the "selected and dragged away" issue
window.addEventListener('dragstart', function(e){
  e.preventDefault();
}, true);

var TITLEBAR_H=38;

// We specifically look for Wails' inline style attributes injected by Vue,
// and custom drag classes, avoiding generic headers like .section-header
var dragSel='.logo_row,.menu_top,.drag-region,[data-wails-drag],' +
  '[style*="--wails-draggable: drag"],[style*="--wails-draggable:drag"]';

var noDragSel='button,a,input,select,textarea,[role="button"],' +
  '.t-button,.t-input,.t-select,.t-textarea,' +
  '.header-actions,.header-action-btn,.sidebar-toggle,.logo_box,' +
  '.close-btn,.menu_item,.submenu,.submenu_item,.menu_bottom,' +
  '.t-popup,.t-dropdown,.t-tooltip,.t-dialog,[data-no-drag],' +
  '[style*="--wails-draggable: no-drag"],[style*="--wails-draggable:no-drag"]';

var layoutClasses=['main','chat','dialogue-wrap','kb-list-container','kb-list-content',
  'agent-list-container','agent-list-content','org-list-container','org-list-content','aside_box',
  'ks-container','ks-content','settings-overlay','knowledge-layout',
  'faq-manager-wrapper','login-layout'];

function sendDrag(){
  try{window.webkit.messageHandlers.external.postMessage('drag')}
  catch(_){try{window.WailsInvoke('drag')}catch(e){}}
}

function isLayoutEl(el){
  for(var i=0;i<layoutClasses.length;i++){
    if(el.classList.contains(layoutClasses[i]))return true;
  }
  var tag=el.tagName;
  return tag==='BODY'||tag==='HTML';
}

function shouldDrag(el,y){
  if(!(el instanceof Element))return false;
  if(el.closest(noDragSel))return false;
  if(el.closest(dragSel))return true;
  if(y<=TITLEBAR_H&&isLayoutEl(el))return true;
  return false;
}

window.addEventListener('mousedown',function(e){
  var target=e.target;
  if(target&&target.nodeType===Node.TEXT_NODE){
    target=target.parentElement;
  }
  if(e.button!==0||e.detail!==1)return;
  if(!shouldDrag(target,e.clientY))return;
  e.preventDefault();
  sendDrag();
},true);

// Intercept external link clicks and window.open so they open in the system browser
document.addEventListener('click',function(e){
  var el=e.target;
  while(el&&el.tagName!=='A')el=el.parentElement;
  if(!el||!el.href)return;
  var href=el.href;
  if(href.indexOf('http://')===0||href.indexOf('https://')===0){
    if(window.runtime&&window.runtime.BrowserOpenURL){
      e.preventDefault();
      e.stopPropagation();
      window.runtime.BrowserOpenURL(href);
    }
  }
},true);

var origOpen=window.open;
window.open=function(url){
  if(url&&(typeof url==='string')&&(url.indexOf('http://')===0||url.indexOf('https://')===0)){
    if(window.runtime&&window.runtime.BrowserOpenURL){
      window.runtime.BrowserOpenURL(url);
      return null;
    }
  }
  return origOpen.apply(window,arguments);
};
})();`

// wailsThemeSyncJS：与 index.html 首屏一致，在 DomReady 再跑一遍，覆盖 Ctrl+R 后 runtime 就绪时机
const wailsThemeSyncJS = `(function(){try{var t=localStorage.getItem('WeKnora_theme')||'light';if(t==='system')t=window.matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light';var bg=t==='dark'?'#181818':'#eee';document.documentElement.setAttribute('theme-mode',t);document.documentElement.style.background=bg;document.documentElement.style.minHeight='100%';document.documentElement.colorScheme=t==='dark'?'dark':'light';if(document.body){document.body.style.background=bg;document.body.style.minHeight='100%';}var w=window.runtime;if(!w)return;if(t==='dark'){if(w.WindowSetDarkTheme)w.WindowSetDarkTheme();if(w.WindowSetBackgroundColour)w.WindowSetBackgroundColour(24,24,24,255);}else{if(w.WindowSetLightTheme)w.WindowSetLightTheme();if(w.WindowSetBackgroundColour)w.WindowSetBackgroundColour(238,238,238,255);}}catch(e){}})()`

const productGitHubRepoURL = "https://github.com/miskiki/ZhiSuRag-main"

// desktopModes for the app shell.
const (
	modeUnset  = ""       // show setup page (first run or broken config)
	modeLocal  = "local"  // embedded backend, data on this machine
	modeRemote = "remote" // proxy to a user-configured ZhiShu server
)

func main() {
	// Windows 分发诊断：启动黑匣子（写 exe 旁 startup-error.log），同事机器
	// 上任何静默失败都能留下可回传的痕迹。panic 也会被记录。
	startupLogInit()
	defer startupCrashGuard()
	defer startupLogClose()
	startupLogf("boot: version=%s os=%s", desktopAboutVersion(), goruntime.GOOS)

	// For macOS .app bundle, the working directory is usually "/" or the MacOS folder.
	// We need to change the working directory to the Resources folder where our configs are.
	execPath, errPath := os.Executable()
	if errPath == nil && strings.Contains(execPath, ".app/Contents/MacOS") {
		resPath := filepath.Join(filepath.Dir(filepath.Dir(execPath)), "Resources")
		_ = os.Chdir(resPath)
	} else if _, err := os.Stat(filepath.Join("config", "config.yaml")); os.IsNotExist(err) {
		// wails build 生成绑定时 cwd 多为 cmd/desktop，LoadConfig 默认找 ./config/config.yaml；
		// 仓库实际配置在 <repo>/config/，向上两级即可。
		repoRoot := filepath.Clean(filepath.Join("..", ".."))
		if _, err := os.Stat(filepath.Join(repoRoot, "config", "config.yaml")); err == nil {
			_ = os.Chdir(repoRoot)
		} else if exeDir := filepath.Dir(execPath); errPath == nil {
			// Packaged Windows build: config/ ships next to the exe.
			if _, err := os.Stat(filepath.Join(exeDir, "config", "config.yaml")); err == nil {
				_ = os.Chdir(exeDir)
			}
		}
	}

	// Load .env explicitly for the desktop app so DB_DRIVER gets loaded
	_ = godotenv.Load()
	logger.ConfigureFromEnv()

	// Determine run mode: "" -> setup page, "local" -> embedded backend,
	// "remote" -> reverse proxy to the configured server.
	mode := loadDesktopMode()
	if mode == modeRemote && loadDesktopServerURL() == "" {
		mode = modeUnset // broken config, fall back to setup
	}

	// Set Gin mode
	if os.Getenv("GIN_MODE") == "release" {
		gin.SetMode(gin.ReleaseMode)
	} else {
		gin.SetMode(gin.DebugMode)
	}

	// Initialize the WeKnora App struct
	app := NewApp()

	// Error channel to capture server startup errors (local mode)
	serverErrCh := make(chan error, 1)

	useGateway := goruntime.GOOS == "windows" && mode != modeUnset

	var assetHandler http.Handler

	if !useGateway {
		switch mode {
		case modeUnset:
			// First run / reconfiguration: serve the standalone setup page.
			assetHandler = http.HandlerFunc(serveDesktopSetup)

		case modeRemote:
			base := loadDesktopServerURL()
			app.backendURL = base
			target, err := url.Parse(base)
			if err != nil {
				logger.Fatalf(context.Background(), "Invalid server URL %q: %v", base, err)
			}
			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
				logger.Warnf(context.Background(), "Proxy error for %s: %v", r.URL.Path, err)
				w.Header().Set("Content-Type", "text/html; charset=utf-8")
				w.WriteHeader(http.StatusBadGateway)
				_, _ = w.Write([]byte(`<html><head><meta charset="utf-8"><title>无法连接服务器</title></head>` +
					`<body style="font-family:'Microsoft YaHei',system-ui,sans-serif;background:#eef7f1;display:flex;` +
					`align-items:center;justify-content:center;height:96vh;color:#1f2d27;">` +
					`<div style="text-align:center;"><h2 style="font-weight:600;">暂时无法连接服务器</h2>` +
					`<p style="color:#6b7f74;">请确认服务器已启动、网络可达，或在菜单「服务器设置…」中检查地址。</p></div></body></html>`))
			}
			assetHandler = withDesktopSetupRoute(proxy)

		default: // modeLocal (macOS path)
			configureDesktopStorage(execPath)
			runtime.SilenceGinRouteSpam()
			runtime.LogStartupEnv(context.Background())

			// Build dependency injection container
			c := container.BuildContainer(runtime.GetContainer())

			// Run backend in a separate goroutine
			go func() {
				err := c.Invoke(func(
					cfg *config.Config,
					router *gin.Engine,
					resourceCleaner interfaces.ResourceCleaner,
				) error {
					server := &http.Server{Handler: router}

					runtime.LogGinRouteCount(context.Background())

					// 127.0.0.1 + saved port from settings (desktop-prefs.json), or :0 for random free port.
					addr := desktopBackendListenAddr()

					listener, err := listenWithRetry(addr, 10, 300*time.Millisecond)
					if err != nil {
						return fmt.Errorf("failed to start server: %v", err)
					}

					tcpAddr := listener.Addr().(*net.TCPAddr)
					port := tcpAddr.Port
					app.listenPublic = LoadDesktopHTTPBindPublic()
					// Reverse proxy and webview API calls always use loopback; avoid 0.0.0.0 / [::] as dial target.
					app.backendURL = fmt.Sprintf("http://127.0.0.1:%d", port)
					app.apiLanBaseURL = ""
					if app.listenPublic {
						if ip := desktopPreferredLANIPv4(); ip != nil {
							app.apiLanBaseURL = fmt.Sprintf("http://%s:%d/api/v1", ip.String(), port)
						}
					}

					// Handle graceful shutdown from Wails OnShutdown hook
					go func() {
						<-app.shutdownCh
						logger.Infof(context.Background(), "Wails shutting down, stopping Go backend...")

						listener.Close()
						shutdownTimeout := cfg.Server.ShutdownTimeout
						if shutdownTimeout == 0 {
							shutdownTimeout = 30 * time.Second
						}
						shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
						defer cancel()

						if err := server.Shutdown(shutdownCtx); err != nil {
							server.Close()
						}
						resourceCleaner.Cleanup(shutdownCtx)
					}()

					// Also listen for OS signals just in case
					signals := make(chan os.Signal, 1)
					signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
					go func() {
						<-signals
						app.shutdownCh <- struct{}{} // trigger shutdown
					}()

					logger.Infof(context.Background(), "Server is running at %s (proxy -> %s)", tcpAddr.String(), app.backendURL)
					if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
						return fmt.Errorf("server error: %v", err)
					}
					return nil
				})

				if err != nil {
					serverErrCh <- err
					logger.Fatalf(context.Background(), "Failed to run backend: %v", err)
				}
			}()

			// Give the server a moment to start and determine its port
			time.Sleep(500 * time.Millisecond)

			// Wait for the backend URL to be set
			targetURL, _ := url.Parse(app.backendURL)
			proxy := httputil.NewSingleHostReverseProxy(targetURL)
			assetHandler = withDesktopSetupRoute(proxy)
		}
	}

	if useGateway {
		// Windows: the webview must load the UI over real loopback HTTP.
		// WebView2's virtual-host channel buffers streamed bodies, which
		// kills chat streaming (SSE) and the retrieval/thinking progress.
		// See desktopGateway for the details.
		var target string
		if mode == modeRemote {
			target = loadDesktopServerURL()
			if strings.TrimSpace(target) == "" {
				mode = modeUnset // broken config, fall back to setup page below
			}
		}
		if mode != modeUnset {
			if mode == modeLocal {
				configureDesktopStorage(execPath)
				runtime.SilenceGinRouteSpam()
				runtime.LogStartupEnv(context.Background())

				// Build dependency injection container
				c := container.BuildContainer(runtime.GetContainer())

				// Run backend in a separate goroutine
				go func() {
					err := c.Invoke(func(
						cfg *config.Config,
						router *gin.Engine,
						resourceCleaner interfaces.ResourceCleaner,
					) error {
						server := &http.Server{Handler: router}

						runtime.LogGinRouteCount(context.Background())

						addr := desktopBackendListenAddr()

						listener, err := listenWithRetry(addr, 10, 300*time.Millisecond)
						if err != nil {
							return fmt.Errorf("failed to start server: %v", err)
						}

						tcpAddr := listener.Addr().(*net.TCPAddr)
						port := tcpAddr.Port
						app.listenPublic = LoadDesktopHTTPBindPublic()
						app.backendURL = fmt.Sprintf("http://127.0.0.1:%d", port)
						app.apiLanBaseURL = ""
						if app.listenPublic {
							if ip := desktopPreferredLANIPv4(); ip != nil {
								app.apiLanBaseURL = fmt.Sprintf("http://%s:%d/api/v1", ip.String(), port)
							}
						}

						// Handle graceful shutdown from Wails OnShutdown hook
						go func() {
							<-app.shutdownCh
							logger.Infof(context.Background(), "Wails shutting down, stopping Go backend...")

							listener.Close()
							shutdownTimeout := cfg.Server.ShutdownTimeout
							if shutdownTimeout == 0 {
								shutdownTimeout = 30 * time.Second
							}
							shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
							defer cancel()

							if err := server.Shutdown(shutdownCtx); err != nil {
								server.Close()
							}
							resourceCleaner.Cleanup(shutdownCtx)
						}()

						// Also listen for OS signals just in case
						signals := make(chan os.Signal, 1)
						signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)
						go func() {
							<-signals
							app.shutdownCh <- struct{}{} // trigger shutdown
						}()

						logger.Infof(context.Background(), "Server is running at %s (gateway -> %s)", tcpAddr.String(), app.backendURL)
						if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
							return fmt.Errorf("server error: %v", err)
						}
						return nil
					})

					if err != nil {
						serverErrCh <- err
						logger.Fatalf(context.Background(), "Failed to run backend: %v", err)
					}
				}()

				// Wait for the embedded backend to pick its port.
				deadline := time.Now().Add(8 * time.Second)
				for strings.TrimSpace(app.backendURL) == "" {
					if time.Now().After(deadline) {
						logger.Fatalf(context.Background(), "Embedded backend did not start in time")
					}
					time.Sleep(50 * time.Millisecond)
				}
				target = app.backendURL
			}

			gateway, err := startDesktopGateway(app, target)
			if err != nil {
				logger.Fatalf(context.Background(), "Failed to start desktop gateway: %v", err)
			}
			logger.Infof(context.Background(), "Desktop gateway: %s (target -> %s)", gateway.baseURL, target)
			startupLogf("gateway ready: %s -> %s", gateway.baseURL, target)
			assetHandler = gateway.bootstrapHandler()
		}
		if mode == modeUnset {
			assetHandler = http.HandlerFunc(serveDesktopSetup)
		}
	}

	// Create application menu
	// macOS app menu
	AppMenu := menu.NewMenu()
	FileMenu := AppMenu.AddSubmenu("ZhiShu Lite")
	FileMenu.AddText("About ZhiShu", keys.CmdOrCtrl("i"), func(_ *menu.CallbackData) {
		if app.ctx == nil {
			return
		}
		choice, err := wailsruntime.MessageDialog(app.ctx, wailsruntime.MessageDialogOptions{
			Type:          wailsruntime.InfoDialog,
			Title:         "ZhiShu Lite",
			Message:       fmt.Sprintf("ZhiShu Lite — Desktop Edition\n\nA RAG framework for document understanding and semantic Q&A over complex, heterogeneous content.\n\nVersion %s\n© 2026 智枢文渊\n\nGitHub:\n%s", desktopAboutVersion(), productGitHubRepoURL),
			Buttons:       []string{"Open GitHub", "OK"},
			DefaultButton: "OK",
		})
		if err != nil {
			logger.Warnf(context.Background(), "About dialog: %v", err)
			return
		}
		if choice == "Open GitHub" {
			wailsruntime.BrowserOpenURL(app.ctx, productGitHubRepoURL)
		}
	})
	FileMenu.AddText("服务器设置…", nil, func(_ *menu.CallbackData) {
		if app.ctx == nil {
			return
		}
		wailsruntime.WindowExecJS(app.ctx, fmt.Sprintf("location.href=%s;", strconv.Quote(desktopSetupPath)))
	})
	FileMenu.AddText("Check for Updates...", nil, func(_ *menu.CallbackData) {
		app.CheckForUpdates()
	})
	FileMenu.AddSeparator()
	FileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		app.shutdown(context.Background())
		os.Exit(0)
	})

	AppMenu.Append(menu.EditMenu())

	ViewMenu := AppMenu.AddSubmenu("View")
	ViewMenu.AddText("Reload", keys.CmdOrCtrl("r"), func(_ *menu.CallbackData) {
		if app.ctx != nil {
			wailsruntime.EventsEmit(app.ctx, "app:reload")
		}
	})

	// Windows: make sure the WebView2 runtime exists before creating the
	// window, so a missing runtime produces a friendly Chinese guide instead
	// of a silent launch failure on colleague machines.
	if goruntime.GOOS == "windows" {
		startupLogf("mode=%s target=%q webview2=%v", mode, loadDesktopServerURL(), webview2RuntimeInstalled())
		if !ensureWebView2Runtime() {
			startupLogf("exit: webview2 runtime missing")
			startupLogClose()
			os.Exit(1)
		}
		startupLogf("webview2 runtime ok")
	}

	// Start Wails application
	// The asset handler is a Reverse Proxy to seamlessly proxy Wails' frontend
	// to our Go backend (local or remote), or the setup page on first run.
	err := wails.Run(&options.App{
		Title:         "ZhiShu Lite",
		Width:         1280,
		Height:        800,
		DisableResize: false,
		Menu:          AppMenu,
		AssetServer: &assetserver.Options{
			Handler: assetHandler,
		},
		StartHidden: false, // Show window on startup
		OnStartup:   app.startup,
		OnDomReady: func(ctx context.Context) {
			wailsruntime.WindowExecJS(ctx, wailsThemeSyncJS)
			if goruntime.GOOS == "darwin" {
				// macOS keeps the wails:// origin with the Wails runtime, so
				// the drag regions and API-base injection apply there.
				wailsruntime.WindowExecJS(ctx, dragHandlerJS)
				if mode != modeUnset {
					if u := strings.TrimSpace(app.backendURL); u != "" {
						apiRoot := strings.TrimRight(u, "/") + "/api/v1"
						inject := fmt.Sprintf(`try{window.__ZHISHU_API_BASE__=%s}catch(e){}`, strconv.Quote(apiRoot))
						wailsruntime.WindowExecJS(ctx, inject)
					}
					if lan := strings.TrimSpace(app.apiLanBaseURL); lan != "" {
						injectLan := fmt.Sprintf(`try{window.__ZHISHU_API_LAN_BASE__=%s}catch(e){}`, strconv.Quote(lan))
						wailsruntime.WindowExecJS(ctx, injectLan)
					}
				}
			}
			// Windows non-setup: the desktop gateway injects the desktop
			// shell and API base server-side into every HTML response — the
			// webview lives on the gateway's HTTP origin, where Wails
			// page hooks do not apply.
		},
		OnShutdown: app.shutdown,
		Bind: []interface{}{
			app,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		Mac: &mac.Options{
			TitleBar:             mac.TitleBarHiddenInset(),
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})

	if err != nil {
		startupLogf("wails.Run failed: %v", err)
		startupLogClose()
		println("Error:", err.Error())
	}

	select {
	case err := <-serverErrCh:
		_ = err // already logged via logger.Fatalf
	default:
	}
}

// withDesktopSetupRoute serves the desktop setup page at desktopSetupPath in
// front of the proxied backend, so run mode can be changed at any time from
// the menu.
func withDesktopSetupRoute(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == desktopSetupPath {
			serveDesktopSetup(w, r)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func configureDesktopStorage(execPath string) {
	switch goruntime.GOOS {
	case "darwin":
		configureDesktopStorageMac(execPath)
	case "windows":
		configureDesktopStorageWindows()
	}
}

func configureDesktopStorageMac(execPath string) {
	if execPath == "" || !strings.Contains(execPath, ".app/Contents/MacOS") {
		return
	}

	appSupportDir, err := defaultMacAppSupportDir(execPath)
	if err != nil {
		logger.Warnf(context.Background(), "Failed to resolve app support dir: %v", err)
		return
	}

	legacyResourcesDir := filepath.Join(filepath.Dir(filepath.Dir(execPath)), "Resources")
	targetDataDir := filepath.Join(appSupportDir, "data")
	migrateLegacyDesktopData(legacyResourcesDir, targetDataDir)

	dbPath := resolveDesktopDataPath(os.Getenv("DB_PATH"), filepath.Join("data", "zhishu.db"), appSupportDir)
	filesPath := resolveDesktopDataPath(os.Getenv("LOCAL_STORAGE_BASE_DIR"), filepath.Join("data", "files"), appSupportDir)

	applyDesktopDataPaths(dbPath, filesPath)
}

// configureDesktopStorageWindows redirects the SQLite DB and uploaded files to
// %AppData%\ZhiShu Lite\data so a packaged exe works regardless of CWD.
func configureDesktopStorageWindows() {
	cfgDir, err := os.UserConfigDir()
	if err != nil || cfgDir == "" {
		logger.Warnf(context.Background(), "Failed to resolve user config dir for desktop storage: %v", err)
		return
	}
	appDir := filepath.Join(cfgDir, "ZhiShu Lite")

	dbPath := resolveDesktopDataPath(os.Getenv("DB_PATH"), filepath.Join("data", "zhishu.db"), appDir)
	filesPath := resolveDesktopDataPath(os.Getenv("LOCAL_STORAGE_BASE_DIR"), filepath.Join("data", "files"), appDir)

	applyDesktopDataPaths(dbPath, filesPath)
}

func applyDesktopDataPaths(dbPath, filesPath string) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		logger.Warnf(context.Background(), "Failed to create desktop DB directory %s: %v", filepath.Dir(dbPath), err)
	}
	if err := os.MkdirAll(filesPath, 0o755); err != nil {
		logger.Warnf(context.Background(), "Failed to create desktop files directory %s: %v", filesPath, err)
	}

	_ = os.Setenv("DB_PATH", dbPath)
	_ = os.Setenv("LOCAL_STORAGE_BASE_DIR", filesPath)
	logger.Infof(context.Background(), "Desktop storage: db=%s files=%s", dbPath, filesPath)
}

func defaultMacAppSupportDir(execPath string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	appName := "ZhiShu Lite"
	if idx := strings.Index(execPath, ".app/Contents/MacOS"); idx >= 0 {
		bundleName := filepath.Base(execPath[:idx+4])
		if trimmed := strings.TrimSuffix(bundleName, ".app"); trimmed != "" {
			appName = trimmed
		}
	}

	return filepath.Join(homeDir, "Library", "Application Support", appName), nil
}

func resolveDesktopDataPath(rawPath, defaultRelativePath, appSupportDir string) string {
	trimmed := strings.TrimSpace(rawPath)
	if trimmed == "" {
		trimmed = defaultRelativePath
	}
	if filepath.IsAbs(trimmed) {
		return filepath.Clean(trimmed)
	}
	trimmed = strings.TrimPrefix(trimmed, "."+string(filepath.Separator))
	return filepath.Join(appSupportDir, filepath.Clean(trimmed))
}

// desktopBackendListenAddr returns the TCP address for the embedded Gin server (Wails desktop).
// Binds 127.0.0.1 by default, or 0.0.0.0 when http_bind_public is set in desktop-prefs.json.
func desktopBackendListenAddr() string {
	host := "127.0.0.1"
	if LoadDesktopHTTPBindPublic() {
		host = "0.0.0.0"
	}
	pref := LoadDesktopPrefsHTTPPort()
	if pref >= 1 && pref <= 65535 {
		return net.JoinHostPort(host, strconv.Itoa(pref))
	}
	return net.JoinHostPort(host, "0")
}

// desktopPreferredLANIPv4 picks a non-loopback IPv4 for “other devices on this network” URL hints.
func desktopPreferredLANIPv4() net.IP {
	ifaces, err := net.Interfaces()
	if err == nil {
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, aerr := iface.Addrs()
			if aerr != nil {
				continue
			}
			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok || ipNet.IP == nil {
					continue
				}
				ip4 := ipNet.IP.To4()
				if ip4 == nil || ip4.IsLoopback() {
					continue
				}
				if ip4.IsPrivate() {
					return ip4
				}
			}
		}
		for _, iface := range ifaces {
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, aerr := iface.Addrs()
			if aerr != nil {
				continue
			}
			for _, addr := range addrs {
				ipNet, ok := addr.(*net.IPNet)
				if !ok || ipNet.IP == nil {
					continue
				}
				ip4 := ipNet.IP.To4()
				if ip4 == nil || ip4.IsLoopback() || ip4.IsUnspecified() {
					continue
				}
				return ip4
			}
		}
	}
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return nil
	}
	defer conn.Close()
	la, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok || la.IP == nil {
		return nil
	}
	return la.IP.To4()
}

func migrateLegacyDesktopData(resourcesDir, targetDataDir string) {
	legacyDataDir := filepath.Join(resourcesDir, "data")
	if info, err := os.Stat(legacyDataDir); err != nil || !info.IsDir() {
		return
	}
	if _, err := os.Stat(targetDataDir); err == nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(targetDataDir), 0o755); err != nil {
		logger.Warnf(context.Background(), "Failed to create app support parent dir %s: %v", filepath.Dir(targetDataDir), err)
		return
	}
	if err := os.Rename(legacyDataDir, targetDataDir); err != nil {
		logger.Warnf(context.Background(), "Failed to migrate legacy desktop data from %s to %s: %v", legacyDataDir, targetDataDir, err)
		return
	}
	logger.Infof(context.Background(), "Migrated legacy desktop data to %s", targetDataDir)
}

func listenWithRetry(addr string, maxRetries int, baseDelay time.Duration) (net.Listener, error) {
	var lastErr error
	for i := 0; i < maxRetries; i++ {
		listener, err := net.Listen("tcp", addr)
		if err == nil {
			return listener, nil
		}
		lastErr = err
		if i < maxRetries-1 {
			delay := baseDelay * time.Duration(1<<uint(i))
			if delay > 3*time.Second {
				delay = 3 * time.Second
			}
			time.Sleep(delay)
		}
	}
	return nil, lastErr
}

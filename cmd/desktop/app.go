package main

import (
	"context"
	"encoding/base64"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App holds Wails-bound state for the desktop shell.
type App struct {
	ctx           context.Context
	backendURL    string
	apiLanBaseURL string
	listenPublic  bool
	gatewayURL    string // loopback HTTP origin the webview loads (Windows gateway)
	shutdownCh    chan struct{}
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{
		shutdownCh: make(chan struct{}, 1),
	}
}

// startup is called when the application starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	a.shutdownCh <- struct{}{}
}

// GetAPIBaseURL returns the local HTTP base URL for REST API calls (e.g. http://127.0.0.1:PORT/api/v1).
// The desktop shell proxies the webview to this address; window.location.origin is not the API host.
func (a *App) GetAPIBaseURL() string {
	if a.backendURL == "" {
		return ""
	}
	return strings.TrimRight(a.backendURL, "/") + "/api/v1"
}

// GetDesktopHTTPPortSetting returns the saved local API port (0 = random port each launch).
func (a *App) GetDesktopHTTPPortSetting() int {
	return LoadDesktopPrefsHTTPPort()
}

// SetDesktopHTTPPortSetting saves the preferred local API port to application support. Restart the app for it to take effect unless it matches the current listener.
func (a *App) SetDesktopHTTPPortSetting(port int) error {
	return SaveDesktopHTTPPortPreference(port)
}

// GetDesktopHTTPBindPublicSetting returns whether API listens on all interfaces (0.0.0.0).
func (a *App) GetDesktopHTTPBindPublicSetting() bool {
	return LoadDesktopHTTPBindPublic()
}

// SetDesktopHTTPBindPublicSetting saves LAN/public listen preference. Restart the app for it to take effect.
func (a *App) SetDesktopHTTPBindPublicSetting(v bool) error {
	return SaveDesktopHTTPBindPublicPreference(v)
}

// GetAPILanBaseURL returns a suggested base URL for other devices on the LAN (…/api/v1), or empty if not in bind-public mode or IP detection failed.
func (a *App) GetAPILanBaseURL() string {
	return a.apiLanBaseURL
}

// GetDesktopListenPublicActive is true when this session’s API server is listening on all interfaces (runtime), not the saved preference.
func (a *App) GetDesktopListenPublicActive() bool {
	return a.listenPublic
}

// CheckForUpdates manually triggers the update check from the frontend.
//
// DISABLED for this fork: the upstream updater targets Tencent/WeKnora's
// GitHub releases — "upgrading" would replace this customized build with the
// stock upstream binary. Re-enable only after pointing checkUpdate() at this
// fork's own release feed.
func (a *App) CheckForUpdates() {
	if a.ctx == nil {
		return
	}
	wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.InfoDialog,
		Title:         "Check for Updates",
		Message:       "当前为定制版本，自动更新已停用。\n如需获取新版本，请联系系统管理员。",
		Buttons:       []string{"OK"},
		DefaultButton: "OK",
	})
}

// AutoCheckForUpdates is a no-op for this fork — see CheckForUpdates.
// The startup auto-check must never offer the upstream binary to users.
func (a *App) AutoCheckForUpdates() {
	// intentionally empty
}

// ── Desktop run-mode (local / remote) ────────────────────────────────────────

// TestServerURL probes a candidate server address for a running ZhiShu
// backend by requesting GET {url}/health. Returns "" on success or a
// user-readable error message (bound method surfaces it to the setup page).
func (a *App) TestServerURL(raw string) string {
	base := NormalizeServerURL(raw)
	if base == "" {
		return "地址无效，请输入形如 http://192.168.1.100:8080 的服务器地址"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(base + "/health")
	if err != nil {
		return "无法连接该地址，请检查服务器是否已启动、IP 和端口是否正确、防火墙是否放行"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "该地址可达，但返回了异常状态码 " + strconv.Itoa(resp.StatusCode) + "，请确认这是智述系统服务端口"
	}
	return ""
}

// GetDesktopSetupState returns the saved run mode and server URL for the
// setup page to prefill.
func (a *App) GetDesktopSetupState() map[string]string {
	return map[string]string{
		"mode":      loadDesktopMode(),
		"serverURL": loadDesktopServerURL(),
	}
}

// ApplyDesktopSetup validates and persists the chosen run mode, then
// restarts the app so the new mode takes effect. Returns "" on success or a
// user-readable error message.
func (a *App) ApplyDesktopSetup(mode, serverURL string) string {
	switch mode {
	case "local":
		// no address needed
	case "remote":
		if msg := a.TestServerURL(serverURL); msg != "" {
			return msg
		}
	default:
		return "请先选择运行方式"
	}
	if err := SaveDesktopServerConfig(mode, serverURL); err != nil {
		return err.Error()
	}
	go func() {
		time.Sleep(400 * time.Millisecond) // let the webview receive the response
		restartApp(a.ctx)
	}()
	return ""
}

// restartApp spawns a new instance of this executable and quits the current
// one. Used after setup changes that require re-initializing the backend or
// proxy target.
func restartApp(ctx context.Context) {
	execPath, err := os.Executable()
	if err != nil {
		os.Exit(1)
	}
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd.exe", "/C", "start", "", execPath)
		if err := cmd.Start(); err != nil {
			os.Exit(1)
		}
	} else {
		if err := exec.Command(execPath).Start(); err != nil {
			os.Exit(1)
		}
	}
	if ctx != nil {
		wailsruntime.Quit(ctx)
	} else {
		os.Exit(0)
	}
}

// ── Native file helpers (downloads / open / reveal) ──────────────────────────

// maxBridgePayloadBytes caps payloads transferred through the JS bridge
// (base64 in a JSON string). Larger payloads fall back to the webview's
// default download behaviour.
const maxBridgePayloadBytes = 48 << 20

// SaveBlobFile opens a native "Save as" dialog and writes the base64-encoded
// payload provided by the webview. Returns the saved absolute path, or ""
// when the user cancels or the write fails.
func (a *App) SaveBlobFile(filename string, dataB64 string) string {
	if a.ctx == nil {
		return ""
	}
	data, err := base64.StdEncoding.DecodeString(dataB64)
	if err != nil {
		return ""
	}
	name := filepath.Base(strings.TrimSpace(filename))
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = "download.bin"
	}
	startDir := downloadsDir()
	path, err := wailsruntime.SaveFileDialog(a.ctx, wailsruntime.SaveDialogOptions{
		Title:            "保存文件",
		DefaultFilename:  name,
		DefaultDirectory: startDir,
		Filters: []wailsruntime.FileFilter{
			{DisplayName: "所有文件 (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil || path == "" {
		return ""
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return ""
	}
	return path
}

// OpenFilePath opens a local file with the OS default application.
func (a *App) OpenFilePath(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", abs)
	case "darwin":
		cmd = exec.Command("open", abs)
	default:
		cmd = exec.Command("xdg-open", abs)
	}
	return cmd.Start() == nil
}

// RevealInFolder opens the OS file manager with the given file selected.
func (a *App) RevealInFolder(path string) bool {
	if strings.TrimSpace(path) == "" {
		return false
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("explorer", "/select,", abs)
	case "darwin":
		cmd = exec.Command("open", "-R", abs)
	default:
		cmd = exec.Command("xdg-open", filepath.Dir(abs))
	}
	return cmd.Start() == nil
}

func downloadsDir() string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ""
	}
	dir := filepath.Join(home, "Downloads")
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return ""
	}
	return dir
}

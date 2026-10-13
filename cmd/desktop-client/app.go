package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

const clientPrefsFileName = "client-prefs.json"

// clientPrefs persists the only thing the slim client needs to know: where
// the server is.
type clientPrefs struct {
	ServerURL string `json:"server_url"`
}

func clientPrefsDir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cfg, "ZhiShu Client")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func loadClientServerURL() string {
	dir, err := clientPrefsDir()
	if err != nil {
		return ""
	}
	data, err := os.ReadFile(filepath.Join(dir, clientPrefsFileName))
	if err != nil {
		return ""
	}
	var p clientPrefs
	if json.Unmarshal(data, &p) != nil {
		return ""
	}
	return NormalizeServerURL(p.ServerURL)
}

func saveClientServerURL(raw string) error {
	dir, err := clientPrefsDir()
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(clientPrefs{ServerURL: NormalizeServerURL(raw)}, "", "  ")
	return os.WriteFile(filepath.Join(dir, clientPrefsFileName), data, 0o600)
}

// clientApp holds Wails-bound state for the slim client shell.
type clientApp struct {
	ctx        context.Context
	serverURL  string
	gatewayURL string
}

func newClientApp() *clientApp { return &clientApp{} }

// NormalizeServerURL cleans a user-entered server address: adds http:// when
// the scheme is missing, strips trailing slashes and any /api/v1 suffix.
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
	u.Path = strings.TrimSuffix(u.Path, "/")
	u.Path = strings.TrimSuffix(u.Path, "/api/v1")
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func (a *clientApp) startup(ctx context.Context) { a.ctx = ctx }

// TestServerURL probes GET {url}/health on a candidate server. Returns ""
// on success or a user-readable error (bound method for the setup page).
func (a *clientApp) TestServerURL(raw string) string {
	base := NormalizeServerURL(raw)
	if base == "" {
		return "地址无效，请输入形如 http://192.168.1.100:80 的服务器地址"
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get(base + "/health")
	if err != nil {
		return "无法连接该地址，请检查服务器是否已启动、IP 和端口是否正确、防火墙是否放行"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "该地址可达，但返回了异常状态码 " + strconv.Itoa(resp.StatusCode) + "，请确认这是智述系统前端地址"
	}
	return ""
}

// GetClientSetupState returns the saved server URL for the setup page.
func (a *clientApp) GetClientSetupState() map[string]string {
	return map[string]string{"serverURL": loadClientServerURL()}
}

// ApplyClientSetup validates and persists the server address, then restarts
// the app so the connection takes effect. Returns "" on success or a
// user-readable error message.
func (a *clientApp) ApplyClientSetup(raw string) string {
	if msg := a.TestServerURL(raw); msg != "" {
		return msg
	}
	if err := saveClientServerURL(raw); err != nil {
		return err.Error()
	}
	go func() {
		time.Sleep(400 * time.Millisecond) // let the webview receive the response
		restartClient(a.ctx)
	}()
	return ""
}

// restartClient spawns a fresh instance and quits the current one.
func restartClient(ctx context.Context) {
	execPath, err := os.Executable()
	if err != nil {
		os.Exit(1)
	}
	if runtime.GOOS == "windows" {
		if err := exec.Command("cmd.exe", "/C", "start", "", execPath).Start(); err != nil {
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

// ── Native helpers used by the gateway endpoints ────────────────────────────

// SaveBlobFile opens a native "Save as" dialog and writes the base64 payload
// from the webview. Returns the saved path, or "" on cancel/failure.
func (a *clientApp) SaveBlobFile(filename string, dataB64 string) string {
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
	startDir := clientDownloadsDir()
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
func (a *clientApp) OpenFilePath(path string) bool {
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
func (a *clientApp) RevealInFolder(path string) bool {
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

func clientDownloadsDir() string {
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

// CheckForUpdates is a stub: the slim client follows the server deployment.
func (a *clientApp) CheckForUpdates() {
	if a.ctx == nil {
		return
	}
	wailsruntime.MessageDialog(a.ctx, wailsruntime.MessageDialogOptions{
		Type:          wailsruntime.InfoDialog,
		Title:         "检查更新",
		Message:       "客户端随服务器部署更新，请联系系统管理员获取新版本。",
		Buttons:       []string{"OK"},
		DefaultButton: "OK",
	})
}

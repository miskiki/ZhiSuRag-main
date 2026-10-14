package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
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
	// 服务器地址只保留 origin：用户常会带上 /login、/api/v1 等路径，
	// 一律剥离（端口保留），避免代理基址带路径导致资源 404。
	u.Path = ""
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
	// 探针用前端自带的 /config.js（nginx 静态提供，不受 SPA fallback 影响）：
	// 200 且内容非 HTML 才认定为智述前端地址。
	resp, err := client.Get(base + "/config.js")
	if err != nil {
		return "无法连接该地址，请检查服务器是否已启动、IP 和端口是否正确、防火墙是否放行"
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "该地址可达，但返回了异常状态码 " + strconv.Itoa(resp.StatusCode) + "，请确认这是智述系统前端地址"
	}
	buf := make([]byte, 256)
	n, _ := resp.Body.Read(buf)
	if strings.Contains(strings.ToLower(string(buf[:n])), "<!doctype") ||
		strings.Contains(strings.ToLower(string(buf[:n])), "<html") {
		return "该地址返回的不是智述系统页面，请确认端口是否为前端端口（默认 80，不带 /login 等路径）"
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

// ── Desktop API handlers (shared by wails:// asset server and gateway) ──────

func (a *clientApp) handleRPC(w http.ResponseWriter, r *http.Request) {
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
		result = a.TestServerURL(args[0])
	case "ApplyClientSetup":
		result = a.ApplyClientSetup(args[0])
	case "GetClientSetupState":
		result = a.GetClientSetupState()
	}
	writeJSON(w, map[string]interface{}{"ok": true, "result": result})
}

func (a *clientApp) handleSaveBlob(w http.ResponseWriter, r *http.Request) {
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
	path := a.SaveBlobFile(req.Name, req.DataB64)
	if path == "" {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "cancelled or failed"})
		return
	}
	writeJSON(w, map[string]interface{}{"ok": true, "path": path})
}

func (a *clientApp) handleOpen(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{"ok": a.OpenFilePath(r.URL.Query().Get("path"))})
}

func (a *clientApp) handleReveal(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, map[string]interface{}{"ok": a.RevealInFolder(r.URL.Query().Get("path"))})
}

func (a *clientApp) handleOpenExternal(w http.ResponseWriter, r *http.Request) {
	target := strings.TrimSpace(r.URL.Query().Get("url"))
	if !strings.HasPrefix(target, "http://") && !strings.HasPrefix(target, "https://") {
		writeJSON(w, map[string]interface{}{"ok": false, "error": "invalid url"})
		return
	}
	if a.ctx != nil {
		wailsruntime.BrowserOpenURL(a.ctx, target)
	}
	writeJSON(w, map[string]interface{}{"ok": true})
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(v)
}

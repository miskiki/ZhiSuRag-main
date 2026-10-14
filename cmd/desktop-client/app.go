package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
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

// ── Prefs: last server + connection history ────────────────────────────────

const clientPrefsFileName = "client-prefs.json"

const maxHistoryEntries = 5

// clientPrefs persists the last connected server and recent history.
type clientPrefs struct {
	ServerURL string   `json:"server_url"`
	History   []string `json:"history"`
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

func loadClientPrefs() clientPrefs {
	var p clientPrefs
	dir, err := clientPrefsDir()
	if err != nil {
		return p
	}
	data, err := os.ReadFile(filepath.Join(dir, clientPrefsFileName))
	if err != nil {
		return p
	}
	if json.Unmarshal(data, &p) != nil {
		return clientPrefs{}
	}
	p.ServerURL = NormalizeServerURL(p.ServerURL)
	clean := p.History[:0]
	seen := map[string]bool{}
	for _, h := range p.History {
		h = NormalizeServerURL(h)
		if h == "" || seen[h] {
			continue
		}
		seen[h] = true
		clean = append(clean, h)
	}
	p.History = clean
	return p
}

func saveClientPrefs(p clientPrefs) error {
	dir, err := clientPrefsDir()
	if err != nil {
		return err
	}
	data, _ := json.MarshalIndent(p, "", "  ")
	return os.WriteFile(filepath.Join(dir, clientPrefsFileName), data, 0o600)
}

// recordConnected sets the last-used server and moves it to the front of the
// connection history (deduped, capped at maxHistoryEntries).
func recordConnected(raw string) {
	u := NormalizeServerURL(raw)
	if u == "" {
		return
	}
	p := loadClientPrefs()
	p.ServerURL = u
	history := []string{u}
	for _, h := range p.History {
		if h == u {
			continue
		}
		history = append(history, h)
		if len(history) >= maxHistoryEntries {
			break
		}
	}
	p.History = history
	_ = saveClientPrefs(p)
}

// ── App state ───────────────────────────────────────────────────────────────

// clientApp holds Wails-bound state for the slim client shell.
type clientApp struct {
	ctx        context.Context
	gateway    *clientGateway
	gatewayURL string
}

func newClientApp() *clientApp { return &clientApp{} }

func (a *clientApp) startup(ctx context.Context) { a.ctx = ctx }

// ensureGateway starts the loopback gateway on first use and (re)points it at
// targetURL. Reuses the existing gateway on later calls, so switching servers
// takes effect without restarting the app.
func (a *clientApp) ensureGateway(targetURL string) (string, error) {
	if a.gateway != nil {
		if err := a.gateway.SetTarget(targetURL); err != nil {
			return "", err
		}
		return a.gateway.baseURL, nil
	}
	g, err := startClientGateway(a, targetURL)
	if err != nil {
		return "", err
	}
	a.gateway = g
	a.gatewayURL = g.baseURL
	return g.baseURL, nil
}

// ── Setup-page API (Wails bindings on wails:// origin, RPC on gateway) ─────

// TestServerURL probes GET {url}/config.js on a candidate server. Returns ""
// on success or a user-readable error (bound method for the setup page).
func (a *clientApp) TestServerURL(raw string) string {
	base := NormalizeServerURL(raw)
	if base == "" {
		return "地址无效，请输入形如 http://192.168.10.131 的服务器地址（不带 /login 等路径）"
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
	lower := strings.ToLower(string(buf[:n]))
	if strings.Contains(lower, "<!doctype") || strings.Contains(lower, "<html") {
		return "该地址返回的不是智述系统页面，请确认端口是否为前端端口（默认 80，不带 /login 等路径）"
	}
	return ""
}

// GetClientSetupState returns everything the setup page needs to prefill:
// last server, connection history, and the baked-in default address.
func (a *clientApp) GetClientSetupState() map[string]interface{} {
	p := loadClientPrefs()
	return map[string]interface{}{
		"serverURL":  p.ServerURL,
		"history":    p.History,
		"defaultURL": defaultServerURL,
	}
}

// ApplyClientSetup validates and persists the server address, points the
// gateway at it, records connection history, and returns the gateway URL the
// setup page should navigate to — no app restart needed. Returns a map with
// "error" (empty on success) and "gatewayURL".
func (a *clientApp) ApplyClientSetup(raw string) map[string]interface{} {
	if msg := a.TestServerURL(raw); msg != "" {
		return map[string]interface{}{"error": msg}
	}
	base := NormalizeServerURL(raw)
	gwURL, err := a.ensureGateway(base)
	if err != nil {
		return map[string]interface{}{"error": "启动连接通道失败：" + err.Error()}
	}
	recordConnected(base)
	return map[string]interface{}{"error": "", "gatewayURL": gwURL}
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

// ── Desktop API handlers（wails:// assetserver 与网关共用）──────────────────

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

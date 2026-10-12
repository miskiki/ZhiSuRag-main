package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

const desktopPrefsFileName = "desktop-prefs.json"

type desktopPrefs struct {
	HTTPPort       int    `json:"http_port"`
	HTTPBindPublic bool   `json:"http_bind_public"`
	Mode           string `json:"mode"`       // "", "local", "remote"
	ServerURL      string `json:"server_url"` // remote mode backend base URL, e.g. http://192.168.1.10:8080
}

// NormalizeServerURL cleans a user-entered server address: adds http:// when
// the scheme is missing, strips a trailing slash and any /api/v1 suffix so
// users can paste either the root or an API URL.
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

func desktopPrefsDir() (string, error) {
	cfg, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(cfg, "ZhiShu Lite")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

func desktopPrefsFilePath() (string, error) {
	dir, err := desktopPrefsDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, desktopPrefsFileName), nil
}

func loadDesktopPrefs() desktopPrefs {
	path, err := desktopPrefsFilePath()
	if err != nil {
		return desktopPrefs{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return desktopPrefs{}
	}
	var p desktopPrefs
	if json.Unmarshal(data, &p) != nil {
		return desktopPrefs{}
	}
	if p.HTTPPort < 0 || p.HTTPPort > 65535 {
		p.HTTPPort = 0
	}
	return p
}

func saveDesktopPrefs(p desktopPrefs) error {
	path, err := desktopPrefsFilePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// LoadDesktopPrefsHTTPPort returns http_port from prefs file, or 0 if unset / invalid (ephemeral port on each launch).
func LoadDesktopPrefsHTTPPort() int {
	return loadDesktopPrefs().HTTPPort
}

// LoadDesktopHTTPBindPublic returns whether the embedded API server should listen on all interfaces (0.0.0.0).
func LoadDesktopHTTPBindPublic() bool {
	return loadDesktopPrefs().HTTPBindPublic
}

// SaveDesktopHTTPPortPreference persists listen port preference. port 0 means use a random free port on each launch.
func SaveDesktopHTTPPortPreference(port int) error {
	if port < 0 || port > 65535 {
		return fmt.Errorf("invalid port")
	}
	cur := loadDesktopPrefs()
	cur.HTTPPort = port
	return saveDesktopPrefs(cur)
}

// SaveDesktopHTTPBindPublicPreference persists whether to listen on 0.0.0.0 for LAN/public access.
func SaveDesktopHTTPBindPublicPreference(v bool) error {
	cur := loadDesktopPrefs()
	cur.HTTPBindPublic = v
	return saveDesktopPrefs(cur)
}

// SaveDesktopServerConfig persists the run mode ("local" / "remote") and, for
// remote mode, the backend base URL. mode "" clears the saved choice so the
// next launch shows the setup page again.
func SaveDesktopServerConfig(mode, serverURL string) error {
	if mode != "local" && mode != "remote" && mode != "" {
		return fmt.Errorf("invalid mode")
	}
	cur := loadDesktopPrefs()
	cur.Mode = mode
	switch mode {
	case "remote":
		normalized := NormalizeServerURL(serverURL)
		if normalized == "" {
			return fmt.Errorf("服务器地址无效")
		}
		cur.ServerURL = normalized
	case "local":
		cur.ServerURL = ""
	}
	return saveDesktopPrefs(cur)
}

func loadDesktopMode() string {
	mode := strings.TrimSpace(loadDesktopPrefs().Mode)
	if mode != "local" && mode != "remote" {
		return ""
	}
	return mode
}

func loadDesktopServerURL() string {
	return NormalizeServerURL(loadDesktopPrefs().ServerURL)
}

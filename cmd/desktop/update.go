package main

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/Tencent/WeKnora/internal/handler"
)

// desktopAboutVersion resolves the displayed version for the About dialog.
// It prefers the version injected at build time (scripts/get_version.sh) and
// falls back to the repository VERSION file for local `wails dev` runs.
func desktopAboutVersion() string {
	if v := strings.TrimSpace(handler.Version); v != "" && v != "unknown" {
		return v
	}
	for _, p := range []string{
		"VERSION",
		filepath.Join("..", "..", "VERSION"),
		filepath.Join("..", "..", "..", "VERSION"),
	} {
		b, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		if v := strings.TrimSpace(string(b)); v != "" {
			return v
		}
	}
	return "unknown"
}

// 更新检查已在 v1.6 中停用：本仓库为定制版本，上游（Tencent/WeKnora）的
// 自动更新会用原版二进制覆盖定制功能。App.CheckForUpdates /
// App.AutoCheckForUpdates 目前为空实现；恢复更新能力前需先将更新源指向
// 本仓库自己的 release 渠道。

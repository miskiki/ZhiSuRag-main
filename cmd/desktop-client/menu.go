package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/wailsapp/wails/v2/pkg/menu"
	"github.com/wailsapp/wails/v2/pkg/menu/keys"
	wailsruntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// buildAppMenu wires the slim client's menu bar: About (version + repo),
// server settings navigation, and Quit. No update checks — the client follows
// the server deployment.
func buildAppMenu(app *clientApp) *menu.Menu {
	AppMenu := menu.NewMenu()
	FileMenu := AppMenu.AddSubmenu("ZhiShu Client")

	FileMenu.AddText("About ZhiShu Client", keys.CmdOrCtrl("i"), func(_ *menu.CallbackData) {
		if app.ctx == nil {
			return
		}
		choice, err := wailsruntime.MessageDialog(app.ctx, wailsruntime.MessageDialogOptions{
			Type:          wailsruntime.InfoDialog,
			Title:         "About ZhiShu Client",
			Message:       fmt.Sprintf("ZhiShu Client（智枢文渊客户端）\n\n连接服务器模式的轻量桌面客户端：\n账号、知识库与配置均保存在服务器上，与网页端完全一致。\n\nVersion %s\n© 2026 智枢文渊\n\nGitHub:\n%s", clientVersion, productGitHubRepoURL),
			Buttons:       []string{"Open GitHub", "OK"},
			DefaultButton: "OK",
		})
		if err != nil {
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
		target := clientSetupPath
		if gw := app.GatewayURL(); gw != "" {
			// webview 当前在网关 HTTP 源上，用绝对地址确保可达
			target = gw + clientSetupPath
		}
		wailsruntime.WindowExecJS(app.ctx, fmt.Sprintf("location.href=%s;", quoteJS(target)))
	})

	FileMenu.AddSeparator()
	FileMenu.AddText("Quit", keys.CmdOrCtrl("q"), func(_ *menu.CallbackData) {
		wailsruntime.Quit(app.ctx)
		os.Exit(0)
	})

	AppMenu.Append(menu.EditMenu())

	ViewMenu := AppMenu.AddSubmenu("View")
	ViewMenu.AddText("Reload", keys.CmdOrCtrl("r"), func(_ *menu.CallbackData) {})

	return AppMenu
}

func quoteJS(s string) string {
	q := strings.ReplaceAll(s, `\`, `\\`)
	q = strings.ReplaceAll(q, `"`, `\"`)
	return `"` + q + `"`
}

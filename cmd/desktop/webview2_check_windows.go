//go:build windows

package main

import (
	"golang.org/x/sys/windows/registry"
)

// webview2ClientGUID is the evergreen WebView2 runtime registration GUID
// (per Microsoft's distribution docs). Presence under EdgeUpdate\Clients
// (HKLM for machine-wide, HKCU for per-user installs) means the runtime is
// installed and the embedded browser can start.
const webview2ClientGUID = `{F3017226-FE2A-4295-8BDF-00C3A9A7E4C5}`

const webview2DownloadURL = "https://go.microsoft.com/fwlink/p/?LinkId=2124703"

// webview2RuntimeInstalled reports whether the WebView2 evergreen runtime
// is registered on this machine.
func webview2RuntimeInstalled() bool {
	roots := []struct {
		root   registry.Key
		suffix string
	}{
		{registry.LOCAL_MACHINE, `SOFTWARE\WOW6432Node\Microsoft\EdgeUpdate\Clients\`},
		{registry.LOCAL_MACHINE, `SOFTWARE\Microsoft\EdgeUpdate\Clients\`},
		{registry.CURRENT_USER, `SOFTWARE\Microsoft\EdgeUpdate\Clients\`},
	}
	for _, r := range roots {
		k, err := registry.OpenKey(r.root, r.suffix+webview2ClientGUID, registry.QUERY_VALUE)
		if err == nil {
			k.Close()
			return true
		}
	}
	return false
}

// ensureWebView2Runtime verifies the WebView2 runtime before the window is
// created. On machines without it the app would otherwise fail with an
// obscure error; a native message box explains the one-time fix instead.
// Returns false when the runtime is missing and the app should exit.
func ensureWebView2Runtime() bool {
	if webview2RuntimeInstalled() {
		return true
	}
	title := "智枢文渊 · 缺少运行组件"
	msg := "本应用需要微软 WebView2 运行组件（Windows 10/11 通常已内置，" +
		"当前电脑未检测到）。\n\n" +
		"请用浏览器打开下面的链接，下载并安装一次即可（约 2 分钟）：\n" +
		webview2DownloadURL + "\n\n" +
		"安装完成后重新打开本应用。"
	messageBox(title, msg)
	return false
}

//go:build bindings

package main

import (
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
)

// Wails 绑定生成阶段（wails generate / bindings 步骤）使用本文件单独编译，
// 不启动网关与设置逻辑。
func main() {
	app := newClientApp()
	_ = wails.Run(&options.App{
		Title: "ZhiShu Client",
		Bind:  []interface{}{app},
	})
}

//go:build windows

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"time"
)

// startupLogPath resolves where the startup black-box log lives: next to the
// exe when writable (portable distribution), otherwise %AppData%\ZhiShu Lite.
// Returns "" when neither location is writable.
func startupLogPath() string {
	var candidates []string
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Dir(exe))
	}
	if appdata, err := os.UserConfigDir(); err == nil {
		candidates = append(candidates, filepath.Join(appdata, "ZhiShu Lite"))
	}
	for _, dir := range candidates {
		probe := filepath.Join(dir, ".startup-write-probe")
		if f, err := os.Create(probe); err == nil {
			f.Close()
			_ = os.Remove(probe)
			return filepath.Join(dir, "startup-error.log")
		}
	}
	return ""
}

var startupLogFile *os.File

// startupLogInit opens the black-box log and records the launch.
func startupLogInit() {
	path := startupLogPath()
	if path == "" {
		return
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	startupLogFile = f
	startupLogf("==== launch (pid %d) ====", os.Getpid())
}

// startupLogf appends one timestamped line to the black-box log.
func startupLogf(format string, args ...interface{}) {
	if startupLogFile == nil {
		return
	}
	fmt.Fprintf(startupLogFile, "[%s] %s\n",
		time.Now().Format("2006-01-02 15:04:05.000"),
		fmt.Sprintf(format, args...))
}

// startupCrashGuard converts an unexpected panic during startup into a
// written crash trace (colleague machines have no console to show it).
func startupCrashGuard() {
	if r := recover(); r != nil {
		startupLogf("PANIC: %v\n%s", r, debug.Stack())
		startupLogClose()
		panic(r)
	}
}

func startupLogClose() {
	if startupLogFile != nil {
		startupLogf("==== exit ====")
		startupLogFile.Close()
		startupLogFile = nil
	}
}

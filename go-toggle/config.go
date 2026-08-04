package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config 程序配置
type Config struct {
	ToggleHotkey          string  `json:"toggle_hotkey"`
	ToggleEnabled         bool    `json:"toggle_enabled"`
	ExitHotkey            string  `json:"exit_hotkey"`
	ExitEnabled           bool    `json:"exit_enabled"`
	MonitorHotkey         string  `json:"monitor_hotkey"`
	MonitorEnabled        bool    `json:"monitor_enabled"`
	DblclickToggleEnabled bool    `json:"dblclick_toggle_enabled"`
	DblclickDesktopEnabled bool   `json:"dblclick_desktop_enabled"`
	DblclickTaskbarEnabled bool   `json:"dblclick_taskbar_enabled"`
	IdleHideTimeout       float64 `json:"idle_hide_timeout"`
}

func defaultConfig() *Config {
	return &Config{
		ToggleHotkey:           "ctrl+space",
		ToggleEnabled:          true,
		ExitHotkey:             "ctrl+shift+q",
		ExitEnabled:            true,
		MonitorHotkey:          "ctrl+win+alt",
		MonitorEnabled:         true,
		DblclickToggleEnabled:  true,
		DblclickDesktopEnabled: true,
		DblclickTaskbarEnabled: true,
		IdleHideTimeout:        0.0,
	}
}

func (c *Config) Validate() {
	if c.IdleHideTimeout < 0 {
		c.IdleHideTimeout = 0
	}
}

func getConfigPath() string {
	exe, err := os.Executable()
	if err != nil {
		exe, _ = os.Getwd()
	}
	return filepath.Join(filepath.Dir(exe), "config.json")
}

func loadConfig() *Config {
	cfg := defaultConfig()
	path := getConfigPath()
	data, err := os.ReadFile(path)
	if err == nil {
		var parsed Config
		if err := json.Unmarshal(data, &parsed); err == nil {
			if parsed.ToggleHotkey != "" {
				cfg.ToggleHotkey = parsed.ToggleHotkey
			}
			if parsed.ExitHotkey != "" {
				cfg.ExitHotkey = parsed.ExitHotkey
			}
			if parsed.MonitorHotkey != "" {
				cfg.MonitorHotkey = parsed.MonitorHotkey
			}
			cfg.ToggleEnabled = parsed.ToggleEnabled
			cfg.ExitEnabled = parsed.ExitEnabled
			cfg.MonitorEnabled = parsed.MonitorEnabled
			cfg.DblclickToggleEnabled = parsed.DblclickToggleEnabled
			cfg.DblclickDesktopEnabled = parsed.DblclickDesktopEnabled
			cfg.DblclickTaskbarEnabled = parsed.DblclickTaskbarEnabled
			cfg.IdleHideTimeout = parsed.IdleHideTimeout
		}
	}
	cfg.Validate()
	saveConfig(cfg)
	return cfg
}

func saveConfig(cfg *Config) {
	data, err := json.MarshalIndent(cfg, "", "    ")
	if err != nil {
		return
	}
	os.WriteFile(getConfigPath(), data, 0644)
}
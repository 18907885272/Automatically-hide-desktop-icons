package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// HotkeySpec 解析后的热键规格（不参与 JSON 序列化）
type HotkeySpec struct {
	Ctrl  bool
	Shift bool
	Alt   bool
	Win   bool
	Key   uint32 // 主键 vkCode
	Valid bool
}

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

	ToggleHK  HotkeySpec `json:"-"`
	ExitHK    HotkeySpec `json:"-"`
	MonitorHK HotkeySpec `json:"-"`
}

// hotkeyKeyByName 热键主键名称 → vkCode 映射
var hotkeyKeyByName = map[string]uint32{
	"space": VK_SPACE,
	"q":     VK_Q,
	"down":  VK_DOWN,
	"up":    VK_UP,
	"left":  VK_LEFT,
	"right": VK_RIGHT,
	"esc":   VK_ESCAPE,
	"enter": VK_RETURN,
	"tab":   VK_TAB,
}

func init() {
	// 字母 a-z
	for i := 0; i < 26; i++ {
		hotkeyKeyByName[string(rune('a'+i))] = uint32(0x41 + i) // VK_A = 0x41
	}
	// 数字 0-9
	for i := 0; i < 10; i++ {
		hotkeyKeyByName[string(rune('0'+i))] = uint32(0x30 + i) // VK_0 = 0x30
	}
	// F1-F12
	for i := 1; i <= 12; i++ {
		hotkeyKeyByName[fmt.Sprintf("f%d", i)] = uint32(0x6F + i) // VK_F1 = 0x70
	}
}

// parseHotkeySpec 解析形如 "ctrl+alt+down" 的热键字符串
func parseHotkeySpec(s string) HotkeySpec {
	var spec HotkeySpec
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "+")
	for _, p := range parts {
		p = strings.TrimSpace(p)
		switch p {
		case "ctrl", "control":
			spec.Ctrl = true
		case "shift":
			spec.Shift = true
		case "alt":
			spec.Alt = true
		case "win", "windows":
			spec.Win = true
		default:
			if k, ok := hotkeyKeyByName[p]; ok && spec.Key == 0 {
				spec.Key = k
			}
		}
	}
	spec.Valid = spec.Key != 0
	return spec
}

// ParseHotkeys 根据字符串字段重新解析三个热键规格
func (c *Config) ParseHotkeys() {
	c.ToggleHK = parseHotkeySpec(c.ToggleHotkey)
	c.ExitHK = parseHotkeySpec(c.ExitHotkey)
	c.MonitorHK = parseHotkeySpec(c.MonitorHotkey)
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
	cfg.ParseHotkeys()
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
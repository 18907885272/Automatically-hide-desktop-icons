package main

import (
	"runtime"
	"sync"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ============================================================
// 全局状态
// ============================================================

var (
	mouseHook    HHOOK
	keyboardHook HHOOK

	hookThreadID uint32
	exitCh       chan struct{}

	// 双击检测状态
	lastClickTime uint32
	lastClickPt   POINT

	idleTimer *time.Timer

	// 关屏保护窗口：此时间点前吞掉所有键盘/鼠标输入，防止输入唤醒显示器
	inputBlockUntil time.Time
	inputBlockMu    sync.Mutex
)

// blockInputFor 设置输入屏蔽截止时间（关屏后调用，防止 keyup/鼠标动作唤醒显示器）
func blockInputFor(d time.Duration) {
	inputBlockMu.Lock()
	inputBlockUntil = time.Now().Add(d)
	inputBlockMu.Unlock()
}

// isInputBlocked 判断当前是否处于输入屏蔽窗口内
func isInputBlocked() bool {
	inputBlockMu.Lock()
	defer inputBlockMu.Unlock()
	return time.Now().Before(inputBlockUntil)
}

// ============================================================
// 鼠标钩子
// ============================================================

func mouseHookProc(nCode, wParam, lParam uintptr) uintptr {
	if int(nCode) >= 0 {
		// 关屏保护窗口内吞掉所有鼠标事件，防止鼠标移动/点击唤醒显示器
		if isInputBlocked() {
			return 1
		}
		handleMouseEvent(int(nCode), wParam, lParam)
	}
	return CallNextHookEx(mouseHook, int(nCode), wParam, lParam)
}

func handleMouseEvent(nCode int, wParam, lParam uintptr) {
	if wParam != WM_LBUTTONUP {
		return
	}
	msll := (*MSLLHOOKSTRUCT)(unsafe.Pointer(lParam))
	pt := msll.Pt
	now := uint32(msll.Time)

	// 双击检测（纯内存比较，不调用 Win32 API）
	dt := GetDoubleClickTime()
	isDoubleClick := (now-lastClickTime < dt &&
		abs(int(pt.X-lastClickPt.X)) < 4 &&
		abs(int(pt.Y-lastClickPt.Y)) < 4)

	lastClickTime = now
	lastClickPt = pt

	// 非双击：直接返回，避免每次点击都调用 Win32 API 和写日志
	// 这是低层全局钩子，会收到全系统所有鼠标点击，必须保持轻量
	if !isDoubleClick {
		resetIdleTimer()
		return
	}

	// 以下只在双击时执行
	// 获取点击位置下的窗口
	hwnd := WindowFromPoint(pt)
	className := GetClassNameW(hwnd)

	// 判断是否在任务栏上
	isTaskbar := (className == "MSTaskListWClass" || className == "Shell_TrayWnd" || className == "Shell_SecondaryTrayWnd")

	// 判断是否在桌面上（Progman/WorkerW/SysListView32/SHELLDLL_DefView 都属于桌面区域，
	// SysListView32 覆盖整个桌面包括图标和空白处；
	// 图标隐藏后 WindowFromPoint 会返回其父窗口 SHELLDLL_DefView）
	isDesktop := (className == "Progman" || className == "WorkerW" || className == "SysListView32" || className == "SHELLDLL_DefView")

	// 只有双击任务栏空白区域才触发切换
	if isTaskbar && cfg.DblclickToggleEnabled && cfg.DblclickTaskbarEnabled {
		toggleDesktopIcons()
	} else if isDesktop && cfg.DblclickToggleEnabled && cfg.DblclickDesktopEnabled {
		// 双击桌面：判断是否点在图标上（用第一次点击的位置判断）
		iconHit := isClickOnDesktopIcon(lastClickPt)
		if !iconHit {
			toggleDesktopIcons()
		}
	}

	// 重置空闲定时器
	resetIdleTimer()
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// ============================================================
// 键盘钩子
// ============================================================

func keyboardHookProc(nCode, wParam, lParam uintptr) uintptr {
	if int(nCode) >= 0 {
		kbd := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
		injected := (kbd.Flags & LLKHF_INJECTED) != 0
		// 关屏保护窗口内吞掉真实键盘事件（松手 keyup 等），防止唤醒显示器；
		// 但放行程序注入的 keyup（releaseModifiers 补发的修饰键 keyup，带 LLKHF_INJECTED），
		// 让系统键状态恢复，避免 Ctrl/Alt 卡键导致后续快捷键错乱。
		if isInputBlocked() && !injected {
			return 1
		}
		if wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN {
			if handleHotkey(uint32(kbd.VkCode)) {
				// 命中热键：放行该按键，让修饰键正常释放。
				// 吞键会导致系统认为 Ctrl/Alt/Win 等修饰键一直按下，后续所有快捷键错乱。
				// 正确做法：只放行修饰键的 keyup（releaseModifiers 补发），让系统键状态恢复。
				// 关屏保护窗口内，真实输入（松手 keyup 等）会被吞掉，防止唤醒显示器。
			}
		}
	}
	return CallNextHookEx(keyboardHook, int(nCode), wParam, lParam)
}

// handleHotkey 判断是否命中热键并执行对应动作，返回是否命中（供调用方决定吞键）
func handleHotkey(vkCode uint32) bool {
	// 当前正在按下的键（vkCode）直接视为已按下，因为 GetAsyncKeyState
	// 在低层键盘钩子回调中对当前键可能返回 false
	currentIsCtrl := vkCode == VK_LCONTROL || vkCode == VK_RCONTROL || vkCode == VK_CONTROL
	currentIsShift := vkCode == VK_LSHIFT || vkCode == VK_RSHIFT || vkCode == VK_SHIFT
	currentIsAlt := vkCode == VK_LMENU || vkCode == VK_RMENU || vkCode == VK_MENU
	currentIsWin := vkCode == VK_LWIN || vkCode == VK_RWIN

	// 其他修饰键用 GetAsyncKeyState 检测
	ctrl := currentIsCtrl || (GetAsyncKeyState(VK_CONTROL)&0x8000 != 0)
	shift := currentIsShift || (GetAsyncKeyState(VK_SHIFT)&0x8000 != 0)
	alt := currentIsAlt || (GetAsyncKeyState(VK_MENU)&0x8000 != 0)
	win := currentIsWin || (GetAsyncKeyState(VK_LWIN)&0x8000 != 0 || GetAsyncKeyState(VK_RWIN)&0x8000 != 0)

	// 命中热键时执行（修饰键与配置完全匹配，避免 ctrl+space+alt 误触发 ctrl+space）
	// 注意：动作全部异步执行。低层钩子回调必须快速返回（<300ms），
	// 否则 Windows 会静默卸载钩子，导致后续所有热键失效。
	matched := false
	if cfg.ToggleEnabled && matchesHotkey(cfg.ToggleHK, vkCode, ctrl, shift, alt, win) {
		matched = true
		go func() {
			toggleDesktopIcons()
			resetIdleTimer()
		}()
	}
	if cfg.ExitEnabled && matchesHotkey(cfg.ExitHK, vkCode, ctrl, shift, alt, win) {
		matched = true
		go postQuit()
	}
	if cfg.MonitorEnabled && matchesHotkey(cfg.MonitorHK, vkCode, ctrl, shift, alt, win) {
		matched = true
		turnOffMonitor()
	}
	return matched
}

// matchesHotkey 判断当前按键是否命中热键规格：
// 主键必须等于 spec.Key，且四个修饰键状态与 spec 完全一致（不多不少）
func matchesHotkey(spec HotkeySpec, vkCode uint32, ctrl, shift, alt, win bool) bool {
	if !spec.Valid || vkCode != spec.Key {
		return false
	}
	return ctrl == spec.Ctrl && shift == spec.Shift && alt == spec.Alt && win == spec.Win
}

// ============================================================
// 空闲定时器
// ============================================================

func resetIdleTimer() {
	if cfg.IdleHideTimeout <= 0 {
		return
	}
	if idleTimer != nil {
		idleTimer.Stop()
	}
	idleTimer = time.AfterFunc(time.Duration(cfg.IdleHideTimeout*float64(time.Second)), func() {
		hideDesktopIcons()
	})
}

// ============================================================
// 钩子线程
// ============================================================

func startHookThread() {
	exitCh = make(chan struct{})
	go hookThreadMain()
}

func hookThreadMain() {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// 低层钩子 (WH_MOUSE_LL/WH_KEYBOARD_LL) 回调在当前进程上下文中调用，
	// hMod 传 0 表示使用当前可执行文件句柄
	hModHandle := HINSTANCE(0)

	// 鼠标钩子
	mouseHook = SetWindowsHookExW(WH_MOUSE_LL, windows.NewCallback(mouseHookProc), hModHandle, 0)

	// 键盘钩子
	keyboardHook = SetWindowsHookExW(WH_KEYBOARD_LL, windows.NewCallback(keyboardHookProc), hModHandle, 0)

	hookThreadID = windows.GetCurrentThreadId()

	// 消息循环
	var msg MSG
	for {
		result := GetMessageW(&msg, 0, 0, 0)
		if !result || msg.Message == WM_QUIT {
			break
		}
		select {
		case <-exitCh:
			PostThreadMessageW(hookThreadID, WM_QUIT, 0, 0)
		default:
		}
	}

	// 清理钩子
	if mouseHook != 0 {
		UnhookWindowsHookEx(mouseHook)
	}
	if keyboardHook != 0 {
		UnhookWindowsHookEx(keyboardHook)
	}
}

func postQuit() {
	showDesktopIconsOnly()
	systrayQuit()
}
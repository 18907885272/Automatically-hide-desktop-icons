package main

import (
	"log"
	"runtime"
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
)

// ============================================================
// 鼠标钩子
// ============================================================

func mouseHookProc(nCode, wParam, lParam uintptr) uintptr {
	if int(nCode) >= 0 {
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

	// 双击检测
	dt := GetDoubleClickTime()
	isDoubleClick := (now-lastClickTime < dt &&
		abs(int(pt.X-lastClickPt.X)) < 4 &&
		abs(int(pt.Y-lastClickPt.Y)) < 4)

	lastClickTime = now
	lastClickPt = pt

	// 获取点击位置下的窗口
	hwnd := WindowFromPoint(pt)

	// 方案：使用 GetAncestor(GA_ROOT) 获取顶层父窗口
	// 对于顶层窗口（应用窗口、桌面、任务栏）→ 返回窗口自身
	// 对于子窗口（桌面图标列表、任务栏子控件）→ 返回顶层父窗口
	root := GetAncestor(hwnd, GA_ROOT)
	className := GetClassNameW(root)

	isDesktop := (className == "Progman" || className == "WorkerW")
	isTaskbar := (className == "Shell_TrayWnd" || className == "Shell_SecondaryTrayWnd")

	// 调试日志
	log.Printf("[DEBUG] 双击检测: hwnd=0x%X, root=0x%X, className=%s, isDesktop=%v, isTaskbar=%v, isDoubleClick=%v",
		hwnd, root, className, isDesktop, isTaskbar, isDoubleClick)

	// 只有双击桌面空白区域或任务栏空白区域才触发切换
	// 其他任何区域（图标、应用窗口等）都不触发
	if isDesktop && cfg.DblclickToggleEnabled && cfg.DblclickDesktopEnabled && isDoubleClick {
		log.Printf("[DEBUG] 双击桌面空白处，触发切换")
		toggleDesktopIcons()
	} else if isTaskbar && cfg.DblclickToggleEnabled && cfg.DblclickTaskbarEnabled && isDoubleClick {
		log.Printf("[DEBUG] 双击任务栏空白处，触发切换")
		toggleDesktopIcons()
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
	if int(nCode) >= 0 && (wParam == WM_KEYDOWN || wParam == WM_SYSKEYDOWN) {
		kbd := (*KBDLLHOOKSTRUCT)(unsafe.Pointer(lParam))
		handleHotkey(uint32(kbd.VkCode))
	}
	return CallNextHookEx(keyboardHook, int(nCode), wParam, lParam)
}

func handleHotkey(vkCode uint32) {
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

	log.Printf("[DEBUG] 热键检测: vkCode=0x%X, ctrl=%v, shift=%v, alt=%v, win=%v",
		vkCode, ctrl, shift, alt, win)

	if cfg.ToggleEnabled && ctrl && !shift && !alt && !win && (vkCode == VK_SPACE) {
		log.Printf("[DEBUG] 触发切换热键")
		toggleDesktopIcons()
		resetIdleTimer()
	}
	if cfg.ExitEnabled && ctrl && shift && !alt && !win && (vkCode == VK_Q) {
		log.Printf("[DEBUG] 触发退出热键")
		postQuit()
	}
	if cfg.MonitorEnabled && ctrl && !shift && alt && win && currentIsAlt {
		log.Printf("[DEBUG] 触发关屏热键")
		turnOffMonitor()
	}
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
	if mouseHook == 0 {
		log.Printf("SetWindowsHookEx(WH_MOUSE_LL) failed")
	}

	// 键盘钩子
	keyboardHook = SetWindowsHookExW(WH_KEYBOARD_LL, windows.NewCallback(keyboardHookProc), hModHandle, 0)
	if keyboardHook == 0 {
		log.Printf("SetWindowsHookEx(WH_KEYBOARD_LL) failed")
	}

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
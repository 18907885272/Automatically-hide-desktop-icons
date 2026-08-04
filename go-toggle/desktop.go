package main

import (
	"log"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// ============================================================
// Windows API 类型和常量
// ============================================================

type (
	HWND     uintptr
	HHOOK    uintptr
	HINSTANCE uintptr
	HANDLE   uintptr
	HICON    uintptr
	HMODULE  uintptr
	WPARAM   uintptr
	LPARAM   uintptr
	LRESULT  uintptr
	LPVOID   uintptr
	BOOL     int32
	LONG     int32
	DWORD    uint32
	WORD     uint16
	BYTE     byte
	ULONG    uint32
	HRESULT  int32
)

const (
	SW_HIDE = 0
	SW_SHOW = 5
	SW_MINIMIZE = 6
	SW_RESTORE = 9

	WM_LBUTTONUP     = 0x0202
	WM_LBUTTONDBLCLK = 0x0203
	WM_RBUTTONUP     = 0x0205
	WM_KEYDOWN       = 0x0100
	WM_SYSKEYDOWN    = 0x0104

	GA_ROOT = 2

	WH_MOUSE_LL    = 14
	WH_KEYBOARD_LL = 13

	EVENT_SYSTEM_FOREGROUND = 0x0003
	WINEVENT_OUTOFCONTEXT   = 0x0000

	VK_CONTROL = 0x11
	VK_LCONTROL = 0xA2
	VK_RCONTROL = 0xA3
	VK_SPACE   = 0x20
	VK_SHIFT   = 0x10
	VK_LSHIFT  = 0xA0
	VK_RSHIFT  = 0xA1
	VK_MENU    = 0x12
	VK_LMENU   = 0xA4
	VK_RMENU   = 0xA5
	VK_LWIN          = 0x5B
	VK_RWIN          = 0x5C
	KEYEVENTF_KEYUP  = 0x0002
	VK_Q       = 0x51
)

// RECT 结构
type RECT struct {
	Left, Top, Right, Bottom LONG
}

// POINT 结构
type POINT struct {
	X, Y LONG
}

// MSLLHOOKSTRUCT 低层鼠标钩子数据结构
type MSLLHOOKSTRUCT struct {
	Pt          POINT
	MouseData   DWORD
	Flags       DWORD
	Time        DWORD
	DwExtraInfo uintptr
}

// KBDLLHOOKSTRUCT 低层键盘钩子数据结构
type KBDLLHOOKSTRUCT struct {
	VkCode      DWORD
	ScanCode    DWORD
	Flags       DWORD
	Time        DWORD
	DwExtraInfo uintptr
}

// ============================================================
// Windows API 函数
// ============================================================

var (
	user32 = syscall.NewLazyDLL("user32.dll")
	shell32 = syscall.NewLazyDLL("shell32.dll")

	procFindWindowW              = user32.NewProc("FindWindowW")
	procFindWindowExW            = user32.NewProc("FindWindowExW")
	procShowWindow               = user32.NewProc("ShowWindow")
	procIsWindowVisible          = user32.NewProc("IsWindowVisible")
	procWindowFromPoint          = user32.NewProc("WindowFromPoint")
	procGetAncestor              = user32.NewProc("GetAncestor")
	procGetClassNameW            = user32.NewProc("GetClassNameW")
	procSetWindowsHookExW        = user32.NewProc("SetWindowsHookExW")
	procCallNextHookEx           = user32.NewProc("CallNextHookEx")
	procUnhookWindowsHookEx      = user32.NewProc("UnhookWindowsHookEx")
	procGetDoubleClickTime       = user32.NewProc("GetDoubleClickTime")
	procGetAsyncKeyState         = user32.NewProc("GetAsyncKeyState")
	procSendMessageW             = user32.NewProc("SendMessageW")
	procSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	procGetWindowRect            = user32.NewProc("GetWindowRect")
	procPostMessageW             = user32.NewProc("PostMessageW")
	procPostThreadMessageW       = user32.NewProc("PostThreadMessageW")
	procGetMessageW              = user32.NewProc("GetMessageW")
	procTranslateMessage         = user32.NewProc("TranslateMessage")
	procDispatchMessageW         = user32.NewProc("DispatchMessageW")
	procPostQuitMessage          = user32.NewProc("PostQuitMessage")
	procGetWindow                = user32.NewProc("GetWindow")
	procIsChild                  = user32.NewProc("IsChild")
	procGetParent                = user32.NewProc("GetParent")
	procKeybdEvent               = user32.NewProc("keybd_event")
	procEnumWindows              = user32.NewProc("EnumWindows")
	procIsWindow                 = user32.NewProc("IsWindow")
	procIsIconic                 = user32.NewProc("IsIconic")

	procShellExecuteW            = shell32.NewProc("ShellExecuteW")
)

// ============================================================
// 辅助函数
// ============================================================

func FindWindowW(className, windowName *uint16) HWND {
	ret, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(windowName)))
	return HWND(ret)
}

func FindWindowExW(parent, child HWND, className, windowName *uint16) HWND {
	ret, _, _ := procFindWindowExW.Call(uintptr(parent), uintptr(child), uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(windowName)))
	return HWND(ret)
}

func ShowWindow(hwnd HWND, cmdShow int) bool {
	ret, _, _ := procShowWindow.Call(uintptr(hwnd), uintptr(cmdShow))
	return ret != 0
}

func IsWindowVisible(hwnd HWND) bool {
	ret, _, _ := procIsWindowVisible.Call(uintptr(hwnd))
	return ret != 0
}

func WindowFromPoint(pt POINT) HWND {
	// POINT 结构体在 x64 上按值传递时是 8 字节：
	// 低 32 位 = X, 高 32 位 = Y
	// 必须打包成一个 uintptr 传递，不能分两个参数
	packed := uint64(uint32(pt.X)) | (uint64(uint32(pt.Y)) << 32)
	ret, _, _ := procWindowFromPoint.Call(uintptr(packed))
	return HWND(ret)
}

func GetClassNameW(hwnd HWND) string {
	var buf [256]uint16
	procGetClassNameW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), 256)
	return syscall.UTF16ToString(buf[:])
}

func GetAncestor(hwnd HWND, gaFlags uint) HWND {
	ret, _, _ := procGetAncestor.Call(uintptr(hwnd), uintptr(gaFlags))
	return HWND(ret)
}

func SetWindowsHookExW(hookID int, hookProc uintptr, hMod HINSTANCE, dwThreadID uint32) HHOOK {
	ret, _, _ := procSetWindowsHookExW.Call(uintptr(hookID), hookProc, uintptr(hMod), uintptr(dwThreadID))
	return HHOOK(ret)
}

func CallNextHookEx(hhk HHOOK, nCode int, wParam, lParam uintptr) uintptr {
	ret, _, _ := procCallNextHookEx.Call(uintptr(hhk), uintptr(nCode), wParam, lParam)
	return ret
}

func UnhookWindowsHookEx(hhk HHOOK) bool {
	ret, _, _ := procUnhookWindowsHookEx.Call(uintptr(hhk))
	return ret != 0
}

func GetDoubleClickTime() uint32 {
	ret, _, _ := procGetDoubleClickTime.Call()
	return uint32(ret)
}

func GetAsyncKeyState(vKey int) uint16 {
	ret, _, _ := procGetAsyncKeyState.Call(uintptr(vKey))
	return uint16(ret)
}

func SendMessageW(hwnd HWND, msg uint32, wParam, lParam uintptr) uintptr {
	ret, _, _ := procSendMessageW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func GetWindowRect(hwnd HWND, rect *RECT) bool {
	ret, _, _ := procGetWindowRect.Call(uintptr(hwnd), uintptr(unsafe.Pointer(rect)))
	return ret != 0
}

func PostMessageW(hwnd HWND, msg uint32, wParam, lParam uintptr) bool {
	ret, _, _ := procPostMessageW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret != 0
}

func PostThreadMessageW(threadID uint32, msg uint32, wParam, lParam uintptr) bool {
	ret, _, _ := procPostThreadMessageW.Call(uintptr(threadID), uintptr(msg), wParam, lParam)
	return ret != 0
}

func GetMessageW(msg *MSG, hwnd HWND, msgFilterMin, msgFilterMax uint32) bool {
	ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(msg)), uintptr(hwnd), uintptr(msgFilterMin), uintptr(msgFilterMax))
	return ret != 0
}

func PostQuitMessage(exitCode int32) {
	procPostQuitMessage.Call(uintptr(exitCode))
}

func GetWindow(hwnd HWND, flag uint32) HWND {
	ret, _, _ := procGetWindow.Call(uintptr(hwnd), uintptr(flag))
	return HWND(ret)
}

func IsChild(parent, child HWND) bool {
	ret, _, _ := procIsChild.Call(uintptr(parent), uintptr(child))
	return ret != 0
}

func GetParent(hwnd HWND) HWND {
	ret, _, _ := procGetParent.Call(uintptr(hwnd))
	return HWND(ret)
}

func ShellExecuteW(hwnd HWND, operation, file, parameters, directory *uint16, nCmdShow int) {
	procShellExecuteW.Call(
		uintptr(hwnd),
		uintptr(unsafe.Pointer(operation)),
		uintptr(unsafe.Pointer(file)),
		uintptr(unsafe.Pointer(parameters)),
		uintptr(unsafe.Pointer(directory)),
		uintptr(nCmdShow),
	)
}

// 被最小化的应用窗口列表（用于恢复）
var (
	minimizedWindows   []HWND
	minimizedWindowsMu sync.Mutex
)

// EnumWindows 回调（最小化所有可见顶层窗口，排除桌面和任务栏）
var enumWindowsCB uintptr
var enumWindowsCBOnce sync.Once

func getEnumWindowsCB() uintptr {
	enumWindowsCBOnce.Do(func() {
		enumWindowsCB = syscall.NewCallback(minimizeEnumProc)
	})
	return enumWindowsCB
}

func minimizeEnumProc(hwnd HWND, lParam uintptr) uintptr {
	// 只处理可见且未最小化的窗口
	if !IsWindowVisible(hwnd) || IsIconic(hwnd) {
		return 1 // 继续枚举
	}

	// 获取窗口类名
	var buf [256]uint16
	procGetClassNameW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), 256)
	cls := syscall.UTF16ToString(buf[:])

	// 跳过桌面、任务栏、程序自带设置窗口
	if cls == "Progman" || cls == "WorkerW" || cls == "Shell_TrayWnd" || cls == "Shell_SecondaryTrayWnd" || cls == "隐藏桌面图标设置" {
		return 1
	}

	// 跳过无标题的窗口（通常是系统内部窗口）
	// 检查窗口是否拥有 WS_CAPTION 样式 - 简化处理：直接保存并最小化
	minimizedWindowsMu.Lock()
	minimizedWindows = append(minimizedWindows, hwnd)
	minimizedWindowsMu.Unlock()

	ShowWindow(hwnd, SW_MINIMIZE)
	return 1
}

func minimizeAllWindows() {
	minimizedWindowsMu.Lock()
	minimizedWindows = nil
	minimizedWindowsMu.Unlock()

	procEnumWindows.Call(getEnumWindowsCB(), 0)
}

func restoreAllWindows() {
	minimizedWindowsMu.Lock()
	defer minimizedWindowsMu.Unlock()

	for _, hwnd := range minimizedWindows {
		// 检查窗口是否仍有效
		ret, _, _ := procIsWindow.Call(uintptr(hwnd))
		if ret == 0 {
			continue
		}
		ShowWindow(hwnd, SW_RESTORE)
	}
	minimizedWindows = nil
}

func IsWindow(hwnd HWND) bool {
	ret, _, _ := procIsWindow.Call(uintptr(hwnd))
	return ret != 0
}

func IsIconic(hwnd HWND) bool {
	ret, _, _ := procIsIconic.Call(uintptr(hwnd))
	return ret != 0
}

// ============================================================
// 桌面图标控制
// ============================================================

var (
	desktopIconsVisible = true
	desktopIconsMu      sync.Mutex
	lastToggleTime      time.Time
)

func findDesktopIconView() HWND {
	progman := FindWindowW(syscall.StringToUTF16Ptr("Progman"), nil)
	if progman == 0 {
		return 0
	}
	defView := FindWindowExW(progman, 0, syscall.StringToUTF16Ptr("SHELLDLL_DefView"), nil)
	if defView == 0 {
		// 某些系统 WorkerW 包含桌面图标
		workerW := FindWindowW(syscall.StringToUTF16Ptr("WorkerW"), nil)
		for workerW != 0 {
			defView = FindWindowExW(workerW, 0, syscall.StringToUTF16Ptr("SHELLDLL_DefView"), nil)
			if defView != 0 {
				break
			}
			workerW = FindWindowExW(0, workerW, syscall.StringToUTF16Ptr("WorkerW"), nil)
		}
	}
	if defView == 0 {
		return 0
	}
	listView := FindWindowExW(defView, 0, syscall.StringToUTF16Ptr("SysListView32"), nil)
	return listView
}

func toggleDesktopIcons() {
	// 防抖：500ms 内禁止重复切换，防止闪动
	if time.Since(lastToggleTime) < 500*time.Millisecond {
		return
	}
	lastToggleTime = time.Now()

	log.Printf("[DEBUG] toggleDesktopIcons 被调用")

	desktopIconsMu.Lock()
	defer desktopIconsMu.Unlock()

	listView := findDesktopIconView()
	if listView == 0 {
		return
	}

	if desktopIconsVisible {
		// 隐藏图标 + 恢复应用窗口
		ShowWindow(listView, SW_HIDE)
		desktopIconsVisible = false
		setTrayIconHidden()
		restoreAllWindows()
	} else {
		// 显示图标 + 最小化应用窗口
		minimizeAllWindows()
		ShowWindow(listView, SW_SHOW)
		desktopIconsVisible = true
		setTrayIconShown()
	}
}

func showDesktopIcons() {
	log.Printf("[DEBUG] showDesktopIcons 被调用")
	desktopIconsMu.Lock()
	defer desktopIconsMu.Unlock()

	listView := findDesktopIconView()
	if listView == 0 {
		return
	}
	if !desktopIconsVisible {
		// 显示桌面图标 + 最小化应用窗口
		minimizeAllWindows()
		ShowWindow(listView, SW_SHOW)
		desktopIconsVisible = true
		setTrayIconShown()
	}
}

func hideDesktopIcons() {
	log.Printf("[DEBUG] hideDesktopIcons 被调用")
	desktopIconsMu.Lock()
	defer desktopIconsMu.Unlock()

	listView := findDesktopIconView()
	if listView == 0 {
		return
	}
	if desktopIconsVisible {
		// 隐藏图标 + 恢复应用窗口
		ShowWindow(listView, SW_HIDE)
		desktopIconsVisible = false
		setTrayIconHidden()
		restoreAllWindows()
	}
}

func isDesktopIconsVisible() bool {
	desktopIconsMu.Lock()
	defer desktopIconsMu.Unlock()
	return desktopIconsVisible
}

// ============================================================
// 关闭显示器
// ============================================================

func turnOffMonitor() {
	log.Printf("[DEBUG] turnOffMonitor 被调用")
	hwnd := FindWindowW(syscall.StringToUTF16Ptr("Progman"), nil)
	if hwnd != 0 {
		// 先尝试发送到 Progman 窗口
		SendMessageW(hwnd, 0x0112, 0xF170, 2) // WM_SYSCOMMAND, SC_MONITORPOWER, 2=off
	} else {
		// 如果找不到 Progman，尝试广播到所有窗口
		SendMessageW(HWND(0xFFFF), 0x0112, 0xF170, 2)
	}
}

// ============================================================
// 窗口类名常量
// ============================================================

const (
	desktopClassProgman    = "Progman"
	desktopClassWorkerW    = "WorkerW"
	desktopClassDefView    = "SHELLDLL_DefView"
	desktopClassListView   = "SysListView32"
	taskbarClass           = "Shell_TrayWnd"
	taskbarSecondaryClass  = "Shell_SecondaryTrayWnd"
	taskbarButtonStrip     = "MSTaskListWClass"
	taskbarTrayNotify      = "TrayNotifyWnd"
)

// GetWindow 指令常量
const (
	GW_OWNER = 4 // 获取窗口的所有者
)

const WM_QUIT = 0x0012

// ============================================================
// ListView 消息常量
// ============================================================

const (
	LVM_FIRST     = 0x1000
	LVM_HITTEST   = LVM_FIRST + 18
	LVHT_NOWHERE  = 0x0001
	LVHT_ONITEM   = 0x0002 | 0x0004 | 0x0008 // icon | label | state icon
)

// LVHITTESTINFO 是 ListView 命中测试结构
type LVHITTESTINFO struct {
	Pt       POINT
	Flags    uint32
	IItem    int32
	ISubItem int32
	IGroup   int32
}
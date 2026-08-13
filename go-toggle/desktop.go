package main

import (
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
	WM_SYSCOMMAND    = 0x0112

	HWND_BROADCAST = 0xFFFF

	SC_MONITORPOWER = 0xF170

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
	LLKHF_INJECTED   = 0x0010
	VK_Q       = 0x51
	VK_DOWN    = 0x28
	VK_UP      = 0x26
	VK_LEFT    = 0x25
	VK_RIGHT   = 0x27
	VK_ESCAPE  = 0x1B
	VK_RETURN  = 0x0D
	VK_TAB     = 0x09
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
	procScreenToClient           = user32.NewProc("ScreenToClient")
	procClientToScreen           = user32.NewProc("ClientToScreen")
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

func ScreenToClient(hwnd HWND, pt *POINT) bool {
	ret, _, _ := procScreenToClient.Call(uintptr(hwnd), uintptr(unsafe.Pointer(pt)))
	return ret != 0
}

func ClientToScreen(hwnd HWND, pt *POINT) bool {
	ret, _, _ := procClientToScreen.Call(uintptr(hwnd), uintptr(unsafe.Pointer(pt)))
	return ret != 0
}

// ============================================================
// ============================================================
// 桌面图标命中检测（图标网格推断方案）
// ============================================================

// 桌面 ListView 不支持 LVM_HITTEST / LVM_GETITEMPOSITION（Windows 特殊控件），
// 但支持 LVM_GETITEMCOUNT（图标数量）和 LVM_GETITEMSPACING（网格间距）。
// 方案：用图标数量 + 网格间距推断每个图标的位置，双击时纯内存判断。

var (
	iconRects     []RECT
	iconRectsTime time.Time
	iconRectsMu   sync.Mutex
)

// getIconRects 推断桌面所有图标的屏幕坐标矩形，带 10 秒缓存
// 基于 LVM_GETITEMSPACING 返回的网格间距（如 76x98）计算
func getIconRects() []RECT {
	iconRectsMu.Lock()
	defer iconRectsMu.Unlock()

	if len(iconRects) > 0 && time.Since(iconRectsTime) < 10*time.Second {
		return iconRects
	}

	listView := findDesktopIconView()
	if listView == 0 {
		return nil
	}

	// 获取图标数量（该消息可靠且快速）
	count, _, _ := procSendMessageW.Call(uintptr(listView), LVM_GETITEMCOUNT, 0, 0)
	n := int(count)
	if n <= 0 || n > 2000 {
		iconRects = nil
		iconRectsTime = time.Now()
		return nil
	}

	// 获取网格间距：LOWORD = 水平间距，HIWORD = 垂直间距
	spacing, _, _ := procSendMessageW.Call(uintptr(listView), LVM_GETITEMSPACING, 0, 0)
	spX := int(spacing & 0xFFFF)
	spY := int(spacing >> 16)
	if spX < 40 || spY < 40 {
		spX, spY = 76, 98 // 默认值
	}

	// 获取 ListView 客户区宽度 → 计算每行图标数
	var winRect RECT
	procGetWindowRect.Call(uintptr(listView), uintptr(unsafe.Pointer(&winRect)))
	clientW := int(winRect.Right - winRect.Left)
	cols := clientW / spX
	if cols < 1 {
		cols = 1
	}

	// 生成图标矩形（网格中心 ± 20px 命中区域）
	rects := make([]RECT, 0, n)
	for i := 0; i < n; i++ {
		cx := (i%cols)*spX + spX/2
		cy := (i/cols)*spY + spY/2
		rects = append(rects, RECT{
			Left:   LONG(cx - 22),
			Top:    LONG(cy - 22),
			Right:  LONG(cx + 22),
			Bottom: LONG(cy + 22),
		})
	}

	iconRects = rects
	iconRectsTime = time.Now()
	return rects
}

// isClickOnDesktopIcon 判断屏幕坐标 pt 是否命中桌面图标
// 基于推断的图标网格做纯内存判断，速度快且不依赖 Explorer 响应
func isClickOnDesktopIcon(pt POINT) bool {
	rects := getIconRects()
	for _, r := range rects {
		if pt.X >= r.Left && pt.X <= r.Right && pt.Y >= r.Top && pt.Y <= r.Bottom {
			return true
		}
	}
	return false
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

	ShowWindow(hwnd, SW_MINIMIZE)
	return 1
}

func minimizeAllWindows() {
	procEnumWindows.Call(getEnumWindowsCB(), 0)
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

	// 缓存桌面图标 ListView 句柄，避免每次调用重复遍历窗口树
	cachedListView   HWND
	cacheValidTime   time.Time
	listViewMu       sync.Mutex
)

// findDesktopIconView 查找桌面图标 ListView 窗口句柄
// 结果会被缓存（缓存 10 秒），避免每次调用都遍历窗口树
func findDesktopIconView() HWND {
	listViewMu.Lock()
	defer listViewMu.Unlock()

	// 缓存有效期内直接返回缓存值
	if cachedListView != 0 && time.Since(cacheValidTime) < 10*time.Second {
		return cachedListView
	}

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
	cachedListView = listView
	cacheValidTime = time.Now()
	return listView
}

func toggleDesktopIcons() {
	// 防抖：500ms 内禁止重复切换，防止闪动
	if time.Since(lastToggleTime) < 500*time.Millisecond {
		return
	}
	lastToggleTime = time.Now()

	desktopIconsMu.Lock()
	defer desktopIconsMu.Unlock()

	listView := findDesktopIconView()
	if listView == 0 {
		return
	}

	if desktopIconsVisible {
		// 隐藏图标（不恢复应用窗口，已最小化的保持最小化）
		ShowWindow(listView, SW_HIDE)
		desktopIconsVisible = false
		setTrayIconHidden()
	} else {
		// 显示图标 + 最小化应用窗口
		minimizeAllWindows()
		ShowWindow(listView, SW_SHOW)
		desktopIconsVisible = true
		setTrayIconShown()
	}
}

func showDesktopIcons() {
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

// showDesktopIconsOnly 只显示桌面图标，不操作应用窗口（用于退出时）
func showDesktopIconsOnly() {
	desktopIconsMu.Lock()
	defer desktopIconsMu.Unlock()

	listView := findDesktopIconView()
	if listView == 0 {
		return
	}
	if !desktopIconsVisible {
		ShowWindow(listView, SW_SHOW)
		desktopIconsVisible = true
		setTrayIconShown()
	}
}

func hideDesktopIcons() {
	desktopIconsMu.Lock()
	defer desktopIconsMu.Unlock()

	listView := findDesktopIconView()
	if listView == 0 {
		return
	}
	if desktopIconsVisible {
		// 隐藏图标（不恢复应用窗口，已最小化的保持最小化）
		ShowWindow(listView, SW_HIDE)
		desktopIconsVisible = false
		setTrayIconHidden()
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

const (
	// monitorOffProtectDuration 关屏后防误唤醒保护窗口时长：
	// Windows 下松开热键、鼠标动作都会立即点亮已关闭的显示器，
	// 在保护窗口内吞掉所有输入并周期重发关屏命令，覆盖这些"立即唤醒"。
	monitorOffProtectDuration = 3 * time.Second
	// monitorOffProtectInterval 保护窗口内重发关屏命令的间隔
	monitorOffProtectInterval = 250 * time.Millisecond
)

// keybdEvent 调用 Win32 keybd_event 模拟键盘事件
func keybdEvent(vk, scan, flags, extra uint32) {
	procKeybdEvent.Call(uintptr(vk), uintptr(scan), uintptr(flags), uintptr(extra))
}

// releaseModifiers 补发修饰键 keyup，修复系统键状态。
// 关屏热键（如 Ctrl+Alt+Down）的修饰键 keydown 已透传给系统，
// 但松开时的 keyup 会被关屏保护窗口吞掉，导致系统认为 Ctrl/Alt 一直按住、
// 后续所有快捷键错乱。补发的 keyup 带 LLKHF_INJECTED 标志，会被钩子放行透传。
func releaseModifiers() {
	keybdEvent(VK_LCONTROL, 0, KEYEVENTF_KEYUP, 0)
	keybdEvent(VK_RCONTROL, 0, KEYEVENTF_KEYUP, 0)
	keybdEvent(VK_LMENU, 0, KEYEVENTF_KEYUP, 0)
	keybdEvent(VK_RMENU, 0, KEYEVENTF_KEYUP, 0)
	keybdEvent(VK_LSHIFT, 0, KEYEVENTF_KEYUP, 0)
	keybdEvent(VK_RSHIFT, 0, KEYEVENTF_KEYUP, 0)
	keybdEvent(VK_LWIN, 0, KEYEVENTF_KEYUP, 0)
	keybdEvent(VK_RWIN, 0, KEYEVENTF_KEYUP, 0)
}

func turnOffMonitor() {
	// 先补发修饰键 keyup，修复系统键状态（避免吞 keyup 导致 Ctrl/Alt 卡键）
	releaseModifiers()

	// 输入屏蔽必须同步设置（在钩子回调返回前生效），防止松开热键的 keyup 唤醒显示器
	blockInputFor(monitorOffProtectDuration)

	// 广播关屏必须异步执行：
	// SendMessageW(HWND_BROADCAST) 是同步广播，会阻塞等待所有顶层窗口处理，
	// 若在低层钩子回调中同步调用，回调超时会导致钩子被系统卸载（后续热键全部失效）。
	go func() {
		SendMessageW(HWND_BROADCAST, WM_SYSCOMMAND, SC_MONITORPOWER, 2)
	}()

	// 防误唤醒保护：周期重发关屏命令兜底
	go func() {
		deadline := time.Now().Add(monitorOffProtectDuration)
		for time.Now().Before(deadline) {
			time.Sleep(monitorOffProtectInterval)
			SendMessageW(HWND_BROADCAST, WM_SYSCOMMAND, SC_MONITORPOWER, 2)
		}
	}()
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
	LVM_FIRST        = 0x1000
	LVM_GETITEMCOUNT = LVM_FIRST + 4
	LVM_GETITEMSPACING = LVM_FIRST + 51
	LVM_GETITEMPOSITION = LVM_FIRST + 16
	LVM_HITTEST      = LVM_FIRST + 18
	LVHT_NOWHERE     = 0x0001
	LVHT_ONITEM      = 0x0002 | 0x0004 | 0x0008 // icon | label | state icon
)

// LVHITTESTINFO 是 ListView 命中测试结构
type LVHITTESTINFO struct {
	Pt       POINT
	Flags    uint32
	IItem    int32
	ISubItem int32
	IGroup   int32
}
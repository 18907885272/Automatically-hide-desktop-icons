package main

import (
	"fmt"
	"runtime"
	"strconv"
	"syscall"
	"unsafe"
)

// ============================================================
// 设置对话框 - 使用 Win32 原生窗口
// ============================================================

var (
	procCreateWindowExW = user32.NewProc("CreateWindowExW")
	procDefWindowProcW  = user32.NewProc("DefWindowProcW")
	procDestroyWindow   = user32.NewProc("DestroyWindow")
	procSetWindowTextW  = user32.NewProc("SetWindowTextW")
	procGetWindowTextW  = user32.NewProc("GetWindowTextW")
	procGetDlgItem      = user32.NewProc("GetDlgItem")
	procEnableWindow    = user32.NewProc("EnableWindow")
	procCheckDlgButton  = user32.NewProc("CheckDlgButton")
	procIsDlgButtonChecked = user32.NewProc("IsDlgButtonChecked")
	procSetDlgItemTextW = user32.NewProc("SetDlgItemTextW")
	procGetDlgItemTextW = user32.NewProc("GetDlgItemTextW")
	procRegisterClassW  = user32.NewProc("RegisterClassW")
)

const (
	WS_OVERLAPPEDWINDOW = 0xCF0000
	WS_CHILD            = 0x40000000
	WS_VISIBLE          = 0x10000000
	WS_TABSTOP          = 0x00010000
	WS_BORDER           = 0x00800000
	WS_CAPTION          = 0x00C00000
	WS_SYSMENU          = 0x00080000
	WS_MINIMIZEBOX      = 0x00020000
	WS_MAXIMIZEBOX      = 0x00010000
	WS_THICKFRAME       = 0x00040000
	WS_EX_DLGMODALFRAME = 0x00000001
	WS_EX_TOOLWINDOW    = 0x00000080

	BS_GROUPBOX      = 0x00000007
	BS_AUTOCHECKBOX  = 0x00000003
	BS_PUSHBUTTON    = 0x00000000
	BS_DEFPUSHBUTTON = 0x00000001

	ES_LEFT         = 0x00000000
	ES_AUTOHSCROLL  = 0x00000080
	ES_NUMBER       = 0x00002000

	SS_LEFT = 0x00000000

	WM_COMMAND    = 0x0111
	WM_CLOSE      = 0x0010
	WM_DESTROY    = 0x0002
	WM_CREATE     = 0x0001

	CS_DBLCLKS = 0x0008
	COLOR_BTNFACE = 15
)

// 控件 ID
const (
	IDC_CHK_TOGGLE     = 101
	IDC_EDIT_TOGGLE    = 102
	IDC_CHK_EXIT       = 104
	IDC_EDIT_EXIT      = 105
	IDC_CHK_MONITOR    = 107
	IDC_EDIT_MONITOR   = 108

	IDC_CHK_DBLCLICK   = 301
	IDC_CHK_DBLTASKBAR = 302
	IDC_CHK_DBLDESKTOP = 303

	IDC_EDIT_IDLE      = 401

	IDC_BTN_OK         = 500
	IDC_BTN_CANCEL     = 501
)

var settingsHwnd HWND

// WNDCLASSW 结构
type WNDCLASSW struct {
	Style        uint32
	LpfnWndProc  uintptr
	CbClsExtra   int32
	CbWndExtra   int32
	HInstance    HINSTANCE
	HIcon        uintptr
	HCursor      uintptr
	HbrBackground uintptr
	LpszMenuName *uint16
	LpszClassName *uint16
}

const settingsWindowClass = "ToggleSettingsWnd"

func RegisterClassW(wc *WNDCLASSW) uint16 {
	ret, _, _ := procRegisterClassW.Call(uintptr(unsafe.Pointer(wc)))
	return uint16(ret)
}

func init() {
	// 注册自定义窗口类
	wc := &WNDCLASSW{
		Style:        CS_DBLCLKS,
		LpfnWndProc:  syscall.NewCallback(settingsDialogProc),
		CbClsExtra:   0,
		CbWndExtra:   0,
		HInstance:    0,
		HIcon:        0,
		HCursor:      0,
		HbrBackground: uintptr(COLOR_BTNFACE + 1),
		LpszMenuName: nil,
		LpszClassName: syscall.StringToUTF16Ptr(settingsWindowClass),
	}
	RegisterClassW(wc)
}

// ============================================================
// Windows API 封装
// ============================================================

func CreateWindowExW(exStyle uint32, className, windowName *uint16, style uint32, x, y, width, height int, parent HWND, menu uintptr, instance HINSTANCE, param uintptr) HWND {
	ret, _, _ := procCreateWindowExW.Call(uintptr(exStyle), uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(windowName)), uintptr(style), uintptr(x), uintptr(y), uintptr(width), uintptr(height), uintptr(parent), menu, uintptr(instance), param)
	return HWND(ret)
}

func DefWindowProcW(hwnd HWND, msg uint32, wParam, lParam uintptr) uintptr {
	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(msg), wParam, lParam)
	return ret
}

func DestroyWindow(hwnd HWND) bool {
	ret, _, _ := procDestroyWindow.Call(uintptr(hwnd))
	return ret != 0
}

func GetDlgItem(hDlg HWND, nIDDlgItem int) HWND {
	ret, _, _ := procGetDlgItem.Call(uintptr(hDlg), uintptr(nIDDlgItem))
	return HWND(ret)
}

func EnableWindow(hwnd HWND, enable bool) bool {
	v := uintptr(0)
	if enable {
		v = 1
	}
	ret, _, _ := procEnableWindow.Call(uintptr(hwnd), v)
	return ret != 0
}

func CheckDlgButton(hDlg HWND, nIDButton int, check bool) {
	v := uintptr(0)
	if check {
		v = 1
	}
	procCheckDlgButton.Call(uintptr(hDlg), uintptr(nIDButton), v)
}

func IsDlgButtonChecked(hDlg HWND, nIDButton int) bool {
	ret, _, _ := procIsDlgButtonChecked.Call(uintptr(hDlg), uintptr(nIDButton))
	return ret != 0
}

func SetDlgItemTextW(hDlg HWND, nIDDlgItem int, str *uint16) bool {
	ret, _, _ := procSetDlgItemTextW.Call(uintptr(hDlg), uintptr(nIDDlgItem), uintptr(unsafe.Pointer(str)))
	return ret != 0
}

func GetDlgItemTextW(hDlg HWND, nIDDlgItem int, buf *uint16, maxLen int) int {
	ret, _, _ := procGetDlgItemTextW.Call(uintptr(hDlg), uintptr(nIDDlgItem), uintptr(unsafe.Pointer(buf)), uintptr(maxLen))
	return int(ret)
}

// ============================================================
// 创建控件辅助函数
// ============================================================

func createStatic(parent HWND, id, x, y, w, h int, text string) HWND {
	return CreateWindowExW(0, syscall.StringToUTF16Ptr("STATIC"),
		syscall.StringToUTF16Ptr(text),
		WS_CHILD|WS_VISIBLE|SS_LEFT,
		x, y, w, h, parent, uintptr(id), 0, 0)
}

func createEdit(parent HWND, id, x, y, w, h int, text string, numberOnly bool) HWND {
	style := WS_CHILD | WS_VISIBLE | WS_BORDER | WS_TABSTOP | ES_LEFT | ES_AUTOHSCROLL
	if numberOnly {
		style |= ES_NUMBER
	}
	return CreateWindowExW(0, syscall.StringToUTF16Ptr("EDIT"),
		syscall.StringToUTF16Ptr(text),
		uint32(style),
		x, y, w, h, parent, uintptr(id), 0, 0)
}

func createCheckbox(parent HWND, id, x, y, w, h int, text string, checked bool) HWND {
	hwnd := CreateWindowExW(0, syscall.StringToUTF16Ptr("BUTTON"),
		syscall.StringToUTF16Ptr(text),
		WS_CHILD|WS_VISIBLE|WS_TABSTOP|BS_AUTOCHECKBOX,
		x, y, w, h, parent, uintptr(id), 0, 0)
	if checked {
		CheckDlgButton(parent, id, true)
	}
	return hwnd
}

func createGroupbox(parent HWND, id, x, y, w, h int, text string) HWND {
	return CreateWindowExW(0, syscall.StringToUTF16Ptr("BUTTON"),
		syscall.StringToUTF16Ptr(text),
		WS_CHILD|WS_VISIBLE|BS_GROUPBOX,
		x, y, w, h, parent, uintptr(id), 0, 0)
}

func createButton(parent HWND, id, x, y, w, h int, text string, isDefault bool) HWND {
	style := WS_CHILD | WS_VISIBLE | WS_TABSTOP | BS_PUSHBUTTON
	if isDefault {
		style |= BS_DEFPUSHBUTTON
	}
	return CreateWindowExW(0, syscall.StringToUTF16Ptr("BUTTON"),
		syscall.StringToUTF16Ptr(text),
		uint32(style),
		x, y, w, h, parent, uintptr(id), 0, 0)
}

// ============================================================
// 窗口过程
// ============================================================

func settingsDialogProc(hwnd HWND, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case WM_COMMAND:
		cmd := wParam & 0xFFFF
		notif := (wParam >> 16) & 0xFFFF
		_ = notif
		switch int(cmd) {
		case IDC_CHK_TOGGLE:
			enabled := IsDlgButtonChecked(hwnd, IDC_CHK_TOGGLE)
			EnableWindow(GetDlgItem(hwnd, IDC_EDIT_TOGGLE), enabled)
		case IDC_CHK_EXIT:
			enabled := IsDlgButtonChecked(hwnd, IDC_CHK_EXIT)
			EnableWindow(GetDlgItem(hwnd, IDC_EDIT_EXIT), enabled)
		case IDC_CHK_MONITOR:
			enabled := IsDlgButtonChecked(hwnd, IDC_CHK_MONITOR)
			EnableWindow(GetDlgItem(hwnd, IDC_EDIT_MONITOR), enabled)
		case IDC_BTN_OK:
			saveSettingsFromDialog(hwnd)
			DestroyWindow(hwnd)
		case IDC_BTN_CANCEL:
			DestroyWindow(hwnd)
		}
		return 0

	case WM_CLOSE:
		DestroyWindow(hwnd)
		return 0

	case WM_DESTROY:
		PostQuitMessage(0)
		return 0
	}
	return DefWindowProcW(hwnd, msg, wParam, lParam)
}

// ============================================================
// 初始化控件
// ============================================================

func initSettingsDialog(hwnd HWND) {
	// 快捷键分组
	createGroupbox(hwnd, 100, 10, 10, 365, 90, "快捷键设置")

	createStatic(hwnd, 0, 20, 30, 80, 20, "切换快捷键:")
	createCheckbox(hwnd, IDC_CHK_TOGGLE, 105, 30, 20, 20, "", cfg.ToggleEnabled)
	createEdit(hwnd, IDC_EDIT_TOGGLE, 130, 30, 225, 22, cfg.ToggleHotkey, false)
	EnableWindow(GetDlgItem(hwnd, IDC_EDIT_TOGGLE), cfg.ToggleEnabled)

	createStatic(hwnd, 0, 20, 55, 80, 20, "退出快捷键:")
	createCheckbox(hwnd, IDC_CHK_EXIT, 105, 55, 20, 20, "", cfg.ExitEnabled)
	createEdit(hwnd, IDC_EDIT_EXIT, 130, 55, 225, 22, cfg.ExitHotkey, false)
	EnableWindow(GetDlgItem(hwnd, IDC_EDIT_EXIT), cfg.ExitEnabled)

	createStatic(hwnd, 0, 20, 80, 80, 20, "关屏快捷键:")
	createCheckbox(hwnd, IDC_CHK_MONITOR, 105, 80, 20, 20, "", cfg.MonitorEnabled)
	createEdit(hwnd, IDC_EDIT_MONITOR, 130, 80, 225, 22, cfg.MonitorHotkey, false)
	EnableWindow(GetDlgItem(hwnd, IDC_EDIT_MONITOR), cfg.MonitorEnabled)

	// 双击设置分组
	createGroupbox(hwnd, 300, 10, 110, 365, 55, "双击设置")

	createCheckbox(hwnd, IDC_CHK_DBLCLICK, 20, 130, 150, 20, "桌面双击切换", cfg.DblclickToggleEnabled)
	createCheckbox(hwnd, IDC_CHK_DBLTASKBAR, 180, 130, 150, 20, "任务栏双击切换", cfg.DblclickTaskbarEnabled)
	createCheckbox(hwnd, IDC_CHK_DBLDESKTOP, 20, 155, 150, 20, "双击桌面空白处", cfg.DblclickDesktopEnabled)

	// 其他设置分组
	createGroupbox(hwnd, 400, 10, 175, 365, 50, "其他")

	createStatic(hwnd, 0, 20, 195, 160, 20, "空闲超时(秒,0=禁用):")
	createEdit(hwnd, IDC_EDIT_IDLE, 185, 195, 80, 22, fmt.Sprintf("%.0f", cfg.IdleHideTimeout), true)

	// 按钮
	createButton(hwnd, IDC_BTN_OK, 210, 240, 70, 26, "确定", true)
	createButton(hwnd, IDC_BTN_CANCEL, 290, 240, 70, 26, "取消", false)
}

// ============================================================
// 保存设置
// ============================================================

func saveSettingsFromDialog(hwnd HWND) {
	newCfg := *cfg

	newCfg.ToggleEnabled = IsDlgButtonChecked(hwnd, IDC_CHK_TOGGLE)
	newCfg.ExitEnabled = IsDlgButtonChecked(hwnd, IDC_CHK_EXIT)
	newCfg.MonitorEnabled = IsDlgButtonChecked(hwnd, IDC_CHK_MONITOR)
	newCfg.DblclickToggleEnabled = IsDlgButtonChecked(hwnd, IDC_CHK_DBLCLICK)
	newCfg.DblclickTaskbarEnabled = IsDlgButtonChecked(hwnd, IDC_CHK_DBLTASKBAR)
	newCfg.DblclickDesktopEnabled = IsDlgButtonChecked(hwnd, IDC_CHK_DBLDESKTOP)

	var buf [256]uint16

	GetDlgItemTextW(hwnd, IDC_EDIT_TOGGLE, &buf[0], 256)
	newCfg.ToggleHotkey = syscall.UTF16ToString(buf[:])

	GetDlgItemTextW(hwnd, IDC_EDIT_EXIT, &buf[0], 256)
	newCfg.ExitHotkey = syscall.UTF16ToString(buf[:])

	GetDlgItemTextW(hwnd, IDC_EDIT_MONITOR, &buf[0], 256)
	newCfg.MonitorHotkey = syscall.UTF16ToString(buf[:])

	GetDlgItemTextW(hwnd, IDC_EDIT_IDLE, &buf[0], 256)
	if val, err := strconv.ParseFloat(syscall.UTF16ToString(buf[:]), 64); err == nil && val >= 0 {
		newCfg.IdleHideTimeout = val
	}

	newCfg.Validate()
	*cfg = newCfg
	saveConfig(cfg)
}

// ============================================================
// 打开设置窗口
// ============================================================

// MSG 结构体 - Windows 消息
type MSG struct {
	Hwnd    HWND
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	PtX     int32
	PtY     int32
}

func showSettingsDialog() {
	// 锁定到当前 OS 线程，确保窗口消息正确路由
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	if settingsHwnd != 0 {
		SetForegroundWindow(settingsHwnd)
		return
	}

	hwnd := CreateWindowExW(WS_EX_DLGMODALFRAME|WS_EX_TOOLWINDOW,
		syscall.StringToUTF16Ptr(settingsWindowClass),
		syscall.StringToUTF16Ptr("设置 - 桌面图标切换"),
		WS_OVERLAPPEDWINDOW&^WS_MAXIMIZEBOX&^WS_THICKFRAME|WS_VISIBLE,
		200, 200, 400, 320, 0, 0, 0, 0)

	if hwnd == 0 {
		return
	}

	settingsHwnd = hwnd

	// 初始化控件（在窗口创建后、消息循环前调用）
	initSettingsDialog(hwnd)

	// 运行消息循环直到窗口关闭
	var msg MSG
	for {
		ret, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&msg)))
	}

	settingsHwnd = 0
}

// 额外需要的 API
var (
	procSetForegroundWindow2 = user32.NewProc("SetForegroundWindow")
)

func SetForegroundWindow(hwnd HWND) bool {
	ret, _, _ := procSetForegroundWindow2.Call(uintptr(hwnd))
	return ret != 0
}
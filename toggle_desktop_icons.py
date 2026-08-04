"""
Desktop Icon Toggle
  双击桌面空白处 → 隐藏/显示桌面图标（默认启用，支持 config 开关）
  双击任务栏空白处 → 隐藏/显示桌面图标（默认启用，支持 config 开关）
  Ctrl+Space 隐藏/显示桌面图标 + 显示/隐藏所有应用窗口
  Ctrl+Win+Alt 关闭显示器（移动鼠标或按任意键唤醒）
  点击任意应用窗口 → 自动延迟隐藏桌面图标
  无操作超时 → 配置 idle_hide_timeout 秒后自动隐藏桌面图标

支持通过 config.json 自定义快捷键和自动隐藏延迟
系统托盘：右键图标弹出"退出"菜单
支持打包为无窗口 EXE (PyInstaller --windowed)
"""

import ctypes
from ctypes import wintypes, byref, CFUNCTYPE, POINTER
import sys
import os
import json
import math
import threading
import time

# ── 依赖检查 ──────────────────────────────────────────────
try:
    import keyboard
except ImportError:
    print("缺少 keyboard 库，请先安装：pip install keyboard")
    sys.exit(1)

try:
    from win32com.client import Dispatch
except ImportError:
    print("缺少 pywin32 库，请先安装：pip install pywin32")
    sys.exit(1)

try:
    import pystray
    from PIL import Image, ImageDraw
except ImportError:
    print("缺少 pystray / Pillow 库，请先安装：pip install pystray pillow")
    sys.exit(1)

# ── 配置加载 ──────────────────────────────────────────────


def get_config_path():
    """获取配置文件路径（与 EXE/脚本同目录）"""
    if getattr(sys, 'frozen', False):
        base = os.path.dirname(sys.executable)
    else:
        base = os.path.dirname(os.path.abspath(__file__))
    return os.path.join(base, 'config.json')


def load_config():
    """加载配置文件，若不存在则创建默认配置"""
    defaults = {
        "hide_delay": 1.0,
        "toggle_hotkey": "ctrl+space",
        "toggle_enabled": True,
        "exit_hotkey": "ctrl+shift+q",
        "exit_enabled": True,
        "monitor_hotkey": "ctrl+win+alt",
        "monitor_enabled": True,
        "auto_hide_enabled": True,
        "dblclick_toggle_enabled": True,
        "dblclick_taskbar_enabled": True,
        "idle_hide_timeout": 0.0,
    }
    path = get_config_path()

    def _read_json():
        """尝试用不同编码读取 JSON，返回 (cfg, encoding) 或 None"""
        for enc in ('utf-8', 'gbk', 'gb2312', 'utf-16'):
            try:
                with open(path, 'r', encoding=enc) as f:
                    return json.load(f), enc
            except (UnicodeDecodeError, json.JSONDecodeError):
                continue
        return None

    result = _read_json()
    if result is not None:
        cfg, enc = result
        # 补全缺失的键
        for k, v in defaults.items():
            cfg.setdefault(k, v)
        # 以 UTF-8 写回，规范化文件
        try:
            with open(path, 'w', encoding='utf-8') as f:
                json.dump(cfg, f, indent=4, ensure_ascii=False)
        except OSError:
            pass
        return cfg

    # 文件不存在或无法解析 → 新建默认配置
    try:
        with open(path, 'w', encoding='utf-8') as f:
            json.dump(defaults, f, indent=4, ensure_ascii=False)
    except OSError:
        pass
    return dict(defaults)


# 全局配置实例
_config = load_config()


# ── Windows API ───────────────────────────────────────────
user32 = ctypes.windll.user32
kernel32 = ctypes.windll.kernel32

SW_HIDE = 0
SW_SHOW = 5

# 窗口查找
FindWindowW = user32.FindWindowW
FindWindowW.argtypes = [wintypes.LPCWSTR, wintypes.LPCWSTR]
FindWindowW.restype = wintypes.HWND

FindWindowExW = user32.FindWindowExW
FindWindowExW.argtypes = [wintypes.HWND, wintypes.HWND, wintypes.LPCWSTR, wintypes.LPCWSTR]
FindWindowExW.restype = wintypes.HWND

ShowWindow = user32.ShowWindow
ShowWindow.argtypes = [wintypes.HWND, ctypes.c_int]
ShowWindow.restype = wintypes.BOOL

IsWindowVisible = user32.IsWindowVisible
IsWindowVisible.argtypes = [wintypes.HWND]
IsWindowVisible.restype = wintypes.BOOL

# 窗口事件钩子（监听前景窗口变化）
WINEVENT_OUTOFCONTEXT = 0
EVENT_SYSTEM_FOREGROUND = 0x0003
WM_QUIT = 0x0012

WINEVENTPROC = CFUNCTYPE(
    None,
    wintypes.HANDLE,
    wintypes.DWORD,
    wintypes.HWND,
    wintypes.LONG,
    wintypes.LONG,
    wintypes.DWORD,
    wintypes.DWORD,
)

SetWinEventHook = user32.SetWinEventHook
SetWinEventHook.argtypes = [
    wintypes.DWORD,
    wintypes.DWORD,
    wintypes.HINSTANCE,
    WINEVENTPROC,
    wintypes.DWORD,
    wintypes.DWORD,
    wintypes.DWORD,
]
SetWinEventHook.restype = wintypes.HANDLE

UnhookWinEvent = user32.UnhookWinEvent
UnhookWinEvent.argtypes = [wintypes.HANDLE]
UnhookWinEvent.restype = wintypes.BOOL

GetParent = user32.GetParent
GetParent.argtypes = [wintypes.HWND]
GetParent.restype = wintypes.HWND

GetMessageW = user32.GetMessageW
GetMessageW.argtypes = [POINTER(wintypes.MSG), wintypes.HWND, wintypes.UINT, wintypes.UINT]
GetMessageW.restype = wintypes.BOOL

TranslateMessage = user32.TranslateMessage
TranslateMessage.argtypes = [POINTER(wintypes.MSG)]
TranslateMessage.restype = wintypes.BOOL

DispatchMessageW = user32.DispatchMessageW
DispatchMessageW.argtypes = [POINTER(wintypes.MSG)]
DispatchMessageW.restype = wintypes.LONG

PostThreadMessageW = user32.PostThreadMessageW
PostThreadMessageW.argtypes = [wintypes.DWORD, wintypes.UINT, wintypes.WPARAM, wintypes.LPARAM]
PostThreadMessageW.restype = wintypes.BOOL

# 窗口消息发送（同步 / 异步）
SendMessageW = user32.SendMessageW
SendMessageW.argtypes = [wintypes.HWND, wintypes.UINT, wintypes.WPARAM, wintypes.LPARAM]
SendMessageW.restype = wintypes.LPARAM

PostMessageW = user32.PostMessageW
PostMessageW.argtypes = [wintypes.HWND, wintypes.UINT, wintypes.WPARAM, wintypes.LPARAM]
PostMessageW.restype = wintypes.BOOL

# ── 鼠标双击检测（GetAsyncKeyState 轮询，不依赖系统钩子）────────
VK_LBUTTON = 0x01

GetAsyncKeyState = user32.GetAsyncKeyState
GetAsyncKeyState.argtypes = [ctypes.c_int]
GetAsyncKeyState.restype = ctypes.c_short

GetCursorPos = user32.GetCursorPos
GetCursorPos.argtypes = [POINTER(wintypes.POINT)]
GetCursorPos.restype = wintypes.BOOL

WindowFromPoint = user32.WindowFromPoint
WindowFromPoint.argtypes = [wintypes.POINT]
WindowFromPoint.restype = wintypes.HANDLE

GetAncestor = user32.GetAncestor
GetAncestor.argtypes = [wintypes.HWND, wintypes.UINT]
GetAncestor.restype = wintypes.HWND

GetClassNameW = user32.GetClassNameW
GetClassNameW.argtypes = [wintypes.HWND, wintypes.LPCWSTR, ctypes.c_int]
GetClassNameW.restype = ctypes.c_int

GA_ROOT = 2


def find_desktop_listview():
    """定位桌面图标所在的 SysListView32 窗口句柄"""
    progman = FindWindowW("Progman", None)
    if progman:
        shell = FindWindowExW(progman, None, "SHELLDLL_DefView", None)
        if shell:
            lv = FindWindowExW(shell, None, "SysListView32", "FolderView")
            if lv:
                return lv

    worker = None
    while True:
        worker = FindWindowExW(None, worker, "WorkerW", None)
        if not worker:
            break
        shell = FindWindowExW(worker, None, "SHELLDLL_DefView", None)
        if shell:
            lv = FindWindowExW(shell, None, "SysListView32", "FolderView")
            if lv:
                return lv

    return None


# ── 切换逻辑 ──────────────────────────────────────────────

# 防止双击和快捷键同时触发造成连续切换
_toggle_lock = threading.Lock()
_scheduled_timer = None


def toggle():
    """切换：图标 <-> 窗口"""
    with _toggle_lock:
        import pythoncom
        pythoncom.CoInitialize()

        hwnd = find_desktop_listview()
        if not hwnd:
            return

        shell = Dispatch("Shell.Application")

        if IsWindowVisible(hwnd):
            ShowWindow(hwnd, SW_HIDE)
            shell.UndoMinimizeAll()
        else:
            ShowWindow(hwnd, SW_SHOW)
            shell.MinimizeAll()


def schedule_toggle(delay=0.25):
    """延迟触发切换（给双击事件留出处理时间）"""
    global _scheduled_timer
    if _scheduled_timer is not None:
        _scheduled_timer.cancel()
    _scheduled_timer = threading.Timer(delay, toggle)
    _scheduled_timer.start()


def auto_hide():
    """仅隐藏桌面图标（不切换窗口）"""
    with _toggle_lock:
        import pythoncom
        pythoncom.CoInitialize()

        hwnd = find_desktop_listview()
        if hwnd and IsWindowVisible(hwnd):
            ShowWindow(hwnd, SW_HIDE)


def show_desktop_icons():
    """显示桌面图标"""
    with _toggle_lock:
        import pythoncom
        pythoncom.CoInitialize()

        hwnd = find_desktop_listview()
        if hwnd and not IsWindowVisible(hwnd):
            ShowWindow(hwnd, SW_SHOW)


def turn_off_monitor():
    """关闭显示器（移动鼠标或按任意键自动唤醒）"""
    HWND_BROADCAST = 0xFFFF
    WM_SYSCOMMAND = 0x0112
    SC_MONITORPOWER = 0xF170
    # 使用 PostMessage 异步发送，避免广播阻塞导致失败
    PostMessageW(HWND_BROADCAST, WM_SYSCOMMAND, SC_MONITORPOWER, 2)
    # 再补发一次：防止按键松开时的 key-up 事件把显示器唤醒
    threading.Timer(0.3, lambda: PostMessageW(
        HWND_BROADCAST, WM_SYSCOMMAND, SC_MONITORPOWER, 2)).start()


def schedule_hide_icons():
    """延迟隐藏桌面图标（从配置读取延迟时间）"""
    global _scheduled_timer
    delay = _config.get("hide_delay", 1.0)
    if _scheduled_timer is not None:
        _scheduled_timer.cancel()
    _scheduled_timer = threading.Timer(delay, auto_hide)
    _scheduled_timer.start()


# ── 前景窗口变化钩子 ──────────────────────────────────────

# 存储钩子句柄和线程 ID
_foreground_hook = None
_foreground_hook_thread_id = 0



def is_desktop_window(hwnd):
    """判断窗口是否属于桌面区域（通过 GetAncestor 直达根窗口）"""
    if not hwnd:
        return False
    root = GetAncestor(hwnd, GA_ROOT)
    if not root:
        return False
    if root == FindWindowW("Progman", None):
        return True
    w = None
    while True:
        w = FindWindowExW(None, w, "WorkerW", None)
        if not w:
            break
        if w == root:
            return True
    return False


def get_class_name(hwnd):
    """获取窗口类名"""
    buf = ctypes.create_unicode_buffer(256)
    if hwnd and GetClassNameW(hwnd, buf, 256):
        return buf.value
    return ""


# 任务栏中不触发切换的区域（按钮、托盘、开始/搜索等）
_TASKBAR_NO_TRIGGER_CLASSES = {
    "MSTaskSwWClass",              # 任务列表容器（MSTaskListWClass 单独处理）
    "TrayNotifyWnd",               # 系统托盘
    "ToolbarWindow32",             # 托盘图标
    "TrayDummySearchControl",      # 搜索框
    "Windows.UI.Core.CoreWindow",  # 开始按钮/搜索/小组件
    "Button",                      # 开始按钮 / 显示桌面细条
    "TrayShowDesktopButtonWClass", # 显示桌面细条
    "NotifyIconOverflowWindow",    # 溢出托盘
}

# Win10 任务栏按钮区域（MSTaskListWClass 同时覆盖按钮与空白处，
# 需要结合 MSAA 命中测试区分，见 _tasklist_point_is_button）
_TASKBAR_BUTTON_STRIP_CLASS = "MSTaskListWClass"

# ── MSAA：任务栏按钮判定 ────────────────────────────────

ROLE_SYSTEM_PUSHBUTTON = 0x2B   # 任务按钮（正在运行/已固定应用）
ROLE_SYSTEM_APPBUTTON = 0x39    # 应用按钮
OBJID_CLIENT = 0xFFFFFFFC

_IID_IAccessible_bytes = (
    0xe0, 0x36, 0x87, 0x61,   # Data1 = 0x618736E0 (little-endian)
    0x3d, 0x3c,               # Data2 = 0x3C3D
    0xcf, 0x11,               # Data3 = 0x11CF
    0x81, 0x0c, 0x00, 0xaa, 0x00, 0x38, 0x9b, 0x71  # Data4 = {618736E0-3C3D-11CF-810C-00AA00389B71}
)


class _VARIANT(ctypes.Structure):
    class _U(ctypes.Union):
        _fields_ = [
            ("lVal", ctypes.c_long),
            ("bstrVal", ctypes.c_wchar_p),
            ("boolVal", ctypes.c_short),
            ("_pad", ctypes.c_byte * 16),
        ]
    _fields_ = [
        ("vt", ctypes.c_ushort),
        ("_r1", ctypes.c_ushort),
        ("_r2", ctypes.c_ulong),
        ("_u", _U),
    ]


_VT_I4 = 3


def _tasklist_point_is_button(pt, tasklist_hwnd):
    """判断任务栏按钮区域（MSTaskListWClass）内的点是否落在具体按钮上"""
    try:
        ole32 = ctypes.windll.ole32
        try:
            ole32.CoInitializeEx(None, 0)
        except OSError:
            try:
                ole32.CoInitializeEx(None, 1)
            except OSError:
                pass

        # AccessibleObjectFromWindow → IAccessible*
        iid_buf = (ctypes.c_ubyte * 16)(*(_IID_IAccessible_bytes))
        p_acc = ctypes.c_void_p()
        hr = ctypes.windll.oleacc.AccessibleObjectFromWindow(
            tasklist_hwnd, OBJID_CLIENT,
            byref(iid_buf),
            ctypes.byref(p_acc))
        print(f"[MSAA] OAF hr={hr} p_acc={p_acc.value}")
        if hr != 0 or not p_acc:
            return True

        # 获取 vtable 指针
        vtbl_ptr = ctypes.c_void_p()
        ctypes.memmove(ctypes.byref(vtbl_ptr), p_acc, ctypes.sizeof(ctypes.c_void_p))
        vtbl = vtbl_ptr.value
        PTR_SIZE = ctypes.sizeof(ctypes.c_void_p)

        # accHitTest(x, y, pvarChild)
        cf = ctypes.CFUNCTYPE(
            ctypes.c_long, ctypes.c_void_p,
            ctypes.c_long, ctypes.c_long, ctypes.c_void_p)
        fn = cf(ctypes.c_void_p.from_address(vtbl + 24 * PTR_SIZE).value)
        var = _VARIANT()
        hr = fn(p_acc.value, pt.x, pt.y, ctypes.byref(var))
        print(f"[MSAA] hitTest hr={hr} vt={var.vt} child={var._u.lVal}")
        if hr != 0:
            return True
        child_id = var._u.lVal
        if child_id <= 0:
            return False

        # accRole(pvarId, pvarRole)
        cf_role = ctypes.CFUNCTYPE(
            ctypes.c_long, ctypes.c_void_p,
            ctypes.c_void_p, ctypes.c_void_p)
        fn_role = cf_role(ctypes.c_void_p.from_address(vtbl + 13 * PTR_SIZE).value)
        id_var = _VARIANT()
        id_var.vt = _VT_I4
        id_var._u.lVal = child_id
        role_var = _VARIANT()
        hr = fn_role(p_acc.value, ctypes.byref(id_var), ctypes.byref(role_var))
        print(f"[MSAA] role hr={hr} role={role_var._u.lVal}")
        if hr != 0:
            return True
        role = role_var._u.lVal
        return role in (ROLE_SYSTEM_PUSHBUTTON, ROLE_SYSTEM_APPBUTTON)
    except Exception as e:
        print(f"[MSAA] EXC {type(e).__name__}: {e}")
        return True


def is_taskbar_empty_area(pt):
    """判断鼠标位置是否位于任务栏空白处"""
    hwnd = WindowFromPoint(pt)
    if not hwnd:
        return False
    root = GetAncestor(hwnd, GA_ROOT)
    if not root:
        return False
    if get_class_name(root) not in ("Shell_TrayWnd", "Shell_SecondaryTrayWnd"):
        return False
    leaf = get_class_name(hwnd)
    if leaf in _TASKBAR_NO_TRIGGER_CLASSES:
        return False
    # Win10 任务栏：按钮条同时覆盖按钮与空白，用 MSAA 命中测试区分
    if leaf == _TASKBAR_BUTTON_STRIP_CLASS:
        return not _tasklist_point_is_button(pt, hwnd)
    return True


def foreground_hook_proc(hWinEventHook, event, hwnd, idObject, idChild, dwEventThread, dwmsEventTime):
    """窗口事件钩子回调 - 检测前景窗口变化"""
    if not hwnd:
        return
    if is_desktop_window(hwnd):
        return
    if not IsWindowVisible(hwnd):
        return
    # 应用窗口被激活 → 1秒后自动隐藏桌面图标
    schedule_hide_icons()


# 保持回调引用防止被 GC
_foreground_hook_proc_ref = WINEVENTPROC(foreground_hook_proc)


def foreground_hook_thread_main():
    """前景窗口事件钩子消息循环（独立线程）"""
    global _foreground_hook, _foreground_hook_thread_id

    _foreground_hook_thread_id = kernel32.GetCurrentThreadId()

    _foreground_hook = SetWinEventHook(
        EVENT_SYSTEM_FOREGROUND,
        EVENT_SYSTEM_FOREGROUND,
        0,
        _foreground_hook_proc_ref,
        0,
        0,
        WINEVENT_OUTOFCONTEXT,
    )

    if not _foreground_hook:
        return

    # Windows 消息泵
    msg = wintypes.MSG()
    while GetMessageW(byref(msg), None, 0, 0) > 0:
        TranslateMessage(byref(msg))
        DispatchMessageW(byref(msg))

    UnhookWinEvent(_foreground_hook)


def start_foreground_hook():
    """启动前景窗口钩子线程"""
    t = threading.Thread(target=foreground_hook_thread_main, daemon=True)
    t.start()
    return t


def stop_foreground_hook():
    """停止前景窗口钩子"""
    global _foreground_hook_thread_id, _scheduled_timer
    if _scheduled_timer is not None:
        _scheduled_timer.cancel()
    if _foreground_hook_thread_id:
        PostThreadMessageW(_foreground_hook_thread_id, WM_QUIT, 0, 0)


# ── 鼠标低层钩子（双击桌面空白处切换图标）────────────────

# ── 无操作自动隐藏（GetLastInputInfo + GetTickCount）───────

class LASTINPUTINFO(ctypes.Structure):
    _fields_ = [("cbSize", wintypes.UINT), ("dwTime", wintypes.UINT)]

GetLastInputInfo = user32.GetLastInputInfo
GetLastInputInfo.argtypes = [POINTER(LASTINPUTINFO)]
GetLastInputInfo.restype = wintypes.BOOL

GetTickCount = kernel32.GetTickCount
GetTickCount.argtypes = []
GetTickCount.restype = wintypes.DWORD

# ── 双击轮询线程（GetAsyncKeyState 方案）────────────────

_mouse_poll_stop = False

# 双击判定参数
_click_times = []                 # 最近两次点击时间戳
_DOUBLECLICK_TIME = 0.5           # 双击时间窗口（秒）
_last_toggle_tick = 0
_TOGGLE_COOLDOWN = 0.5            # 切换冷却（秒）


def mouse_poll_loop():
    """轮询鼠标左键状态，检测按键按下边缘（0→1）来识别双击；同时检测无操作超时"""
    global _click_times, _last_toggle_tick, _mouse_poll_stop
    # 初始化 COM（任务栏按钮判定依赖 MSAA / oleacc）
    ole32 = ctypes.windll.ole32
    ole32.CoInitializeEx.argtypes = [ctypes.c_void_p, wintypes.DWORD]
    com_inited = False
    try:
        ole32.CoInitializeEx(None, 0)   # MTA
        com_inited = True
    except OSError:
        try:
            ole32.CoInitializeEx(None, 1)  # STA fallback
            com_inited = True
        except OSError:
            pass  # COM 已被其他模块初始化，复用现有状态
    prev_state = False
    poll_count = 0
    while not _mouse_poll_stop:
        state = bool(GetAsyncKeyState(VK_LBUTTON) & 0x8000)
        # 边缘检测：从未按 → 按下 = 一次新点击
        if state and not prev_state:
            now = time.time()
            _click_times.append(now)
            # 清理过期时间戳
            while _click_times and now - _click_times[0] > _DOUBLECLICK_TIME:
                _click_times.pop(0)
            # 时间窗口内两次点击 = 双击
            if len(_click_times) >= 2:
                pt = wintypes.POINT()
                GetCursorPos(byref(pt))
                hwnd = WindowFromPoint(pt)
                cls = get_class_name(hwnd) if hwnd else '?'
                root = GetAncestor(hwnd, GA_ROOT) if hwnd else None
                rcls = get_class_name(root) if root else '?'
                on_desktop = hwnd and is_desktop_window(hwnd)
                on_taskbar = is_taskbar_empty_area(pt)
                enabled = (on_desktop and _config.get("dblclick_toggle_enabled", True)) or \
                          (on_taskbar and _config.get("dblclick_taskbar_enabled", True))
                print(f"[DBG] ({pt.x},{pt.y}) leaf={cls} root={rcls} desk={on_desktop} tb={on_taskbar} en={enabled}")
                if enabled:
                    if now - _last_toggle_tick >= _TOGGLE_COOLDOWN:
                        _last_toggle_tick = now
                        _click_times.clear()
                        threading.Timer(0.05, toggle).start()
        prev_state = state
        poll_count += 1
        # 无操作超时隐藏（约每 1 秒检测一次）
        if poll_count >= 66:
            poll_count = 0
            idle_sec = _config.get("idle_hide_timeout", 0.0)
            if idle_sec > 0:
                li = LASTINPUTINFO()
                li.cbSize = ctypes.sizeof(LASTINPUTINFO)
                if GetLastInputInfo(byref(li)):
                    elapsed_ms = GetTickCount() - li.dwTime
                    if elapsed_ms >= int(idle_sec * 1000):
                        threading.Thread(target=auto_hide, daemon=True).start()
        time.sleep(0.015)          # 15ms 轮询间隔，足够捕获快速点击


def start_mouse_hook():
    """启动鼠标轮询线程"""
    global _mouse_poll_stop
    _mouse_poll_stop = False
    t = threading.Thread(target=mouse_poll_loop, daemon=True)
    t.start()
    return t


def stop_mouse_hook():
    """停止鼠标轮询"""
    global _mouse_poll_stop, _click_times
    _mouse_poll_stop = True
    _click_times.clear()


# ── 系统托盘 ──────────────────────────────────────────────

def create_tray_image():
    """生成托盘图标（忍者面罩）"""
    img = Image.new("RGBA", (64, 64), (0, 0, 0, 0))
    draw = ImageDraw.Draw(img)

    # 头部（深灰色圆）
    draw.ellipse([4, 4, 60, 60], fill=(35, 35, 35), outline=(80, 80, 80), width=2)

    # 双眼（白色横条）
    draw.rectangle([14, 25, 25, 29], fill=(220, 220, 220))
    draw.rectangle([39, 25, 50, 29], fill=(220, 220, 220))

    # 头巾（红色横带）
    draw.rectangle([4, 14, 60, 21], fill=(180, 30, 30), outline=(120, 20, 20), width=1)

    # 头巾飘带
    draw.polygon([(60, 14), (64, 10), (64, 17), (60, 21)], fill=(180, 30, 30))
    return img


def on_exit(icon, _item):
    """托盘退出菜单 - 先恢复桌面图标再退出"""
    show_desktop_icons()
    stop_foreground_hook()
    stop_mouse_hook()
    icon.stop()
    os._exit(0)


def _patch_tray_click_handlers(icon):
    """Monkey-patch 托盘图标的 _on_notify 以自定义点击行为：
    - 左键单击：切换隐藏/显示桌面图标
    - 右键单击：弹出退出菜单（默认行为不变）
    """
    import pystray._win32 as _pw
    win32 = _pw.win32

    original_on_notify = icon._on_notify
    click_timer = [None]

    def patched_on_notify(wparam, lparam):
        if lparam == win32.WM_LBUTTONUP:
            # 左键单击：100ms 后切换
            if click_timer[0] is not None:
                click_timer[0].cancel()
            click_timer[0] = threading.Timer(0.1, toggle)
            click_timer[0].start()
        else:
            original_on_notify(wparam, lparam)

    # 必须同时更新 _message_handlers，因为它在 __init__ 时已缓存了旧方法引用
    icon._on_notify = patched_on_notify
    icon._message_handlers[win32.WM_NOTIFY] = patched_on_notify


def create_tray_icon():
    """创建、配置点击处理器并返回托盘图标对象"""
    cfg = load_config()

    # 动态拼接提示文字：只显示已启用的快捷键
    parts = []
    if cfg.get("toggle_enabled", True):
        parts.append(f"{cfg['toggle_hotkey']} 切换")
    if cfg.get("monitor_enabled", True):
        parts.append(f"{cfg['monitor_hotkey']} 关显示器")
    if cfg.get("auto_hide_enabled", True):
        parts.append("点击窗口自动隐藏")
    if cfg.get("dblclick_toggle_enabled", True):
        parts.append("桌面双击切换")
    if cfg.get("dblclick_taskbar_enabled", True):
        parts.append("任务栏双击切换")
    idle_t = cfg.get("idle_hide_timeout", 0.0)
    if idle_t > 0:
        parts.append(f"无操作{idle_t:g}秒隐藏")
    parts.append("左键切换 | 右键退出")
    tooltip = "桌面图标切换\n" + " | ".join(parts)
    icon = pystray.Icon(
        "DesktopToggle",
        create_tray_image(),
        tooltip,
        menu=pystray.Menu(
            pystray.MenuItem("退出", on_exit)
        )
    )
    _patch_tray_click_handlers(icon)
    return icon


# ── 主入口 ────────────────────────────────────────────────

def main():
    cfg = load_config()

    # 1. 启动前景窗口变化钩子（检测应用窗口激活，按配置启用）
    if cfg.get("auto_hide_enabled", True):
        start_foreground_hook()

    # 2. 启动鼠标轮询线程（检测双击 / 无操作超时，任一需要即启动）
    need_poll = cfg.get("dblclick_toggle_enabled", True) or \
                cfg.get("dblclick_taskbar_enabled", True) or \
                cfg.get("idle_hide_timeout", 0.0) > 0
    if need_poll:
        start_mouse_hook()

    # 2. 启动系统托盘图标
    tray_icon = create_tray_icon()
    tray_icon.run_detached()

    # 3. 注册全局热键（按配置启用）
    if cfg.get("toggle_enabled", True):
        keyboard.add_hotkey(cfg["toggle_hotkey"], toggle)
    if cfg.get("monitor_enabled", True):
        keyboard.add_hotkey(cfg["monitor_hotkey"], turn_off_monitor)
    if cfg.get("exit_enabled", True):
        keyboard.add_hotkey(cfg["exit_hotkey"], lambda: on_exit(tray_icon, None))

    # 4. 常驻后台
    keyboard.wait()


if __name__ == "__main__":
    main()

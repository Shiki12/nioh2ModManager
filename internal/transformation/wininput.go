// wininput.go - Win32 窗口定位、前台激活、键鼠注入与坐标换算的封装
//
// 面向对象设计, 对外暴露两个对象:
//
//	GameWindow  代表一个具体的游戏窗口句柄, 提供窗口级操作:
//	            查找/标题/类名/所属PID/前台激活/客户区坐标换算/按键投递。
//	Input       无状态的输入注入器(空结构体), 提供鼠标与键盘注入原语:
//	            取鼠标坐标/移动鼠标/点击/方向键注入/前台查询。
//
// 背景: 仁王2 键盘走 DirectInput 独占, 注入按键(SetForeground+keybd_event 或
// PostMessage)无效; 但鼠标菜单可用 SendInput 驱动且"悬停即选中"。故整套自动化
// 以「移动鼠标 + capture hook」为主线, 键盘键仅保留在 batch 模式的槽位遍历里。
//
// 参考 nioh2mod-js/internal/input/input.go 的轻量做法 (SW_RESTORE +
// SetForegroundWindow + keybd_event), 不做重型激活, 避免拉起 GDI+ Window。
package transformation

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

// 本窗口/输入相关 Win32 API 的懒加载句柄。通过 syscall.NewLazyDLL/NewProc 在
// 首次调用时按名解析导出函数, 避免显式 LoadLibrary, 便于跨版本稳定加载。
// 统一以 w 前缀命名, 与 refashion 包内的底层调用区分。仅供本文件内部使用。
var (
	user32 = syscall.NewLazyDLL("user32.dll")

	// ---- 窗口枚举 / 信息 ----
	wEnumWindows           = user32.NewProc("EnumWindows")              // 枚举全部顶层窗口
	wGetWindowTextW        = user32.NewProc("GetWindowTextW")           // 取窗口标题
	wGetWindowThreadProcID = user32.NewProc("GetWindowThreadProcessId") // 取窗口所属线程/进程 ID
	wGetClassNameW         = user32.NewProc("GetClassNameW")            // 取窗口类名
	wEnumChildWindows      = user32.NewProc("EnumChildWindows")         // 枚举子窗口
	wIsWindowVisible       = user32.NewProc("IsWindowVisible")          // 窗口是否可见

	// ---- 前台 / 激活 ----
	wShowWindow          = user32.NewProc("ShowWindow")          // 恢复/最小化窗口
	wSetForegroundWindow = user32.NewProc("SetForegroundWindow") // 设窗口为前台
	wGetForegroundWindow = user32.NewProc("GetForegroundWindow") // 取当前前台窗口
	wAttachThreadInput   = user32.NewProc("AttachThreadInput")   // 挂接输入队列(绕过前台锁)
	wBringWindowToTop    = user32.NewProc("BringWindowToTop")    // 置顶

	// ---- 键鼠注入 ----
	wKeybdEvent     = user32.NewProc("keybd_event")    // 传统键盘模拟(前台)
	wPostMessageW   = user32.NewProc("PostMessageW")   // 向窗口消息队列投递按键
	wMapVirtualKeyW = user32.NewProc("MapVirtualKeyW") // 虚拟键码 <-> 扫描码换算
	wSendInput      = user32.NewProc("SendInput")      // 高级输入注入(鼠标/键盘带扫描码)
	wGetCursorPos   = user32.NewProc("GetCursorPos")   // 取鼠标屏幕坐标
	wSetCursorPos   = user32.NewProc("SetCursorPos")   // 移动鼠标到屏幕坐标

	// ---- 客户区坐标换算 ----
	wGetClientRect  = user32.NewProc("GetClientRect")  // 客户区矩形(不含边框/标题栏)
	wClientToScreen = user32.NewProc("ClientToScreen") // 客户区坐标 -> 屏幕坐标
	wScreenToClient = user32.NewProc("ScreenToClient") // 屏幕坐标 -> 客户区坐标

winKernel32 = syscall.NewLazyDLL("kernel32.dll")

	// ---- 进程相关 ----
	wGetCurrentThreadID         = winKernel32.NewProc("GetCurrentThreadId")        // 取当前线程 ID
	wOpenProcess                = winKernel32.NewProc("OpenProcess")               // 打开进程
	wQueryFullProcessImageNameW = winKernel32.NewProc("QueryFullProcessImageNameW") // 取进程可执行文件路径
	wCloseHandle                = winKernel32.NewProc("CloseHandle")               // 关闭句柄
)

// Windows 常量: 虚拟键码、消息类型、窗口/进程标志位。
const (
	VK_DOWN                 = 0x28   // 方向键: 下
	VK_UP                   = 0x26   // 方向键: 上
	VK_LEFT                 = 0x25   // 方向键: 左
	VK_RIGHT                = 0x27   // 方向键: 右
	WM_KEYDOWN              = 0x0100 // 键盘按下消息
	WM_KEYUP                = 0x0101 // 键盘抬起消息
	W_SW_RESTORE            = 9      // ShowWindow: 还原(从最小化恢复)
	W_PROCESS_QUERY_LIMITED = 0x1000 // OpenProcess: 受限查询权限(取进程路径)
	TRUE_                   = 1      // 布尔真
	FALSE_                  = 0      // 布尔假
)

// ================================================================
// GameWindow: 游戏窗口句柄封装
// ================================================================

// GameWindow 包装一个具体的游戏窗口句柄, 提供窗口级操作。
// 持有一个未导出的 hwnd, 外部不能直接伪造, 只能通过 FindGameWindow/NativeFindWindow
// 等工厂取得有效实例。
type GameWindow struct {
	hwnd uintptr
}

// FindGameWindow 遍历顶层窗口, 找到属于游戏本体(标题含 "Nioh2" 且可见、非本进程、
// 进程名为 nioh2)的窗口, 包装成 GameWindow 返回; 未找到返回 nil。
func FindGameWindow() *GameWindow {
	if hwnd := findNative(); hwnd != 0 {
		return &GameWindow{hwnd: hwnd}
	}
	return nil
}

// NativeFrom 把一个已知的原始窗口句柄包装成 GameWindow (用于 keytest 等调试流程)。
func NativeFrom(hwnd uintptr) *GameWindow {
	return &GameWindow{hwnd: hwnd}
}

// Handle 返回底层窗口句柄。
func (w *GameWindow) Handle() uintptr { return w.hwnd }

// Title 返回窗口标题文本。
func (w *GameWindow) Title() string {
	var buf [256]uint16
	wGetWindowTextW.Call(w.hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return strings.TrimSpace(syscall.UTF16ToString(buf[:]))
}

// Class 返回窗口类名。
func (w *GameWindow) Class() string {
	var buf [256]uint16
	wGetClassNameW.Call(w.hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return syscall.UTF16ToString(buf[:])
}

// PID 返回窗口所属进程 ID。
func (w *GameWindow) PID() uint32 { return wndPID(w.hwnd) }

// Visible 报告窗口当前是否可见。
func (w *GameWindow) Visible() bool {
	vis, _, _ := wIsWindowVisible.Call(w.hwnd)
	return vis != 0
}

// BringToFront 可靠地把窗口带到前台并返回是否成功。
//
// 单靠 SetForegroundWindow 常被 Windows 的前台锁 (系统只允许"最近活动进程"抢焦点)
// 拦截。这里用 AttachThreadInput 把本线程挂了输入队列到目标/当前前台线程再抢焦点,
// 可越过该限制 (Cheat Engine/按键精灵常用手法)。
func (w *GameWindow) BringToFront() bool {
	if w.hwnd == 0 {
		return false
	}
	wShowWindow.Call(w.hwnd, W_SW_RESTORE)

	// 当前前台窗口线程
	fg := curForeground()
	var fgThread, tgt uintptr
	if fg != 0 {
		fgThread, _, _ = wGetWindowThreadProcID.Call(fg, 0)
	}
	// 我们的输入线程
	my, _, _ := wGetCurrentThreadID.Call()

	if fgThread != 0 && fgThread != my {
		wAttachThreadInput.Call(fgThread, my, TRUE_)
	}
	// 目标窗口线程也挂一次 (有些情况需要)
	tgt, _, _ = wGetWindowThreadProcID.Call(w.hwnd, 0)
	if fgThread == 0 && tgt != 0 && tgt != my {
		wAttachThreadInput.Call(tgt, my, TRUE_)
	}

	wBringWindowToTop.Call(w.hwnd)
	wSetForegroundWindow.Call(w.hwnd)

	// 解除附加
	if fgThread != 0 && fgThread != my {
		wAttachThreadInput.Call(fgThread, my, FALSE_)
	}
	if tgt != 0 && tgt != my {
		wAttachThreadInput.Call(tgt, my, FALSE_)
	}

	// 确认已切到前台
	ok, _, _ := wGetForegroundWindow.Call()
	time.Sleep(300 * time.Millisecond)
	return ok == w.hwnd
}

// IsForeground 报告窗口当前是否为前台窗口 (用于诊断焦点是否被切走)。
func (w *GameWindow) IsForeground() bool {
	ok, _, _ := wGetForegroundWindow.Call()
	return ok == w.hwnd
}

// ClientSize 返回窗口客户区宽高 (不含标题栏/边框)。窗口最小化时可能失败, ok=false。
func (w *GameWindow) ClientSize() (cw, ch int64, ok bool) {
	var rc [16]byte
	ret, _, _ := wGetClientRect.Call(w.hwnd, uintptr(unsafe.Pointer(&rc[0])))
	if ret == 0 {
		return 0, 0, false
	}
	left := int64(int32(binary.LittleEndian.Uint32(rc[0:4])))
	top := int64(int32(binary.LittleEndian.Uint32(rc[4:8])))
	right := int64(int32(binary.LittleEndian.Uint32(rc[8:12])))
	bottom := int64(int32(binary.LittleEndian.Uint32(rc[12:16])))
	return right - left, bottom - top, true
}

// ToScreen 把客户区内的相对坐标 (clientX,clientY) 换算成屏幕绝对坐标。
// 依此把 `bmo` 里的「槽位坐标」视为窗口内相对位置, 全屏/窗口化都能正确定位。
func (w *GameWindow) ToScreen(clientX, clientY int) (int, int, bool) {
	// ClientToScreen: 输入客户区坐标, 返回屏幕坐标 (原地修改 pt)
	var pt [8]byte
	binary.LittleEndian.PutUint32(pt[0:4], uint32(int32(clientX)))
	binary.LittleEndian.PutUint32(pt[4:8], uint32(int32(clientY)))
	ret, _, _ := wClientToScreen.Call(w.hwnd, uintptr(unsafe.Pointer(&pt[0])))
	if ret == 0 {
		return 0, 0, false
	}
	sx := int(int32(binary.LittleEndian.Uint32(pt[0:4])))
	sy := int(int32(binary.LittleEndian.Uint32(pt[4:8])))
	return sx, sy, true
}

// ToClient 把屏幕绝对坐标换算成窗口客户区内的相对坐标 (ToScreen 的逆)。
func (w *GameWindow) ToClient(screenX, screenY int) (int, int, bool) {
	var pt [8]byte
	binary.LittleEndian.PutUint32(pt[0:4], uint32(int32(screenX)))
	binary.LittleEndian.PutUint32(pt[4:8], uint32(int32(screenY)))
	ret, _, _ := wScreenToClient.Call(w.hwnd, uintptr(unsafe.Pointer(&pt[0])))
	if ret == 0 {
		return 0, 0, false
	}
	cx := int(int32(binary.LittleEndian.Uint32(pt[0:4])))
	cy := int(int32(binary.LittleEndian.Uint32(pt[4:8])))
	return cx, cy, true
}

// OriginScreen 返回窗口客户区原点 (0,0 客户点) 的屏幕绝对坐标。
// 有了原点 + 客户区尺寸, 就能把任何窗口内相对坐标换算成屏幕绝对坐标。
func (w *GameWindow) OriginScreen() (int, int, bool) {
	return w.ToScreen(0, 0)
}

// PostArrow 通过 PostMessage 把方向键投递给该窗口 (不依赖前台)。
//
// 相比 keybd_event (需要游戏在前台), PostMessage 直接投递给窗口的 HWND,
// 对"用消息队列接收输入"的游戏更可靠, 也不会被 Windows 前台锁限制。
// 缺点: 若游戏用 DirectInput / GetAsyncKeyState 读取按键, 则此方式无效,
// 此时需退回到"前台 + keybd_event/SendInput"。
func (w *GameWindow) PostArrow(vk uintptr) {
	// lParam 组装扫描码 (需与 vk 对应, MapVirtualKey 换算)
	sc, _, _ := wMapVirtualKeyW.Call(vk, 4 /* MAPVK_VK_TO_VSC */)
	var extended uint32
	switch vk {
	case VK_LEFT, VK_RIGHT, VK_UP, VK_DOWN:
		extended = 0x0100 // KF_EXTENDED
	}
	lPress := uintptr(1) | sc<<16 | uintptr(extended)<<24 // repeat=1 | scancode | extended
	lRelease := lPress | 0xC0000000                       // 加 key-up (bit30) + previous key state (bit31)
	wPostMessageW.Call(w.hwnd, WM_KEYDOWN, vk, lPress)
	wPostMessageW.Call(w.hwnd, WM_KEYUP, vk, lRelease)
	time.Sleep(300 * time.Millisecond)
}

// ================================================================
// Input: 无状态键鼠注入封装
// ================================================================

// Input 是无状态的输入注入器 (空结构体, 方法即命名空间)。
type Input struct{}

// input 是 Input 的包级单例 (空结构体无状态, 便于以 input.MoveTo(...) 调用)。
var input = Input{}

// KEYFACE_DOWN 返回方向名字符串对应的虚拟键码, 默认 "down"。
// 支持 "up" / "left" / "right" / "down" (不分大小写)。
func (Input) KeyDir(dir string) uintptr {
	switch strings.ToLower(dir) {
	case "up":
		return VK_UP
	case "left":
		return VK_LEFT
	case "right":
		return VK_RIGHT
	default:
		return VK_DOWN
	}
}

// SendArrow 用 SendInput 发送一次方向键 (带扫描码+extended位)。
//
// 关键: 只传虚拟键 vk 会被 Nioh2 这类用 DirectInput/Raw Input 读键盘的游戏忽略,
// 必须附带真实扫描码 scancode。这里用 MapVirtualKey 换算。发送前确保游戏在前台。
func (Input) SendArrow(vk uintptr) {
	// 方向键带扩展标志 (scancode 第8位 = extended)
	var extended uintptr
	switch vk {
	case VK_LEFT, VK_RIGHT, VK_UP, VK_DOWN:
		extended = 0x0100 /* KF_EXTENDED */
	}
	sc, _, _ := wMapVirtualKeyW.Call(vk, 4 /* MAPVK_VK_TO_VSC */)

	// INPUT 结构 (x64, 实际 40 字节):
	//   type            DWORD (4)   = INPUT_KEYBOARD=1
	//   padding         4
	//   union (取最大 MOUSEINPUT 32 字节对齐):
	//     wVk           WORD (2)
	//     wScan         WORD (2)
	//     dwFlags       DWORD(4)
	//     time          DWORD(4)
	//     dwExtraInfo   ULONG_PTR(8)
	//   → 4+4+2+2+4+4+8 = 28，union 对齐到 32，总 sizeof(INPUT)=40
	// 注意: SendInput 的 cbSize 必须传 40，传错会直接返回 0 注入失败。
	buildKey := func(flags uint32) []byte {
		b := make([]byte, 40)
		binary.LittleEndian.PutUint32(b[0:4], 1) // INPUT_KEYBOARD
		binary.LittleEndian.PutUint16(b[8:10], uint16(vk))
		binary.LittleEndian.PutUint16(b[10:12], uint16(sc))
		binary.LittleEndian.PutUint32(b[12:16], flags|uint32(extended))
		return b
	}
	press := buildKey(0)        // KEYEVENTF_KEYUP 未设 → 按下
	release := buildKey(0x0002) // KEYEVENTF_KEYUP
	ret1, _, _ := wSendInput.Call(1, uintptr(unsafe.Pointer(&press[0])), 40)
	ret2, _, _ := wSendInput.Call(1, uintptr(unsafe.Pointer(&release[0])), 40)
	fmt.Printf("[SendInput] press=%d release=%d（发送后前台=%X）\n", ret1, ret2, curForeground())
	time.Sleep(150 * time.Millisecond)
}

// MousePos 返回鼠标的屏幕绝对坐标 (x, y)。
func (Input) MousePos() (int, int) {
	var pt [8]byte
	wGetCursorPos.Call(uintptr(unsafe.Pointer(&pt[0])))
	x := int(int32(binary.LittleEndian.Uint32(pt[0:4])))
	y := int(int32(binary.LittleEndian.Uint32(pt[4:8])))
	return x, y
}

// MoveTo 用 SetCursorPos 把鼠标移到屏幕绝对坐标。
func (Input) MoveTo(x, y int) {
	wSetCursorPos.Call(uintptr(x), uintptr(y))
}

// Click 先把鼠标移到屏幕绝对坐标后，再用 SendInput 发一次左键点击。
//
// 关键: 仁王2 的键盘走 DirectInput 独占, 注入按键无效; 但鼠标菜单用 SendInput 可驱动,
// 故用模拟鼠标点击来选中槽位(点击会触发装备编辑器函数, hook 正常捕获)。
func (Input) Click(x, y int) {
	Input{}.MoveTo(x, y)
	time.Sleep(60 * time.Millisecond)
	mouse := func(flags uint32) []byte {
		b := make([]byte, 40)                          // INPUT 结构 x64=40 字节
		binary.LittleEndian.PutUint32(b[0:4], 0)       // INPUT_MOUSE
		binary.LittleEndian.PutUint32(b[20:24], flags) // dwFlags
		return b
	}
	down := mouse(0x0002) // MOUSEEVENTF_LEFTDOWN
	up := mouse(0x0004)   // MOUSEEVENTF_LEFTUP
	wSendInput.Call(1, uintptr(unsafe.Pointer(&down[0])), 40)
	wSendInput.Call(1, uintptr(unsafe.Pointer(&up[0])), 40)
}

// curForeground 返回当前前台窗口句柄。
func curForeground() uintptr {
	ret, _, _ := wGetForegroundWindow.Call()
	return ret
}

// ================================================================
// 底层私有辅助: 窗口搜索 / 进程信息
// (不足 GameWindow / Input 的公开接口, 仅供包内调用)
// ================================================================

// findNative 遍历顶层窗口返回属于游戏本体的原始窗口句柄 (0=未找到)。
func findNative() uintptr {
	var found uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		if hwnd == 0 {
			return 1
		}
		var buf [256]uint16
		wGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		title := strings.TrimSpace(syscall.UTF16ToString(buf[:]))
		matched := false
		for _, t := range []string{"Nioh2 1.28.08", "Nioh2 1.28", "Nioh2"} {
			if t != "" && strings.Contains(title, t) {
				matched = true
				break
			}
		}
		if !matched {
			return 1
		}
		vis, _, _ := wIsWindowVisible.Call(hwnd)
		if vis == 0 {
			return 1
		}
		pid := wndPID(hwnd)
		if pid == 0 || pid == uint32(os.Getpid()) {
			return 1
		}
		img := processImageName(pid)
		if img != "" && !strings.Contains(strings.ToLower(filepath.Base(img)), "nioh2") {
			return 1
		}
		found = hwnd
		return 0
	})
	wEnumWindows.Call(cb, 0)
	return found
}

// wndPID 返回窗口所属进程 PID。
func wndPID(hwnd uintptr) uint32 {
	var pid uint32
	wGetWindowThreadProcID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}

// processImageName 返回进程可执行文件路径 (获取失败返回空串)。
func processImageName(pid uint32) string {
	h, _, _ := wOpenProcess.Call(W_PROCESS_QUERY_LIMITED, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer wCloseHandle.Call(h)
	var buf [512]uint16
	size := uint32(len(buf))
	wQueryFullProcessImageNameW.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	return syscall.UTF16ToString(buf[:])
}

// ListGameWindows 打印所有属于指定 PID 的窗口 (含子窗口)，用于排查输入窗口。
func ListGameWindows(pid uint32) {
	var collected []string
	collect := func(hwnd uintptr) {
		gw := NativeFrom(hwnd)
		t := gw.Title()
		c := gw.Class()
		collected = append(collected, fmt.Sprintf("  %X  title=%q class=%q", hwnd, t, c))
	}

	enum := func(hwnd uintptr, lParam uintptr) uintptr {
		if hwnd == 0 {
			return 1
		}
		if wndPID(hwnd) == pid {
			collect(hwnd)
		}
		return 1
	}
	cb := syscall.NewCallback(enum)
	wEnumWindows.Call(cb, 0)

	// 再枚举所有顶层窗口的子窗口
	childCB := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		if hwnd == 0 {
			return 1
		}
		if wndPID(hwnd) == pid {
			collect(hwnd)
		}
		return 1
	})
	wEnumChildWindows.Call(uintptr(0), childCB, 0)

	for _, line := range collected {
		fmt.Println(line)
	}
	fmt.Printf("共 %d 个窗口\n", len(collected))
}

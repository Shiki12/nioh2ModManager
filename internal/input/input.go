// Package input 通过 Win32 API 查找游戏窗口并模拟按键
package input

import (
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                      = syscall.NewLazyDLL("user32.dll")
	procEnumWindows             = user32.NewProc("EnumWindows")
	procGetWindowTextW          = user32.NewProc("GetWindowTextW")
	procGetWindowThreadProcID   = user32.NewProc("GetWindowThreadProcessId")
	procIsWindowVisible         = user32.NewProc("IsWindowVisible")
	procShowWindow              = user32.NewProc("ShowWindow")
	procSetForegroundWin        = user32.NewProc("SetForegroundWindow")
	procKeybdEvent              = user32.NewProc("keybd_event")
	kernel32                    = syscall.NewLazyDLL("kernel32.dll")
	procOpenProcess             = kernel32.NewProc("OpenProcess")
	procQueryFullProcessImage   = kernel32.NewProc("QueryFullProcessImageNameW")
	procCloseHandle             = kernel32.NewProc("CloseHandle")
)

const (
	VK_F10                        = 0x79
	SW_RESTORE                    = 9
	PROCESS_QUERY_LIMITED_INFORMATION = 0x1000
)

// BringToForeground 把目标窗口带到前台并恢复（若最小化）。
// 参考 verify/verifyPresson：仅 ShowWindow(SW_RESTORE) + SetForegroundWindow，
// 不做 BringWindowToTop / 模拟 Alt / AttachThreadInput 等重型激活，
// 避免把游戏进程的 "GDI+ Window" 辅助窗口激活到 Alt-Tab/任务栏。
func BringToForeground(hwnd uintptr) {
	procShowWindow.Call(hwnd, SW_RESTORE)
	procSetForegroundWin.Call(hwnd)
}

// SendKey 向当前前台窗口发送一次按键（确保游戏在前台后 keybd_event 才生效）
func SendKey(vk uintptr) {
	procKeybdEvent.Call(vk, 0, 0, 0)
	time.Sleep(150 * time.Millisecond)
	procKeybdEvent.Call(vk, 0, 2, 0)
}

// SendF10 发送 F10（纯重载）
func SendF10() { SendKey(VK_F10) }

// FindGameWindow 遍历所有顶层窗口，返回标题包含任一关键词、且确属游戏本体窗口的句柄。
// 为避免误匹配（资源管理器/浏览器标题含 "Nioh2"、隐藏的残留窗口、管理器自身窗口等），
// 额外要求：窗口可见、所属进程不是本程序、且所属进程可执行文件名为 nioh2.exe。
// 返回 0 表示未找到。
func FindGameWindow(titles []string) uintptr {
	var found uintptr
	cb := syscall.NewCallback(func(hwnd uintptr, lParam uintptr) uintptr {
		if hwnd == 0 {
			return 1
		}
		var buf [256]uint16
		procGetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
		title := strings.TrimSpace(syscall.UTF16ToString(buf[:]))
		if title == "" {
			return 1
		}
		matched := false
		for _, t := range titles {
			if t != "" && strings.Contains(title, t) {
				matched = true
				break
			}
		}
		if !matched {
			return 1
		}
		if !isGameWindow(hwnd) {
			return 1
		}
		found = hwnd
		return 0
	})
	procEnumWindows.Call(cb, 0)
	return found
}

// isGameWindow 校验窗口确实是游戏本体窗口：可见 + 非本程序进程 + 所属进程名为 nioh2。
// 进程名拿不到（如权限不足）时退而只要求窗口可见且非本程序，避免误判"游戏已停止"。
func isGameWindow(hwnd uintptr) bool {
	visible, _, _ := procIsWindowVisible.Call(hwnd)
	if visible == 0 {
		return false
	}
	pid := windowPID(hwnd)
	if pid == 0 || pid == uint32(os.Getpid()) {
		return false
	}
	img := processImage(pid)
	if img == "" {
		return true
	}
	return strings.Contains(strings.ToLower(filepath.Base(img)), "nioh2")
}

// windowPID 返回窗口所属进程 PID，失败返回 0
func windowPID(hwnd uintptr) uint32 {
	var pid uint32
	procGetWindowThreadProcID.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
	return pid
}

// processImage 返回进程可执行文件完整路径，无法获取时返回空串
func processImage(pid uint32) string {
	h, _, _ := procOpenProcess.Call(PROCESS_QUERY_LIMITED_INFORMATION, 0, uintptr(pid))
	if h == 0 {
		return ""
	}
	defer procCloseHandle.Call(h)
	var buf [512]uint16
	size := uint32(len(buf))
	procQueryFullProcessImage.Call(h, 0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size)))
	return syscall.UTF16ToString(buf[:])
}

// FindGamePID 返回游戏窗口所属进程 PID，未找到返回 0
func FindGamePID() uint32 {
	hwnd := FindGameWindow([]string{"Nioh2 1.28.08", "Nioh2 1.28", "Nioh2"})
	if hwnd == 0 {
		return 0
	}
	return windowPID(hwnd)
}

// RefreshMods 刷新游戏内 Mod（纯重载，不切换 Mod 引擎开关）：
// 找到游戏窗口 → 带到前台并等待其恢复渲染 → 发送 F10 重新加载 Mod 文件。
// 注意：F2 是引擎的"切换"键，会翻转 $mods 开关，故刷新只发 F10 以避免误关。
// 返回是否找到游戏窗口。
func RefreshMods() bool {
	hwnd := FindGameWindow([]string{"Nioh2 1.28.08", "Nioh2 1.28", "Nioh2"})
	if hwnd == 0 {
		return false
	}
	BringToForeground(hwnd)
	time.Sleep(900 * time.Millisecond)
	SendF10()
	time.Sleep(600 * time.Millisecond)
	return true
}
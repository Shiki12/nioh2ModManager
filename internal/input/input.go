// Package input 通过 Win32 API 查找游戏窗口并模拟按键
package input

import (
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	user32                        = syscall.NewLazyDLL("user32.dll")
	procEnumWindows               = user32.NewProc("EnumWindows")
	procGetWindowTextW            = user32.NewProc("GetWindowTextW")
	procShowWindow                = user32.NewProc("ShowWindow")
	procBringWindowToTop          = user32.NewProc("BringWindowToTop")
	procSetForegroundWin          = user32.NewProc("SetForegroundWindow")
	procGetForegroundWindow       = user32.NewProc("GetForegroundWindow")
	procGetWindowThreadProcessId  = user32.NewProc("GetWindowThreadProcessId")
	procAttachThreadInput         = user32.NewProc("AttachThreadInput")
	procKeybdEvent                = user32.NewProc("keybd_event")
)

const (
	VK_F10     = 0x79
	VK_MENU    = 0x12
	SW_RESTORE = 9
)

// BringToForeground 尽可能把目标窗口带到前台：
// 恢复最小化 → 置顶 → 模拟 Alt 解锁前台限制 → AttachThreadInput 强设前台。
// 全屏独占模式下游戏本就是前台，调用基本无副作用。
func BringToForeground(hwnd uintptr) {
	procShowWindow.Call(hwnd, SW_RESTORE)
	procBringWindowToTop.Call(hwnd)
	procKeybdEvent.Call(VK_MENU, 0, 0, 0)
	procKeybdEvent.Call(VK_MENU, 0, 2, 0)
	fore, _, _ := procGetForegroundWindow.Call()
	_, foreTid, _ := procGetWindowThreadProcessId.Call(fore, 0)
	_, targetTid, _ := procGetWindowThreadProcessId.Call(hwnd, 0)
	if fore != hwnd && foreTid != 0 && targetTid != 0 {
		procAttachThreadInput.Call(foreTid, targetTid, 1)
		procSetForegroundWin.Call(hwnd)
		procAttachThreadInput.Call(foreTid, targetTid, 0)
	}
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

// FindGameWindow 遍历所有顶层窗口，返回标题包含任一关键词的窗口句柄
// 返回 0 表示未找到
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
		for _, t := range titles {
			if t != "" && strings.Contains(title, t) {
				found = hwnd
				return 0
			}
		}
		return 1
	})
	procEnumWindows.Call(cb, 0)
	return found
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
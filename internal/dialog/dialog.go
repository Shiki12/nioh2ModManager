// Package dialog 提供 Windows 现代文件/文件夹选择对话框
// 基于 github.com/ncruces/zenity（Windows 端使用 IFileOpenDialog / COMDLG32）
package dialog

import (
	"errors"
	"fmt"
	"strings"
	"syscall"
	"unsafe"

	"github.com/ncruces/zenity"
)

// MainWindowTitle 主窗口标题，用于 FindWindow 兜底查找主窗口句柄
var MainWindowTitle = "nioh2mod-js"

var (
	user32                  = syscall.NewLazyDLL("user32.dll")
	procGetActiveWindow     = user32.NewProc("GetActiveWindow")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procFindWindow          = user32.NewProc("FindWindowW")
)

// mainWindowHandle 获取主窗口句柄作为对话框 owner。
// 注意：Wails 的 Go 回调运行在工作线程，GetActiveWindow 是线程相关的会返回 0，
// 必须先取系统级的 GetForegroundWindow，再回退到 GetActiveWindow 与按标题 FindWindow。
func mainWindowHandle() uintptr {
	if hwnd, _, _ := procGetForegroundWindow.Call(); hwnd != 0 {
		return hwnd
	}
	if hwnd, _, _ := procGetActiveWindow.Call(); hwnd != 0 {
		return hwnd
	}
	if titlePtr, err := syscall.UTF16PtrFromString(MainWindowTitle); err == nil {
		if hwnd, _, _ := procFindWindow.Call(0, uintptr(unsafe.Pointer(titlePtr))); hwnd != 0 {
			return hwnd
		}
	}
	return 0
}

// parseFilters 把 "描述\x00模式\x00描述\x00模式\x00" 解析成 zenity.FileFilter 列表
func parseFilters(filter string) []zenity.FileFilter {
	parts := strings.Split(filter, "\x00")
	var filters []zenity.FileFilter
	for i := 0; i+1 < len(parts); i += 2 {
		if parts[i] == "" && parts[i+1] == "" {
			continue
		}
		filters = append(filters, zenity.FileFilter{
			Name:     parts[i],
			Patterns: strings.Split(parts[i+1], ";"),
		})
	}
	return filters
}

// baseOptions 构造通用选项：标题、以本应用主窗口为 owner（模态居中于其上）
func baseOptions(title string) []zenity.Option {
	return []zenity.Option{
		zenity.Title(title),
		zenity.Attach(mainWindowHandle()),
		zenity.Modal(),
	}
}

// SelectDirectory 打开 Windows 文件夹选择对话框（以主窗口为 owner，居中显示在其上方）
func SelectDirectory(title string) (string, error) {
	opts := baseOptions(title)
	opts = append(opts, zenity.Directory())
	path, err := zenity.SelectFile(opts...)
	if errors.Is(err, zenity.ErrCanceled) {
		return "", fmt.Errorf("用户取消了选择")
	}
	return path, err
}

// SelectFile 打开 Windows 文件选择对话框
// filter 中 \x00 作为分隔符，例如:
//
//	"图片(*.png;*.jpg)\x00*.png;*.jpg\x00所有文件(*.*)\x00*.*\x00"
func SelectFile(title, filter string) (string, error) {
	opts := baseOptions(title)
	if filters := parseFilters(filter); len(filters) > 0 {
		opts = append(opts, zenity.FileFilters(filters))
	}
	path, err := zenity.SelectFile(opts...)
	if errors.Is(err, zenity.ErrCanceled) {
		return "", fmt.Errorf("用户取消了选择")
	}
	return path, err
}

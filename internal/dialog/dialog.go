// Package dialog 提供 Windows 原生文件夹选择对话框
package dialog

import (
	"fmt"
	"strings"
	"syscall"
	"unsafe"
)

var (
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	ole32                   = syscall.NewLazyDLL("ole32.dll")
	comdlg32                = syscall.NewLazyDLL("comdlg32.dll")
	user32                  = syscall.NewLazyDLL("user32.dll")
	procSHBrowseForFolder   = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDList = shell32.NewProc("SHGetPathFromIDListW")
	procGetOpenFileName     = comdlg32.NewProc("GetOpenFileNameW")
	procCoInitializeEx      = ole32.NewProc("CoInitializeEx")
	procCoUninitialize      = ole32.NewProc("CoUninitialize")
	procGetActiveWindow     = user32.NewProc("GetActiveWindow")
	procGetForegroundWindow = user32.NewProc("GetForegroundWindow")
	procFindWindow          = user32.NewProc("FindWindowW")
)

// MainWindowTitle 主窗口标题，用于 FindWindow 兜底查找主窗口句柄
var MainWindowTitle = "nioh2mod-js"

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

const (
	bifReturnOnlyFSDirs     = 0x00000001
	bifNewDialogStyle       = 0x00000040
	coInitApartmentThreaded = 0x2
)

// SelectDirectory 打开 Windows 文件夹选择对话框（以主窗口为 owner，居中显示在其上方）
func SelectDirectory(title string) (string, error) {
	procCoInitializeEx.Call(0, coInitApartmentThreaded)
	defer procCoUninitialize.Call()

	hwnd := mainWindowHandle()

	titlePtr, _ := syscall.UTF16PtrFromString(title)
	bi := struct {
		hwndOwner      uintptr
		pidlRoot       uintptr
		pszDisplayName *uint16
		lpszTitle      *uint16
		ulFlags        uint32
		lpfn           uintptr
		lParam         uintptr
		iImage         int32
	}{
		hwndOwner: hwnd,
		lpszTitle: titlePtr,
		ulFlags:   bifReturnOnlyFSDirs | bifNewDialogStyle,
	}

	pidl, _, _ := procSHBrowseForFolder.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return "", fmt.Errorf("用户取消了选择")
	}

	var path [260]uint16
	ret, _, _ := procSHGetPathFromIDList.Call(pidl, uintptr(unsafe.Pointer(&path[0])))
	if ret == 0 {
		return "", fmt.Errorf("获取路径失败")
	}
	return syscall.UTF16ToString(path[:]), nil
}

// SelectFile 打开 Windows 文件选择对话框
// filter 中 \x00 作为分隔符，例如:
//
//	"图片(*.png;*.jpg)\x00*.png;*.jpg\x00所有文件(*.*)\x00*.*\x00"
func SelectFile(title, filter string) (string, error) {
	procCoInitializeEx.Call(0, coInitApartmentThreaded)
	defer procCoUninitialize.Call()

	titlePtr, _ := syscall.UTF16PtrFromString(title)
	hwnd := mainWindowHandle()

	parts := strings.Split(filter, "\x00")
	filterUTF16 := make([]uint16, 0, 256)
	for _, p := range parts {
		for _, r := range p {
			filterUTF16 = append(filterUTF16, uint16(r))
		}
		filterUTF16 = append(filterUTF16, 0)
	}
	filterUTF16 = append(filterUTF16, 0)

	const (
		ofnFileMustExist = 0x00001000
		ofnHideReadOnly  = 0x00000004
		ofnPathMustExist = 0x00000800
	)

	fields := []struct {
		off  uintptr
		sz   uintptr
		val  uintptr
	}{
		{0, 4, 152},                                    // lStructSize
		{16, 8, hwnd},                                  // hwndOwner
		{24, 8, uintptr(unsafe.Pointer(&filterUTF16[0]))}, // lpstrFilter
		{44, 4, 1},                                     // nFilterIndex
		{56, 4, 260},                                   // nMaxFile
		{88, 8, uintptr(unsafe.Pointer(titlePtr))},     // lpstrTitle
		{96, 4, ofnFileMustExist | ofnHideReadOnly | ofnPathMustExist}, // Flags
	}

	fileBuf := make([]uint16, 260)
	fields = append(fields,
		struct{off uintptr; sz uintptr; val uintptr}{48, 8, uintptr(unsafe.Pointer(&fileBuf[0]))}, // lpstrFile
	)

	var ofnBuf [152]byte
	ofn := unsafe.Pointer(&ofnBuf[0])
	for _, f := range fields {
		switch f.sz {
		case 4:
			*(*uint32)(unsafe.Add(ofn, f.off)) = uint32(f.val)
		case 8:
			*(*uintptr)(unsafe.Add(ofn, f.off)) = f.val
		}
	}

	ret, _, _ := procGetOpenFileName.Call(uintptr(ofn))
	if ret == 0 {
		return "", fmt.Errorf("用户取消了选择")
	}

	return syscall.UTF16ToString(fileBuf[:]), nil
}

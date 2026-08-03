// Package steam 通过 Steam 注册表和 ACF 文件搜索游戏安装目录
package steam

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"unsafe"
)

var (
	advapi32            = syscall.NewLazyDLL("advapi32.dll")
	procRegOpenKeyEx    = advapi32.NewProc("RegOpenKeyExW")
	procRegQueryValueEx = advapi32.NewProc("RegQueryValueExW")
	procRegCloseKey     = advapi32.NewProc("RegCloseKey")
)

const (
	HKEY_LOCAL_MACHINE = 0x80000002
	KEY_READ           = 0x20019
)

func readRegistryString(keyPath, valueName string) (string, error) {
	var hKey uintptr
	ret, _, _ := procRegOpenKeyEx.Call(
		HKEY_LOCAL_MACHINE,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(keyPath))),
		0,
		KEY_READ,
		uintptr(unsafe.Pointer(&hKey)),
	)
	if ret != 0 {
		return "", fmt.Errorf("RegOpenKeyEx 失败，错误码: %d", ret)
	}
	defer procRegCloseKey.Call(hKey)

	var buf [4096]uint16
	var bufLen uint32 = uint32(len(buf) * 2)
	ret, _, _ = procRegQueryValueEx.Call(
		hKey,
		uintptr(unsafe.Pointer(syscall.StringToUTF16Ptr(valueName))),
		0,
		0,
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&bufLen)),
	)
	if ret != 0 {
		return "", fmt.Errorf("RegQueryValueEx 失败，错误码: %d", ret)
	}
	return syscall.UTF16ToString(buf[:]), nil
}

// InstallPath 从注册表获取 Steam 安装目录
func InstallPath() (string, error) {
	paths := []string{
		`SOFTWARE\WOW6432Node\Valve\Steam`,
		`SOFTWARE\Valve\Steam`,
	}
	for _, p := range paths {
		val, err := readRegistryString(p, "InstallPath")
		if err == nil && val != "" {
			return val, nil
		}
	}
	return "", fmt.Errorf("未在注册表中找到 Steam 安装路径")
}

// LibraryPaths 从 libraryfolders.vdf 提取所有库根路径
func LibraryPaths(steamRoot string) ([]string, error) {
	vdfPath := filepath.Join(steamRoot, "steamapps", "libraryfolders.vdf")
	data, err := os.ReadFile(vdfPath)
	if err != nil {
		return nil, err
	}
	re := regexp.MustCompile(`"path"\s*"([^"]+)"`)
	matches := re.FindAllStringSubmatch(string(data), -1)
	var paths []string
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) >= 2 {
			path := match[1]
			path = strings.ReplaceAll(path, `\\`, `\`)
			if !seen[path] && path != "" {
				seen[path] = true
				paths = append(paths, path)
			}
		}
	}
	if len(paths) == 0 {
		return nil, fmt.Errorf("未在 libraryfolders.vdf 中找到任何库路径")
	}
	return paths, nil
}

// GameRoot 通过 AppID 查找游戏安装目录
// appID: Steam AppID（仁王2 = "1325200"）
// fallbackName: ACF 找不到时的降级目录名（如 "Nioh2"）
func GameRoot(appID, fallbackName string) (string, error) {
	steamRoot, err := InstallPath()
	if err != nil {
		return "", err
	}
	libPaths, err := LibraryPaths(steamRoot)
	if err != nil {
		return "", err
	}
	for _, libPath := range libPaths {
		acfPath := filepath.Join(libPath, "steamapps", "appmanifest_"+appID+".acf")
		if data, err := os.ReadFile(acfPath); err == nil {
			re := regexp.MustCompile(`"installdir"\s*"([^"]+)"`)
			matches := re.FindStringSubmatch(string(data))
			if len(matches) >= 2 {
				gameRoot := filepath.Join(libPath, "steamapps", "common", matches[1])
				if _, err := os.Stat(gameRoot); err == nil {
					return gameRoot, nil
				}
			}
		}
		// fallback
		fallback := filepath.Join(libPath, "steamapps", "common", fallbackName)
		if _, err := os.Stat(fallback); err == nil {
			return fallback, nil
		}
	}
	return "", fmt.Errorf("未找到游戏安装目录")
}

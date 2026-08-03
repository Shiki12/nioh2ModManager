// Package mods 管理 Mod 符号链接（启用/禁用）
package mods

import (
	"fmt"
	"os"
	"path/filepath"
)

// Enable 创建符号链接启用 Mod
func Enable(modDir, linkPath string) error {
	if _, err := os.Stat(modDir); os.IsNotExist(err) {
		return fmt.Errorf("目标 Mod 文件夹不存在: %s", modDir)
	}
	if info, err := os.Lstat(linkPath); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			os.Remove(linkPath)
		} else {
			return fmt.Errorf("路径已存在且不是符号链接: %s", linkPath)
		}
	}
	return os.Symlink(modDir, linkPath)
}

// EnableComposite 启用组合 Mod：在游戏 Mod 目录建立父目录（真实目录），
// 将公共条目（shared，如 meshes/textures）与启用的子 Mod（enabledSubs）以符号链接放入其中。
// 子 Mod 的 ini 相对引用（..\meshes\ 等）在父目录结构下保持成立。
func EnableComposite(parentDir, linkDir string, shared, enabledSubs []string) error {
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		return fmt.Errorf("目标 Mod 文件夹不存在: %s", parentDir)
	}
	if _, err := os.Lstat(linkDir); err == nil {
		if err := os.RemoveAll(linkDir); err != nil {
			return err
		}
	}
	if err := os.MkdirAll(linkDir, 0755); err != nil {
		return err
	}
	for _, name := range shared {
		if err := linkInto(parentDir, linkDir, name); err != nil {
			return err
		}
	}
	for _, name := range enabledSubs {
		if err := linkInto(parentDir, linkDir, name); err != nil {
			return err
		}
	}
	return nil
}

// EnableSubMod 为组合 Mod 的单个子 Mod 创建链接（父目录需已存在）
func EnableSubMod(parentDir, linkDir, subName string) error {
	if _, err := os.Stat(linkDir); err != nil {
		return fmt.Errorf("组合 Mod 父目录未启用: %s", linkDir)
	}
	return linkInto(parentDir, linkDir, subName)
}

// DisableSubMod 移除组合 Mod 单个子 Mod 的链接（不存在视为已禁用）
func DisableSubMod(linkDir, subName string) error {
	dst := filepath.Join(linkDir, subName)
	info, err := os.Lstat(dst)
	if err != nil {
		return nil
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s 不是一个符号链接", dst)
	}
	return os.Remove(dst)
}

// DisableComposite 禁用组合 Mod：整体移除父目录（含其中所有子 Mod 链接）
func DisableComposite(linkDir string) error {
	if _, err := os.Lstat(linkDir); err != nil {
		return nil
	}
	return os.RemoveAll(linkDir)
}

// CompositeEnabled 判断组合 Mod 父目录（含公共链接）是否已启用
func CompositeEnabled(linkDir string) bool {
	info, err := os.Lstat(linkDir)
	return err == nil && info.IsDir()
}

// linkInto 在 dstDir 下为 srcDir 的 name 子项创建符号链接；源不存在则跳过
func linkInto(srcDir, dstDir, name string) error {
	src := filepath.Join(srcDir, name)
	if _, err := os.Lstat(src); err != nil {
		return nil
	}
	dst := filepath.Join(dstDir, name)
	if info, err := os.Lstat(dst); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			os.Remove(dst)
		} else if info.IsDir() {
			return fmt.Errorf("路径已存在且不是符号链接: %s", dst)
		}
	}
	return os.Symlink(src, dst)
}

// Disable 删除符号链接禁用 Mod
func Disable(linkPath string) error {
	info, err := os.Lstat(linkPath)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink == 0 {
		return fmt.Errorf("%s 不是一个符号链接", linkPath)
	}
	return os.Remove(linkPath)
}

// IsEnabled 检查 Mod 符号链接是否存在
func IsEnabled(linkPath string) bool {
	info, err := os.Lstat(linkPath)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// IsActive 判断 Mod 是否已启用：普通 Mod 为符号链接，组合/HDR Mod 为真实目录。
// 扫描对账时用此判断，避免组合 Mod 因非符号链接被误判为禁用。
func IsActive(linkPath string) bool {
	info, err := os.Lstat(linkPath)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0 || info.IsDir()
}

// Info Mod 信息
type Info struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Cover    string `json:"cover"`
	Enabled  bool   `json:"enabled"`
}

// Scan 扫描 Mod 托管目录，返回磁盘上实际存在的 Mod 列表（Name/Path/Enabled 为磁盘真实状态）。
// 仅用于与数据文件对账的“发现”步骤；Mod 的昵称/封面/资源占用等用户数据以数据文件为准。
func Scan(modsRepo, gameModsDir string) ([]Info, error) {
	entries, err := os.ReadDir(modsRepo)
	if err != nil {
		return nil, fmt.Errorf("读取 Mod 目录失败: %v", err)
	}
	var list []Info
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		dirPath := filepath.Join(modsRepo, name)
		list = append(list, Info{
			Name:    name,
			Path:    dirPath,
			Enabled: IsActive(filepath.Join(gameModsDir, name)),
		})
	}
	return list, nil
}

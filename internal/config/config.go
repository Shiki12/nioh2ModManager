// Package config 持久化配置 & Mod 数据（JSON 文件）
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"

	"nioh2mod-js/internal/steam"
)

// decodeUTF8 兼容旧版 GBK 编码的数据文件：
// 合法 UTF-8 原样返回；否则（含无效 UTF-8 字节）按 GBK 转 UTF-8 后返回。
func decodeUTF8(data []byte) []byte {
	if len(data) == 0 || utf8.Valid(data) {
		return data
	}
	if dec, err := simplifiedchinese.GBK.NewDecoder().Bytes(data); err == nil {
		return dec
	}
	return data
}

// ============================================================
// 应用配置 — modman_config.json
// ============================================================

// App 应用配置
type App struct {
	GameRoot  string `json:"gameRoot"`
	ModsRepo  string `json:"modsRepo"`
	UpdateURL string `json:"updateUrl,omitempty"` // 检查更新接口地址（版本清单 URL），留空表示未配置
}

// appDataDir 便携式数据目录：数据文件与程序同目录（单目录自包含，拷贝即备份）。
// 更新时只需替换程序文件、保留数据文件即可无损升级。
func appDataDir() string {
	exe, err := os.Executable()
	if err != nil {
		return "."
	}
	return filepath.Dir(exe)
}

func appPath() string {
	return filepath.Join(appDataDir(), "modman_config.json")
}

// LoadApp 读取配置，首次启动自动搜索游戏目录
func LoadApp() *App {
	cfg := &App{}
	data, err := os.ReadFile(appPath())
	if err != nil {
		if root, err := steam.GameRoot("1325200", "Nioh2"); err == nil {
			cfg.GameRoot = root
		}
		cfg.Save()
		return cfg
	}
	json.Unmarshal(decodeUTF8(data), cfg)
	return cfg
}

func (c *App) Save() error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(appPath(), data, 0644)
}

// GameModsDir 返回游戏 Mods 目录
func (c *App) GameModsDir() string {
	if c.GameRoot == "" {
		return ""
	}
	return filepath.Join(c.GameRoot, "Mods")
}

// ============================================================
// Mod 数据 — modman_data.json
// ============================================================

// LogEntry 操作日志条目
type LogEntry struct {
	Time    string `json:"time"`
	Message string `json:"message"`
}

// LogData 日志数据文件
type LogData struct {
	Logs []LogEntry `json:"logs"`
}

// ModInfo Mod 持久化结构
// Parts 为 部位 -> 占用资源列表：服装部位（头/胸甲/臂甲/膝甲/腿甲）通常每部位一个值，
// 武器部位可占用多个武器（数组）。旧版数据文件 parts 值为单个字符串，读取时自动兼容。
type ModInfo struct {
	Name      string              `json:"name"`
	Nickname  string              `json:"nickname"`
	Path      string              `json:"path"`
	Cover     string              `json:"cover"`
	Preview   []string            `json:"previews,omitempty"` // 多张效果图（相对 Mod 目录路径，首张即封面）
	Enabled   bool                `json:"enabled"`
	Installed bool                `json:"installed,omitempty"`
	Parts     map[string][]string `json:"parts,omitempty"`
	Missing   bool                `json:"missing,omitempty"`
	Category  string              `json:"category,omitempty"` // 分类：armor=服装 / weapon=武器（空=服装）
	SubMods   []SubModInfo        `json:"submods,omitempty"`  // 组合 Mod（如 HDR 合集）的子 Mod，父级本身不占用
}

// SubModInfo 组合 Mod 内的子 Mod：各自独立的占用资源与启用状态
type SubModInfo struct {
	Name    string              `json:"name"`
	Parts   map[string][]string `json:"parts,omitempty"`
	Cover   string              `json:"cover,omitempty"`    // 子 Mod 封面图（相对父 Mod 目录路径）
	Preview []string            `json:"previews,omitempty"` // 子 Mod 多张效果图（相对父 Mod 目录路径）
	Enabled bool                `json:"enabled"`
}

// decodePartRaw 解析占用资源原始字段：兼容旧版单值（"胸甲":"上古之衣-上衣"）
// 与新版多值（"武器":["刀一","刀二"]），统一输出为 部位 -> 字符串列表。
func decodePartRaw(raw map[string]json.RawMessage) map[string][]string {
	if len(raw) == 0 {
		return nil
	}
	out := make(map[string][]string, len(raw))
	for k, r := range raw {
		var one string
		if err := json.Unmarshal(r, &one); err == nil {
			out[k] = []string{one}
			continue
		}
		var many []string
		if err := json.Unmarshal(r, &many); err == nil {
			out[k] = many
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// modRaw / subRaw 用于兼容旧版单值 parts 的自定义反序列化
type modRaw struct {
	Name      string                     `json:"name"`
	Nickname  string                     `json:"nickname"`
	Path      string                     `json:"path"`
	Cover     string                     `json:"cover"`
	Preview   []string                   `json:"previews,omitempty"`
	Enabled   bool                       `json:"enabled"`
	Installed bool                       `json:"installed,omitempty"`
	Parts     map[string]json.RawMessage `json:"parts"`
	Missing   bool                       `json:"missing,omitempty"`
	Category  string                     `json:"category,omitempty"`
	SubMods   []subRaw                   `json:"submods,omitempty"`
}

type subRaw struct {
	Name    string                     `json:"name"`
	Parts   map[string]json.RawMessage `json:"parts"`
	Cover   string                     `json:"cover,omitempty"`
	Preview []string                   `json:"previews,omitempty"`
	Enabled bool                       `json:"enabled"`
}

// UnmarshalJSON 兼容旧版单值 parts（字符串）与新版多值 parts（数组）
func (m *ModInfo) UnmarshalJSON(data []byte) error {
	var raw modRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*m = ModInfo{
		Name:      raw.Name,
		Nickname:  raw.Nickname,
		Path:      raw.Path,
		Cover:     raw.Cover,
		Preview:   raw.Preview,
		Enabled:   raw.Enabled,
		Installed: raw.Installed,
		Parts:     decodePartRaw(raw.Parts),
		Missing:   raw.Missing,
		Category:  raw.Category,
	}
	m.SubMods = make([]SubModInfo, 0, len(raw.SubMods))
	for _, sr := range raw.SubMods {
		m.SubMods = append(m.SubMods, SubModInfo{Name: sr.Name, Parts: decodePartRaw(sr.Parts), Cover: sr.Cover, Preview: sr.Preview, Enabled: sr.Enabled})
	}
	return nil
}

// UnmarshalJSON 兼容旧版单值 parts（字符串）与新版多值 parts（数组）
func (s *SubModInfo) UnmarshalJSON(data []byte) error {
	var raw subRaw
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	*s = SubModInfo{Name: raw.Name, Parts: decodePartRaw(raw.Parts), Cover: raw.Cover, Preview: raw.Preview, Enabled: raw.Enabled}
	return nil
}

// ModData Mod 数据文件
type ModData struct {
	Mods []ModInfo `json:"mods"`
}

func modDataPath() string {
	return filepath.Join(appDataDir(), "modman_data.json")
}

func LoadModData() *ModData {
	d := &ModData{}
	data, err := os.ReadFile(modDataPath())
	if err != nil {
		return d
	}
	if !utf8.Valid(data) {
		// 旧版 GBK 编码数据：转 UTF-8 后立即重写修复
		data = decodeUTF8(data)
		json.Unmarshal(data, d)
		d.Save()
		return d
	}
	json.Unmarshal(data, d)
	return d
}

func (d *ModData) Save() error {
	data, err := json.MarshalIndent(d, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(modDataPath(), data, 0644)
}

func (d *ModData) findIndex(name string) int {
	for i := range d.Mods {
		if d.Mods[i].Name == name {
			return i
		}
	}
	return -1
}

func (d *ModData) SetEnabled(name string, enabled bool) {
	if i := d.findIndex(name); i >= 0 {
		d.Mods[i].Enabled = enabled
		d.Save()
	}
}

func (d *ModData) SetCover(name, cover string) {
	if i := d.findIndex(name); i >= 0 {
		d.Mods[i].Cover = cover
		d.Save()
	}
}

// SetPreviews 设置 Mod 的多张效果图（相对 Mod 目录路径）。列表为空时清空。
func (d *ModData) SetPreviews(name string, previews []string) {
	if i := d.findIndex(name); i >= 0 {
		if len(previews) == 0 {
			d.Mods[i].Preview = nil
		} else {
			d.Mods[i].Preview = previews
		}
		d.Save()
	}
}

// SetSubModPreviews 设置组合 Mod 内某个子 Mod 的多张效果图。
func (d *ModData) SetSubModPreviews(name, subName string, previews []string) {
	i := d.findIndex(name)
	if i < 0 {
		return
	}
	for j := range d.Mods[i].SubMods {
		if d.Mods[i].SubMods[j].Name == subName {
			if len(previews) == 0 {
				d.Mods[i].SubMods[j].Preview = nil
			} else {
				d.Mods[i].SubMods[j].Preview = previews
			}
			d.Save()
			return
		}
	}
}

// SetSubModCover 设置组合 Mod 内某个子 Mod 的封面图（相对父 Mod 目录路径）。
func (d *ModData) SetSubModCover(name, subName, cover string) {
	i := d.findIndex(name)
	if i < 0 {
		return
	}
	for j := range d.Mods[i].SubMods {
		if d.Mods[i].SubMods[j].Name == subName {
			d.Mods[i].SubMods[j].Cover = cover
			d.Save()
			return
		}
	}
}

func (d *ModData) SetNickname(name, nickname string) {
	if i := d.findIndex(name); i >= 0 {
		d.Mods[i].Nickname = nickname
		d.Save()
	}
}

// DeriveCategory 根据占用的资源推导 Mod 分类：
// mixed=服装+武器 / weapon=仅武器 / armor=仅服装（parts 为空时视为 armor）
func DeriveCategory(parts map[string][]string) string {
	hasArmor, hasWeapon := false, false
	for k := range parts {
		if k == "武器" {
			hasWeapon = true
		} else {
			hasArmor = true
		}
	}
	switch {
	case hasWeapon && hasArmor:
		return "mixed"
	case hasWeapon:
		return "weapon"
	default:
		return "armor"
	}
}

func (d *ModData) SetCategory(name, category string) {
	if i := d.findIndex(name); i >= 0 {
		switch category {
		case "weapon", "mixed":
		default:
			category = "armor"
		}
		d.Mods[i].Category = category
		d.Save()
	}
}

func (d *ModData) SetParts(name string, parts map[string][]string) {
	if i := d.findIndex(name); i >= 0 {
		if len(parts) == 0 {
			d.Mods[i].Parts = nil
		} else {
			d.Mods[i].Parts = parts
		}
		d.Mods[i].Category = DeriveCategory(d.Mods[i].Parts)
		d.Save()
	}
}

// SetSubMods 登记组合 Mod 的子 Mod 列表（父级本身不占用）。
// 父级 Category 由子 Mod 占用并集推导，供筛选/冲突检测展示。
func (d *ModData) SetSubMods(name string, submods []SubModInfo) {
	i := d.findIndex(name)
	if i < 0 {
		return
	}
	d.Mods[i].SubMods = submods
	d.Mods[i].Parts = unionSubModParts(submods)
	d.Mods[i].Category = DeriveCategory(d.Mods[i].Parts)
	d.Save()
}

// unionSubModParts 汇总子 Mod 占用为父级并集：同部位多个子 Mod 占用不同值时全部保留。
func unionSubModParts(submods []SubModInfo) map[string][]string {
	union := map[string][]string{}
	for _, sm := range submods {
		for k, vals := range sm.Parts {
			union[k] = append(union[k], vals...)
		}
	}
	if len(union) == 0 {
		return nil
	}
	return union
}

// SetSubModEnabled 设置组合 Mod 内某个子 Mod 的启用状态
func (d *ModData) SetSubModEnabled(name, subName string, enabled bool) {
	i := d.findIndex(name)
	if i < 0 {
		return
	}
	for j := range d.Mods[i].SubMods {
		if d.Mods[i].SubMods[j].Name == subName {
			d.Mods[i].SubMods[j].Enabled = enabled
			d.Save()
			return
		}
	}
}

// DisableAllSubMods 关闭组合 Mod 内的全部子 Mod。
// 父组合包整体关闭时调用：磁盘上子 Mod 链接已一并移除，数据里的启用状态也必须同步置为关闭，
// 否则会出现"父级已关闭但子 Mod 仍显示已启用"的不一致。
func (d *ModData) DisableAllSubMods(name string) {
	i := d.findIndex(name)
	if i < 0 {
		return
	}
	changed := false
	for j := range d.Mods[i].SubMods {
		if d.Mods[i].SubMods[j].Enabled {
			d.Mods[i].SubMods[j].Enabled = false
			changed = true
		}
	}
	if changed {
		d.Save()
	}
}

// SetSubModParts 设置组合 Mod 内某个子 Mod 的占用资源（自动解析失败时供人工填写），
// 并重算父级并集占用与分类。
func (d *ModData) SetSubModParts(name, subName string, parts map[string][]string) {
	i := d.findIndex(name)
	if i < 0 {
		return
	}
	for j := range d.Mods[i].SubMods {
		if d.Mods[i].SubMods[j].Name == subName {
			if len(parts) == 0 {
				d.Mods[i].SubMods[j].Parts = nil
			} else {
				d.Mods[i].SubMods[j].Parts = parts
			}
			d.Mods[i].Parts = unionSubModParts(d.Mods[i].SubMods)
			d.Mods[i].Category = DeriveCategory(d.Mods[i].Parts)
			d.Save()
			return
		}
	}
}

func (d *ModData) Remove(name string) {
	if i := d.findIndex(name); i >= 0 {
		d.Mods = append(d.Mods[:i], d.Mods[i+1:]...)
		d.Save()
	}
}

func (d *ModData) Upsert(name, path string) {
	if i := d.findIndex(name); i >= 0 {
		d.Mods[i].Path = path
		d.Save()
		return
	}
	d.Mods = append(d.Mods, ModInfo{Name: name, Path: path})
	d.Save()
}

func (d *ModData) Find(name string) *ModInfo {
	if i := d.findIndex(name); i >= 0 {
		return &d.Mods[i]
	}
	return nil
}

// Sync 以数据文件为准增量同步 Mod 列表：
// 用磁盘扫描结果校正数据（登记目录、标记缺失、同步启用状态），
// 已存在的记录保持原顺序与用户编辑数据，新发现的 Mod 追加到数据末尾。
// scanned 来自 mods.Scan 的目录发现结果（Name/Path/Enabled 为磁盘真实状态）。
func (d *ModData) Sync(scanned []ModInfo) {
	old := make(map[string]ModInfo, len(d.Mods))
	for _, m := range d.Mods {
		old[m.Name] = m
	}
	onDisk := make(map[string]ModInfo, len(scanned))
	for _, s := range scanned {
		onDisk[s.Name] = s
	}
	out := make([]ModInfo, 0, len(d.Mods)+len(scanned))
	// 已有记录保持原顺序，逐个与磁盘校正
	for _, prev := range d.Mods {
		s, ok := onDisk[prev.Name]
		if !ok {
			prev.Missing = true
			out = append(out, prev)
			continue
		}
		s.Installed = prev.Installed || prev.Enabled
		if prev.Nickname != "" {
			s.Nickname = prev.Nickname
		}
		if prev.Cover != "" {
			s.Cover = prev.Cover
		}
		if len(prev.Parts) > 0 {
			s.Parts = prev.Parts
		}
		if prev.Category != "" {
			s.Category = prev.Category
		}
		if len(prev.SubMods) > 0 {
			s.SubMods = prev.SubMods
		}
		s.Missing = false
		out = append(out, s)
	}
	// 新发现的 Mod 追加到数据末尾
	for _, s := range scanned {
		if _, exists := old[s.Name]; !exists {
			s.Missing = false
			out = append(out, s)
		}
	}
	d.Mods = out
	d.Save()
}

// Uninstall 卸载 Mod：保留记录（文件库继续显示为“未安装”），仅清除安装状态、启用状态与占用的服装。
// Enabled 必须一并清掉：否则启动 Sync 时 prev.Enabled 为 true 会把 Installed 又判定为已安装（卡片复活）。
func (d *ModData) Uninstall(name string) {
	if i := d.findIndex(name); i >= 0 {
		d.Mods[i].Installed = false
		d.Mods[i].Enabled = false
		d.Mods[i].Parts = nil
		d.Save()
	}
}

// Install 将 Mod 登记为已安装并保存占用的服装（不创建符号链接，启用由 EnableMod 控制）
func (d *ModData) Install(name string, parts map[string][]string) {
	if i := d.findIndex(name); i >= 0 {
		d.Mods[i].Installed = true
		if len(parts) == 0 {
			d.Mods[i].Parts = nil
		} else {
			d.Mods[i].Parts = parts
		}
		d.Save()
		return
	}
	d.Mods = append(d.Mods, ModInfo{Name: name, Installed: true, Parts: parts})
	d.Save()
}

// ============================================================
// 操作日志 — modman.log（纯文本，每行一条）
// ============================================================

func logPath() string {
	return filepath.Join(appDataDir(), "modman.log")
}

// LoadLogs 读取 modman.log，逐行解析 [时间] 消息
func LoadLogs() *LogData {
	d := &LogData{}
	data, err := os.ReadFile(logPath())
	if err != nil {
		return d
	}
	for _, line := range strings.Split(string(decodeUTF8(data)), "\n") {
		line = strings.TrimRight(line, "\r")
		if len(line) < 21 || line[0] != '[' {
			continue
		}
		if idx := strings.Index(line, "] "); idx > 0 {
			d.Logs = append(d.Logs, LogEntry{Time: line[1:idx], Message: line[idx+2:]})
		}
	}
	return d
}

// All 返回全部日志（内存中）
func (d *LogData) All() []LogEntry { return d.Logs }

// Append 追加一条日志：写内存并追加写入 modman.log
func (d *LogData) Append(message string) {
	now := time.Now().Format("2006-01-02 15:04:05")
	d.Logs = append(d.Logs, LogEntry{Time: now, Message: message})
	f, err := os.OpenFile(logPath(), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	fmt.Fprintf(f, "[%s] %s\n", now, message)
}

// Clear 清空全部日志（内存与文件）
func (d *LogData) Clear() {
	d.Logs = nil
	os.WriteFile(logPath(), nil, 0644)
}

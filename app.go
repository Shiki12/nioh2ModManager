package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/text/encoding/simplifiedchinese"
	"nioh2mod-js/internal/armordata"
	"nioh2mod-js/internal/config"
	"nioh2mod-js/internal/dialog"
	"nioh2mod-js/internal/input"
	"nioh2mod-js/internal/mods"
	"nioh2mod-js/internal/steam"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// fileHandler 静态资源路由：
//   - /localfile?file=<绝对路径>   读取本地任意路径文件（旧版封面图等）
//   - /modfile?mod=<名称>&file=<相对路径> 读取 Mod 文件夹内的文件（封面图相对文件名，可移植）
func (a *App) fileHandler(w http.ResponseWriter, r *http.Request) {
	switch r.URL.Path {
	case "/modfile":
		repo := a.cfg.ModsRepo
		if repo == "" {
			http.NotFound(w, r)
			return
		}
		mod := r.URL.Query().Get("mod")
		file := r.URL.Query().Get("file")
		base := filepath.Join(repo, mod)
		target := filepath.Clean(filepath.Join(base, file))
		if target != base && !strings.HasPrefix(target, base+string(filepath.Separator)) {
			http.NotFound(w, r)
			return
		}
		http.ServeFile(w, r, target)
	case "/localfile":
		http.ServeFile(w, r, r.URL.Query().Get("file"))
	default:
		http.NotFound(w, r)
	}
}

// App struct
type App struct {
	ctx     context.Context
	cfg     *config.App
	modData *config.ModData
	logData *config.LogData
	// emitProgress 导入进度推送函数，startup 时绑定为 emitModProgressRaw；测试可注入桩
	emitProgress func(step, total int, msg string)
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.cfg = config.LoadApp()
	a.modData = config.LoadModData()
	a.logData = config.LoadLogs()
	a.emitProgress = a.emitModProgressRaw
	dialog.MainWindowTitle = "nioh2mod-js"
	a.log("应用启动")
	a.syncModsFromRepo()
}

// syncModsFromRepo 启动时自动扫描 Mod 仓库：
// 将当前仓库中已有的目录登记进数据文件，新发现的 Mod 追加记录，之后列表读取均以数据文件为准。
func (a *App) syncModsFromRepo() {
	if a.cfg.ModsRepo == "" || a.cfg.GameModsDir() == "" {
		return
	}
	discovered, err := mods.Scan(a.cfg.ModsRepo, a.cfg.GameModsDir())
	if err != nil {
		a.log("启动扫描 Mod 仓库失败: " + err.Error())
		return
	}
	scanned := make([]config.ModInfo, len(discovered))
	for i, m := range discovered {
		scanned[i] = config.ModInfo{Name: m.Name, Path: m.Path, Enabled: m.Enabled}
	}
	a.modData.Sync(scanned)
	// 效果图自动扫描：仅对列表为空的 Mod 填写，保留已人工增删的效果图
	for i := range a.modData.Mods {
		a.refreshModPreviews(&a.modData.Mods[i], true)
	}
	a.rebuildLocks()
	a.log(fmt.Sprintf("启动扫描 Mod 仓库完成，共 %d 个目录", len(discovered)))
}

func (a *App) SearchNioh2Root() (string, error) {
	root, err := steam.GameRoot("1325200", "Nioh2")
	if err != nil {
		a.log("搜索游戏目录失败: " + err.Error())
		return "", err
	}
	a.cfg.GameRoot = root
	a.cfg.Save()
	a.log("搜索到游戏目录: " + root)
	return root, nil
}

// DetectGameRoot 核对游戏安装目录（不写入配置，供前端展示并让用户确认）
func (a *App) DetectGameRoot() (string, error) {
	root, err := steam.GameRoot("1325200", "Nioh2")
	if err != nil {
		a.log("核对游戏目录失败: " + err.Error())
		return "", err
	}
	a.log("系统扫描到游戏目录: " + root)
	return root, nil
}

// EngineFile 引擎特征文件
type EngineFile struct {
	Name   string `json:"name"`
	IsDir  bool   `json:"isDir"`
	Exists bool   `json:"exists"`
}

// EngineStatus 引擎检测结果
type EngineStatus struct {
	GameRoot string       `json:"gameRoot"`
	Present  bool         `json:"present"`
	Files    []EngineFile `json:"files"`
}

// CheckModEngine 检查游戏根目录是否已安装 Mod 引擎（d3dx / 3Dmigoto 系）
func (a *App) CheckModEngine() *EngineStatus {
	root := a.cfg.GameRoot
	if root == "" {
		a.log("检测 Mod 引擎：未设置游戏目录")
		return &EngineStatus{GameRoot: ""}
	}
	spec := []EngineFile{
		{Name: "d3d11.dll"},
		{Name: "d3dx.ini"},
		{Name: "d3dx_user.ini"},
		{Name: "d3dcompiler_46.dll"},
		{Name: "3Dfix-README.txt"},
		{Name: "ShaderFixes", IsDir: true},
	}
	core := map[string]bool{"d3d11.dll": true, "d3dx.ini": true}
	coreFound := 0
	for i := range spec {
		if _, err := os.Stat(filepath.Join(root, spec[i].Name)); err == nil {
			spec[i].Exists = true
			if core[spec[i].Name] {
				coreFound++
			}
		}
	}
	present := coreFound == len(core)
	status := &EngineStatus{GameRoot: root, Files: spec, Present: present}
	if present {
		a.log("检测到 Mod 引擎已安装")
	} else {
		a.log("未检测到 Mod 引擎，请先安装 Mod 引擎")
	}
	return status
}

// InstallModEngine 将 exe 同目录 engine.zip 资源中的 Mod 引擎解压到游戏根目录
func (a *App) InstallModEngine() error {
	err := a.installModEngine()
	if err != nil {
		a.log("安装 Mod 引擎失败: " + err.Error())
	}
	return err
}

func (a *App) installModEngine() error {
	root := a.cfg.GameRoot
	if root == "" {
		return fmt.Errorf("未设置游戏目录")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	zipPath := filepath.Join(filepath.Dir(exe), "res", "engine.zip")
	zr, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("未找到引擎资源 res\\engine.zip，请确认其与程序在同一目录的 res 子目录下")
	}
	defer zr.Close()

	for _, f := range zr.File {
		name := filepath.Clean(filepath.FromSlash(f.Name))
		if name == "." || strings.HasPrefix(name, "..") || filepath.IsAbs(name) {
			continue
		}
		target := filepath.Join(root, name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(target, 0755); err != nil {
				return err
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		rc, err := f.Open()
		if err != nil {
			return err
		}
		data, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return err
		}
		if err := os.WriteFile(target, data, 0644); err != nil {
			return err
		}
	}

	a.log("Mod 引擎已安装到: " + root)
	return nil
}

// GetEnginePath 返回引擎资源 engine.zip 的完整路径
func (a *App) GetEnginePath() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "res", "engine.zip")
}

func (a *App) RefreshGameMods() bool {
	ok := input.RefreshMods()
	if ok {
		a.log("已向游戏窗口发送 F10 → F2 刷新指令")
	} else {
		a.log("刷新游戏 Mod 失败：未找到游戏窗口")
	}
	return ok
}

func (a *App) EnableMod(modName string) error {
	if a.cfg.GameModsDir() == "" {
		return fmt.Errorf("未设置游戏目录")
	}
	modDir := ""
	var md *config.ModInfo
	if md = a.modData.Find(modName); md != nil {
		modDir = md.Path
	}
	if modDir == "" {
		modDir = filepath.Join(a.cfg.ModsRepo, modName)
		a.modData.Upsert(modName, modDir)
	}
	var err error
	if md != nil && len(md.SubMods) > 0 {
		shared, _, _ := compositeEntries(modDir)
		var enabledSubs []string
		for _, sm := range md.SubMods {
			if sm.Enabled {
				enabledSubs = append(enabledSubs, sm.Name)
			}
		}
		err = mods.EnableComposite(modDir, modLinkPath(a.cfg, modName), shared, enabledSubs)
	} else {
		err = mods.Enable(modDir, modLinkPath(a.cfg, modName))
	}
	if err != nil {
		a.log("启用 Mod 失败 [" + modName + "]: " + err.Error())
		return err
	}
	a.modData.SetEnabled(modName, true)
	a.syncLock(modName)
	a.log("已启用 Mod: " + modName)
	return nil
}

// EnableModAndRefresh 启用 Mod 并自动刷新游戏
func (a *App) EnableModAndRefresh(modName string) error {
	if err := a.EnableMod(modName); err != nil {
		return err
	}
	a.RefreshGameMods()
	return nil
}

func (a *App) DisableMod(modName string) error {
	if a.cfg.GameModsDir() == "" {
		return fmt.Errorf("未设置游戏目录")
	}
	var err error
	if md := a.modData.Find(modName); md != nil && len(md.SubMods) > 0 {
		err = mods.DisableComposite(modLinkPath(a.cfg, modName))
		if err == nil {
			// 父组合包整体关闭：磁盘上子 Mod 链接已一并移除，数据里子 Mod 也要同步关闭
			a.modData.DisableAllSubMods(modName)
		}
	} else {
		err = mods.Disable(modLinkPath(a.cfg, modName))
	}
	if err != nil {
		a.log("禁用 Mod 失败 [" + modName + "]: " + err.Error())
		return err
	}
	a.modData.SetEnabled(modName, false)
	a.syncLock(modName)
	a.log("已禁用 Mod: " + modName)
	return nil
}

// DisableModAndRefresh 禁用 Mod 并自动刷新游戏
func (a *App) DisableModAndRefresh(modName string) error {
	if err := a.DisableMod(modName); err != nil {
		return err
	}
	a.RefreshGameMods()
	return nil
}

// EnableHdrSubMod 启用组合 Mod 内的单个子 Mod（需父组合包已启用）
func (a *App) EnableHdrSubMod(modName, subName string) error {
	if a.cfg.GameModsDir() == "" {
		return fmt.Errorf("未设置游戏目录")
	}
	md := a.modData.Find(modName)
	if md == nil {
		return fmt.Errorf("Mod 不存在: %s", modName)
	}
	modDir := md.Path
	if modDir == "" {
		modDir = filepath.Join(a.cfg.ModsRepo, modName)
	}
	linkPath := modLinkPath(a.cfg, modName)
	if !mods.CompositeEnabled(linkPath) {
		err := fmt.Errorf("请先启用组合包「%s」，再单独启用子 Mod", modName)
		a.log("启用子 Mod 失败: " + err.Error())
		return err
	}
	if err := mods.EnableSubMod(modDir, linkPath, subName); err != nil {
		a.log("启用子 Mod 失败 [" + modName + " / " + subName + "]: " + err.Error())
		return err
	}
	a.modData.SetSubModEnabled(modName, subName, true)
	a.syncLock(modName)
	a.log("已启用子 Mod: " + modName + " / " + subName)
	return nil
}

// EnableHdrSubModAndRefresh 启用子 Mod 并自动刷新游戏
func (a *App) EnableHdrSubModAndRefresh(modName, subName string) error {
	if err := a.EnableHdrSubMod(modName, subName); err != nil {
		return err
	}
	a.RefreshGameMods()
	return nil
}

// DisableHdrSubMod 禁用组合 Mod 内的单个子 Mod（仅移除该子 Mod 链接）
func (a *App) DisableHdrSubMod(modName, subName string) error {
	if a.cfg.GameModsDir() == "" {
		return fmt.Errorf("未设置游戏目录")
	}
	if err := mods.DisableSubMod(modLinkPath(a.cfg, modName), subName); err != nil {
		a.log("禁用子 Mod 失败 [" + modName + " / " + subName + "]: " + err.Error())
		return err
	}
	a.modData.SetSubModEnabled(modName, subName, false)
	a.syncLock(modName)
	a.log("已禁用子 Mod: " + modName + " / " + subName)
	return nil
}

// DisableHdrSubModAndRefresh 禁用子 Mod 并自动刷新游戏
func (a *App) DisableHdrSubModAndRefresh(modName, subName string) error {
	if err := a.DisableHdrSubMod(modName, subName); err != nil {
		return err
	}
	a.RefreshGameMods()
	return nil
}

func modLinkPath(cfg *config.App, modName string) string {
	return filepath.Join(cfg.GameModsDir(), modName)
}

// InstallMod 安装 Mod：登记为已安装并记录占用的服装（不创建符号链接，启用由 EnableMod 控制）
func (a *App) InstallMod(modName string, parts map[string][]string) error {
	a.modData.Install(modName, parts)
	a.syncLock(modName)
	a.log("已安装 Mod: " + modName)
	return nil
}

// ImportResult 自动安装导入结果
type ImportResult struct {
	Name        string              `json:"name"`
	Nickname    string              `json:"nickname"`
	Category    string              `json:"category"`           // armor=服装 / weapon=武器
	Cover       string              `json:"cover"`              // 封面图文件名（相对 mod 文件夹）
	Previews    []string            `json:"previews,omitempty"` // 多张效果图（相对 mod 文件夹）
	Parts       map[string][]string `json:"parts"`
	SubMods     []SubModConfig      `json:"submods,omitempty"` // 组合 Mod 的子 Mod 占用明细（父级本身不占用）
	ConfigFound bool                `json:"configFound"`
}

// partsOrder 占用的资源键的固定顺序：服装部位在前，武器最后
var partsOrder = []string{"头", "胸甲", "臂甲", "膝甲", "腿甲", "武器"}

// orderedParts 有序的占用资源表：MarshalJSON 时按 partsOrder 输出，未收录的未知键按字母序追加到末尾。
// 值为字符串数组（武器可占用多个）；读取时兼容旧版单值（"武器":"备前景安"）自动规整为数组。
type orderedParts map[string][]string

func (o orderedParts) MarshalJSON() ([]byte, error) {
	if o == nil {
		return []byte("null"), nil
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	first := true
	write := func(k string, vals []string) {
		if !first {
			buf.WriteByte(',')
		}
		first = false
		key, _ := json.Marshal(k)
		val, _ := json.Marshal(vals)
		buf.Write(key)
		buf.WriteByte(':')
		buf.Write(val)
	}
	seen := map[string]bool{}
	for _, k := range partsOrder {
		if v, ok := o[k]; ok {
			write(k, v)
			seen[k] = true
		}
	}
	rest := []string{}
	for k := range o {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	for _, k := range rest {
		write(k, o[k])
	}
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// UnmarshalJSON 兼容旧版单值（"胸甲":"上古之衣-上衣"）与新版多值（"武器":["刀一","刀二"]）
func (o *orderedParts) UnmarshalJSON(data []byte) error {
	raw := map[string]json.RawMessage{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}
	out := map[string][]string{}
	for k, r := range raw {
		var one string
		if err := json.Unmarshal(r, &one); err == nil {
			out[k] = []string{one}
			continue
		}
		var many []string
		if err := json.Unmarshal(r, &many); err != nil {
			return err
		}
		out[k] = many
	}
	*o = out
	return nil
}

// modConfig mod 文件夹内 mod.json 约定格式
type modConfig struct {
	Nickname string         `json:"nickname"`
	Category string         `json:"category"`
	Cover    string         `json:"cover"`
	Previews []string       `json:"previews,omitempty"` // 多张效果图（相对 Mod 目录路径，首张即封面）
	Parts    orderedParts   `json:"parts"`
	SubMods  []SubModConfig `json:"submods,omitempty"`
}

// SubModConfig 组合 Mod 内的子 Mod 声明：各自独立的占用资源与封面
type SubModConfig struct {
	Name     string       `json:"name"`
	Parts    orderedParts `json:"parts"`
	Cover    string       `json:"cover,omitempty"`    // 子 Mod 封面图（相对合集目录路径）
	Previews []string     `json:"previews,omitempty"` // 子 Mod 多张效果图（相对合集目录路径）
}

// InstallProgress Mod 导入进度（通过事件 "modInstallProgress" 推送给前端）
type InstallProgress struct {
	Step    int    `json:"step"`    // 当前步骤：1=移动文件夹，2=登记到数据仓库
	Total   int    `json:"total"`   // 总步骤数
	Percent int    `json:"percent"` // 跨盘复制时的文件进度百分比（0-100），同盘移动为 0
	Message string `json:"message"` // 进度文案
}

// emitModProgress 推送导入进度事件（未绑定推送函数时静默，测试场景不依赖 wails 上下文）
func (a *App) emitModProgress(step, total int, msg string) {
	if a.emitProgress != nil {
		a.emitProgress(step, total, msg)
	}
}

// emitModProgressRaw 直接通过 wails runtime 推送（需有效上下文）
func (a *App) emitModProgressRaw(step, total int, msg string) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, "modInstallProgress", InstallProgress{Step: step, Total: total, Message: msg})
}

// emitModCopyProgress 跨盘复制时按文件数回调进度
func (a *App) emitModCopyProgress(copied, total int) {
	if a.ctx == nil {
		return
	}
	percent := 0
	if total > 0 {
		percent = copied * 100 / total
	}
	runtime.EventsEmit(a.ctx, "modInstallProgress", InstallProgress{
		Step:    1,
		Total:   3,
		Percent: percent,
		Message: fmt.Sprintf("正在移动文件夹到托管目录…（%d/%d 个文件）", copied, total),
	})
}

// ImportMod 自动安装：将目录移入托管目录并读取 mod.json（如有）解析占用服装。
// 返回的 Parts 只含校验通过的有效项；ConfigFound 表示目录内是否存在 mod.json。
func (a *App) ImportMod(folder string) (*ImportResult, error) {
	repo := a.cfg.ModsRepo
	if repo == "" {
		err := fmt.Errorf("未设置 Mod 托管目录")
		a.log("导入 Mod 失败: " + err.Error())
		return nil, err
	}
	info, err := os.Stat(folder)
	if err != nil || !info.IsDir() {
		err = fmt.Errorf("选择的目录无效: %s", folder)
		a.log("导入 Mod 失败: " + err.Error())
		return nil, err
	}
	name := filepath.Base(strings.TrimRight(folder, `\/`))
	if name == "" || name == "." || name == "\\" || name == "/" {
		err = fmt.Errorf("无法确定 Mod 名称")
		a.log("导入 Mod 失败: " + err.Error())
		return nil, err
	}
	dest := filepath.Join(repo, name)

	// 目录已在托管目录内（如 F:\Mod\xxx）：跳过移动步骤，直接登记
	absFolder, _ := filepath.Abs(folder)
	absRepo, _ := filepath.Abs(repo)
	repoPrefix := strings.ToLower(absRepo)
	folderLower := strings.ToLower(absFolder)
	inRepo := folderLower == repoPrefix || strings.HasPrefix(folderLower, repoPrefix+string(filepath.Separator))

	if !inRepo {
		if _, err := os.Stat(dest); err == nil {
			err = fmt.Errorf("托管目录已存在同名 Mod: %s", name)
			a.log("导入 Mod 失败: " + err.Error())
			return nil, err
		}
		a.emitModProgress(1, 3, "正在移动文件夹到托管目录…")
		if err := moveDir(folder, dest, a.emitModCopyProgress); err != nil {
			a.emitModProgress(0, 3, "移动文件夹失败: "+err.Error())
			a.log("导入 Mod 失败: 移动目录失败: " + err.Error())
			return nil, err
		}
	} else {
		a.log("目录已在托管目录内，直接登记: " + name)
	}

	// 直接登记进数据文件（未安装状态），无需依赖磁盘扫描展示文件库
	a.emitModProgress(2, 3, "正在登记到 Mod 数据仓库…")
	a.modData.Upsert(name, dest)

	// 识别该 Mod 占用的资源（服装部位 / 武器）与封面图
	a.emitModProgress(3, 3, "正在识别该 Mod 占用的服装/武器资源…")
	nickname, category, cover, previews, parts, submods, found := a.readModConfig(dest)
	// HDR 合集自动识别：目录含公共 meshes/textures 但未登记子 Mod 时，
	// 自动识别子 Mod 并生成 mod.json，保证组合包始终以组合形式登记/安装
	if len(submods) == 0 {
		if _, ok, cerr := a.ensureCompositeConfig(dest, name); cerr != nil {
			a.log("识别 HDR 合集失败: " + cerr.Error())
		} else if ok {
			nickname, category, cover, previews, parts, submods, found = a.readModConfig(dest)
			found = true
		}
	}
	if nickname != "" {
		a.modData.SetNickname(name, nickname)
	}
	if category != "" {
		a.modData.SetCategory(name, category)
	}
	if cover != "" {
		a.modData.SetCover(name, cover)
	}
	if len(previews) > 0 {
		a.modData.SetPreviews(name, previews)
	}
	// 组合 Mod（HDR 合集等）：按子 Mod 分别登记（父级本身不占用），Parts 存并集供冲突检测/展示
	if len(submods) > 0 {
		a.registerSubMods(name, submods)
		md := a.modData.Find(name)
		if md != nil {
			parts = md.Parts
			category = md.Category
		}
	}
	a.emitModProgress(3, 3, "识别完成")
	a.log("已导入 Mod: " + name)
	return &ImportResult{Name: name, Nickname: nickname, Category: category, Cover: cover, Previews: previews, Parts: parts, SubMods: submods, ConfigFound: found}, nil
}

// InstallHdrMod 安装 HDR 合集：校验为合集后（无 mod.json 时先自动识别生成），
// 再移入托管目录并按子 Mod 登记（父级不占用，子 Mod 独立启用）。
func (a *App) InstallHdrMod(folder string) (*ImportResult, error) {
	info, err := os.Stat(folder)
	if err != nil || !info.IsDir() {
		err = fmt.Errorf("选择的目录无效: %s", folder)
		a.log("安装 HDR 合集失败: " + err.Error())
		return nil, err
	}
	if !hasSubDir(folder, "meshes") || !hasSubDir(folder, "textures") {
		err = fmt.Errorf("该目录不是 HDR 合集（缺少公共 meshes/textures 子目录）")
		a.log("安装 HDR 合集失败: " + err.Error())
		return nil, err
	}
	a.emitModProgress(1, 3, "正在识别 HDR 合集并生成 mod.json…")
	if _, _, cerr := a.ensureCompositeConfig(folder, filepath.Base(strings.TrimRight(folder, `\/`))); cerr != nil {
		err = cerr
		a.emitModProgress(1, 3, "识别 HDR 合集失败: "+cerr.Error())
		a.log("安装 HDR 合集失败: " + err.Error())
		return nil, err
	}
	res, err := a.ImportMod(folder)
	if err != nil {
		return nil, err
	}
	if len(res.SubMods) == 0 {
		err = fmt.Errorf("导入完成但未登记到子 Mod，请检查 mod.json 格式")
		a.log("安装 HDR 合集失败: " + err.Error())
		return nil, err
	}
	// HDR 合集装完即视为已安装（父级卡片直接出现在"已安装"标签，子 Mod 待父级启用后独立开关）
	a.modData.Install(res.Name, res.Parts)
	a.emitModProgress(3, 3, fmt.Sprintf("HDR 合集安装完成：%d 个子 Mod", len(res.SubMods)))
	a.log(fmt.Sprintf("已安装 HDR 合集「%s」，共 %d 个子 Mod", res.Nickname, len(res.SubMods)))
	return res, nil
}

// GetModConfig 读取托管目录中指定 Mod 的 mod.json，返回昵称与占用的装备资源。
// 供前端安装弹窗预填；没有 mod.json 时 ConfigFound 为 false，由用户手动输入。
func (a *App) GetModConfig(modName string) (*ImportResult, error) {
	if a.cfg.ModsRepo == "" {
		return nil, fmt.Errorf("未设置 Mod 托管目录")
	}
	dir := filepath.Join(a.cfg.ModsRepo, modName)
	if _, err := os.Stat(dir); err != nil {
		return nil, fmt.Errorf("Mod 文件夹不存在: %s", modName)
	}
	nickname, category, cover, previews, parts, submods, found := a.readModConfig(dir)
	// HDR 合集自动识别并登记：重装/编辑场景下数据记录可能缺失子 Mod，
	// 若目录为合集（含公共 meshes/textures）则自动识别并写入子 Mod，保证以组合包形式展示
	if len(submods) == 0 {
		if _, ok, cerr := a.ensureCompositeConfig(dir, modName); cerr != nil {
			a.log("识别 HDR 合集失败: " + cerr.Error())
		} else if ok {
			nickname, category, cover, previews, parts, submods, found = a.readModConfig(dir)
			found = true
			if len(submods) > 0 {
				a.registerSubMods(modName, submods)
			}
		}
	}
	return &ImportResult{Name: modName, Nickname: nickname, Category: category, Cover: cover, Previews: previews, Parts: parts, SubMods: submods, ConfigFound: found}, nil
}

// ModCard 作者工具生成的卡片数据（写入 mod 文件夹的 mod.json）
type ModCard struct {
	Nickname string              `json:"nickname"`
	Category string              `json:"category"`
	Cover    string              `json:"cover"`
	Previews []string            `json:"previews,omitempty"` // 多张效果图（相对 Mod 目录路径）
	Parts    map[string][]string `json:"parts"`
}

// ReadModConfig 读取任意文件夹内的 mod.json（作者工具载入已有配置）
func (a *App) ReadModConfig(folder string) (*ImportResult, error) {
	if folder == "" {
		return nil, fmt.Errorf("未指定文件夹")
	}
	nickname, category, cover, previews, parts, submods, found := a.readModConfig(folder)
	return &ImportResult{Name: filepath.Base(folder), Nickname: nickname, Category: category, Cover: cover, Previews: previews, Parts: parts, SubMods: submods, ConfigFound: found}, nil
}

// ListFolderImages 列出文件夹根目录下的图片文件名（作者工具选封面）
func (a *App) ListFolderImages(folder string) ([]string, error) {
	if folder == "" {
		return nil, fmt.Errorf("未指定文件夹")
	}
	entries, err := os.ReadDir(folder)
	if err != nil {
		return nil, err
	}
	var imgs []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if previewExts[strings.ToLower(filepath.Ext(e.Name()))] {
			imgs = append(imgs, e.Name())
		}
	}
	return imgs, nil
}

// AddCoverImage 将外部图片复制到 mod 文件夹根目录作为封面，返回复制后的文件名。
// 统一命名为 preview.<ext>，已存在时追加序号避免覆盖。
func (a *App) AddCoverImage(folder, srcPath string) (string, error) {
	if folder == "" || srcPath == "" {
		return "", fmt.Errorf("未指定文件夹或图片")
	}
	info, err := os.Stat(srcPath)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("图片文件不存在: %s", srcPath)
	}
	ext := strings.ToLower(filepath.Ext(srcPath))
	base := "preview" + ext
	target := filepath.Join(folder, base)
	for i := 2; ; i++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			break
		}
		base = fmt.Sprintf("preview_%d%s", i, ext)
		target = filepath.Join(folder, base)
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(target, data, 0644); err != nil {
		return "", err
	}
	a.log("作者工具：已复制封面图片 → " + target)
	return base, nil
}

// refreshModPreviews 从磁盘扫描 Mod 的多张效果图并写回数据文件。
// 组合 Mod：父级取合集根目录图片，各子 Mod 递归取子目录图片。
// onlyIfEmpty=true 时仅在列表为空时填写（保留人工增删的效果图，不被扫描覆盖）。
func (a *App) refreshModPreviews(md *config.ModInfo, onlyIfEmpty bool) {
	if md == nil || a.cfg.ModsRepo == "" {
		return
	}
	dir := md.Path
	if dir == "" {
		dir = filepath.Join(a.cfg.ModsRepo, md.Name)
	}
	if info, err := os.Stat(dir); err != nil || !info.IsDir() {
		return
	}
	if len(md.SubMods) > 0 {
		if !onlyIfEmpty || len(md.Preview) == 0 {
			a.modData.SetPreviews(md.Name, collectFolderImages(dir, dir, false, 12))
		}
		for _, sm := range md.SubMods {
			if onlyIfEmpty && len(sm.Preview) > 0 {
				continue
			}
			subDir := filepath.Join(dir, sm.Name)
			if info, err := os.Stat(subDir); err != nil || !info.IsDir() {
				continue
			}
			previews := collectFolderImages(subDir, dir, true, 12)
			if cover := subModCover(subDir, sm.Name); cover != "" {
				for i, p := range previews {
					if p == cover {
						previews[0], previews[i] = previews[i], previews[0]
						break
					}
				}
			}
			a.modData.SetSubModPreviews(md.Name, sm.Name, previews)
		}
		return
	}
	if !onlyIfEmpty || len(md.Preview) == 0 {
		a.modData.SetPreviews(md.Name, collectFolderImages(dir, dir, false, 12))
	}
}

// GetModPreviews 返回 Mod 自身（父级）的多张效果图（相对 Mod 目录路径）。
func (a *App) GetModPreviews(modName string) ([]string, error) {
	md := a.modData.Find(modName)
	if md == nil {
		return nil, fmt.Errorf("Mod 不存在: %s", modName)
	}
	return append([]string{}, md.Preview...), nil
}

// GetSubModPreviews 返回组合 Mod 内某子 Mod 的多张效果图（相对父 Mod 目录路径）。
func (a *App) GetSubModPreviews(parentName, subName string) ([]string, error) {
	md := a.modData.Find(parentName)
	if md == nil {
		return nil, fmt.Errorf("Mod 不存在: %s", parentName)
	}
	for _, sm := range md.SubMods {
		if sm.Name == subName {
			return append([]string{}, sm.Preview...), nil
		}
	}
	return nil, fmt.Errorf("子 Mod 不存在: %s", subName)
}

// RefreshModPreviews 从磁盘重新扫描 Mod 的多张效果图并保存（强制覆盖），返回父级自身效果图。
func (a *App) RefreshModPreviews(modName string) ([]string, error) {
	md := a.modData.Find(modName)
	if md == nil {
		return nil, fmt.Errorf("Mod 不存在: %s", modName)
	}
	a.refreshModPreviews(md, false)
	return a.GetModPreviews(modName)
}

// AddModPreview 将外部图片复制到 Mod 文件夹根目录作为一张效果图，返回相对文件名。
// 统一命名为 preview.<ext>，已存在时追加序号避免覆盖。
func (a *App) AddModPreview(modName, srcPath string) (string, error) {
	if modName == "" || srcPath == "" {
		return "", fmt.Errorf("未指定 Mod 或图片")
	}
	md := a.modData.Find(modName)
	if md == nil {
		return "", fmt.Errorf("Mod 不存在: %s", modName)
	}
	info, err := os.Stat(srcPath)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("图片文件不存在: %s", srcPath)
	}
	if !previewExts[strings.ToLower(filepath.Ext(srcPath))] {
		return "", fmt.Errorf("不支持的图片格式: %s", filepath.Ext(srcPath))
	}
	dir := md.Path
	if dir == "" {
		dir = filepath.Join(a.cfg.ModsRepo, modName)
	}
	ext := strings.ToLower(filepath.Ext(srcPath))
	base := "preview" + ext
	target := filepath.Join(dir, base)
	for i := 2; ; i++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			break
		}
		base = fmt.Sprintf("preview_%d%s", i, ext)
		target = filepath.Join(dir, base)
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(target, data, 0644); err != nil {
		return "", err
	}
	a.modData.SetPreviews(modName, append(md.Preview, base))
	a.log(fmt.Sprintf("已添加效果图 [%s]: %s", modName, base))
	return base, nil
}

// RemoveModPreview 从 Mod 效果图列表中移除指定图片（仅移除列表记录，不删除文件）。
func (a *App) RemoveModPreview(modName, file string) error {
	md := a.modData.Find(modName)
	if md == nil {
		return fmt.Errorf("Mod 不存在: %s", modName)
	}
	var keep []string
	for _, p := range md.Preview {
		if p != file {
			keep = append(keep, p)
		}
	}
	a.modData.SetPreviews(modName, keep)
	a.log(fmt.Sprintf("已移除效果图 [%s]: %s", modName, file))
	return nil
}

// AddSubModPreview 将外部图片复制到组合 Mod 内某子 Mod 目录作为一张效果图，
// 返回相对父 Mod 目录的路径（如 01_[A]/preview.png）。
func (a *App) AddSubModPreview(parentName, subName, srcPath string) (string, error) {
	if parentName == "" || subName == "" || srcPath == "" {
		return "", fmt.Errorf("未指定 Mod、子 Mod 或图片")
	}
	md := a.modData.Find(parentName)
	if md == nil {
		return "", fmt.Errorf("Mod 不存在: %s", parentName)
	}
	info, err := os.Stat(srcPath)
	if err != nil || info.IsDir() {
		return "", fmt.Errorf("图片文件不存在: %s", srcPath)
	}
	if !previewExts[strings.ToLower(filepath.Ext(srcPath))] {
		return "", fmt.Errorf("不支持的图片格式: %s", filepath.Ext(srcPath))
	}
	dir := md.Path
	if dir == "" {
		dir = filepath.Join(a.cfg.ModsRepo, parentName)
	}
	subDir := filepath.Join(dir, subName)
	if info, err := os.Stat(subDir); err != nil || !info.IsDir() {
		return "", fmt.Errorf("子 Mod 目录不存在: %s", subName)
	}
	ext := strings.ToLower(filepath.Ext(srcPath))
	base := "preview" + ext
	target := filepath.Join(subDir, base)
	for i := 2; ; i++ {
		if _, err := os.Stat(target); os.IsNotExist(err) {
			break
		}
		base = fmt.Sprintf("preview_%d%s", i, ext)
		target = filepath.Join(subDir, base)
	}
	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(target, data, 0644); err != nil {
		return "", err
	}
	rel := filepath.ToSlash(filepath.Join(subName, base))
	var keep []string
	for _, sm := range md.SubMods {
		if sm.Name == subName {
			keep = append(keep, sm.Preview...)
			break
		}
	}
	a.modData.SetSubModPreviews(parentName, subName, append(keep, rel))
	a.log(fmt.Sprintf("已添加子 Mod 效果图 [%s/%s]: %s", parentName, subName, rel))
	return rel, nil
}

// RemoveSubModPreview 从组合 Mod 内某子 Mod 的效果图列表中移除指定图片（仅移除记录）。
func (a *App) RemoveSubModPreview(parentName, subName, file string) error {
	md := a.modData.Find(parentName)
	if md == nil {
		return fmt.Errorf("Mod 不存在: %s", parentName)
	}
	for _, sm := range md.SubMods {
		if sm.Name == subName {
			var keep []string
			for _, p := range sm.Preview {
				if p != file {
					keep = append(keep, p)
				}
			}
			a.modData.SetSubModPreviews(parentName, subName, keep)
			a.log(fmt.Sprintf("已移除子 Mod 效果图 [%s/%s]: %s", parentName, subName, file))
			return nil
		}
	}
	return fmt.Errorf("子 Mod 不存在: %s", subName)
}

// SetSubModCover 设置组合 Mod 内某子 Mod 的封面图（相对父 Mod 目录路径，如 01_[A]/preview.png）。
func (a *App) SetSubModCover(parentName, subName, cover string) error {
	md := a.modData.Find(parentName)
	if md == nil {
		return fmt.Errorf("Mod 不存在: %s", parentName)
	}
	found := false
	for _, sm := range md.SubMods {
		if sm.Name == subName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("子 Mod 不存在: %s", subName)
	}
	cover = strings.TrimSpace(cover)
	a.modData.SetSubModCover(parentName, subName, cover)
	a.log(fmt.Sprintf("已更新子 Mod 封面 [%s/%s]: %s", parentName, subName, cover))
	return nil
}

// WriteModCard 校验并生成 mod.json 写入指定文件夹（作者工具：预打包卡片）
func (a *App) WriteModCard(folder string, card ModCard) (*ImportResult, error) {
	if folder == "" {
		return nil, fmt.Errorf("未指定文件夹")
	}
	valid := orderedParts{}
	for slot, vals := range card.Parts {
		for _, val := range vals {
			val = strings.TrimSpace(val)
			if val == "" {
				continue
			}
			var part *armordata.Part
			if slot == "武器" {
				part = armordata.FindWeapon(val)
				if part == nil {
					part = armordata.FindWeaponByName(val)
				}
			} else {
				part = armordata.Find(slot, val)
				if part == nil {
					part = armordata.FindByName(slot, val)
				}
			}
			if part != nil {
				valid[slot] = append(valid[slot], part.Name)
			}
		}
	}
	cover := strings.TrimSpace(card.Cover)
	if i := strings.LastIndexAny(cover, `\/`); i >= 0 {
		cover = cover[i+1:]
	}
	var previews []string
	for _, p := range card.Previews {
		if p = strings.TrimSpace(p); p != "" {
			previews = append(previews, p)
		}
	}
	out := modConfig{Nickname: strings.TrimSpace(card.Nickname), Category: config.DeriveCategory(valid), Cover: cover, Previews: previews, Parts: valid}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(folder, "mod.json"), data, 0644); err != nil {
		return nil, err
	}
	a.log("作者工具：已生成 mod.json → " + folder)
	return &ImportResult{Name: filepath.Base(folder), Nickname: out.Nickname, Category: out.Category, Cover: out.Cover, Previews: previews, Parts: valid, ConfigFound: true}, nil
}

// ---- 批量识别 HDR 合集 ----

// PendingItem 批量识别中未能自动匹配的占用条目，需人工确认
type PendingItem struct {
	Mod     string `json:"mod"`     // 合集名
	SubMod  string `json:"subMod"`  // 子 Mod 名
	Slot    string `json:"slot"`    // armordata 槽位（胸甲等）
	Chinese string `json:"chinese"` // Read Me 中的中文套装名
	English string `json:"english"` // Read Me 中的英文装备名
}

// BatchGenerateResult 批量识别 HDR 合集的结果
type BatchGenerateResult struct {
	Total     int           `json:"total"`     // 识别到的 HDR 合集数
	Generated int           `json:"generated"` // 成功生成 mod.json 的合集数
	Mods      []modConfig   `json:"mods"`      // 生成的 mod.json 内容（含 submods）
	Pending   []PendingItem `json:"pending"`   // 未能自动匹配、需人工确认的占用条目
	Errors    []string      `json:"errors"`    // 处理失败的合集信息
}

// readMeSlotMap HDR Read Me 中的英文槽位 → armordata 槽位
// （膝甲即腰甲、腿甲即脚，按作者命名约定映射）
var readMeSlotMap = map[string]string{
	"Chest": "胸甲",
	"Hand":  "臂甲",
	"Waist": "膝甲",
	"Foot":  "腿甲",
	"Head":  "头",
}

// subModDirRe 识别组合包中的子 Mod 目录：以 数字_[字母] 开头（如 "01_[A] ..."）
var subModDirRe = regexp.MustCompile(`^\s*\d+\s*_\s*\[[A-Za-z]`)

// readMeOccupyRe 解析 Read Me 占用行，形如 ";-胸 - 讨魔首领 \ Chest - DemonSlayer"：
// [1]=反斜杠前的中文段 [2]=英文槽位 [3]=英文装备名
var readMeOccupyRe = regexp.MustCompile(`^(.+?)\s*\\\s*(Chest|Hand|Waist|Foot|Head)\s*[-:]\s*([^\r\n]+?)\s*$`)

// BatchGenerateModCards 批量识别指定目录下的 HDR 合集：
// 合集判定 = 目录同时含 meshes/ 与 textures/；子 Mod 判定 = 顶层 数字_[字母] 目录且根目录含 .ini。
// 解析各子 Mod 的 Read Me 提取占用并匹配 armordata，自动生成带 submods 的 mod.json（父级不占用）。
// 支持两种用法：选择含多个合集的上级目录（扫描其直接子目录），或直接选择单个合集目录本身。
func (a *App) BatchGenerateModCards(root string) (*BatchGenerateResult, error) {
	if root == "" {
		return nil, fmt.Errorf("未指定目录")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, fmt.Errorf("选择的目录无效: %s", root)
	}
	res := &BatchGenerateResult{}

	// 收集合集目录：根目录本身是合集则直接处理；否则扫描直接子目录
	var cols []string
	if hasSubDir(root, "meshes") && hasSubDir(root, "textures") {
		cols = append(cols, root)
	} else {
		entries, err := os.ReadDir(root)
		if err != nil {
			return nil, fmt.Errorf("读取目录失败: %v", err)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			dir := filepath.Join(root, e.Name())
			if hasSubDir(dir, "meshes") && hasSubDir(dir, "textures") {
				cols = append(cols, dir)
			}
		}
	}
	if len(cols) == 0 {
		a.log("批量识别：所选目录下未找到 HDR 合集（需目录同时含 meshes/ 与 textures/ 子目录）")
		return res, nil
	}

	for _, dir := range cols {
		name := filepath.Base(dir)
		res.Total++
		cfg, pending, warn := a.recognizeComposite(dir, name)
		res.Pending = append(res.Pending, pending...)
		if warn != "" {
			msg := name + ": " + warn
			res.Errors = append(res.Errors, msg)
			a.log("批量识别失败 [" + msg + "]")
			continue
		}
		data, err := json.MarshalIndent(cfg, "", "  ")
		if err != nil {
			msg := name + ": " + err.Error()
			res.Errors = append(res.Errors, msg)
			a.log("批量识别失败 [" + msg + "]")
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, "mod.json"), data, 0644); err != nil {
			msg := name + ": " + err.Error()
			res.Errors = append(res.Errors, msg)
			a.log("批量识别失败 [" + msg + "]")
			continue
		}
		res.Generated++
		res.Mods = append(res.Mods, *cfg)
		a.log("批量识别已生成 mod.json: " + name)
		if len(pending) > 0 {
			a.log("批量识别存在待确认占用项: " + name)
		}
	}
	a.log(fmt.Sprintf("批量识别完成：共 %d 个合集，生成 %d 个，失败 %d 个，待确认 %d 项", res.Total, res.Generated, len(res.Errors), len(res.Pending)))
	return res, nil
}

// hasSubDir 判断 base 下是否存在名为 name 的目录
func hasSubDir(base, name string) bool {
	info, err := os.Stat(filepath.Join(base, name))
	return err == nil && info.IsDir()
}

// ensureCompositeConfig 检测目录是否为 HDR 合集（含公共 meshes/textures 子目录）。
// mod.json 缺失或未含子 Mod 时，自动识别子 Mod 并写入 mod.json。
// 返回是否识别为合集；非合集返回 (nil, false, nil)，识别失败返回错误。
func (a *App) ensureCompositeConfig(dir, name string) (*modConfig, bool, error) {
	if !hasSubDir(dir, "meshes") || !hasSubDir(dir, "textures") {
		return nil, false, nil
	}
	if _, err := os.Stat(filepath.Join(dir, "mod.json")); err == nil {
		// 已有 mod.json：仅当其中已含子 Mod 时视为已识别，否则继续自动识别
		_, _, _, _, _, submods, _ := a.readModConfig(dir)
		if len(submods) > 0 {
			return nil, true, nil
		}
	}
	cfg, pending, warn := a.recognizeComposite(dir, name)
	if warn != "" {
		return nil, false, fmt.Errorf("识别合集失败: %s", warn)
	}
	if len(cfg.SubMods) == 0 {
		return nil, false, fmt.Errorf("识别合集失败: 未识别到子 Mod")
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return nil, false, err
	}
	if err := os.WriteFile(filepath.Join(dir, "mod.json"), data, 0644); err != nil {
		return nil, false, err
	}
	a.log("已生成 HDR 合集 mod.json: " + dir)
	if len(pending) > 0 {
		a.log("HDR 合集存在待确认占用项: " + dir)
	}
	return cfg, true, nil
}

// registerSubMods 将识别出的子 Mod 登记进数据仓库（父级不占用，并集写入 Parts）
func (a *App) registerSubMods(name string, submods []SubModConfig) {
	subInfos := make([]config.SubModInfo, 0, len(submods))
	for _, sm := range submods {
		subInfos = append(subInfos, config.SubModInfo{Name: sm.Name, Parts: sm.Parts, Cover: sm.Cover, Preview: sm.Previews})
	}
	a.modData.SetSubMods(name, subInfos)
}

// recognizeComposite 识别一个 HDR 合集文件夹，返回 modConfig、待确认条目与警告信息
func (a *App) recognizeComposite(dir, name string) (*modConfig, []PendingItem, string) {
	cfg := &modConfig{Nickname: name}
	var pending []PendingItem
	entries, err := os.ReadDir(dir)
	if err != nil {
		return cfg, nil, err.Error()
	}
	var submods []SubModConfig
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sn := e.Name()
		if !subModDirRe.MatchString(sn) || strings.HasPrefix(strings.ToUpper(sn), "DISABLED") {
			continue
		}
		subDir := filepath.Join(dir, sn)
		if !hasIniFile(subDir) {
			continue
		}
		parts, sp, err := a.parseReadMeOccupancy(subDir)
		previews := collectFolderImages(subDir, dir, true, 12)
		cover := subModCover(subDir, sn)
		if cover != "" {
			for i, p := range previews {
				if p == cover {
					previews[0], previews[i] = previews[i], previews[0]
					break
				}
			}
		}
		sm := SubModConfig{Name: sn, Parts: parts, Cover: cover, Previews: previews}
		if err != nil {
			sp = append(sp, PendingItem{Mod: name, SubMod: sn, Chinese: "（无 Read Me 或解析失败: " + err.Error() + "）"})
		}
		pending = append(pending, sp...)
		if len(sm.Parts) > 0 {
			submods = append(submods, sm)
		}
	}
	// 父级效果图：合集根目录下的图片
	cfg.Previews = collectFolderImages(dir, dir, false, 12)
	if len(submods) == 0 {
		return cfg, pending, "未识别到子 Mod"
	}
	cfg.SubMods = submods
	// 父级本身不占用；category 由子 Mod 占用并集推导，仅作展示用
	union := orderedParts{}
	for _, sm := range submods {
		for k, vals := range sm.Parts {
			union[k] = append(union[k], vals...)
		}
	}
	cfg.Category = config.DeriveCategory(union)
	return cfg, pending, ""
}

// hasIniFile 判断目录根下是否存在 .ini 文件
func hasIniFile(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".ini") {
			return true
		}
	}
	return false
}

// previewExts 效果图图片扩展名集合
var previewExts = map[string]bool{".png": true, ".jpg": true, ".jpeg": true, ".webp": true, ".bmp": true, ".gif": true}

// collectFolderImages 收集目录内的图片路径（按名称排序），返回相对 base 的斜杠路径。
// recursive=true 时递归子目录（用于子 Mod 效果图），否则仅根目录（用于父级效果图）。
// 当 base=dir 时根目录图片返回文件名本身；子 Mod 效果图传 base=合集根目录以得到
// "01_[A]/p1.jpg" 这类相对父级路径。最多 limit 张。
func collectFolderImages(dir, base string, recursive bool, limit int) []string {
	var imgs []string
	add := func(path string) {
		if len(imgs) >= limit {
			return
		}
		if previewExts[strings.ToLower(filepath.Ext(path))] {
			if rel, rerr := filepath.Rel(base, path); rerr == nil {
				imgs = append(imgs, filepath.ToSlash(rel))
			}
		}
	}
	if recursive {
		_ = filepath.Walk(dir, func(path string, fi os.FileInfo, err error) error {
			if err != nil || fi.IsDir() {
				return nil
			}
			add(path)
			return nil
		})
	} else {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return nil
		}
		for _, e := range entries {
			if !e.IsDir() {
				add(filepath.Join(dir, e.Name()))
			}
		}
	}
	sort.Strings(imgs)
	return imgs
}

// subModCover 在子 Mod 目录内查找封面预览图（首个图片文件），
// 返回相对合集目录的路径（如 01_[A]/preview.png）；未找到返回空串。
func subModCover(subDir, subName string) string {
	var found string
	_ = filepath.Walk(subDir, func(path string, fi os.FileInfo, err error) error {
		if err != nil || found != "" || fi.IsDir() {
			return nil
		}
		switch strings.ToLower(filepath.Ext(path)) {
		case ".png", ".jpg", ".jpeg", ".bmp", ".webp":
			rel, rerr := filepath.Rel(filepath.Dir(subDir), path)
			if rerr == nil {
				found = filepath.ToSlash(rel)
			}
		}
		return nil
	})
	return found
}

// optionalDirRe 识别合集内非共享的可选变体目录（本期不链接/登记）
var optionalDirRe = regexp.MustCompile(`(?i)^DISABLED|^THEME\s*COLOR`)

// compositeEntries 识别组合 Mod 目录内的三类条目：
// shared=公共条目（meshes/textures 等，父级负责链接）；subMods=子 Mod 目录；optional=可选变体（跳过）。
func compositeEntries(modDir string) (shared []string, subMods []string, optional []string) {
	entries, err := os.ReadDir(modDir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if optionalDirRe.MatchString(name) {
			optional = append(optional, name)
			continue
		}
		if e.IsDir() && subModDirRe.MatchString(name) && hasIniFile(filepath.Join(modDir, name)) {
			subMods = append(subMods, name)
			continue
		}
		shared = append(shared, name)
	}
	return
}

// parseReadMeOccupancy 解析子 Mod 目录内 Read Me 文件的占用声明，
// 返回规范化占用表与未能自动匹配的待确认条目
func (a *App) parseReadMeOccupancy(subDir string) (orderedParts, []PendingItem, error) {
	entries, err := os.ReadDir(subDir)
	if err != nil {
		return nil, nil, err
	}
	var readMe string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		low := strings.ToLower(e.Name())
		if strings.Contains(low, "read me") || strings.Contains(low, "readme") || strings.Contains(low, "使用说明") || strings.Contains(low, "说明") {
			readMe = filepath.Join(subDir, e.Name())
			break
		}
	}
	if readMe == "" {
		return nil, nil, fmt.Errorf("未找到 Read Me 文件")
	}
	data, err := os.ReadFile(readMe)
	if err != nil {
		return nil, nil, err
	}
	text := decodeReadMe(data)
	parts := orderedParts{}
	var pending []PendingItem
	modName := filepath.Base(filepath.Dir(subDir))
	subName := filepath.Base(subDir)
	for _, line := range strings.Split(text, "\n") {
		m := readMeOccupyRe.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		slot, ok := readMeSlotMap[strings.TrimSpace(m[2])]
		if !ok {
			continue
		}
		setName := parseReadMeSetName(m[1])
		enName := strings.TrimSpace(m[3])
		if setName == "" {
			continue
		}
		if p := armordata.FindBySetName(slot, setName); p != nil {
			parts[slot] = append(parts[slot], p.Name)
			continue
		}
		pending = append(pending, PendingItem{Mod: modName, SubMod: subName, Slot: slot, Chinese: setName, English: enName})
	}
	return parts, pending, nil
}

// parseReadMeSetName 从占用行反斜杠前的中文段提取套装名（取最后一个 "-" 之后的部分）
func parseReadMeSetName(s string) string {
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), ";"))
	parts := strings.Split(s, "-")
	return strings.TrimSpace(parts[len(parts)-1])
}

// decodeReadMe 解码 Read Me：文件通常为 UTF-8 编码，个别作者用 GBK，
// 以"是否合法 UTF-8"判断编码，非法时回退 GBK 解码。
func decodeReadMe(data []byte) string {
	if utf8.Valid(data) {
		return string(data)
	}
	if out, err := simplifiedchinese.GBK.NewDecoder().Bytes(data); err == nil {
		return string(out)
	}
	return string(data)
}

// validateParts 校验占用资源表：键为"武器"时按武器数据校验，其余按对应服装部位校验，
// 可同时占用服装与武器；武器部位可含多个值。无效项忽略，返回规范化后的有效表。
func validateParts(raw map[string][]string) orderedParts {
	valid := orderedParts{}
	for slot, vals := range raw {
		for _, val := range vals {
			val = strings.TrimSpace(val)
			if val == "" {
				continue
			}
			var part *armordata.Part
			if slot == "武器" {
				part = armordata.FindWeapon(val)
				if part == nil {
					part = armordata.FindWeaponByName(val)
				}
			} else {
				part = armordata.Find(slot, val)
				if part == nil {
					part = armordata.FindByName(slot, val)
				}
			}
			if part != nil {
				valid[slot] = append(valid[slot], part.Name)
			}
		}
	}
	return valid
}

// readModConfig 读取 mod 文件夹内的 mod.json，解析昵称、分类（服装/武器）、封面图文件名、
// 多张效果图、占用资源与子 Mod 明细（无效项忽略）。父级占用为空时，组合 Mod 的占用在各子 Mod 上。
func (a *App) readModConfig(dir string) (nickname string, category string, cover string, previews []string, parts map[string][]string, submods []SubModConfig, found bool) {
	data, err := os.ReadFile(filepath.Join(dir, "mod.json"))
	if err != nil {
		return "", "armor", "", nil, nil, nil, false
	}
	found = true
	var cfg modConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		a.log("mod.json 解析失败: " + err.Error())
		return "", "armor", "", nil, nil, nil, true
	}
	cover = strings.TrimSpace(cfg.Cover)
	for _, p := range cfg.Previews {
		if p = strings.TrimSpace(p); p != "" {
			previews = append(previews, p)
		}
	}
	if len(previews) == 0 && cover != "" {
		previews = []string{cover}
	}
	valid := validateParts(cfg.Parts)
	for _, sm := range cfg.SubMods {
		var sp []string
		for _, p := range sm.Previews {
			if p = strings.TrimSpace(p); p != "" {
				sp = append(sp, p)
			}
		}
		submods = append(submods, SubModConfig{Name: strings.TrimSpace(sm.Name), Parts: validateParts(sm.Parts), Cover: strings.TrimSpace(sm.Cover), Previews: sp})
	}
	category = config.DeriveCategory(valid)
	return cfg.Nickname, category, cover, previews, valid, submods, found
}

// moveDir 移动目录：同盘直接重命名，跨盘（Rename 失败）时复制后删除源目录。
// onProgress 在跨盘复制时按文件回调进度（copied/total）。
func moveDir(src, dst string, onProgress func(copied, total int)) error {
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyDir(src, dst, onProgress); err != nil {
		return err
	}
	return os.RemoveAll(src)
}

// copyDir 递归复制目录，每复制完一个文件回调一次进度
func copyDir(src, dst string, onProgress func(copied, total int)) error {
	total := 0
	_ = filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err == nil && !fi.IsDir() {
			total++
		}
		return nil
	})
	copied := 0
	return filepath.Walk(src, func(path string, fi os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if fi.IsDir() {
			return os.MkdirAll(target, fi.Mode())
		}
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return err
		}
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		out, err := os.Create(target)
		if err != nil {
			in.Close()
			return err
		}
		_, cpErr := io.Copy(out, in)
		in.Close()
		cerr := out.Close()
		if cpErr != nil {
			return cpErr
		}
		if cerr != nil {
			return cerr
		}
		copied++
		if onProgress != nil {
			onProgress(copied, total)
		}
		return nil
	})
}

// UninstallMod 卸载 Mod：移除符号链接并清除安装状态，保留记录（文件库中显示为“未安装”）。
// 不会删除托管目录中的文件夹，因此刷新/启动扫描后不会重新出现。
func (a *App) UninstallMod(modName string) error {
	link := modLinkPath(a.cfg, modName)
	md := a.modData.Find(modName)
	if md != nil && len(md.SubMods) > 0 {
		// 组合包：父目录是真实目录（含各子 Mod 链接），整体移除
		if err := mods.DisableComposite(link); err != nil {
			a.log("卸载组合 Mod 时移除父目录失败: " + err.Error())
		}
		// 组合包卸载后，其子 Mod 一并标记为关闭（父级链接已移除，数据保持一致）
		a.modData.DisableAllSubMods(modName)
	} else if mods.IsActive(link) {
		if err := mods.Disable(link); err != nil {
			a.log("卸载 Mod 时移除符号链接失败: " + err.Error())
		}
	}
	a.modData.Uninstall(modName)
	a.syncLock(modName)
	a.log("已卸载 Mod: " + modName)
	return nil
}

// RemoveModRecord 清除 Mod 记录（物理删除记录），同时移除残留的符号链接。
// 仅用于磁盘上已缺失（文件已被删除）的 mod 清理记录。
// 普通 Mod 为符号链接，组合/HDR Mod 为游戏 Mods 目录下的真实目录（内含各子 Mod 软链），
// 两者都要整体移除，否则组合 Mod 会在游戏目录留下孤儿父目录与子 Mod 软链。
func (a *App) RemoveModRecord(modName string) error {
	if link := modLinkPath(a.cfg, modName); mods.IsActive(link) {
		var err error
		if info, e := os.Lstat(link); e == nil && info.Mode()&os.ModeSymlink == 0 {
			err = mods.DisableComposite(link)
		} else {
			err = mods.Disable(link)
		}
		if err != nil {
			a.log("清除 Mod 记录时移除残留失败: " + err.Error())
		}
	}
	a.modData.Remove(modName)
	a.log("已清除 Mod 记录: " + modName)
	return nil
}

// ScanMods 同步 Mod 数据：磁盘扫描仅用于校正数据文件
// （发现新目录/检测缺失/同步启用状态），返回结果以数据文件为准。
func (a *App) ScanMods() ([]config.ModInfo, error) {
	if a.cfg.ModsRepo == "" {
		return nil, fmt.Errorf("未设置 Mod 托管目录")
	}
	if a.cfg.GameModsDir() == "" {
		return nil, fmt.Errorf("未设置游戏目录")
	}
	discovered, err := mods.Scan(a.cfg.ModsRepo, a.cfg.GameModsDir())
	if err != nil {
		a.log("同步 Mod 数据失败: " + err.Error())
		return nil, err
	}
	scanned := make([]config.ModInfo, len(discovered))
	for i, m := range discovered {
		scanned[i] = config.ModInfo{Name: m.Name, Path: m.Path, Enabled: m.Enabled}
	}
	a.modData.Sync(scanned)
	// 效果图自动扫描：仅对列表为空的 Mod 填写，保留已人工增删的效果图
	for i := range a.modData.Mods {
		a.refreshModPreviews(&a.modData.Mods[i], true)
	}
	a.reconcileCompositeRecords()
	a.rebuildLocks()
	a.log(fmt.Sprintf("同步 Mod 数据完成，共 %d 个", len(a.modData.Mods)))
	return a.modData.Mods, nil
}

func (a *App) GetConfig() *config.App { return a.cfg }
func (a *App) GetMods() []config.ModInfo {
	a.reconcileCompositeRecords()
	return a.modData.Mods
}

// reconcileCompositeRecords 数据记录与磁盘对齐：对缺少子 Mod 的 HDR 合集记录，
// 从 mod.json（缺失时自动识别）补齐子 Mod，保证组合包始终以组合形式展示而非单个 Mod。
// 判别依据与 InstallHdrMod 一致：目录含公共 meshes/ 与 textures/ 即视为 HDR 合集；
// 仅对"无子 Mod 且目录为合集"的记录执行一次磁盘识别，补齐后下次直接跳过。
func (a *App) reconcileCompositeRecords() {
	for i := range a.modData.Mods {
		m := &a.modData.Mods[i]
		if len(m.SubMods) > 0 || m.Path == "" || m.Missing {
			continue
		}
		if !hasSubDir(m.Path, "meshes") || !hasSubDir(m.Path, "textures") {
			continue
		}
		_, _, _, _, _, submods, _ := a.readModConfig(m.Path)
		if len(submods) == 0 {
			if _, ok, err := a.ensureCompositeConfig(m.Path, m.Name); err == nil && ok {
				_, _, _, _, _, submods, _ = a.readModConfig(m.Path)
			}
		}
		if len(submods) > 0 {
			a.registerSubMods(m.Name, submods)
			a.log("已按 HDR 合集补齐子 Mod 记录: " + m.Name)
		}
	}
}

// ModConflict 资源占用冲突项：某 Mod 与另一已启用 Mod 在同一部位占用了同一资源
type ModConflict struct {
	ModName  string `json:"modName"`
	Nickname string `json:"nickname"`
	Slot     string `json:"slot"`
	Value    string `json:"value"`
}

// ModConflictInfo 单个 Mod 的冲突汇总：该 Mod 与哪些已启用 Mod 在哪些资源上冲突
type ModConflictInfo struct {
	ModName   string        `json:"modName"`
	Nickname  string        `json:"nickname"`
	Conflicts []ModConflict `json:"conflicts"`
}

// conflictsBetween 计算 curParts 与 otherParts 之间的资源占用冲突：
// 冲突 = 两个 Mod 占用同一部位（slot）且资源值（value）相同。
// 同一部位可能被多个子 Mod 占用多个不同值（如组合包内两个子 Mod 都改了胸甲），
// 只要任意一个值与对方相同即算冲突。
// other 为冲突对方 Mod（用于展示其名称）。
func conflictsBetween(curParts, otherParts map[string][]string, otherName, otherNickname string) []ModConflict {
	var res []ModConflict
	for slot, vals := range curParts {
		ovals, ok := otherParts[slot]
		if !ok || len(ovals) == 0 {
			continue
		}
		for _, v := range vals {
			for _, ov := range ovals {
				if v == ov {
					res = append(res, ModConflict{
						ModName:  otherName,
						Nickname: otherNickname,
						Slot:     slot,
						Value:    v,
					})
				}
			}
		}
	}
	return res
}

// effectiveParts 返回 Mod 实际生效的占用资源（数据文件回退版）：
// 组合 Mod（含 SubMods）父级本身不占用，取其所有已启用子 Mod 的占用；
// 同一部位若多个已启用子 Mod 占用不同值则全部保留（避免互相覆盖导致冲突漏检）。
// 普通 Mod 直接返回自身占用（武器部位可含多个值）。
func effectiveParts(m config.ModInfo) map[string][]string {
	if len(m.SubMods) > 0 {
		union := map[string][]string{}
		for _, sm := range m.SubMods {
			if sm.Enabled {
				for k, v := range sm.Parts {
					union[k] = append(union[k], v...)
				}
			}
		}
		return union
	}
	return m.Parts
}

// ============================================================
// 全局占用锁文件 parts.json（程序目录下唯一一份）：
// 记录当前已启用 Mod 实际占用的装备资源（占用锁）。
// 启用 Mod 时把其占用写入该文件，关闭时移出；冲突检测直接读该文件判定。
// 组合 Mod 存已启用子 Mod 的占用并集；数据文件 parts 仅作展示。
// ============================================================

// lockEntry 全局占用锁文件中的一条：某个已启用 Mod 及其占用的资源
// （同一部位可被组合包内多个子 Mod 占用多个不同值，故 value 为列表）
type lockEntry struct {
	Name  string              `json:"name"`
	Parts map[string][]string `json:"parts"`
}

// lockFile 全局占用锁文件内容
type lockFile struct {
	Mods []lockEntry `json:"mods"`
}

// locksFilePath 全局占用锁文件路径（程序目录下 parts.json，随程序便携），测试可覆盖
var locksFilePath = func() string {
	exe, err := os.Executable()
	if err != nil {
		return "parts.json"
	}
	return filepath.Join(filepath.Dir(exe), "parts.json")
}()

// readLocks 读取全局占用锁文件，返回 modName -> 占用资源（文件缺失/损坏时为空表）
func readLocks() map[string]map[string][]string {
	m := map[string]map[string][]string{}
	data, err := os.ReadFile(locksFilePath)
	if err != nil {
		return m
	}
	var f lockFile
	if err := json.Unmarshal(data, &f); err != nil {
		return m
	}
	for _, e := range f.Mods {
		m[e.Name] = e.Parts
	}
	return m
}

// writeLocks 把锁表写回全局文件（按名称排序保证输出稳定）
func writeLocks(m map[string]map[string][]string) error {
	names := make([]string, 0, len(m))
	for n := range m {
		names = append(names, n)
	}
	sort.Strings(names)
	f := lockFile{}
	for _, n := range names {
		f.Mods = append(f.Mods, lockEntry{Name: n, Parts: m[n]})
	}
	data, err := json.MarshalIndent(f, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(locksFilePath, data, 0644)
}

// syncLock 按数据文件当前状态把某个 Mod 的占用写入/移出全局锁文件：
// 已启用且有实际占用 → 写入；否则移出。在启用/关闭/子 Mod 开关/占用修改后调用。
func (a *App) syncLock(modName string) {
	m := readLocks()
	md := a.modData.Find(modName)
	if md == nil || !md.Enabled {
		delete(m, modName)
	} else if parts := effectiveParts(*md); len(parts) > 0 {
		m[modName] = parts
	} else {
		delete(m, modName)
	}
	if err := writeLocks(m); err != nil {
		a.log("更新全局占用锁 parts.json 失败 [" + modName + "]: " + err.Error())
	}
}

// rebuildLocks 按数据文件的启用状态重建全局占用锁文件（启动/刷新时调用），
// 保证锁文件与当前启用状态一致，作为冲突检测的唯一依据。
func (a *App) rebuildLocks() {
	m := map[string]map[string][]string{}
	for _, md := range a.modData.Mods {
		if !md.Enabled {
			continue
		}
		if parts := effectiveParts(md); len(parts) > 0 {
			m[md.Name] = parts
		}
	}
	if err := writeLocks(m); err != nil {
		a.log("重建全局占用锁 parts.json 失败: " + err.Error())
	}
}

// CheckModConflicts 检查启用 modName 时，与已启用 Mod 之间的资源占用冲突。
// 冲突依据为全局占用锁文件 parts.json 中其他已启用 Mod 的占用。
// 仅返回信息供前端提醒，不阻止操作。
func (a *App) CheckModConflicts(modName string) []ModConflict {
	md := a.modData.Find(modName)
	if md == nil {
		return nil
	}
	curParts := effectiveParts(*md)
	if len(curParts) == 0 {
		return nil
	}
	locks := readLocks()
	var res []ModConflict
	for otherName, otherParts := range locks {
		if otherName == modName {
			continue
		}
		nickname := ""
		if other := a.modData.Find(otherName); other != nil {
			nickname = other.Nickname
		}
		res = append(res, conflictsBetween(curParts, otherParts, otherName, nickname)...)
	}
	return res
}

// CheckAllModConflicts 检查全部已启用 Mod 之间的资源占用冲突。
// 冲突依据为全局占用锁文件 parts.json；返回值非空即表示存在冲突。
func (a *App) CheckAllModConflicts() []ModConflictInfo {
	locks := readLocks()
	names := make([]string, 0, len(locks))
	for n := range locks {
		names = append(names, n)
	}
	sort.Strings(names)
	var res []ModConflictInfo
	for i, name := range names {
		parts := locks[name]
		var confs []ModConflict
		for j, otherName := range names {
			if i == j {
				continue
			}
			nickname := ""
			if other := a.modData.Find(otherName); other != nil {
				nickname = other.Nickname
			}
			confs = append(confs, conflictsBetween(parts, locks[otherName], otherName, nickname)...)
		}
		if len(confs) > 0 {
			nickname := ""
			if m := a.modData.Find(name); m != nil {
				nickname = m.Nickname
			}
			res = append(res, ModConflictInfo{
				ModName:   name,
				Nickname:  nickname,
				Conflicts: confs,
			})
		}
	}
	return res
}

func (a *App) SetModsRepo(repo string) error {
	a.cfg.ModsRepo = repo
	if err := a.cfg.Save(); err != nil {
		a.log("保存 Mod 托管目录失败: " + err.Error())
		return err
	}
	a.log("Mod 托管目录: " + repo)
	_, err := a.ScanMods()
	return err
}
func (a *App) SetGameRoot(root string) error {
	a.cfg.GameRoot = root
	if err := a.cfg.Save(); err != nil {
		a.log("保存游戏目录失败: " + err.Error())
		return err
	}
	a.log("游戏目录: " + root)
	return nil
}

// SetModCover 设置 Mod 封面，统一存储为相对 Mod 文件夹的路径：
//   - 传入绝对路径 → 先把图片拷贝进 Mod 文件夹并重命名为 cover.<原扩展名>，保存相对路径
//   - 传入相对路径 → 原样保存（需已存在于 Mod 文件夹内）
//   - 空字符串 → 清除封面
//
// 返回最终保存的封面路径（相对 Mod 文件夹）。
func (a *App) SetModCover(modName, coverPath string) (string, error) {
	md := a.modData.Find(modName)
	if md == nil {
		return "", fmt.Errorf("Mod 不存在: %s", modName)
	}
	modDir := md.Path
	if modDir == "" {
		modDir = filepath.Join(a.cfg.ModsRepo, modName)
	}
	coverPath = strings.TrimSpace(coverPath)
	if coverPath == "" {
		a.modData.SetCover(modName, "")
		a.log("已清除 Mod 封面: " + modName)
		return "", nil
	}
	saved := coverPath
	if filepath.IsAbs(coverPath) {
		ext := strings.ToLower(filepath.Ext(coverPath))
		if ext == "" {
			ext = ".png"
		}
		target := filepath.Join(modDir, "cover"+ext)
		if err := copyFile(coverPath, target); err != nil {
			return "", fmt.Errorf("拷贝封面图片失败: %w", err)
		}
		saved = filepath.Base(target)
	}
	a.modData.SetCover(modName, saved)
	a.log("更新 Mod 封面 [" + modName + "]: " + saved)
	return saved, nil
}

// copyFile 复制单个文件（src → dst），dst 目录需已存在
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return nil
}

// GetArmorParts 返回按身体部位分组的装备资源列表（供服装 Mod 选择占用资源）
func (a *App) GetArmorParts() []armordata.Slot {
	return armordata.AllSlots()
}

// GetWeaponParts 返回全部武器资源列表（供武器 Mod 选择占用的武器）
func (a *App) GetWeaponParts() []armordata.Part {
	return armordata.AllWeapons()
}

// SetModParts 保存 Mod 占用的装备资源（部位 -> 资源ID列表，空 map 表示清除）
func (a *App) SetModParts(modName string, parts map[string][]string) error {
	a.modData.SetParts(modName, parts)
	a.syncLock(modName)
	a.log("更新 Mod 资源占用 [" + modName + "]")
	return nil
}

// SetSubModParts 手动设置组合 Mod 内某个子 Mod 的占用资源（自动解析失败时供人工填写），
// 内部会重算父级并集占用与分类，并同步写回合集目录的 mod.json（缺失的子 Mod 条目一并补上）。
func (a *App) SetSubModParts(modName, subName string, parts map[string][]string) error {
	md := a.modData.Find(modName)
	if md == nil {
		return fmt.Errorf("Mod 不存在: %s", modName)
	}
	found := false
	for _, sm := range md.SubMods {
		if sm.Name == subName {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("子 Mod 不存在: %s", subName)
	}
	a.modData.SetSubModParts(modName, subName, parts)
	a.syncLock(modName)
	a.log(fmt.Sprintf("已更新子 Mod 资源占用 [%s]: %s", modName, subName))
	return nil
}

// GenerateSubModModJson 手动生成/刷新 mod.json：
// 组合包写入全部子 Mod 占用，普通 Mod 写入自身占用；已存在则更新、缺失则追加，文件不存在时按数据仓库信息生成。
func (a *App) GenerateSubModModJson(modName string) error {
	md := a.modData.Find(modName)
	if md == nil {
		return fmt.Errorf("Mod 不存在: %s", modName)
	}
	modDir := md.Path
	if modDir == "" {
		modDir = filepath.Join(a.cfg.ModsRepo, modName)
	}
	modJSON := filepath.Join(modDir, "mod.json")
	cfg := &modConfig{Nickname: md.Nickname, Cover: md.Cover, Category: md.Category}
	if data, err := os.ReadFile(modJSON); err == nil {
		if uerr := json.Unmarshal(data, cfg); uerr != nil {
			return fmt.Errorf("mod.json 解析失败: %s", uerr.Error())
		}
	}
	if cfg.Nickname == "" {
		cfg.Nickname = md.Nickname
	}
	cfg.Previews = md.Preview
	if len(md.SubMods) > 0 {
		for _, sm := range md.SubMods {
			updated := false
			for i := range cfg.SubMods {
				if cfg.SubMods[i].Name == sm.Name {
					cfg.SubMods[i].Parts = orderedParts(sm.Parts)
					if cfg.SubMods[i].Cover == "" {
						cfg.SubMods[i].Cover = sm.Cover
					}
					cfg.SubMods[i].Previews = sm.Preview
					updated = true
					break
				}
			}
			if !updated {
				cfg.SubMods = append(cfg.SubMods, SubModConfig{Name: sm.Name, Parts: orderedParts(sm.Parts), Cover: sm.Cover, Previews: sm.Preview})
			}
		}
	} else {
		cfg.Parts = orderedParts(md.Parts)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(modJSON, data, 0644); err != nil {
		return err
	}
	a.syncLock(modName)
	a.log("已生成 mod.json: " + modName)
	return nil
}

// SetModCategory 保存 Mod 分类（armor=服装 / weapon=武器）
func (a *App) SetModCategory(modName, category string) error {
	a.modData.SetCategory(modName, category)
	a.log("更新 Mod 分类 [" + modName + "]: " + category)
	return nil
}

func (a *App) SetModNickname(modName, nickname string) error {
	a.modData.SetNickname(modName, nickname)
	a.log("更新 Mod 昵称 [" + modName + "]: " + nickname)
	return nil
}

func (a *App) SelectDirectory() (string, error) {
	return dialog.SelectDirectory("请选择目录")
}

func (a *App) SelectImageFile() (string, error) {
	return dialog.SelectFile("选择封面图片", "图片文件(*.png;*.jpg;*.jpeg)\x00*.png;*.jpg;*.jpeg\x00所有文件(*.*)\x00*.*\x00")
}

func (a *App) OpenDirectory(dir string) error {
	return exec.Command("explorer", dir).Start()
}

// LaunchGame 启动游戏（优先使用根目录下的 nioh2.exe，否则取根目录下第一个 exe）
func (a *App) LaunchGame() error {
	root := a.cfg.GameRoot
	if root == "" {
		return fmt.Errorf("未设置游戏目录")
	}
	exe := filepath.Join(root, "nioh2.exe")
	if _, err := os.Stat(exe); err != nil {
		matches, gerr := filepath.Glob(filepath.Join(root, "*.exe"))
		if gerr != nil || len(matches) == 0 {
			return fmt.Errorf("未找到游戏可执行文件")
		}
		exe = matches[0]
	}
	if err := exec.Command(exe).Start(); err != nil {
		a.log("启动游戏失败: " + err.Error())
		return err
	}
	a.log("已启动游戏: " + exe)
	return nil
}

// AboutInfo 关于页条目
type AboutInfo struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// wailsProjectInfo 对应 wails.json 的 info 配置（唯一版本/产品信息来源）
type wailsProjectInfo struct {
	Info struct {
		ProductName    string `json:"productName"`
		ProductVersion string `json:"productVersion"`
		CompanyName    string `json:"companyName"`
		Copyright      string `json:"copyright"`
		Comments       string `json:"comments"`
	} `json:"info"`
}

// projectInfo 解析内嵌的 wails.json，字段缺失时返回对应默认值。
func projectInfo() wailsProjectInfo {
	var w wailsProjectInfo
	_ = json.Unmarshal(wailsJsonContent, &w)
	if w.Info.ProductName == "" {
		w.Info.ProductName = "Nioh2ModManager"
	}
	if w.Info.ProductVersion == "" {
		w.Info.ProductVersion = "0.0.1"
	}
	if w.Info.Copyright == "" {
		w.Info.Copyright = "©shiki"
	}
	return w
}

// GetAbout 返回关于页信息（有序）。数据全部来自 wails.json 的 info 段，改一处即可。
func (a *App) GetAbout() []AboutInfo {
	pi := projectInfo()
	return []AboutInfo{
		{"名称", pi.Info.ProductName},
		{"版本", pi.Info.ProductVersion},
		{"作者", pi.Info.Copyright},
	}
}

// AppVersion 当前应用版本（对应 wails.json 的 info.productVersion，检查更新对比用）。
// 与打包/关于页共用同一版本号，只需修改 wails.json 一处。
var AppVersion = projectInfo().Info.ProductVersion

// UpdateInfo 检查更新结果
type UpdateInfo struct {
	CurrentVersion string `json:"currentVersion"` // 当前版本
	LatestVersion  string `json:"latestVersion"`  // 最新版本（远端清单）
	DownloadURL    string `json:"downloadUrl"`    // 新版本下载地址
	Notes          string `json:"notes"`          // 更新说明
	HasUpdate      bool   `json:"hasUpdate"`      // 是否有新版本
	Message        string `json:"message"`        // 提示文案
}

// versionManifest 远端版本清单约定格式（如 version.json）：
// { "version": "0.2.0", "url": "https://.../setup.exe", "notes": "更新说明" }
type versionManifest struct {
	Version     string `json:"version"`
	DownloadURL string `json:"url,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

// CheckUpdate 检查是否有新版本：
// 未配置更新源时返回提示；已配置时拉取版本清单并与当前版本比对。
// 已预留接口：后续接入 GitHub / Gitee Releases 时只需调整 versionManifest 与下载地址解析。
func (a *App) CheckUpdate() *UpdateInfo {
	url := strings.TrimSpace(a.cfg.UpdateURL)
	if url == "" {
		return &UpdateInfo{CurrentVersion: AppVersion, Message: "未配置更新源，请在设置中填写更新接口地址"}
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		a.log("检查更新失败: " + err.Error())
		return &UpdateInfo{CurrentVersion: AppVersion, Message: "检查更新失败（网络或地址有误）"}
	}
	defer resp.Body.Close()
	var m versionManifest
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil {
		a.log("更新清单解析失败: " + err.Error())
		return &UpdateInfo{CurrentVersion: AppVersion, Message: "更新清单解析失败"}
	}
	info := &UpdateInfo{CurrentVersion: AppVersion, LatestVersion: m.Version, DownloadURL: m.DownloadURL, Notes: m.Notes}
	if compareVersions(m.Version, AppVersion) > 0 {
		info.HasUpdate = true
		info.Message = "发现新版本 " + m.Version
		a.log("发现新版本: " + m.Version)
	} else {
		info.Message = "当前已是最新版本"
	}
	return info
}

// SetUpdateUrl 保存更新源地址（版本清单 URL）
func (a *App) SetUpdateUrl(url string) error {
	a.cfg.UpdateURL = strings.TrimSpace(url)
	a.cfg.Save()
	a.log("已更新更新源地址: " + a.cfg.UpdateURL)
	return nil
}

// compareVersions 简单版本号比较：按 "." 分段逐段比较，返回正数表示 a>b。
func compareVersions(a, b string) int {
	sa, sb := strings.Split(a, "."), strings.Split(b, ".")
	n := len(sa)
	if len(sb) > n {
		n = len(sb)
	}
	for i := 0; i < n; i++ {
		var x, y int
		fmt.Sscanf(seg(i, sa), "%d", &x)
		fmt.Sscanf(seg(i, sb), "%d", &y)
		if x > y {
			return 1
		}
		if x < y {
			return -1
		}
	}
	return 0
}

func seg(i int, parts []string) string {
	if i < len(parts) {
		return parts[i]
	}
	return "0"
}

// ---- 日志 ----

func (a *App) log(msg string) {
	a.logData.Append(msg)
}

// GetLogs 返回全部操作日志（从后端读取）
func (a *App) GetLogs() []config.LogEntry {
	return a.logData.All()
}

// AddLog 追加一条操作日志（供前端写入/记录操作与结果），返回全部日志
func (a *App) AddLog(message string) []config.LogEntry {
	a.logData.Append(message)
	return a.logData.All()
}

// ClearLogs 清空操作日志
func (a *App) ClearLogs() error {
	a.logData.Clear()
	return nil
}

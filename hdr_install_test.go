package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"nioh2mod-js/internal/config"
	"nioh2mod-js/internal/mods"
)

// compositeEntries 应识别公共条目（meshes/textures 等）、子 Mod 目录与可选变体（DISABLED/THEME COLOR）
func TestCompositeEntries(t *testing.T) {
	dir := t.TempDir()
	mkdir(t, filepath.Join(dir, "meshes"))
	mkdir(t, filepath.Join(dir, "textures"))
	mkdir(t, filepath.Join(dir, "01_[A] Special Costume Set"))
	writeFile(t, filepath.Join(dir, "01_[A] Special Costume Set", "mod.ini"), "[mod]\n")
	mkdir(t, filepath.Join(dir, "02_[B] Demon Slayer"))
	writeFile(t, filepath.Join(dir, "02_[B] Demon Slayer", "mod.ini"), "[mod]\n")
	mkdir(t, filepath.Join(dir, "DISABLED 03_[C] old variant"))
	writeFile(t, filepath.Join(dir, "DISABLED 03_[C] old variant", "mod.ini"), "[mod]\n")
	mkdir(t, filepath.Join(dir, "THEME COLOR OPTIONS [04]"))
	writeFile(t, filepath.Join(dir, "preview.jpg"), "x")
	writeFile(t, filepath.Join(dir, "Read Me.txt"), "x")

	shared, subMods, optional := compositeEntries(dir)
	contains := func(list []string, want string) bool {
		for _, s := range list {
			if s == want {
				return true
			}
		}
		return false
	}
	for _, want := range []string{"meshes", "textures", "preview.jpg", "Read Me.txt"} {
		if !contains(shared, want) {
			t.Errorf("公共条目缺少 %q: %v", want, shared)
		}
	}
	if len(subMods) != 2 || !contains(subMods, "01_[A] Special Costume Set") || !contains(subMods, "02_[B] Demon Slayer") {
		t.Errorf("子 Mod 识别错误: %v", subMods)
	}
	if !contains(optional, "DISABLED 03_[C] old variant") || !contains(optional, "THEME COLOR OPTIONS [04]") {
		t.Errorf("可选变体识别错误: %v", optional)
	}
	// 公共条目中不应包含子 Mod 或可选变体
	for _, bad := range subMods {
		if contains(shared, bad) {
			t.Errorf("公共条目误包含子 Mod %q", bad)
		}
	}
	for _, bad := range optional {
		if contains(shared, bad) {
			t.Errorf("公共条目误包含可选变体 %q", bad)
		}
	}
}

// SetSubMods 登记子 Mod：父级 Parts 存并集，Category 由并集推导
func TestSetSubModsUnion(t *testing.T) {
	md := &config.ModData{Mods: []config.ModInfo{{Name: "HDR Test", Path: "dummy"}}}
	subs := []config.SubModInfo{
		{Name: "01_[A]", Parts: map[string][]string{"胸甲": {"讨魔首领之铠"}, "头": {"鬼玄之兜"}}},
		{Name: "02_[B]", Parts: map[string][]string{"膝甲": {"伴御众之铠"}}},
	}
	md.SetSubMods("HDR Test", subs)
	got := md.Find("HDR Test")
	if got == nil {
		t.Fatal("Mod 未登记")
	}
	if len(got.Parts) != 3 {
		t.Fatalf("并集应含 3 个占用，得到 %v", got.Parts)
	}
	if got.Category != "armor" {
		t.Fatalf("并集推导分类期望 armor，得到 %q", got.Category)
	}
	if len(got.SubMods) != 2 {
		t.Fatalf("子 Mod 数量错误: %d", len(got.SubMods))
	}

	md.SetSubModEnabled("HDR Test", "01_[A]", true)
	if !md.Find("HDR Test").SubMods[0].Enabled {
		t.Fatal("子 Mod 启用状态未持久化")
	}
	if md.Find("HDR Test").SubMods[1].Enabled {
		t.Fatal("未启用的子 Mod 不应为 true")
	}
}

// 组合 Mod 嵌套符号链接：父目录建公共链接 + 子 Mod 链接，禁用/启用行为正确
// 目录符号链接在 Windows 需管理员/开发者模式，权限不足时跳过。
func TestCompositeSymlinks(t *testing.T) {
	repo := t.TempDir()
	parentDir := filepath.Join(repo, "HDR Collection")
	linkDir := filepath.Join(repo, "game_mods", "HDR Collection")

	// 构造合集目录
	mkdir(t, filepath.Join(parentDir, "meshes"))
	writeFile(t, filepath.Join(parentDir, "meshes", "a.bin"), "x")
	mkdir(t, filepath.Join(parentDir, "textures"))
	mkdir(t, filepath.Join(parentDir, "01_[A]"))
	writeFile(t, filepath.Join(parentDir, "01_[A]", "mod.ini"), "[mod]\n")
	mkdir(t, filepath.Join(parentDir, "02_[B]"))
	writeFile(t, filepath.Join(parentDir, "02_[B]", "mod.ini"), "[mod]\n")

	// 校验当前环境能否创建目录符号链接，失败则跳过
	probe := filepath.Join(repo, "probe")
	if err := os.Symlink(parentDir, probe); err != nil {
		t.Skipf("当前环境无法创建符号链接（需管理员或开发者模式）: %v", err)
	}
	os.Remove(probe)

	if err := mods.EnableComposite(parentDir, linkDir, []string{"meshes", "textures"}, []string{"01_[A]"}); err != nil {
		t.Fatalf("EnableComposite 失败: %v", err)
	}
	if !mods.CompositeEnabled(linkDir) {
		t.Fatal("CompositeEnabled 应为 true")
	}
	// 公共链接与启用的子 Mod 应存在
	for _, name := range []string{"meshes", "textures", "01_[A]"} {
		info, err := os.Lstat(filepath.Join(linkDir, name))
		if err != nil {
			t.Fatalf("链接缺失 %s: %v", name, err)
		}
		if info.Mode()&os.ModeSymlink == 0 {
			t.Fatalf("%s 不是符号链接", name)
		}
	}
	// 未启用的子 Mod 不应存在
	if _, err := os.Lstat(filepath.Join(linkDir, "02_[B]")); err == nil {
		t.Fatal("未启用的子 Mod 不应有链接")
	}

	// 启用第二个子 Mod
	if err := mods.EnableSubMod(parentDir, linkDir, "02_[B]"); err != nil {
		t.Fatalf("EnableSubMod 失败: %v", err)
	}
	if info, err := os.Lstat(filepath.Join(linkDir, "02_[B]")); err != nil || info.Mode()&os.ModeSymlink == 0 {
		t.Fatal("子 Mod 链接应存在")
	}

	// 禁用单个子 Mod
	if err := mods.DisableSubMod(linkDir, "01_[A]"); err != nil {
		t.Fatalf("DisableSubMod 失败: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(linkDir, "01_[A]")); err == nil {
		t.Fatal("子 Mod 链接应已移除")
	}
	if !mods.CompositeEnabled(linkDir) {
		t.Fatal("父目录应仍启用")
	}

	// 禁用整个组合：父目录整体移除
	if err := mods.DisableComposite(linkDir); err != nil {
		t.Fatalf("DisableComposite 失败: %v", err)
	}
	if mods.CompositeEnabled(linkDir) {
		t.Fatal("父目录应已禁用")
	}
}

// 组合 Mod 目录内应保证只有合法的 ini 引用资源被链接；meshes 链接后子 Mod ini 的 ..\meshes 相对引用成立
func TestCompositeSharedResolution(t *testing.T) {
	repo := t.TempDir()
	parentDir := filepath.Join(repo, "HDR Collection")
	linkDir := filepath.Join(repo, "game_mods", "HDR Collection")

	mkdir(t, filepath.Join(parentDir, "meshes", "DemonSlayer"))
	writeFile(t, filepath.Join(parentDir, "meshes", "DemonSlayer", "x.bin"), "x")
	mkdir(t, filepath.Join(parentDir, "01_[A]"))
	ini := "filename = ..\\meshes\\DemonSlayer\\x.bin\n"
	writeFile(t, filepath.Join(parentDir, "01_[A]", "mod.ini"), ini)

	if err := os.Symlink(parentDir, filepath.Join(repo, "probe")); err != nil {
		t.Skipf("当前环境无法创建符号链接: %v", err)
	}

	if err := mods.EnableComposite(parentDir, linkDir, []string{"meshes"}, []string{"01_[A]"}); err != nil {
		t.Fatalf("EnableComposite 失败: %v", err)
	}
	// 通过链接路径应能访问到子 Mod ini 引用的共享网格
	target := filepath.Join(linkDir, "01_[A]", "..", "meshes", "DemonSlayer", "x.bin")
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("相对引用跨链接解析失败: %v", err)
	}
	if strings.TrimSpace(string(data)) != "x" {
		t.Fatalf("内容不符: %q", string(data))
	}
}

// InstallHdrMod 端到端：预置带 submods 的 mod.json 的合集目录
// → 移入托管 → 按子 Mod 登记（父级不占用，并集写入 Parts）
func TestInstallHdrModEndToEnd(t *testing.T) {
	repo := t.TempDir()
	src := t.TempDir()
	app := &App{cfg: &config.App{ModsRepo: repo}, modData: &config.ModData{}, logData: &config.LogData{}}

	// 构造 HDR 合集（含公共 meshes/textures 与两个子 Mod）
	mkdir(t, filepath.Join(src, "meshes"))
	mkdir(t, filepath.Join(src, "textures"))
	mkdir(t, filepath.Join(src, "01_[A] Costume Set"))
	mkdir(t, filepath.Join(src, "02_[B] Demon Slayer"))
	writeFile(t, filepath.Join(src, "01_[A] Costume Set", "mod.ini"), "[mod]\n")
	writeFile(t, filepath.Join(src, "02_[B] Demon Slayer", "mod.ini"), "[mod]\n")
	cfgJSON := `{
  "nickname": "测试HDR合集",
  "submods": [
    {"name": "01_[A] Costume Set", "parts": {"胸甲": "讨魔首领之铠-胸甲", "臂甲": "守护大名之铠-臂甲"}},
    {"name": "02_[B] Demon Slayer", "parts": {"膝甲": "日轮之子之铠-膝甲", "腿甲": "刑部轻铠-腿甲"}}
  ]
}`
	writeFile(t, filepath.Join(src, "mod.json"), cfgJSON)

	res, err := app.InstallHdrMod(src)
	if err != nil {
		t.Fatalf("InstallHdrMod 失败: %v", err)
	}
	if res.Nickname != "测试HDR合集" {
		t.Fatalf("昵称错误: %q", res.Nickname)
	}
	if len(res.SubMods) != 2 {
		t.Fatalf("期望 2 个子 Mod，得到 %d", len(res.SubMods))
	}
	md := app.modData.Find(res.Name)
	if md == nil {
		t.Fatal("Mod 未登记")
	}
	if len(md.SubMods) != 2 {
		t.Fatalf("数据仓库子 Mod 数量错误: %d", len(md.SubMods))
	}
	if !md.Installed {
		t.Fatal("HDR 合集安装后应标记为已安装")
	}
	// 父级不占用：并集 = 4 个部位
	if len(md.Parts) != 4 {
		t.Fatalf("并集应含 4 个占用，得到 %v", md.Parts)
	}
	// 原目录应已移走（moveDir）
	if _, err := os.Stat(src); !os.IsNotExist(err) {
		t.Fatalf("原合集目录应已移动: %v", err)
	}
	// 非合集目录应报错
	plain := t.TempDir()
	mkdir(t, filepath.Join(plain, "x"))
	if _, err := app.InstallHdrMod(plain); err == nil {
		t.Fatal("非合集目录不应安装成功")
	}
}

// ImportMod 对已在托管目录内的目录应直接登记，而不是尝试移动进自己
func TestImportModAlreadyInRepo(t *testing.T) {
	repo := t.TempDir()
	col := filepath.Join(repo, "My Mod")
	mkdir(t, col)
	app := &App{cfg: &config.App{ModsRepo: repo}, modData: &config.ModData{}, logData: &config.LogData{}}
	res, err := app.ImportMod(col)
	if err != nil {
		t.Fatalf("已在托管目录内的目录应直接登记: %v", err)
	}
	if res.Name != "My Mod" {
		t.Fatalf("名称错误: %q", res.Name)
	}
	if app.modData.Find("My Mod") == nil {
		t.Fatal("Mod 未登记")
	}
	// 目录应原地保留，未被移动/删除
	if _, err := os.Stat(col); err != nil {
		t.Fatalf("原目录应保留: %v", err)
	}
}

// Sync 必须保留已有记录的子 Mod/分类/占用（磁盘扫描结果不含这些字段，合并时不得覆盖丢失）
func TestSyncPreservesSubMods(t *testing.T) {
	md := &config.ModData{Mods: []config.ModInfo{
		{
			Name: "HDR", Path: "x", Installed: true, Category: "armor",
			Parts:   map[string][]string{"胸甲": {"讨魔首领之铠-胸甲"}},
			SubMods: []config.SubModInfo{{Name: "01_[A]", Parts: map[string][]string{"胸甲": {"讨魔首领之铠-胸甲"}}, Enabled: true}},
		},
	}}
	// 模拟磁盘扫描结果：只有 Name/Path/Enabled
	md.Sync([]config.ModInfo{{Name: "HDR", Path: "x", Enabled: false}})
	got := md.Find("HDR")
	if got == nil {
		t.Fatal("未找到 Mod")
	}
	if len(got.SubMods) != 1 || !got.SubMods[0].Enabled {
		t.Fatalf("Sync 应保留 SubMods 及其启用状态: %+v", got.SubMods)
	}
	if got.Category != "armor" {
		t.Fatalf("Sync 应保留 Category: %q", got.Category)
	}
	if _, ok := got.Parts["胸甲"]; !ok {
		t.Fatalf("Sync 应保留 Parts: %v", got.Parts)
	}
	if !got.Installed {
		t.Fatal("Sync 应保留 Installed")
	}
}

// 父组合包关闭时，子 Mod 的启用状态应一并关闭（避免"父级关闭但子 Mod 仍显示已启用"）
func TestDisableAllSubMods(t *testing.T) {
	md := &config.ModData{Mods: []config.ModInfo{
		{
			Name: "HDR Test", Path: "dummy", Enabled: true,
			SubMods: []config.SubModInfo{
				{Name: "01_[A]", Enabled: true},
				{Name: "02_[B]", Enabled: false},
			},
		},
	}}
	md.DisableAllSubMods("HDR Test")
	got := md.Find("HDR Test")
	if got == nil {
		t.Fatal("Mod 未找到")
	}
	for _, sm := range got.SubMods {
		if sm.Enabled {
			t.Fatalf("父级关闭后子 Mod %q 应置为关闭", sm.Name)
		}
	}
}

// SetSubModParts 手动填写子 Mod 占用后，父级并集与分类应随之重算
func TestSetSubModPartsRecalculatesUnion(t *testing.T) {
	md := &config.ModData{Mods: []config.ModInfo{
		{
			Name: "HDR", Path: "x", Installed: true,
			SubMods: []config.SubModInfo{
				{Name: "01_[A]", Parts: map[string][]string{"胸甲": {"讨魔首领之铠-胸甲"}}},
				{Name: "02_[B]", Parts: map[string][]string{"腿甲": {"讨魔首领之铠-腿甲"}}},
			},
		},
	}}
	// 模拟自动解析失败后人工补填：子 Mod B 改为占用武器
	md.SetSubModParts("HDR", "02_[B]", map[string][]string{"武器": {"大太刀"}})
	got := md.Find("HDR")
	if got == nil {
		t.Fatal("未找到 Mod")
	}
	if len(got.SubMods) != 2 {
		t.Fatalf("子 Mod 数量错误: %d", len(got.SubMods))
	}
	if len(got.SubMods[1].Parts["武器"]) != 1 || got.SubMods[1].Parts["武器"][0] != "大太刀" {
		t.Fatalf("子 Mod 占用未更新: %v", got.SubMods[1].Parts)
	}
	// 父级并集应 = 子A(胸甲) + 子B(武器)
	if len(got.Parts["胸甲"]) != 1 || got.Parts["胸甲"][0] != "讨魔首领之铠-胸甲" ||
		len(got.Parts["武器"]) != 1 || got.Parts["武器"][0] != "大太刀" {
		t.Fatalf("父级并集错误: %v", got.Parts)
	}
	if got.Category != "mixed" {
		t.Fatalf("父级分类应重算为 mixed，得到 %q", got.Category)
	}
}

// subModCover 应在子 Mod 目录内找到首个图片并返回相对合集路径
func TestSubModCover(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "02_[A]")
	if err := os.MkdirAll(filepath.Join(sub, "subdir"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "preview.png"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "subdir", "x.jpg"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sub, "tex.dds"), []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}
	if got := subModCover(sub, "02_[A]"); got != "02_[A]/preview.png" {
		t.Fatalf("子 Mod 封面应取首个图片的相对路径，得到 %q", got)
	}
	empty := filepath.Join(root, "noimg")
	if err := os.MkdirAll(empty, 0755); err != nil {
		t.Fatal(err)
	}
	if got := subModCover(empty, "noimg"); got != "" {
		t.Fatalf("无图片时应返回空串，得到 %q", got)
	}
}

// 构造一个不含 mod.json 的 HDR 合集（含公共 meshes/textures 与一个可解析子 Mod）
func makeHdrCollection(t *testing.T, dir string) {
	t.Helper()
	mkdir(t, filepath.Join(dir, "meshes"))
	mkdir(t, filepath.Join(dir, "textures"))
	subA := filepath.Join(dir, "01_[A] Special Costume Set")
	mkdir(t, subA)
	writeGBK(t, filepath.Join(subA, "01 TYPE [A] Read Me.txt"),
		";-01_[A] Special Costume Set\r\n"+
			";-生效幻化素材: \\ Effective Materials:\r\n"+
			";-胸 - 讨魔首领 \\ Chest - DemonSlayer\r\n")
	writeFile(t, filepath.Join(subA, "[01A] A1 - x.ini"), "hash = 1234\n")
}

// 回归：卸载后经普通"安装 Mod"流程重装的 HDR 合集（无 mod.json），
// ImportMod 应自动识别为组合包并登记子 Mod，而不是当作单个 Mod
func TestImportModAutoDetectHdrCollection(t *testing.T) {
	repo := t.TempDir()
	src := t.TempDir()
	makeHdrCollection(t, filepath.Join(src, "01_HDR Test Collection"))

	app := &App{cfg: &config.App{ModsRepo: repo}, modData: &config.ModData{}, logData: &config.LogData{}}
	res, err := app.ImportMod(filepath.Join(src, "01_HDR Test Collection"))
	if err != nil {
		t.Fatalf("ImportMod 失败: %v", err)
	}
	if len(res.SubMods) != 1 {
		t.Fatalf("期望自动识别为 1 个子 Mod，得到 %d", len(res.SubMods))
	}
	md := app.modData.Find(res.Name)
	if md == nil {
		t.Fatal("Mod 未登记")
	}
	if len(md.SubMods) != 1 {
		t.Fatalf("数据仓库子 Mod 数量错误: %d", len(md.SubMods))
	}
	// 识别后应生成 mod.json
	if _, err := os.Stat(filepath.Join(repo, res.Name, "mod.json")); err != nil {
		t.Fatalf("识别后应生成 mod.json: %v", err)
	}
}

// 回归：以单个 Mod 形式登记过的 HDR 合集重装时，GetModConfig 应自动识别组合包
// 并登记子 Mod，保证安装弹窗以组合包形式展示（而非单个 Mod 表单）
func TestGetModConfigAutoDetectHdrCollection(t *testing.T) {
	repo := t.TempDir()
	name := "01_HDR Test Collection"
	col := filepath.Join(repo, name)
	makeHdrCollection(t, col)

	app := &App{cfg: &config.App{ModsRepo: repo}, modData: &config.ModData{}, logData: &config.LogData{}}
	app.modData.Upsert(name, col)
	res, err := app.GetModConfig(name)
	if err != nil {
		t.Fatalf("GetModConfig 失败: %v", err)
	}
	if len(res.SubMods) != 1 {
		t.Fatalf("期望识别为 1 个子 Mod，得到 %d", len(res.SubMods))
	}
	md := app.modData.Find(name)
	if md == nil {
		t.Fatal("Mod 未登记")
	}
	if len(md.SubMods) != 1 {
		t.Fatalf("GetModConfig 后数据仓库应登记子 Mod，得到 %d", len(md.SubMods))
	}
}

// 回归：历史以单个 Mod 形式登记的 HDR 合集记录，在 GetMods/ScanMods 时应自动
// 从磁盘（mod.json / 自动识别）补齐子 Mod，卡片恢复"组合包"展示
func TestReconcileCompositeRecords(t *testing.T) {
	repo := t.TempDir()
	name := "01_HDR Test Collection"
	col := filepath.Join(repo, name)
	makeHdrCollection(t, col)

	app := &App{cfg: &config.App{ModsRepo: repo}, modData: &config.ModData{}, logData: &config.LogData{}}
	// 模拟历史记录：仅以单个 Mod 形式登记，无子 Mod；但磁盘上是 HDR 合集
	app.modData.Upsert(name, col)

	app.GetMods()
	md := app.modData.Find(name)
	if md == nil {
		t.Fatal("Mod 未登记")
	}
	if len(md.SubMods) != 1 {
		t.Fatalf("GetMods 后应补齐子 Mod，得到 %d", len(md.SubMods))
	}
	if md.Category == "" {
		t.Fatal("补齐后应推导分类")
	}
	// 再次调用应幂等（不重复识别/写入）
	app.GetMods()
	if len(app.modData.Find(name).SubMods) != 1 {
		t.Fatal("重复调用不应改变子 Mod 数量")
	}
}

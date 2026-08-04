package main

import (
	"os"
	"path/filepath"
	"testing"

	"nioh2mod-js/internal/config"
	"nioh2mod-js/internal/mods"
)

// setLocksPath 为测试隔离全局占用锁文件路径
func setLocksPath(t *testing.T) {
	t.Helper()
	locksFilePath = filepath.Join(t.TempDir(), "parts.json")
}

func writeTestLocks(t *testing.T, m map[string]map[string][]string) {
	t.Helper()
	if err := writeLocks(m); err != nil {
		t.Fatalf("writeLocks 失败: %v", err)
	}
}

func contains(list []string, v string) bool {
	for _, s := range list {
		if s == v {
			return true
		}
	}
	return false
}

// 组合 Mod 占用取已启用子 Mod 的占用（同一部位多个值全部保留）：
// syncLock 把该占用写入全局锁文件，冲突检测基于锁文件判定。
func TestSyncLockCompositeWritesEnabledSubMods(t *testing.T) {
	setLocksPath(t)
	app := &App{modData: &config.ModData{Mods: []config.ModInfo{
		{Name: "09_HDR", Enabled: true, SubMods: []config.SubModInfo{
			{Name: "A", Enabled: true, Parts: map[string][]string{"胸甲": {"上古之衣-上衣"}, "腿甲": {"大型八龙-腿甲"}}},
			{Name: "B", Enabled: false, Parts: map[string][]string{"胸甲": {"无缘人之铠-胸甲"}, "膝甲": {"无缘人之铠-膝甲"}}},
		}},
	}}}
	app.syncLock("09_HDR")
	locks := readLocks()
	parts, ok := locks["09_HDR"]
	if !ok {
		t.Fatal("09_HDR 应已写入全局锁文件")
	}
	if !contains(parts["胸甲"], "上古之衣-上衣") {
		t.Fatalf("组合 Mod 应写已启用子 Mod 的占用，胸甲应含 上古之衣-上衣，实际=%v", parts["胸甲"])
	}
	if _, has := parts["膝甲"]; has {
		t.Fatalf("不应包含已禁用子 Mod B 的占用: %+v", parts)
	}
}

// 禁用后 syncLock 应从锁文件移出该 Mod。
func TestSyncLockDisableRemoves(t *testing.T) {
	setLocksPath(t)
	app := &App{modData: &config.ModData{Mods: []config.ModInfo{
		{Name: "绯雪", Enabled: false, Parts: map[string][]string{"胸甲": {"上古之衣-上衣"}}},
	}}}
	writeTestLocks(t, map[string]map[string][]string{"绯雪": {"胸甲": {"上古之衣-上衣"}}})
	app.syncLock("绯雪")
	if locks := readLocks(); len(locks) != 0 {
		t.Fatalf("禁用后应从锁文件移出，实际: %+v", locks)
	}
}

// 冲突检测以全局锁文件为准：绯雪与 09_HDR（启用子 Mod A）都占用上古之衣-上衣 → 冲突。
func TestCheckAllModConflictsFromLockFile(t *testing.T) {
	setLocksPath(t)
	writeTestLocks(t, map[string]map[string][]string{
		"绯雪":   {"胸甲": {"上古之衣-上衣"}},
		"09_HDR": {"胸甲": {"上古之衣-上衣"}},
	})
	app := &App{modData: &config.ModData{Mods: []config.ModInfo{
		{Name: "绯雪", Nickname: "绯雪", Enabled: true, Parts: map[string][]string{"胸甲": {"上古之衣-上衣"}}},
		{Name: "09_HDR", Enabled: true, SubMods: []config.SubModInfo{
			{Name: "A", Enabled: true, Parts: map[string][]string{"胸甲": {"上古之衣-上衣"}}},
			{Name: "B", Enabled: false, Parts: map[string][]string{"胸甲": {"无缘人之铠-胸甲"}}},
		}},
	}}}
	res := app.CheckAllModConflicts()
	if len(res) != 2 {
		t.Fatalf("两个 Mod 互指冲突，期望 2 条冲突信息，得到 %d: %+v", len(res), res)
	}
	found := false
	for _, info := range res {
		for _, c := range info.Conflicts {
			if c.Slot == "胸甲" && c.Value == "上古之衣-上衣" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("缺少「胸甲 → 上古之衣-上衣」冲突: %+v", res)
	}
}

// 回归：HDR 组合包全部子 Mod 开启时，若两个子 Mod 改同一部位但值不同，
// 后一个子 Mod 不能覆盖前一个，冲突检测必须仍能发现与外部 Mod 的冲突。
func TestCheckAllModConflictsCompositeAllSubsEnabled(t *testing.T) {
	setLocksPath(t)
	app := &App{modData: &config.ModData{Mods: []config.ModInfo{
		{Name: "绯雪", Nickname: "绯雪", Enabled: true, Parts: map[string][]string{"胸甲": {"上古之衣-上衣"}}},
		{Name: "09_HDR", Nickname: "09_HDR", Enabled: true, SubMods: []config.SubModInfo{
			{Name: "A", Enabled: true, Parts: map[string][]string{"胸甲": {"上古之衣-上衣"}, "腿甲": {"大型八龙-腿甲"}}},
			{Name: "B", Enabled: true, Parts: map[string][]string{"胸甲": {"无缘人之铠-胸甲"}, "膝甲": {"无缘人之铠-膝甲"}}},
		}},
	}}}
	app.rebuildLocks()
	locks := readLocks()
	if parts := locks["09_HDR"]; !contains(parts["胸甲"], "上古之衣-上衣") {
		t.Fatalf("全部子 Mod 开启时，胸甲必须同时保留 上古之衣-上衣，实际=%v", parts["胸甲"])
	}
	res := app.CheckAllModConflicts()
	found := false
	for _, info := range res {
		if info.ModName != "09_HDR" {
			continue
		}
		for _, c := range info.Conflicts {
			if c.ModName == "绯雪" && c.Slot == "胸甲" && c.Value == "上古之衣-上衣" {
				found = true
			}
		}
	}
	if !found {
		t.Fatalf("HDR 全部子 Mod 开启时仍应检测到与绯雪在 上古之衣-上衣 的冲突: %+v", res)
	}
}

// 启用前检查：目标 Mod 的占用与锁文件中其他 Mod 的占用对比。
func TestCheckModConflictsAgainstLockFile(t *testing.T) {
	setLocksPath(t)
	// 锁文件中只有绯雪占用上古之衣-上衣（09_HDR 尚未启用，不在锁文件里）
	writeTestLocks(t, map[string]map[string][]string{
		"绯雪": {"胸甲": {"上古之衣-上衣"}},
	})
	app := &App{modData: &config.ModData{Mods: []config.ModInfo{
		{Name: "绯雪", Nickname: "绯雪", Enabled: true, Parts: map[string][]string{"胸甲": {"上古之衣-上衣"}}},
		{Name: "09_HDR", Nickname: "09_HDR", Enabled: false, SubMods: []config.SubModInfo{
			{Name: "A", Enabled: true, Parts: map[string][]string{"胸甲": {"上古之衣-上衣"}}},
		}},
	}}}
	res := app.CheckModConflicts("09_HDR")
	if len(res) == 0 {
		t.Fatal("启用 09_HDR 应检测到与锁文件中绯雪的冲突")
	}
	if res[0].ModName != "绯雪" || res[0].Slot != "胸甲" || res[0].Value != "上古之衣-上衣" {
		t.Fatalf("冲突明细错误: %+v", res)
	}
}

// 相同部位不同资源不冲突。
func TestCheckAllModConflictsDifferentResourceNoConflict(t *testing.T) {
	setLocksPath(t)
	writeTestLocks(t, map[string]map[string][]string{
		"A": {"胸甲": {"甲一"}},
		"B": {"胸甲": {"甲二"}},
	})
	if res := (&App{modData: &config.ModData{}}).CheckAllModConflicts(); len(res) != 0 {
		t.Fatalf("不同资源不应冲突: %+v", res)
	}
}

// 回归：卸载 HDR 组合包（含已启用子 Mod）时，游戏 Mods 目录下的组合父目录
// （内含 meshes/textures 与各子 Mod 软链）必须整体移除，不允许残留子 Mod 软链；
// 数据父级回到未启用未安装、子 Mod 全部关闭，全局锁文件清除该 Mod 占用。
func TestUninstallModCompositeRemovesSubModLinks(t *testing.T) {
	setLocksPath(t)
	gameRoot := t.TempDir()
	repo := t.TempDir()
	parentDir := filepath.Join(repo, "09_HDR")

	// 构造合集源目录（含公共 meshes/textures 与两个子 Mod）
	mkdir(t, filepath.Join(parentDir, "meshes"))
	writeFile(t, filepath.Join(parentDir, "meshes", "a.bin"), "x")
	mkdir(t, filepath.Join(parentDir, "textures"))
	mkdir(t, filepath.Join(parentDir, "01_[A]"))
	writeFile(t, filepath.Join(parentDir, "01_[A]", "mod.ini"), "[mod]\n")
	mkdir(t, filepath.Join(parentDir, "02_[B]"))
	writeFile(t, filepath.Join(parentDir, "02_[B]", "mod.ini"), "[mod]\n")

	// 校验当前环境能否创建符号链接，否则跳过
	probe := filepath.Join(repo, "probe")
	if err := os.Symlink(parentDir, probe); err != nil {
		t.Skipf("当前环境无法创建符号链接（需管理员或开发者模式）: %v", err)
	}
	os.Remove(probe)

	app := &App{
		cfg:     &config.App{GameRoot: gameRoot, ModsRepo: repo},
		logData: &config.LogData{},
		modData: &config.ModData{Mods: []config.ModInfo{{
			Name: "09_HDR", Installed: true, Enabled: true, Path: parentDir,
			SubMods: []config.SubModInfo{
				{Name: "01_[A]", Enabled: true, Parts: map[string][]string{"胸甲": {"上古之衣-上衣"}}},
				{Name: "02_[B]", Enabled: true, Parts: map[string][]string{"膝甲": {"无缘人之铠-膝甲"}}},
			},
		}}},
	}

	link := modLinkPath(app.cfg, "09_HDR")
	// 模拟组合包已启用：父目录 + 公共链接 + 两个已启用子 Mod 软链
	if err := mods.EnableComposite(parentDir, link, []string{"meshes", "textures"}, []string{"01_[A]", "02_[B]"}); err != nil {
		t.Fatalf("EnableComposite 失败: %v", err)
	}
	// 父级占用写入锁文件
	app.syncLock("09_HDR")
	if _, ok := readLocks()["09_HDR"]; !ok {
		t.Fatal("卸载前 09_HDR 应已在锁文件中")
	}

	if err := app.UninstallMod("09_HDR"); err != nil {
		t.Fatalf("UninstallMod 失败: %v", err)
	}

	// 1) 组合父目录（含全部子 Mod 软链）应已整体移除，无残留软链
	if _, err := os.Lstat(link); err == nil {
		t.Fatal("卸载后组合父目录应已移除（含子 Mod 软链）")
	}
	for _, sm := range []string{"meshes", "textures", "01_[A]", "02_[B]"} {
		if _, err := os.Lstat(filepath.Join(link, sm)); err == nil {
			t.Fatalf("卸载后不应残留软链: %s", sm)
		}
	}
	// 2) 数据：父级未启用未安装、子 Mod 全部关闭（记录仍保留）
	md := app.modData.Find("09_HDR")
	if md == nil {
		t.Fatal("卸载后记录应保留")
	}
	if md.Enabled || md.Installed {
		t.Fatalf("卸载后父级应为未启用未安装: %+v", md)
	}
	for _, sm := range md.SubMods {
		if sm.Enabled {
			t.Fatalf("子 Mod %s 应已关闭", sm.Name)
		}
	}
	// 3) 全局锁文件已清除该 Mod 占用
	if _, ok := readLocks()["09_HDR"]; ok {
		t.Fatal("卸载后锁文件应清除该 Mod 占用")
	}
}

// 回归：清除组合 Mod 记录时，游戏 Mods 目录下启用的组合真实目录（含子 Mod 软链）
// 也必须整体移除，避免留下孤儿目录与子 Mod 软链。
func TestRemoveModRecordCompositeCleansGameModsDir(t *testing.T) {
	setLocksPath(t)
	gameRoot := t.TempDir()
	repo := t.TempDir()
	parentDir := filepath.Join(repo, "09_HDR")

	mkdir(t, filepath.Join(parentDir, "meshes"))
	writeFile(t, filepath.Join(parentDir, "meshes", "a.bin"), "x")
	mkdir(t, filepath.Join(parentDir, "textures"))
	mkdir(t, filepath.Join(parentDir, "01_[A]"))
	writeFile(t, filepath.Join(parentDir, "01_[A]", "mod.ini"), "[mod]\n")

	probe := filepath.Join(repo, "probe")
	if err := os.Symlink(parentDir, probe); err != nil {
		t.Skipf("当前环境无法创建符号链接: %v", err)
	}
	os.Remove(probe)

	app := &App{
		cfg:     &config.App{GameRoot: gameRoot, ModsRepo: repo},
		logData: &config.LogData{},
		modData: &config.ModData{Mods: []config.ModInfo{{
			Name: "09_HDR", Installed: true, Enabled: true, Path: parentDir, Missing: true,
			SubMods: []config.SubModInfo{{Name: "01_[A]", Enabled: true, Parts: map[string][]string{"胸甲": {"上古之衣-上衣"}}}},
		}}},
	}

	link := modLinkPath(app.cfg, "09_HDR")
	if err := mods.EnableComposite(parentDir, link, []string{"meshes", "textures"}, []string{"01_[A]"}); err != nil {
		t.Fatalf("EnableComposite 失败: %v", err)
	}

	if err := app.RemoveModRecord("09_HDR"); err != nil {
		t.Fatalf("RemoveModRecord 失败: %v", err)
	}

	if _, err := os.Lstat(link); err == nil {
		t.Fatal("清除记录后组合父目录应已整体移除（含子 Mod 软链）")
	}
	for _, sm := range []string{"meshes", "textures", "01_[A]"} {
		if _, err := os.Lstat(filepath.Join(link, sm)); err == nil {
			t.Fatalf("不应残留软链: %s", sm)
		}
	}
	if app.modData.Find("09_HDR") != nil {
		t.Fatal("清除记录后数据仓库应无该记录")
	}
}

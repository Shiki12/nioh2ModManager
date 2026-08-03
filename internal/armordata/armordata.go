// Package armordata 提供游戏装备资源数据（按身体部位分组，供前端选择 Mod 占用资源）
package armordata

import (
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

//go:embed armor_parts.json
var embeddedData []byte

//go:embed weapon_parts.json
var embeddedWeaponData []byte

// Part 单个装备资源
type Part struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// Slot 一个身体部位的可选资源列表
type Slot struct {
	Name  string `json:"name"`
	Parts []Part `json:"parts"`
}

var (
	once   sync.Once
	slots  []Slot
	bySlot map[string][]Part
)

// externalPath 返回 exe 同目录 res 子目录下的资源文件路径
func externalPath() string {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join("res", "armor_parts.json")
	}
	return filepath.Join(filepath.Dir(exe), "res", "armor_parts.json")
}

// load 解析并缓存数据：优先读取 exe 同目录的 armor_parts.json（可被替换/更新），
// 不存在或解析失败时回退到编译期内置的数据。
func load() {
	once.Do(func() {
		data := embeddedData
		if raw, err := os.ReadFile(externalPath()); err == nil {
			data = raw
		}
		var m map[string][]Part
		if err := json.Unmarshal(data, &m); err != nil {
			return
		}
		bySlot = m
		order := []string{"头", "胸甲", "臂甲", "膝甲", "腿甲"}
		seen := map[string]bool{}
		keys := []string{}
		for _, name := range order {
			if _, ok := m[name]; ok {
				keys = append(keys, name)
				seen[name] = true
			}
		}
		for k := range m {
			if !seen[k] {
				seen[k] = true
				keys = append(keys, k)
			}
		}
		for _, name := range keys {
			parts := m[name]
			sort.Slice(parts, func(i, j int) bool { return parts[i].ID < parts[j].ID })
			slots = append(slots, Slot{Name: name, Parts: parts})
		}
	})
}

// AllSlots 返回按部位分组的全部装备资源（已缓存）
func AllSlots() []Slot {
	load()
	return slots
}

// SlotNames 返回部位名称列表（有序）
func SlotNames() []string {
	load()
	names := make([]string, 0, len(slots))
	for _, s := range slots {
		names = append(names, s.Name)
	}
	return names
}

// Find 在指定部位中按 ID 查找资源，未找到返回 nil
func Find(slot, id string) *Part {
	load()
	for _, p := range bySlot[slot] {
		if p.ID == id {
			return &p
		}
	}
	return nil
}

// FindByName 在指定部位中按名称查找资源，未找到返回 nil
func FindByName(slot, name string) *Part {
	load()
	for _, p := range bySlot[slot] {
		if p.Name == name {
			return &p
		}
	}
	return nil
}

// FindBySetName 在指定部位中按套装名匹配资源（模糊匹配），未找到返回 nil。
// armor_parts.json 命名不统一（"套装-部位" / "套装 部位" / 特殊部位名等），
// 因此按"去掉部位后缀后的套装名"与给定套装名做相似度匹配，取唯一最佳者；
// 存在多个并列候选时返回 nil，避免误匹配（由调用方交给人工确认）。
func FindBySetName(slot, setName string) *Part {
	load()
	name := strings.TrimSpace(setName)
	if name == "" {
		return nil
	}
	parts := bySlot[slot]
	bestScore := 0
	best := -1
	count := 0
	for i := range parts {
		score := setSimilarity(setOf(parts[i].Name, slot), name)
		if score > bestScore {
			bestScore = score
			best = i
			count = 1
		} else if score == bestScore && score > 0 {
			count++
		}
	}
	if count != 1 || best < 0 {
		return nil
	}
	return &parts[best]
}

// setOf 从部位部件名中提取套装名（去掉常见部位后缀：-部位、空格+部位）
func setOf(partName, slot string) string {
	s := partName
	if strings.HasSuffix(s, "-"+slot) {
		s = s[:len(s)-len(slot)-1]
	} else if strings.HasSuffix(s, " "+slot) {
		s = s[:len(s)-len(slot)]
	} else if strings.HasSuffix(s, slot) {
		s = s[:len(s)-len(slot)]
	}
	return strings.TrimSpace(s)
}

// setSimilarity 套装名相似度打分：0=无关，40=包含关系，60=前缀关系，100=完全相等
func setSimilarity(a, b string) int {
	if a == "" || b == "" {
		return 0
	}
	if a == b {
		return 100
	}
	if strings.HasPrefix(a, b) || strings.HasPrefix(b, a) {
		return 60
	}
	if strings.Contains(a, b) || strings.Contains(b, a) {
		return 40
	}
	return 0
}

// ============================================================
// 武器资源（武器 Mod 的占用目标）
// ============================================================

var (
	weaponOnce sync.Once
	weapons    []Part
)

// weaponExternalPath 返回 exe 同目录 res 子目录下的武器资源文件路径
func weaponExternalPath() string {
	exe, err := os.Executable()
	if err != nil {
		return filepath.Join("res", "weapon_parts.json")
	}
	return filepath.Join(filepath.Dir(exe), "res", "weapon_parts.json")
}

// loadWeapons 解析并缓存武器数据：优先读取 exe 同目录 weapon_parts.json，回退到编译期内置数据
func loadWeapons() {
	weaponOnce.Do(func() {
		data := embeddedWeaponData
		if raw, err := os.ReadFile(weaponExternalPath()); err == nil {
			data = raw
		}
		var list []Part
		if err := json.Unmarshal(data, &list); err != nil {
			return
		}
		sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
		weapons = list
	})
}

// AllWeapons 返回全部武器资源（已缓存）
func AllWeapons() []Part {
	loadWeapons()
	return weapons
}

// FindWeapon 按 ID 查找武器，未找到返回 nil
func FindWeapon(id string) *Part {
	loadWeapons()
	for i := range weapons {
		if weapons[i].ID == id {
			return &weapons[i]
		}
	}
	return nil
}

// FindWeaponByName 按名称查找武器，未找到返回 nil
func FindWeaponByName(name string) *Part {
	loadWeapons()
	for i := range weapons {
		if weapons[i].Name == name {
			return &weapons[i]
		}
	}
	return nil
}

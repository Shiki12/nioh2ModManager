// 装备物品结构字段偏移常量 (全部来自 CT 表「装备\道具编辑器」, 见 data/equipment_field_offsets.md)。
//
// 所有偏移都是相对"当前选中物品指针"(equipment_ptr/rcx 捕获到的装备地址)的:
//
//	字段绝对地址 = item.Addr + offsetXxx
//
// token说明:
//   - 多个字段可能共享同一字节的不同 bit (如 +11/+12/+13), 改写这类字段必须"读-改-写"整字节,
//     不能直接覆盖相邻 bit 的字段。
//   - 魂核类字段必须在背包里修改, 否则游戏会崩溃。
package transformation

// ---- 顶层字段 (0x00 ~ 0x2F) ----
const (
	// offsetItemID 是物品ID (u16)。
	offsetItemID = 0x00

	// offsetModelID 是幻化ID (u16), 动态幻化改写主目标。
	offsetModelID = 0x02

	// offsetQuantity 是数量 (u16)。
	offsetQuantity = 0x04

	// offsetSoulMatchLv 是合魂等级 (u16), CT 原字段"合魂等级"。
	offsetSoulMatchLv = 0x06

	// offsetBaseLv 是原始等级 (u16), CT 原字段"原始等级"。
	offsetBaseLv = 0x08

	// offsetPlusValue 是加值+ (u16), 即武器/防具的 +# 强化值。
	offsetPlusValue = 0x0A

	// offsetFamiliarity 是爱用度 (u32)。
	offsetFamiliarity = 0x0C

	// offsetMarkB0 是"+10 字节"的位0-3, CT 字段"标记 #1":
	// 0无 / 1满爱用度 / 4受保护 / 5满爱用度+受保护。
	offsetMarkB0 = 0x10

	// offsetSourceB0 是"+11 字节"的 bit0-1, CT 字段"来源" (装备统计列表)。
	offsetSourceB0 = 0x11

	// offsetSoulPurifiedB2 是"+11 字节"的 bit2, 魂核字段"魂核净化" (0是/1否)。
	offsetSoulPurifiedB2 = 0x11

	// offsetAppraisedB6 是"+11 字节"的 bit6, CT 字段"鉴定茶器" (0已鉴/1未鉴)。
	offsetAppraisedB6 = 0x11

	// offsetFavoriteB1 是"+12 字节"的 bit1, CT 字段"喜好项目"。
	offsetFavoriteB1 = 0x12

	// offsetScrollPurifiedB3 是"+12 字节"的 bit3, 百鬼绘卷"是否净化"。
	offsetScrollPurifiedB3 = 0x12

	// offsetClearFlagB7 是"+12 字节"的 bit7, 百鬼绘卷"Clear Flag"。
	offsetClearFlagB7 = 0x12

	// offsetSetBonusB0 是"+13 字节"的 bit0, CT 字段"套装+"。
	offsetSetBonusB0 = 0x13

	// offsetRarity 是品质 (u8)。
	offsetRarity = 0x14

	// offsetSoulClass 是魂核阶级 (u8)。
	offsetSoulClass = 0x15

	// offsetFlagB0 是"+16" u16, CT 未命名占位。
	offsetFlagB0 = 0x16

	// offsetSoulTypeB0 是"+17 字节"的 bit0-3, 魂核"类型"。
	offsetSoulTypeB0 = 0x17

	// offsetAccumulation 是妖武器妖念累积 (u16)。
	offsetAccumulation = 0x18

	// offsetRecLevel 是百鬼绘卷"建议等级" (u16, 游戏中不直接显示)。
	offsetRecLevel = 0x1A

	// offsetUnknownB2 是"+1C" 字节, CT 未命名占位。
	offsetUnknownB2 = 0x1C

	// offsetRemodel 是改造类型 (u8, 改造列表)。
	offsetRemodel = 0x1E

	// offsetAttempts 是百鬼绘卷"可挑战次数" (u8)。
	offsetAttempts = 0x1F

	// ---- 系统内置应用 (内部字段, 一般不动) ----
	// offsetItemNumbers 是 "+20" (u32), 系统内置 Item numbers。
	offsetItemNumbers = 0x20

	// offsetRemark 是 "+24" (u32), 系统内置 Remark?。
	offsetRemark = 0x24

	// offsetHiddenC 是 "+28" (u32), 系统内置未命名。
	offsetHiddenC = 0x28

	// offsetHiddenD 是 "+2C" (u32), 系统内置未命名。
	offsetHiddenD = 0x2C
)

// ---- 词条区 (0x30 起, 每槽 0x0C) ----
const (
	// offsetFamBase 是词条区起始偏移。
	offsetFamBase = 0x30

	// famSlotSize 是单个词条槽大小 (0x0C 字节)。
	famSlotSize = 0x0C

	// famSlotCount 是词条槽最大数量 (词条代码 1~7)。
	famSlotCount = 7
)

// famSlotOff 返回第 n 个词条槽的字节偏移 (n=0..6)。
// 词条槽 n 的绝对地址 = item.Addr + famSlotOff(n)。
func famSlotOff(n uint) uint {
	return offsetFamBase + n*famSlotSize
}

const (
	// ---- 词条槽内子字段偏移 ----
	// offsetFamID   是词条 ID   (u32)。
	// offsetFamVal  是词条数值  (u16)。
	// offsetFamEff  是词条效果  (bit0-13, 14bit)。
	// offsetFamIc1  是词条图标Pt.1 (bit6-7)。
	// offsetFamIc2  是词条图标Pt.2 (u16)。
	offsetFamID  = 0x00
	offsetFamVal = 0x04
	offsetFamEff = 0x08
	offsetFamIc1 = 0x09
	offsetFamIc2 = 0x0A
)
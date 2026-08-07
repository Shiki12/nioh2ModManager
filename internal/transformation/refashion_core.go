// Package refashion 提供 Nioh2 动态幻化(Refashion)的内存注入与改写能力。
//
// # 原理
//
// 仁王2 的"装备\道具编辑器"函数 (装备编辑器) 入口由 CT 表确认:
//
//	前5字节: C3 CC CC CC CC        (上一个函数的 ret + 对齐填充, 即函数边界)
//	入口:    48 83 EC 28 0F B7 11  (sub rsp,28; movzx edx,word ptr [rcx])
//	函数体:  48 8B 05 ?? mov rax,[rip+disp32] / mov rcx,[rax+20] / mov rcx,[rcx+428]
//
// 该函数在游戏"选中一件物品并进入详情/编辑器"时被调用, 此时 rcx = 当前选中物品指针。
// 我们在此函数入口打 hook: 先把 rcx (物品指针) 写进我们分配的内存槽 equipment_ptr,
// 再放行原指令, 游戏继续正常运行。
//
// 随后外部程序轮询 equipment_ptr 槽拿到选中物品指针, 改写物品结构偏移 +2 的幻化ID。
//
// # 绿色干净
//
// 纯内存操作: 不写存档、不引入额外 DLL。游戏重启后 hook 消失, 恢复原状。
//
// # 使用流程 (一键)
//
//	svc := refashion.NewRefashion(pid)
//	hook := svc.InstallHook()          // 定位+注入
//	item := hook.WaitForItem(timeout)  // 等玩家选中物品
//	hook.SetRefashionID(item, 0x5900)  // 改写幻化ID
//	hook.Uninstall()                   // 恢复原代码
package transformation

import (
	"encoding/binary"
	"fmt"
	"syscall"
	"time"
	"unsafe"
)

// ================================================================
// 特征常量: 装备编辑器函数在 nioh2.exe 中的机器码指纹
// ================================================================
var (
	// hookPattern 是 7 字节核心模式, 即装备编辑器函数入口。
	// 48 83 EC 28  = sub rsp, 28        (函数序言, 预留栈空间)
	// 0F B7 11     = movzx edx,[rcx]    (解引用 rcx, 读取物品ID)
	hookPattern = []byte{0x48, 0x83, 0xEC, 0x28, 0x0F, 0xB7, 0x11}

	// ctPattern 是 CT 表(Cheat Engine 表)注释的完整 12 字节模式。
	// 相比 7 字节模式多了前 5 字节的函数边界, 用于唯一性确认:
	// C3 CC CC CC CC  = ret + 3 个 int3 对齐填充 (上一函数的结尾)
	ctPattern = []byte{0xC3, 0xCC, 0xCC, 0xCC, 0xCC, 0x48, 0x83, 0xEC, 0x28, 0x0F, 0xB7, 0x11}

	// bodyWant 是注入点之后(+7)的函数体特征, 用于二次确认这是装备编辑器
	// 而非碰巧有相同序言的其他函数。其中 disp32 偏移量会随版本变化, 用 0xAA 占位,
	// 比对时跳过 (索引 3~6):
	//   48 8B 05 disp32       mov rax,[rip+disp32]      (加载全局对象指针)
	//   48 8B 48 20           mov rcx,[rax+20]          (取其成员)
	//   48 8B 89 28 04 00 00  mov rcx,[rcx+428]         (再取成员 = 当前选中物品)
	bodyWant = []byte{
		0x48, 0x8B, 0x05, 0xAA, 0xAA, 0xAA, 0xAA, // mov rax,[rip+disp32]
		0x48, 0x8B, 0x48, 0x20, // mov rcx,[rax+20]
		0x48, 0x8B, 0x89, 0x28, 0x04, 0x00, 0x00, // mov rcx,[rcx+428]
	}

	// boundary 是函数边界特征: 前一函数以 ret(C3) 结尾后跟对齐填充(CC)。
	// 只有真函数入口前才是这个序列; 函数内部的碰巧字节会被这里排除。
	boundary = []byte{0xC3, 0xCC, 0xCC, 0xCC, 0xCC}
)

const (
	// ---- 物品结构字段偏移 (CT 表确认) ----
	offsetItemID   = 0x00 // 物品ID        (2字节) 如 5900 = 怪童大铠-胸甲
	offsetModelID  = 0x02 // 幻化ID        (2字节) 本方案改写的目标字段
	offsetQuantity = 0x04 // 数量          (2字节)

	// hookLen 是被我们覆盖的原始指令长度 (48 83 EC 28 0F B7 11 = 7字节)。
	// 写入 cave 的代码必须重放这些被覆盖的指令, 保证函数行为不变。
	hookLen = 7

	// caveSize 是 code cave 分配大小。cave 内同时容纳 hook 机器码 + equipment_ptr 槽。
	caveSize = 0x100

	// slotOff 是 cave 内 equipment_ptr 槽相对 cave 起点的偏移。
	// 槽 = cave + 0x20, 与 hook 代码(0x19字节)不冲突。
	slotOff = 0x20

	// defaultTimeout 是等待玩家选中物品的默认超时。
	defaultTimeout = 300 * time.Second

	// pollInterval 是轮询 equipment_ptr 槽的间隔。
	pollInterval = 250 * time.Millisecond
)

// ================================================================
// Process: 目标进程的内存访问封装
// ================================================================

// Process 封装对目标进程(Nioh2)的句柄与基础读写能力。
type Process struct {
	Pid  uint32  // 目标进程 ID
	hand uintptr // OpenProcess 返回的进程句柄 (非导出, 防止外部误用)
}

// NewProcess 通过 OpenProcess 打开目标进程, 获得全部访问权限。
// 调用方必须在用完句柄后调用 Close 释放, 否则句柄会泄漏。
func NewProcess(pid uint32) (*Process, error) {
	hand, err := openProcess(pid)
	if err != nil {
		return nil, err
	}
	return &Process{Pid: pid, hand: hand}, nil
}

// Close 关闭进程句柄。
func (p *Process) Close() {
	procCloseHandle.Call(p.hand)
}

// read 是内部读内存方法 (小写, 仅供包内调用), 统一走 readMem。
func (p *Process) read(addr uintptr, out []byte) (int, error) {
	return readMem(p.hand, addr, out)
}

// write 是内部写内存方法 (小写, 仅供包内调用), 统一走 writeMem。
func (p *Process) write(addr uintptr, in []byte) error {
	return writeMem(p.hand, addr, in)
}

// Read 读取目标进程内存到 out, 返回实际读取字节数。
// addr 为目标进程内的虚拟地址。
func (p *Process) Read(addr uintptr, out []byte) (int, error) {
	return p.read(addr, out)
}

// Write 把 in 写入目标进程内存。
// addr 为目标进程内的虚拟地址。
func (p *Process) Write(addr uintptr, in []byte) error {
	return p.write(addr, in)
}

// ReadUint64 读取地址处的 8 字节小端无符号整数。
// 用于读取 equipment_ptr 槽 (槽内保存的是 64 位指针)。
func (p *Process) ReadUint64(addr uintptr) (uint64, error) {
	buf := make([]byte, 8)
	if _, err := p.read(addr, buf); err != nil {
		return 0, err
	}
	return binary.LittleEndian.Uint64(buf), nil
}

// ReadItem 读取物品头部信息。
// 物品结构头部 (共 0x20 字节, 我们只关心前 6 字节):
//
//	+0  物品ID   (2字节)
//	+2  幻化ID   (2字节)  <- 改写目标
//	+4  数量     (2字节)
func (p *Process) ReadItem(addr uintptr) (*Item, error) {
	hdr := make([]byte, 0x20)
	if _, err := p.read(addr, hdr); err != nil {
		return nil, err
	}
	return &Item{
		Addr:    addr,
		ItemID:  binary.LittleEndian.Uint16(hdr[offsetItemID:]),
		ModelID: binary.LittleEndian.Uint16(hdr[offsetModelID:]),
		Qty:     binary.LittleEndian.Uint16(hdr[offsetQuantity:]),
	}, nil
}

// ================================================================
// Item: 物品头部数据
// ================================================================

// Item 表示捕获到的物品头部信息。
type Item struct {
	Addr    uintptr // 物品对象在游戏进程内的地址 (即 hook 捕获的 rcx)
	ItemID  uint16  // 物品ID (如 0x5900 怪童大铠-胸甲)
	ModelID uint16  // 幻化ID (当前外观, 即 +2 字段的值)
	Qty     uint16  // 数量
}

// ================================================================
// InjectedHook: 一次注入会话
// ================================================================

// InjectedHook 表示一个已注入成功的 hook 会话。
// 持有注入点、cave、equipment_ptr 槽以及原始字节, 便于读取捕获、改写幻化和卸载。
type InjectedHook struct {
	proc     *Process // 所属进程封装
	base     uintptr  // nioh2.exe 模块基址
	hookSite uintptr  // 被 patch 的函数入口地址 (原本是 48 83 EC 28...)
	cave     uintptr  // code cave 起始地址 (存 hook 机器码 + equipment_ptr 槽)
	slot     uintptr  // equipment_ptr 槽地址 = cave + slotOff
	orig     []byte   // hookSite 处原始 7 字节, 卸载时用于还原
	patched  bool     // 是否已 patch (防止重复卸载)
}

// ================================================================
// Refashion: 顶层服务
// ================================================================

// Refashion 顶层服务, 提供完整的高层接口。
type Refashion struct {
	proc *Process
}

// NewRefashion 创建幻化服务并打开游戏进程。
// 完成后需调用 Close 释放进程句柄。
func NewRefashion(pid uint32) (*Refashion, error) {
	proc, err := NewProcess(pid)
	if err != nil {
		return nil, err
	}
	return &Refashion{proc: proc}, nil
}

// Close 释放进程句柄。
func (r *Refashion) Close() {
	r.proc.Close()
}

// Process 返回底层进程封装, 供需要直接读写内存的调用方使用。
func (r *Refashion) Process() *Process { return r.proc }

// LocateHookSite 定位装备编辑器函数入口地址 (只读, 不注入)。
//
// 步骤:
//  1. 取 nioh2.exe 模块基址与大小
//  2. 全模块扫描 7 字节核心模式, 得若干候选点
//  3. 对每个候选点做特征校验 (函数边界 + 函数体), 筛掉函数内部的假命中
//  4. 用 CT 表 12 字节完整模式复核唯一性
//
// 返回 (hookSite, base, error)。hookSite 即 48 83 EC 28 那 7 字节的起始地址。
func (r *Refashion) LocateHookSite() (uintptr, uintptr, error) {
	base, size, err := getModuleBase(r.proc.Pid)
	if err != nil {
		return 0, 0, err
	}
	all, err := aobScanAll(r.proc, base, uintptr(size), hookPattern)
	if err != nil {
		return 0, 0, err
	}
	for _, h := range all {
		if verifyHookSite(r.proc, h) {
			// 用 CT 12 字节模式复核: 12字节模式起点 + 5 应等于 7字节模式的起点
			ct, _ := aobScanAll(r.proc, base, uintptr(size), ctPattern)
			for _, c := range ct {
				if c+5 == h {
					return h, base, nil
				}
			}
		}
	}
	return 0, 0, fmt.Errorf("未找到符合装备编辑器特征的位置")
}

// InstallHook 定位并注入 hook, 返回已注入的会话。
// 调用方需在完成后调用 Uninstall 恢复原代码。
// 若游戏已存在上一次的 hook 残留 (AOB 找不到原始字节), 会返回错误, 需重启游戏。
func (r *Refashion) InstallHook() (*InjectedHook, error) {
	hookSite, base, err := r.LocateHookSite()
	if err != nil {
		return nil, err
	}
	return r.InstallHookAt(hookSite, base)
}

// InstallHookAt 在指定位置注入 hook (通常来自 LocateHookSite)。
// 完整注入流程:
//
//  1. 校验 hookSite 原始字节 = 48 83 EC 28 0F B7 11 (防重复注入)
//  2. 在 hookSite 附近 ±2GB 内分配 code cave (E9 相对跳转有 ±2GB 限制)
//  3. 把重放代码写入 cave
//  4. 把 hookSite 前 7 字节 patch 成: jmp cave; nop; nop
//  5. 刷新指令缓存
//
// 之后游戏每次执行该函数都会先跳到 cave: 把 rcx (选中物品指针) 写入槽,
// 再重放原指令并从原位置继续。
func (r *Refashion) InstallHookAt(hookSite, base uintptr) (*InjectedHook, error) {
	p := r.proc

	// (1) 校验原始字节: 确保 hookSite 尚未被 patch, 且版本匹配。
	orig := make([]byte, hookLen)
	if _, err := p.read(hookSite, orig); err != nil {
		return nil, err
	}
	for i := 0; i < hookLen; i++ {
		if orig[i] != hookPattern[i] {
			return nil, fmt.Errorf("注入点字节不符 (已有hook或版本不同) @%X", hookSite)
		}
	}

	// (2) 分配 cave。必须在 hookSite 附近, 因为 E9 相对跳转只有 ±2GB 寻址范围。
	cave, err := virtAllocExNear(p.hand, caveSize, hookSite)
	if err != nil {
		return nil, err
	}
	slot := cave + slotOff // equipment_ptr 槽地址

	// (3) 构建并写入 cave 代码 (见 buildHookCode)。
	code := buildHookCode(cave, slot, hookSite)
	if err := p.write(cave, code); err != nil {
		procVirtualFreeEx.Call(p.hand, cave, 0, MEM_RELEASE)
		return nil, err
	}

	// (4) patch hookSite: 前5字节 = jmp cave (E9 rel32), 后2字节 = NOP NOP。
	//     先要把代码段页保护改为可写 (EXE 的 .text 段默认只读)。
	if _, err := protectMem(p.hand, hookSite, hookLen, PAGE_EXECUTE_READWRITE); err != nil {
		procVirtualFreeEx.Call(p.hand, cave, 0, MEM_RELEASE)
		return nil, err
	}
	patch := make([]byte, hookLen)
	patch[0] = 0xE9 // jmp 短跳转
	relHook := int64(cave) - int64(hookSite) - 5
	binary.LittleEndian.PutUint32(patch[1:5], uint32(relHook))
	patch[5] = 0x90 // nop
	patch[6] = 0x90 // nop
	if err := p.write(hookSite, patch); err != nil {
		procVirtualFreeEx.Call(p.hand, cave, 0, MEM_RELEASE)
		return nil, err
	}

	// (5) 刷新指令缓存, 确保游戏 CPU 不再使用旧的预取指令。
	procFlushInstrCache.Call(p.hand, hookSite, hookLen)

	return &InjectedHook{
		proc: p, base: base, hookSite: hookSite,
		cave: cave, slot: slot, orig: orig, patched: true,
	}, nil
}

// buildHookCode 生成 cave 内的机器码。
//
// 被覆盖的原指令是 48 83 EC 28 0F B7 11 (7字节), 必须在 cave 里重放,
// 否则函数栈/寄存器状态会错乱。cave 代码布局如下:
//
//	偏移  机器码            含义
//	0     48 89 0D rel32    mov [slot], rcx      (把选中物品指针写进槽, 7字节)
//	7     48 83 EC 28       sub rsp, 28          (重放原指令1)
//	11    0F B7 11          movzx edx,[rcx]      (重放原指令2)
//	14    E9 rel32          jmp returnAddr       (跳回原函数继续执行, 5字节)
//
// 注意: mov [rip+disp32],rcx 是 7 字节指令, 排布时后一条指令必须从偏移 7 开始,
// 之前曾因按 6 字节排布导致 disp32 被覆盖、跳转错乱、游戏崩溃。
func buildHookCode(cave, slot, hookSite uintptr) []byte {
	code := make([]byte, 0x28)
	returnAddr := hookSite + hookLen // 回到原函数中未被覆盖的位置

	// 指令1: mov [slot], rcx
	code[0], code[1], code[2] = 0x48, 0x89, 0x0D
	rel := int64(slot) - int64(cave) - 7 // 相对下一条指令 RIP (=cave+7)
	binary.LittleEndian.PutUint32(code[3:7], uint32(rel))

	// 指令2: sub rsp, 28 (重放)
	code[7], code[8], code[9], code[10] = 0x48, 0x83, 0xEC, 0x28

	// 指令3: movzx edx, word ptr [rcx] (重放)
	code[11], code[12], code[13] = 0x0F, 0xB7, 0x11

	// 指令4: jmp returnAddr
	code[14] = 0xE9
	relJmp := int64(returnAddr) - int64(cave) - 19 // 相对下一条指令 RIP (=cave+19)
	binary.LittleEndian.PutUint32(code[15:19], uint32(relJmp))
	return code
}

// DefaultTimeout 返回默认等待捕获的超时时间。
func DefaultTimeout() time.Duration { return defaultTimeout }

// WaitForItem 轮询等待捕获选中物品指针。
//
// 玩家在游戏里"选中一件物品并进入详情/编辑器"时, 装备编辑器函数被调用,
// hook 把物品指针写入 slot。这里轮询 slot, 一旦非 0 即认为捕获成功。
//
// 返回 *Item (含物品ID/幻化ID/数量)。超时返回错误。
func (h *InjectedHook) WaitForItem(timeout time.Duration) (*Item, error) {
	if h.slot == 0 {
		return nil, fmt.Errorf("hook 未注入")
	}
	deadline := time.Now().Add(timeout)
	buf := make([]byte, 8)
	for time.Now().Before(deadline) {
		if _, err := h.proc.read(h.slot, buf); err == nil {
			addr := binary.LittleEndian.Uint64(buf)
			if addr != 0 {
				return h.readItem(uintptr(addr))
			}
		}
		time.Sleep(pollInterval)
	}
	return nil, fmt.Errorf("超时: 未捕获到选中物品, 请在游戏里重新选中该物品")
}

// readItem 从指定地址读取物品头部。
func (h *InjectedHook) readItem(addr uintptr) (*Item, error) {
	return h.proc.ReadItem(addr)
}

// SetRefashionID 改写捕获物品的幻化ID并回读确认。
//
// 写入物品地址 +2 的 2 字节字段为目标幻化ID, 然后立即回读验证写入成功。
//
// 注意: 游戏引擎会缓存已加载的模型, 改写字段后外观不会立刻刷新。
// 玩家需在游戏里"取下该装备再穿上"(或切换选中) 才会用新ID重新渲染。
// 纯内存修改, 不写存档, 游戏重启后恢复原外观。
func (h *InjectedHook) SetRefashionID(item *Item, newID uint16) error {
	if item == nil {
		return fmt.Errorf("item 为空")
	}
	// newID 拆成 2 字节小端
	buf := []byte{byte(newID), byte(newID >> 8)}
	if err := h.proc.write(item.Addr+offsetModelID, buf); err != nil {
		return err
	}
	// 回读确认
	if _, err := h.proc.read(item.Addr+offsetModelID, buf); err != nil {
		return err
	}
	after := binary.LittleEndian.Uint16(buf)
	if after != newID {
		return fmt.Errorf("写入校验失败: 当前=%04X 目标=%04X", after, newID)
	}
	return nil
}

// ReadItemBySlot 从已注入的 slot 直接读取当前捕获物品 (无需等待捕获)。
// 若槽尚未被 hook 写入 (玩家还没选中过物品) 则报错。
func (h *InjectedHook) ReadItemBySlot() (*Item, error) {
	buf := make([]byte, 8)
	if _, err := h.proc.read(h.slot, buf); err != nil {
		return nil, err
	}
	addr := binary.LittleEndian.Uint64(buf)
	if addr == 0 {
		return nil, fmt.Errorf("slot 尚未捕获物品")
	}
	return h.readItem(uintptr(addr))
}

// Slot 返回 equipment_ptr 槽地址。
func (h *InjectedHook) Slot() uintptr { return h.slot }

// Patched 报告该 hook 是否仍处于已 patch 状态（用于外部决定是否需要 Uninstall）。
func (h *InjectedHook) Patched() bool { return h != nil && h.patched }

// HookSite 返回注入点地址。
func (h *InjectedHook) HookSite() uintptr { return h.hookSite }

// UninstallLeftover 手动清理一次注入残留 (不依赖 AOB 定位, 供游戏重启前的原地还原)。
//
// 适用于"上次 apply/batch 被中断、hook 残留在游戏里、AOB 已找不到原始字节"的情形。
// 需要已知注入点 hookSite 与槽地址 slot (cave = slot - slotOff)。把这些地方的
// 原始 7 字节还原为 hookPattern、刷新指令缓存、并释放 cave 内存。
// 若 hookSite 处并非跳转 (未注入), 幂等安全返回 nil。返回是否改动 (cave 释放后不可再复用)。
func (r *Refashion) UninstallLeftover(hookSite, slot uintptr) (bool, error) {
	p := r.proc

	// 只有 hookSite 前5字节是 E9 跳转时才需要还原
	cur := make([]byte, hookLen)
	if _, err := p.read(hookSite, cur); err != nil {
		return false, err
	}
	if cur[0] != 0xE9 {
		return false, nil // 未注入, 无需处理
	}

	if _, err := protectMem(p.hand, hookSite, hookLen, PAGE_EXECUTE_READWRITE); err != nil {
		return false, err
	}
	copied := make([]byte, hookLen)
	copy(copied, hookPattern)
	if err := p.write(hookSite, copied); err != nil {
		return false, err
	}
	procFlushInstrCache.Call(p.hand, hookSite, hookLen)

	// 释放 cave (=slot - slotOff)
	cave := slot - slotOff
	procVirtualFreeEx.Call(p.hand, cave, 0, MEM_RELEASE)
	return true, nil
}

// untouchedHookSize 装备编辑器函数入口相对模块基址的固定偏移 (nioh2.exe + 0x1022540)。
// 残留 hook 无法用 AOB 定位时, 用模块基址 + 该偏移直接算出 hook 位置再还原。
const untouchedHookSize = 0x1022540

// CleanupLeftover 不依赖 AOB, 直接用「模块基址 + 固定偏移」还原上次残留 hook 的原始字节。
//
// 场景: 上次 apply/bmo/batch 被强杀, hookSite 已被 patch 成 E9, AOB 扫描找不到
// 原始字节 → verify/LocateHookSite 失败。此时用基址+偏移算出 hookSite, 调 UninstallLeftover
// 把原始 7 字节写回, 游戏即可恢复可注入状态。
//
// 注意: 返回的 slot 仅用于释放 cave; 若无法得知旧 cave 地址, 传 0 则跳过释放佐雏码
// (代码还原为主, 泄漏的 cave 对大多数进程无实际危害)。
func (r *Refashion) CleanupLeftover() bool {
	p := r.proc
	base, _, err := getModuleBase(p.Pid)
	if err != nil {
		fmt.Printf("取模块基址失败: %v\n", err)
		return false
	}
	hookSite := base + untouchedHookSize
	ok, err := r.UninstallLeftover(hookSite, 0)
	if err != nil {
		fmt.Printf("还原失败: %v\n", err)
		return false
	}
	fmt.Printf("CleanupLeftover: hookSite=%X, 还原=%v\n", hookSite, ok)
	return ok
}

// Uninstall 恢复原始代码并释放 cave。
// 即把 hookSite 的 7 字节还原为 48 83 EC 28 0F B7 11, 并释放 cave 内存。
// 幂等: 重复调用无副作用。
func (h *InjectedHook) Uninstall() error {
	if h == nil || !h.patched {
		return nil
	}
	// 先把代码段页改回可写
	if _, err := protectMem(h.proc.hand, h.hookSite, hookLen, PAGE_EXECUTE_READWRITE); err != nil {
		return err
	}
	// 还原原始字节
	if err := h.proc.write(h.hookSite, h.orig); err != nil {
		return err
	}
	procFlushInstrCache.Call(h.proc.hand, h.hookSite, hookLen)
	// 释放 cave
	procVirtualFreeEx.Call(h.proc.hand, h.cave, 0, MEM_RELEASE)
	h.patched = false
	return nil
}

// DiagCave 诊断: 分配 cave、写入代码、回读机器码并检查页面保护。
// 不写 patch, 因此游戏不受影响 (不会崩溃)。用于排查:
//   - cave 机器码是否正确排布
//   - cave 页面是否具备 EXECUTE 权限
func (r *Refashion) DiagCave(hookSite uintptr) error {
	p := r.proc
	cave, err := virtAllocExNear(p.hand, caveSize, hookSite)
	if err != nil {
		return err
	}
	defer procVirtualFreeEx.Call(p.hand, cave, 0, MEM_RELEASE)
	slot := cave + slotOff

	if err := p.write(cave, buildHookCode(cave, slot, hookSite)); err != nil {
		return err
	}
	// 回读 cave 前 0x18 字节, 人工比对指令排布是否正确
	rb := make([]byte, 0x18)
	if _, err := p.read(cave, rb); err != nil {
		return err
	}
	fmt.Printf("cave 回读机器码: % X\n", rb)

	// VirtualQueryEx 检查页面保护 (确认可执行)
	mbi := make([]byte, 48)
	ret, _, _ := procVirtualQueryEx.Call(p.hand, cave, uintptr(unsafe.Pointer(&mbi[0])), uintptr(len(mbi)))
	if ret == 0 {
		return fmt.Errorf("VirtualQueryEx 失败")
	}
	// 64位 MEMORY_BASIC_INFORMATION 布局:
	// BaseAddress(8) AllocationBase(8) AllocationProtect(4) pad(4)
	// RegionSize(8) State(4) Protect(4) Type(4)
	allocProtect := binary.LittleEndian.Uint32(mbi[16:20])
	state := binary.LittleEndian.Uint32(mbi[32:36])
	protect := binary.LittleEndian.Uint32(mbi[36:40])
	mtype := binary.LittleEndian.Uint32(mbi[40:44])
	fmt.Printf("VirtualQueryEx: AllocProtect=%X State=%X Protect=%X Type=%X\n", allocProtect, state, protect, mtype)
	return nil
}

// ================================================================
// 底层 win32 API 封装 (仅供本包内部使用)
// ================================================================

var (
	kernel32 = syscall.NewLazyDLL("kernel32.dll")

	// 进程/内存操作
	procOpenProcess      = kernel32.NewProc("OpenProcess")           // 打开进程
	procReadProcessMem   = kernel32.NewProc("ReadProcessMemory")     // 读目标进程内存
	procWriteProcessMem  = kernel32.NewProc("WriteProcessMemory")    // 写目标进程内存
	procVirtualAllocEx   = kernel32.NewProc("VirtualAllocEx")        // 在目标进程分配内存
	procVirtualProtectEx = kernel32.NewProc("VirtualProtectEx")      // 修改页面保护
	procVirtualFreeEx    = kernel32.NewProc("VirtualFreeEx")         // 释放目标进程内存
	procVirtualQueryEx   = kernel32.NewProc("VirtualQueryEx")        // 查询页面信息
	procCloseHandle      = kernel32.NewProc("CloseHandle")           // 关闭句柄
	procGetLastError     = kernel32.NewProc("GetLastError")          // 取错误码
	procFlushInstrCache  = kernel32.NewProc("FlushInstructionCache") // 刷新指令缓存

	// Toolhelp 快照 (枚举模块)
	procCreateToolhelp32Snapshot = kernel32.NewProc("CreateToolhelp32Snapshot")
	procModule32FirstW           = kernel32.NewProc("Module32FirstW")
	procModule32NextW            = kernel32.NewProc("Module32NextW")
)

const (
	TH32CS_SNAPMODULE      = 0x00000008 // 快照类型: 枚举模块
	PROCESS_ALL_ACCESS     = 0x001F0FFF // 进程全部访问权限
	MEM_COMMIT             = 0x1000     // 内存已提交
	MEM_RESERVE            = 0x2000     // 内存已保留
	MEM_RELEASE            = 0x8000     // 释放内存
	PAGE_EXECUTE_READWRITE = 0x40       // 页面保护: 可执行+可读+可写
)

// moduleEntry32W 是 Win32 结构 MODULEENTRY32W 的内存布局（x64 下结构按 8 字节对齐）。
// 用于遍历进程模块快照以匹配 nioh2.exe，字段顺序/类型必须严格对齐 Win32 定义。
type moduleEntry32W struct {
	dwSize        uint32          // 结构大小(ms 数), 用前须赋为 sizeof
	th32ModuleID  uint32          // 模块 ID
	th32ProcessID uint32          // 所属进程 ID
	GlblcntUsage  uint32          // 模块为全局加载的次数
	ProccntUsage  uint32          // 模块为进程加载的次数
	modBaseAddr   uintptr         // 模块基址(关键: 用于基址+偏移定位 hook)
	modBaseSize   uint32          // 模块内存大小(用于限定 AOB 扫描范围)
	hModule       uintptr         // 模块句柄
	szModule      [256]uint16     // 模块名(如 "nioh2.exe")
	szExePath     [260]uint16     // 模块完整路径
}

// getLastErr 返回上次 Win32 调用的错误码。
func getLastErr() uintptr {
	ret, _, _ := procGetLastError.Call()
	return ret
}

// openProcess 调用 OpenProcess 以全部权限打开进程, 返回句柄。
func openProcess(pid uint32) (uintptr, error) {
	ret, _, _ := procOpenProcess.Call(PROCESS_ALL_ACCESS, 0, uintptr(pid))
	if ret == 0 {
		return 0, fmt.Errorf("OpenProcess failed, error=%d", getLastErr())
	}
	return ret, nil
}

// readMem 调用 ReadProcessMemory 从目标进程地址 addr 读取 len(out) 字节。
func readMem(hProc uintptr, addr uintptr, out []byte) (int, error) {
	var read uintptr
	ret, _, _ := procReadProcessMem.Call(hProc, addr, uintptr(unsafe.Pointer(&out[0])), uintptr(len(out)), uintptr(unsafe.Pointer(&read)))
	if ret == 0 {
		return int(read), fmt.Errorf("ReadProcessMemory failed @%X error=%d", addr, getLastErr())
	}
	return int(read), nil
}

// writeMem 调用 WriteProcessMemory 向目标进程地址 addr 写入 in。
func writeMem(hProc uintptr, addr uintptr, in []byte) error {
	var written uintptr
	ret, _, _ := procWriteProcessMem.Call(hProc, addr, uintptr(unsafe.Pointer(&in[0])), uintptr(len(in)), uintptr(unsafe.Pointer(&written)))
	if ret == 0 {
		return fmt.Errorf("WriteProcessMemory failed @%X error=%d", addr, getLastErr())
	}
	return nil
}

// virtAllocExNear 在目标地址 near 附近(±2GB内)分配可执行内存。
//
// 为什么必须靠近: hook 用的是 E9 相对跳转, 偏移量只有 32 位有符号 (±2GB)。
// 如果 cave 分配得过远 (比如距注入点几 TB), E9 的 rel32 会溢出成垃圾地址,
// 游戏跳到错误位置直接崩溃 (本项目曾因此反复崩溃)。
//
// 实现: 从 near 起向低地址和高地址依次探测空闲区域, 成功后校验距离在 ±2GB 内。
// 行为等价于 Cheat Engine 的 alloc(newmem,size,nearAddr)。
func virtAllocExNear(hProc uintptr, size int, near uintptr) (uintptr, error) {
	candidates := []uintptr{
		(near - 0x10000000) &^ 0xFFF, // -256MB
		(near - 0x20000000) &^ 0xFFF, // -512MB
		(near - 0x40000000) &^ 0xFFF, // -1GB
		(near - 0x80000000) &^ 0xFFF, // -2GB
		(near + 0x10000000) &^ 0xFFF, // +256MB
		(near + 0x40000000) &^ 0xFFF, // +1GB
	}
	var lastErr error
	for _, cand := range candidates {
		if cand == 0 {
			continue
		}
		ret, _, _ := procVirtualAllocEx.Call(hProc, cand, uintptr(size), MEM_COMMIT|MEM_RESERVE, PAGE_EXECUTE_READWRITE)
		if ret != 0 {
			// 校验距离在 ±2GB 内
			dist := int64(ret) - int64(near)
			if dist >= -0x7FFFFFFF && dist <= 0x7FFFFFFF {
				return ret, nil
			}
			// 距离不达标, 释放后继续找下一个候选
			procVirtualFreeEx.Call(hProc, ret, 0, MEM_RELEASE)
			continue
		}
		lastErr = fmt.Errorf("alloc @%X failed error=%d", cand, getLastErr())
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("no free region near %X", near)
	}
	return 0, lastErr
}

// protectMem 调用 VirtualProtectEx 修改目标进程页面的保护属性, 返回旧保护值。
// 注入前必须把 EXE 的 .text 段 (只读) 改成可写, 才能写入 patch 字节。
func protectMem(hProc uintptr, addr uintptr, size uintptr, newProtect uint32) (uint32, error) {
	var old uint32
	ret, _, _ := procVirtualProtectEx.Call(hProc, addr, size, uintptr(newProtect), uintptr(unsafe.Pointer(&old)))
	if ret == 0 {
		return 0, fmt.Errorf("VirtualProtectEx failed @%X error=%d", addr, getLastErr())
	}
	return old, nil
}

// getModuleBase 通过 Toolhelp 快照获取 nioh2.exe 模块基址与大小。
// 用于限定 AOB 扫描范围, 避免扫描整个地址空间。
func getModuleBase(pid uint32) (uintptr, uint32, error) {
	snap, _, _ := procCreateToolhelp32Snapshot.Call(TH32CS_SNAPMODULE, uintptr(pid))
	if snap == uintptr(0xFFFFFFFF) || snap == 0 {
		return 0, 0, fmt.Errorf("CreateToolhelp32Snapshot failed")
	}
	defer procCloseHandle.Call(snap)

	me := moduleEntry32W{}
	me.dwSize = uint32(unsafe.Sizeof(me))
	ret, _, _ := procModule32FirstW.Call(snap, uintptr(unsafe.Pointer(&me)))
	if ret == 0 {
		return 0, 0, fmt.Errorf("Module32First failed")
	}
	for ret != 0 {
		name := syscall.UTF16ToString(me.szModule[:])
		if name == "nioh2.exe" {
			return me.modBaseAddr, me.modBaseSize, nil
		}
		ret, _, _ = procModule32NextW.Call(snap, uintptr(unsafe.Pointer(&me)))
	}
	return 0, 0, fmt.Errorf("nioh2.exe module not found")
}

// aobScanAll 在 [base, base+size) 范围内扫描字节模式 pattern, 返回所有匹配地址。
//
// 实现: 按 0x1000 字节分块读取, 块与块之间保留 plen-1 字节重叠, 防止模式跨块
// 被漏掉。读取失败的分块跳过继续。
func aobScanAll(p *Process, base uintptr, size uintptr, pattern []byte) ([]uintptr, error) {
	const chunk = 0x1000
	plen := len(pattern)
	var hits []uintptr
	buf := make([]byte, chunk+plen-1)
	pos := uintptr(0)
	carry := 0
	for pos < size {
		toRead := uintptr(chunk)
		if pos+toRead > size {
			toRead = size - pos
		}
		n, err := p.read(base+pos, buf[carry:carry+int(toRead)])
		if err != nil {
			pos += toRead
			carry = 0
			continue
		}
		total := carry + n
		for i := 0; i+plen <= total; i++ {
			match := true
			for j := 0; j < plen; j++ {
				if buf[i+j] != pattern[j] {
					match = false
					break
				}
			}
			if match {
				hits = append(hits, base+pos+uintptr(i)-uintptr(carry))
			}
		}
		// 保留末尾 plen-1 字节作为下一个块的头部重叠
		keep := plen - 1
		if keep > total {
			keep = total
		}
		copy(buf[:keep], buf[total-keep:total])
		carry = keep
		pos += uintptr(n)
	}
	if len(hits) == 0 {
		return nil, fmt.Errorf("AOB pattern not found")
	}
	return hits, nil
}

// verifyHookSite 校验候选点 site 是否是真装备编辑器**函数入口**。
//
// 两层校验 (缺一不可):
//  1. 函数边界: site-5 起 5 字节 = C3 CC CC CC CC (上一函数 ret + 对齐填充)。
//     只有真函数入口前才是这个序列, 函数内部的碰巧字节在此被排除。
//  2. 函数体: site+7 起匹配 bodyWant 特征 (mov rax / mov rcx / mov rcx)。
//     disp32 (索引3~6) 随版本变化, 比对时跳过。
//
// 这套守卫解决了早期"7字节AOB命中多处、patch 到函数内部导致游戏崩溃"的问题。
func verifyHookSite(p *Process, site uintptr) bool {
	// 校验 1: 函数边界
	pre := make([]byte, len(boundary))
	if _, err := p.read(site-5, pre); err != nil {
		return false
	}
	for i := 0; i < len(boundary); i++ {
		if pre[i] != boundary[i] {
			return false
		}
	}

	// 校验 2: 函数体特征 (注入点+7 起)
	body := make([]byte, len(bodyWant))
	if _, err := p.read(site+hookLen, body); err != nil {
		return false
	}
	// disp32 (索引 3~6) 允许任意值
	for i := 0; i < len(body); i++ {
		if i >= 3 && i <= 6 {
			continue
		}
		if body[i] != bodyWant[i] {
			return false
		}
	}
	return true
}

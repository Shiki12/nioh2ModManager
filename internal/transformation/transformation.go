// Package transformation 提供 Nioh2 动态幻化(Refashion)的 Wails 绑定组件。
//
// 直接在与 *App 并列的 Bind 列表中注册后，Wails v2 会按本包名生成前端命名空间
// window.go.transformation.Ref。前端通过 RefashionArmor / RefashionWeapon 发起
// 一套异步自动幻化流程：后端跑 goroutine 逐格改写，遇到需要玩家配合的交互点
// （如"请用方向键把光标吸附到头槽"，或"确认当前选中是否头槽"）会向后端 emit
// "refashionPrompt" 事件并阻塞等待；前端在当前弹窗打印该提示后，玩家完成动作并
// 点击确认 → 前端调用 Ref.Confirm() 推进。进度文本经 "refashionLog" 事件推送；
// 最终结果经 "refashionDone" 事件通知。Cancel 随时可中止并清理已注入 hook。
package transformation

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	modinput "nioh2mod-js/internal/input"
	"nioh2mod-js/internal/config"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// Kind 幻化类型：仅服装 vs 武器。当前两者的自动流程共用同一套注入/捕获/改写核心，
// 仅起点槽位语义不同（服装=装备栏头槽起逐格纵向，武器=武器列表逐件）。
type Kind string

const (
	// KindArmor 服装幻化（装备栏纵向逐格）。
	KindArmor Kind = "armor"
	// KindWeapon 武器幻化（武器列表逐件）。
	KindWeapon Kind = "weapon"
)

const (
	// slotCaptureWait 单槽捕获等待窗口。悬停即选中，装备槽应在数秒内被 hook 捕获；
	// 超过此窗口仍无新捕获，判定为“空槽/无装备”，跳过该槽继续，避免整个流程卡死。
	slotCaptureWait = 2 * time.Second
)

// 事件名常量（前端以此订阅）。
const (
	EvtLog    = "refashionLog"    // 步骤/进度文本
	EvtPrompt = "refashionPrompt" // 需要玩家配合并点确认的交互提示
	EvtDone   = "refashionDone"   // 流程结束（含错误）
)

// RefLog 推给前端的进度/提示消息。
type RefLog struct {
	Stage   int    `json:"stage"`   // 当前阶段: 0=校准 1=确认 2=自动执行
	Step    int    `json:"step"`    // 当前已完成步骤序号
	Total   int    `json:"total"`   // 总幻化槽位数
	Message string `json:"message"` // 中文文本
}

// RefPrompt 交互提示：后端等待玩家协助并点确认后调用 Confirm()。
type RefPrompt struct {
	ID      int    `json:"id"`      // 交互序号（每轮流程递增）
	Kind    string `json:"kind"`    // 交互类型: calibrate/confirm-head
	Stage   int    `json:"stage"`   // 交互所属阶段
	Message string `json:"message"` // 引导文本
	Item    *Item  `json:"item"`    // 若有，附带当前捕获物品信息供玩家核对
}

// RefDone 流程结束结果。
type RefDone struct {
	OK      bool   `json:"ok"`
	Message string `json:"message"`
}

// Ref 是动态幻化组件的 Wails 绑定实例。
type Ref struct {
	ctx context.Context

	mu        sync.Mutex
	idGen     int            // 交互序号
	active    bool           // 当前是否有流程在跑
	confirmCh chan struct{}  // 等待前端确认（一次交互一个 token）
	cancelCh  chan struct{}  // 前端取消信号
	cancelOne sync.Once      // 保证 cancelCh 只 close 一次

	svc  *Refashion    // 当前流程的游戏进程服务
	hook *InjectedHook // 当前流程已注入的 hook

	stage int // 当前流程阶段: 0=校准 1=确认 2=自动执行
}

// pLog 幻化日志持久化句柄：写入 config 管理的 modman.log（与应用操作日志同源）。
var pLog = config.LoadLogs()

// NewRef 构造组件实例（外部注册用；ctx 由 OnStartup 或 SetContext 注入）。
func NewRef() *Ref { return &Ref{} }

// OnStartup 注入应用上下文，供事件推送。
// 注意：Wails 对绑定对象的 OnStartup 是否自动调用不可依赖——main.go 的
// OnStartup 已显式调用 SetContext(ctx)，这里保留仅供兼容/双保险。
func (r *Ref) OnStartup(ctx context.Context) { r.SetContext(ctx) }

// SetContext 设置组件上下文（main.go 启动时注入），事件推送依赖它。
func (r *Ref) SetContext(ctx context.Context) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.ctx = ctx
}

// RefashionArmor 发起服装幻化：按装备栏槽位序列逐格写入幻化ID。
// ids 为前端弹窗确认后传入的幻化ID数组（0 表示该槽跳过不改写）。
// 本方法立即返回；流程异步执行，经 EvtLog/EvtPrompt/EvtDone 事件上报。
func (r *Ref) RefashionArmor(pid uint32, ids []uint16) error {
	return r.start(KindArmor, pid, ids)
}

// RefashionWeapon 发起武器幻化（参数/语义同上）。
func (r *Ref) RefashionWeapon(pid uint32, ids []uint16) error {
	return r.start(KindWeapon, pid, ids)
}

// Confirm 供前端在玩家完成交互动作后推进流程：解除后端对应交互点的阻塞。
func (r *Ref) Confirm() {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.confirmCh != nil {
		select {
		case r.confirmCh <- struct{}{}:
		default:
		}
	}
}

// Cancel 取消当前流程：解除阻塞并清理已注入的 hook。
func (r *Ref) Cancel() {
	r.cancelOne.Do(func() {
		if r.cancelCh != nil {
			close(r.cancelCh)
		}
	})
}

// IsActive 报告当前是否有幻化流程在运行。
func (r *Ref) IsActive() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.active
}

// start 启动一次幻化流程的公共入口。
func (r *Ref) start(kind Kind, pid uint32, ids []uint16) error {
	if len(ids) == 0 {
		return fmt.Errorf("至少需要一个幻化ID")
	}
	if pid == 0 {
		gw := FindGameWindow()
		if gw == nil {
			return fmt.Errorf("未找到游戏窗口（请先启动游戏）")
		}
		pid = gw.PID()
		if pid == 0 {
			return fmt.Errorf("无法获取游戏进程 PID")
		}
	}
	r.mu.Lock()
	if r.active {
		r.mu.Unlock()
		return fmt.Errorf("已有幻化流程在运行（先 Cancel 或等待完成）")
	}
	r.active = true
	r.stage = 0
	r.confirmCh = make(chan struct{}, 1)
	r.cancelCh = make(chan struct{})
	r.cancelOne = sync.Once{}
	r.mu.Unlock()

	go r.runFlow(kind, pid, ids)
	return nil
}

// runFlow 在 goroutine 中执行完整幻化流程（阻塞式，直至完成或取消）。
func (r *Ref) runFlow(kind Kind, pid uint32, ids []uint16) {
	defer func() {
		// 流程结束恢复主窗口非置顶、并带来前台方便用户查看结果
		runtime.WindowSetAlwaysOnTop(r.ctx, false)
		runtime.WindowShow(r.ctx)
		r.mu.Lock()
		r.active = false
		r.mu.Unlock()
	}()

	// 幻化期间把主窗口置顶但不抢焦点：进度/日志浮在游戏之上可见，
	// 又不强制获取键盘/鼠标焦点，玩家鼠标仍可正常在游戏槽位悬停。
	if r.ctx != nil {
		runtime.WindowSetAlwaysOnTop(r.ctx, true)
	}

	// 记录“用户发起幻化”这一操作本身（含类型与逐槽幻化ID清单），持久化到操作日志
	{
		kindName := "服装"
		if kind == KindWeapon {
			kindName = "武器"
		}
		parts := make([]string, 0, len(ids))
		for i, id := range ids {
			if id == 0 {
				parts = append(parts, fmt.Sprintf("#%d=skip", i+1))
			} else {
				parts = append(parts, fmt.Sprintf("#%d=%04X", i+1, id))
			}
		}
		pLog.Append(fmt.Sprintf("用户发起幻化[%s]: %s", kindName, strings.Join(parts, " ")))
	}

	svc, err := NewRefashion(pid)
	if err != nil {
		r.emitDone(false, "打开游戏进程失败: "+err.Error())
		return
	}
	r.svc = svc
	defer svc.Close()

	err = r.doFlow(kind, ids)
	if err != nil {
		r.emitDone(false, err.Error())
		return
	}
	r.emitDone(true, "幻化完成，已刷新 Mod 缓存。请回到游戏取下装备再穿上以刷新外观")
}

// doFlow 执行从注入到逐格改写的完整流程；任一交互点阻塞等待前端确认/取消。
func (r *Ref) doFlow(kind Kind, ids []uint16) error {
	gw := FindGameWindow()
	if gw == nil {
		return fmt.Errorf("未找到游戏窗口")
	}
	gw.BringToFront()

	oX, oY, ook := gw.OriginScreen()
	cw, ch, cok := gw.ClientSize()
	if !ook || !cok || cw <= 0 || ch <= 0 {
		return fmt.Errorf("获取窗口客户区原点/尺寸失败（可能最小化）")
	}
	r.emitLog(0, len(ids), fmt.Sprintf("客户区: 原点(%d,%d) 尺寸 %dx%d", oX, oY, cw, ch))

	// 先把鼠标挪到屏幕空白区（避开装备槽），再注入 hook，避免捕获到第一个装备栏。
	r.emitLog(0, len(ids), "先把鼠标移到屏幕空白区（避开装备槽）…")
	input.MoveTo(40, 40)
	time.Sleep(300 * time.Millisecond)

	hook, err := r.svc.InstallHook()
	if err != nil {
		return fmt.Errorf("注入失败（上游有残留 hook 时需先 cleanup）: %w", err)
	}
	r.hook = hook
	defer r.unhook()
	r.emitLog(0, len(ids), fmt.Sprintf("hook 已注入，共 %d 件", len(ids)))

	// ================================================================
	// 阶段一：校准（回车确认，失败可重复校准）
	// 每个采样点都用游戏内方向键吸附槽位中心，基准一致，避免偏差累计跳行。
	// 注意：此处确认必须用「回车」而非鼠标点击——点击按钮会把鼠标移到按钮上，
	// 后端此时读 MousePos 会读到按钮坐标而非游戏槽位中心，导致基准全错。
	// ================================================================
	r.stage = 0
	r.emitLog(0, len(ids), "【步骤1/3】确定原始坐标和间距：校准两个槽位")
	sampleSlot := func(label string, step int) (int, int, error) {
		msg := fmt.Sprintf("校准（第%d点）：请回游戏用方向键吸附到【%s】槽中心，回车确认", step, label)
		item, ok := r.waitPrompt("calibrate", msg, nil)
		if !ok {
			return 0, 0, fmt.Errorf("已取消")
		}
		_ = item
		px, py := input.MousePos()
		cx, cy, sck := gw.ToClient(px, py)
		if !sck {
			return 0, 0, fmt.Errorf("采样换算客户区坐标失败")
		}
		return cx, cy, nil
	}

	var cxHead, cyHead, rowDX, rowDY int
	calibOK := false
	for !calibOK {
		if !r.alive() {
			return fmt.Errorf("已取消")
		}
		cxHead, cyHead, err = sampleSlot("头槽", 1)
		if err != nil {
			return err
		}
		cxNext, cyNext, err := sampleSlot("头槽正下方那一格", 2)
		if err != nil {
			return err
		}
		rowDX = cxNext - cxHead
		rowDY = cyNext - cyHead
		if rowDY <= 0 {
			r.emitLog(0, len(ids), "校准失败：行高须为正（顺序反了或采样偏差），准备重新校准")
			_, ok := r.waitPrompt("recalibrate", "校准失败，请重试：\n回游戏吸附【头槽】→ 回车；再吸附【头槽正下方那一格】→ 回车。", nil)
			if !ok {
				return fmt.Errorf("已取消")
			}
			continue
		}
		calibOK = true
	}
	r.emitLog(0, len(ids), fmt.Sprintf("校准完成: 头槽客户区(%d,%d) 行向量(%d,%d)", cxHead, cyHead, rowDX, rowDY))
	// 校准成功后明确弹窗告知（用户需来回切换游戏/弹窗屏幕，必须给出清晰反馈）
	_, ok := r.waitPrompt("calib-ok", "校准成功。请回游戏把光标对准【头槽】，回车确认", nil)
	if !ok {
		return fmt.Errorf("已取消")
	}

	slotClient := func(i int) (int, int) {
		return cxHead + i*rowDX, cyHead + i*rowDY
	}

	// ================================================================
	// 阶段二：回到头部槽位，捕获并确认是头槽（按钮确认）
	// ================================================================
	r.stage = 1
	r.emitLog(1, len(ids), "【步骤2/3】确定是否准确：回到头部槽位并确认…")
	_, err = r.confirmStartSlotHead(gw, slotClient, 0)
	if err != nil {
		return err
	}

	// ================================================================
	// 阶段三：逐格幻化（自动，从头槽 i=0 开始遍历）
	// ================================================================
	r.stage = 2
	r.emitLog(1, len(ids), "【步骤3/4】校准通过，自动执行：逐格幻化…")
	// ---- 逐格下移：甩开鼠标 → 游戏带回前台 → 定位目标槽 → 捕获 → 改写 ----
	// 从头槽(i=0)开始遍历：某槽为空（无装备）时直接跳过并上报前端。
	prevAddr := uintptr(0)
	for i := 0; i < len(ids); i++ {
		if !r.alive() {
			return fmt.Errorf("已取消")
		}
		input.MoveTo(40, 40)
		time.Sleep(100 * time.Millisecond)
		r.clearSlot()
		gw.BringToFront()
		time.Sleep(100 * time.Millisecond)
		cx, cy := slotClient(i)
		sx, sy, ok := gw.ToScreen(cx, cy)
		if !ok {
			return fmt.Errorf("槽位坐标换算失败")
		}
		input.MoveTo(sx, sy)
		time.Sleep(200 * time.Millisecond)

		next, captured, err := r.traverseCapture(prevAddr, slotCaptureWait)
		if err != nil {
			return fmt.Errorf("捕获第 %d 件失败: %w", i+1, err)
		}
		if !captured {
			// 空槽（无装备）：编辑器入口不触发、slot 里没有新指针 → 跳过，不阻塞流程
			r.emitLog(i, len(ids), fmt.Sprintf("[%d/%d] 第 %d 格无装备，跳过", i+1, len(ids), i+1))
			continue
		}
		r.emitLog(i, len(ids), fmt.Sprintf("第%d件: 物品ID=%04X 幻化ID=%04X", i+1, next.ItemID, next.ModelID))
		if ids[i] == 0 {
			r.emitLog(i+1, len(ids), fmt.Sprintf("[%d/%d] 该槽位无幻化，跳过改写", i+1, len(ids)))
		} else if err := r.hook.SetRefashionID(next, ids[i]); err != nil {
			return fmt.Errorf("改写第 %d 件失败: %w", i+1, err)
		} else {
			r.emitLog(i+1, len(ids), fmt.Sprintf("[%d/%d] 幻化ID -> %04X", i+1, len(ids), ids[i]))
		}
		prevAddr = next.Addr
	}

	// ================================================================
	// 阶段四：刷新游戏内 Mod 缓存（F10 重载）
	// ================================================================
	r.stage = 3
	r.emitLog(len(ids), len(ids), "【步骤4/4】刷新游戏内 Mod 缓存…")
	if modinput.RefreshMods() {
		r.emitLog(len(ids), len(ids), "Mod 缓存已刷新")
	} else {
		r.emitLog(len(ids), len(ids), "未找到游戏窗口，跳过 Mod 刷新")
	}
	return nil
}

// confirmStartSlotHead 捕获一次起始槽（头），打印捕获物品让玩家确认是头槽。
// 玩家确认后返回该物品；不确认则让玩家回游戏移动光标后重新捕获。
// 若头槽为空（超时未捕获到装备），返回 (nil, nil)，调用方应跳过头部改写。
func (r *Ref) confirmStartSlotHead(gw *GameWindow, slotClient func(int) (int, int), idx int) (*Item, error) {
	for {
		if !r.alive() {
			return nil, fmt.Errorf("已取消")
		}
		// 把鼠标甩开再落回目标槽，确保游戏重新悬停在该槽
		input.MoveTo(40, 40)
		time.Sleep(100 * time.Millisecond)
		r.clearSlot()
		gw.BringToFront()
		time.Sleep(100 * time.Millisecond)
		cx, cy := slotClient(idx)
		sx, sy, _ := gw.ToScreen(cx, cy)
		input.MoveTo(sx, sy)
		time.Sleep(200 * time.Millisecond)

		item, captured, err := r.traverseCapture(0, slotCaptureWait)
		if err != nil {
			return nil, err
		}
		if !captured {
			// 头槽为空：无装备可捕获，直接返回空，由调用方按空槽跳过
			return nil, nil
		}
		msg := fmt.Sprintf("光标是否停在【头部装备栏】？捕获：物品ID=%04X 幻化ID=%04X。\n是 → 回车；不是 → 回游戏对准头槽后重试", item.ItemID, item.ModelID)
		_, ok := r.waitPrompt("confirm-head", msg, item)
		if !ok {
			return nil, fmt.Errorf("已取消")
		}
		return item, nil
	}
}

// clearSlot 把 hook 槽（equipment_ptr）清零，消除上一次悬停残留的陈旧指针。
// 若不清理，空槽会被误判：slot 里旧的指针既可能被当成"新捕获"，
// 也可能等于 prevAddr 导致有装备的槽超时被跳过。
func (r *Ref) clearSlot() {
	if r.hook == nil {
		return
	}
	_ = r.svc.Process().Write(r.hook.Slot(), make([]byte, 8))
}

// traverseCapture 轮询已注入 hook 的 slot，等待捕获到与 prevAddr 不同的新物品指针。
// prevAddr=0 表示等待任意非空捕获（用于起始槽位）。
//
// 返回值：
//   - captured=true：捕获到新物品，item 为其头部信息；
//   - captured=false：等待超时仍无新指针 —— 视为该槽为空（无装备），
//     调用方应跳过该槽继续，而不是中止整个流程。
//   - err：流程已取消或读取失败，调用方应中止。
func (r *Ref) traverseCapture(prevAddr uintptr, timeout time.Duration) (*Item, bool, error) {
	deadline := time.Now().Add(timeout)
	slot := r.hook.Slot()
	prev := uint64(prevAddr)
	for time.Now().Before(deadline) {
		if !r.alive() {
			return nil, false, fmt.Errorf("已取消")
		}
		addr, err := r.svc.Process().ReadUint64(slot)
		if err == nil && addr != 0 && addr != prev {
			item, err := r.svc.Process().ReadItem(uintptr(addr))
			if err != nil {
				return nil, false, err
			}
			return item, true, nil
		}
		time.Sleep(120 * time.Millisecond)
	}
	return nil, false, nil
}

// waitPrompt 推交互提示并阻塞等待前端 Confirm()（返回 true）或 Cancel（返回 false）。
func (r *Ref) waitPrompt(kind string, msg string, item *Item) (*Item, bool) {
	r.mu.Lock()
	r.idGen++
	id := r.idGen
	confirm := r.confirmCh
	cancel := r.cancelCh
	r.mu.Unlock()

	r.emitPrompt(id, kind, msg, item)
	select {
	case <-cancel:
		return nil, false
	case <-confirm:
		return item, true
	}
}

// alive 报告流程是否仍未被取消（内部交互点轮询用）。
func (r *Ref) alive() bool {
	select {
	case <-r.cancelCh:
		return false
	default:
		return true
	}
}

// unhook 卸载当前 hook（结束或失败时调用）。
func (r *Ref) unhook() {
	if r.hook != nil && r.hook.Patched() {
		_ = r.hook.Uninstall()
	}
	r.hook = nil
}

// emitLog 推送一条进度日志。
func (r *Ref) emitLog(step, total int, msg string) {
	fmt.Printf("[refashion] stage=%d step=%d/%d %s\n", r.stage, step, total, msg)
	pLog.Append(msg)
	if r.ctx != nil {
		runtime.EventsEmit(r.ctx, EvtLog, RefLog{Stage: r.stage, Step: step, Total: total, Message: msg})
	}
}

// emitPrompt 推送一条交互提示。
func (r *Ref) emitPrompt(id int, kind string, msg string, item *Item) {
	fmt.Printf("[refashion:prompt] id=%d stage=%d kind=%s\n  %s\n", id, r.stage, kind, msg)
	pLog.Append("[交互] " + msg)
	if r.ctx != nil {
		runtime.EventsEmit(r.ctx, EvtPrompt, RefPrompt{ID: id, Kind: kind, Stage: r.stage, Message: msg, Item: item})
	}
}

// emitDone 推送流程结束事件。
func (r *Ref) emitDone(ok bool, msg string) {
	fmt.Printf("[refashion:done] ok=%v %s\n", ok, msg)
	if ok {
		pLog.Append("[完成] " + msg)
	} else {
		pLog.Append("[失败] " + msg)
	}
	if r.ctx != nil {
		runtime.EventsEmit(r.ctx, EvtDone, RefDone{OK: ok, Message: msg})
	}
}

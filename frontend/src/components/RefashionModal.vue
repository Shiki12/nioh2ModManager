<template>
  <a-modal
    :open="open"
    :title="`一键幻化：${mod?.nickname || mod?.name || ''}`"
    :closable="!running"
    :mask-closable="false"
    width="min(640px, 92vw)"
    centered
    :z-index="zIndex"
    @cancel="onCancel"
  >
    <!-- 四步流程条：校准 → 确认 → 自动执行 → 刷新缓存 -->
    <a-steps v-if="running" :current="currentStep" size="small" class="flow-steps" style="margin-bottom:12px">
      <a-step title="确定坐标间距" description="校准头槽与间距" />
      <a-step title="确定是否准确" description="核对头槽装备" />
      <a-step title="自动执行" description="逐件幻化" />
      <a-step title="刷新缓存" description="重载 Mod 文件" />
    </a-steps>
    <!-- 待幻化装备列表（未开始 且 未完成） -->
    <div v-if="!running && !finished">
      <div v-if="rows.length === 0" style="color:#888;padding:16px 0">
        该 Mod 未登记占用的装备资源，无法自动幻化。请先在「编辑 Mod」中填写占用。
      </div>
      <template v-else>
        <div class="tip">
          勾选需要幻化的装备（默认全选）。确认后程序将自动进入游戏，逐件改写外观 ID。
        </div>
        <div class="refash-list">
          <div v-for="row in rows" :key="row.slot" class="refash-row" @click="!row.skip && (row.checked = !row.checked)">
            <a-checkbox :checked="row.checked" :disabled="row.skip" @click.stop="!row.skip && (row.checked = !row.checked)" />
            <a-tag class="slot-tag">{{ row.slot }}</a-tag>
            <a-tooltip v-if="row.skip" :title="row.name ? `未找到该装备的幻化ID，将跳过` : '该槽位未占用，将跳过'">
              <span class="eq-name skip">{{ row.name || '（未占用）' }}</span>
              <span class="eq-id skip-tag">skip</span>
            </a-tooltip>
            <template v-else>
              <span class="eq-name" :title="row.name">{{ row.name }}</span>
              <span class="eq-id">ID: {{ row.id }}</span>
            </template>
          </div>
        </div>
      </template>
    </div>

    <!-- 运行中：进度 + 交互提示 + 日志 -->
    <div v-else-if="running">
      <a-spin spinning class="running-spin">
        <div class="running-panel">
          <a-progress
            :percent="progressPercent"
            size="small"
            :stroke-color="{ '0%': '#1677ff', '100%': '#52c41a' }"
            style="margin-bottom:10px"
          />

          <!-- 步骤3: 大号醒目 “正在处理”横幅 -->
          <div v-if="flowStage === 2 && currentItem" class="current-banner">
            <div class="current-pulse" />
            <div class="current-body">
              <div class="current-label">正在幻化 · 第 {{ currentItem.idx + 1 }}/{{ plan.length }} 件</div>
              <div class="current-main">
                <a-tag color="orange" class="current-slot">{{ currentItem.slot }}</a-tag>
                <span class="current-name">{{ currentItem.name }}</span>
                <span class="current-id">→ ID {{ hex(currentItem.id) }}</span>
              </div>
            </div>
          </div>

          <!-- 步骤4: 刷新缓存 状态条 -->
          <a-alert
            v-if="flowStage === 3"
            class="refresh-alert"
            type="info"
            show-icon
            message="正在刷新游戏内 Mod 缓存…"
            description="已发 F10 重载 Mod 文件，稍候即可看到日志确认。"
          />

          <!-- 步骤3: 自动执行 逐件明细 -->
          <div v-if="flowStage === 2 && plan.length" class="exec-card">
            <div class="exec-head">
              <span class="exec-head-title">逐件幻化进度</span>
              <span class="exec-head-count">{{ doneCount }}/{{ plan.length }} 已完成</span>
            </div>
            <div class="exec-list">
              <div
                v-for="(p, idx) in plan"
                :key="p.slot"
                class="exec-row"
                :class="itemState(idx)"
              >
                <span class="exec-no">{{ String(idx + 1).padStart(2, '0') }}</span>
                <span class="exec-status">
                  <a-spin v-if="itemState(idx) === 'active'" :spinning="true" size="small" />
                  <span v-else-if="itemState(idx) === 'done'" class="done-mark">✓</span>
                  <span v-else-if="itemState(idx) === 'skip'" class="skip-mark">–</span>
                  <span v-else class="exec-dot" />
                </span>
                <a-tag class="slot-tag">{{ p.slot }}</a-tag>
                <span class="exec-name">{{ p.skip ? (p.name || '未占用') : p.name }}</span>
                <span class="exec-id">{{ p.skip ? 'skip' : hex(p.id) }}</span>
              </div>
            </div>
          </div>

          <div class="log-box">
            <div v-for="(l, i) in logs" :key="i" class="log-line">{{ l }}</div>
            <div v-if="logs.length === 0" class="log-line dim">等待开始…</div>
          </div>

          <a-alert
            v-if="prompt"
            class="prompt-alert"
            type="warning"
            show-icon
            :message="prompt.message"
          >
            <template #description>
              <div v-if="prompt.item" class="prompt-item">
                当前捕获物品：物品ID={{ hex(prompt.item.ItemID) }} 幻化ID={{ hex(prompt.item.ModelID) }}
              </div>
              <span class="prompt-hint">操作完成后【不要动鼠标】，用键盘【Alt+Tab】切回本窗口，然后【按回车】确认（或点击下方按钮）</span>
            </template>
          </a-alert>
        </div>
      </a-spin>
    </div>

    <!-- 完成通知 -->
    <a-result
      v-else
      :status="doneOk ? 'success' : 'error'"
      :title="doneOk ? '幻化完成' : '幻化失败'"
      :sub-title="doneMsg"
    >
      <template #extra>
        <a-button type="primary" @click="close">关闭</a-button>
      </template>
    </a-result>

    <template #footer>
      <template v-if="!running && !finished">
        <a-button @click="onCancel">取消</a-button>
        <a-button type="primary" :disabled="rows.length === 0 || !checkedCount" @click="start">开始幻化</a-button>
      </template>
      <template v-else-if="running">
        <a-button v-if="prompt" type="primary" @click="confirm">{{ btnLabel }}</a-button>
        <a-button danger @click="cancelFlow">中止</a-button>
      </template>
    </template>
  </a-modal>
</template>

<script setup>
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { message } from 'ant-design-vue'
import { GetArmorParts } from '../../wailsjs/go/main/App'
import { RefashionArmor, Confirm, Cancel } from '../../wailsjs/go/transformation/Ref'
import { EventsOn } from '../../wailsjs/runtime/runtime'

const props = defineProps({
  open: { type: Boolean, default: false },
  mod: { type: Object, default: null },
})
const emit = defineEmits(['update:open', 'done'])

const slots = ref([])     // [{name, parts:[{id,name}]}]
const rows = ref([])      // [{slot,name,id,checked,skip}]
const zIndex = ref(1000)  // 完成后提升 z-index，保证显示在最上层
const running = ref(false)
const finished = ref(false)
const doneOk = ref(false)
const doneMsg = ref('')
const logs = ref([])
const prompt = ref(null)
const keyConfirmPending = ref(false) // 是否等待玩家按回车确认（校准）
const progress = ref({ step: 0, total: 0 })
const flowStage = ref(0) // 当前步骤: 0=校准 1=确认 2=自动执行 3=刷新缓存
const plan = ref([])     // 自动执行阶段的逐件清单 [{slot,name,id,skip}]

// 流程图当前高亮步（0..3），完成时指向末尾
const currentStep = computed(() => {
  if (finished.value) return 4
  return flowStage.value
})

const checkedCount = computed(() => rows.value.filter(r => r.checked).length)
const btnLabel = computed(() => {
  if (!prompt.value) return '继续'
  switch (prompt.value.kind) {
    case 'recalibrate': return '重新校准'
    case 'calib-ok': return '继续（进入下一步）'
    default: return '回车确认'
  }
})
const progressPercent = computed(() => {
  const t = progress.value.total || rows.value.length
  if (!t) return 0
  return Math.min(100, Math.round((progress.value.step / t) * 100))
})
// 已完成件数（含 skip，均视为完成）
const doneCount = computed(() => plan.value.filter((p, idx) => itemState(idx) === 'done' || itemState(idx) === 'skip').length)
// 当前正在处理的件（不含 skip）；无则 null
const currentItem = computed(() => {
  const idx = plan.value.findIndex((p, i) => itemState(i) === 'active')
  return idx >= 0 ? { ...plan.value[idx], idx } : null
})
// 每件装备当前处理状态: 0=待处理 1=正在处理 2=完成 skip=true 为跳过
const itemState = (idx) => {
  const p = plan.value[idx]
  if (!p || p.skip) return p?.skip ? 'skip' : 'todo'
  const i = progress.value.step
  if (i <= 0) return 'todo'
  if (i >= plan.value.length) return 'done'
  if (idx < i) return 'done'
  if (idx === i) return 'active'
  return 'todo'
}

// 部位显示顺序（与后端 bmo 固定槽位逐格语义一致）
const slotOrder = ['头', '胸甲', '臂甲', '膝甲', '腿甲']

// 名字 → 幻化ID 反查（armordata.Part.ID）
function findId(slotName, eqName) {
  const name = (eqName || '').trim()
  if (!name) return ''
  const s = slots.value.find(s => s.name === slotName)
  const p = s?.parts?.find(pp => pp.name === name || pp.name === eqName)
  return p ? p.id : ''
}

// 组装固定槽位行：5 个部位固定展示，缺失/无头装饰的槽位标注 skip（id=0）。
// 与后端 parseRefIDList 的 `0=skip` 语义一致：该槽跳过改写但光标照常移过。
function buildRows(mod) {
  const out = []
  if (!mod?.parts) {
    for (const slotName of slotOrder) out.push({ slot: slotName, name: '', id: '', checked: false, skip: true })
    return out
  }
  const firstOf = (slotName) => {
    const names = Array.isArray(mod.parts[slotName]) ? mod.parts[slotName] : (mod.parts[slotName] ? [mod.parts[slotName]] : [])
    return names[0] || ''
  }
  for (const slotName of slotOrder) {
    const name = firstOf(slotName)
    const id = name ? findId(slotName, name) : ''
    out.push({ slot: slotName, name, id, checked: !!name, skip: !name || !id })
  }
  return out
}

watch(() => props.open, async (open) => {
  if (!open) return
  reset()
  try {
    slots.value = await GetArmorParts()
  } catch (e) {
    message.error(`加载装备数据失败: ${e}`)
  }
  rows.value = buildRows(props.mod)
  subscribe()
})

function reset() {
  running.value = false
  finished.value = false
  doneOk.value = false
  doneMsg.value = ''
  logs.value = []
  prompt.value = null
  keyConfirmPending.value = false
  progress.value = { step: 0, total: 0 }
  flowStage.value = 0
}

// 校准交互需要玩家「用游戏内方向键吸附槽位中心后，不动鼠标，直接按回车确认」——
// 因为 Wails 前端点击确认会把鼠标移到按钮上，后端此时读 MousePos 会读到按钮坐标
// 而非游戏槽位中心，导致坐标基准全错。回车确认则鼠标全程停在游戏里。
function onKeydown(e) {
  if (e.key !== 'Enter') return
  if (running.value && prompt.value && keyConfirmPending.value) {
    e.preventDefault()
    e.stopPropagation()
    confirm()
  }
}
onMounted(() => window.addEventListener('keydown', onKeydown))
onUnmounted(() => window.removeEventListener('keydown', onKeydown))

// ---- 事件订阅（进入弹窗时注册一次，退出时注销）----
const offs = []
function subscribe() {
  offs.length = 0
  offs.push(EventsOn('refashionLog', (p) => {
    if (p?.message) {
      logs.value.push(p.message)
      if (logs.value.length > 200) logs.value.shift()
    }
    progress.value = { step: p?.step || 0, total: p?.total || 0 }
    if (typeof p?.stage === 'number') flowStage.value = p.stage
  }))
  offs.push(EventsOn('refashionPrompt', (p) => {
    if (p?.message) {
      prompt.value = p
      if (typeof p?.stage === 'number') flowStage.value = p.stage
      // 全程统一回车确认：任一交互点都要求鼠标停在游戏槽位、不按键、用 Alt+Tab 切回弹窗按回车
      keyConfirmPending.value = true
    }
  }))
  offs.push(EventsOn('refashionDone', (p) => {
    running.value = false
    finished.value = true
    doneOk.value = !!p?.ok
    doneMsg.value = p?.message || ''
    zIndex.value += 1
    window.focus()
    emit('done', p)
  }))
}
function unsubscribe() {
  for (const off of offs) off()
  offs.length = 0
}

function hex(v) {
  return '0x' + (v != null ? Number(v).toString(16).toUpperCase().padStart(4, '0') : '----')
}

async function start() {
  const armorRows = rows.value.filter(r => r.checked)
  if (!armorRows.length) {
    message.warning('请至少勾选一件装备')
    return
  }
  logs.value = []
  prompt.value = null
  running.value = true
  // 自动执行阶段的逐件清单：固定 5 槽位（头/胸/臂/膝/腿），缺失或未勾选 → skip
  plan.value = slotOrder.map(slotName => {
    const row = rows.value.find(r => r.slot === slotName)
    const skip = !row || row.skip || !row.checked
    return {
      slot: slotName,
      name: row?.name || '',
      id: skip ? 0 : parseInt(row.id, 16),
      skip,
    }
  })
  try {
    // 服装：固定 5 槽位对齐（头/胸/臂/膝/腿），槽位缺失或未勾选 → 0=skip，逐个元素对应后端逐格遍历
    const list = plan.value.map(p => (p.skip ? 0 : p.id))
    if (!list.some(v => v)) { running.value = false; message.warning('请至少勾选一个已知幻化ID的装备部位'); return }
    await RefashionArmor(0, list)
  } catch (e) {
    running.value = false
    finished.value = true
    doneOk.value = false
    doneMsg.value = String(e)
  }
}

function confirm() {
  prompt.value = null
  keyConfirmPending.value = false
  Confirm()
}

async function cancelFlow() {
  try { await Cancel() } catch (_) {}
}

function onCancel() {
  if (running.value) return
  close()
}

function close() {
  unsubscribe()
  emit('update:open', false)
}
</script>

<style scoped>
.tip { color:#666; font-size:13px; margin-bottom:10px; }
.refash-list { max-height: 46vh; overflow:auto; border:1px solid #eee; border-radius:8px; }
.refash-row { display:flex; align-items:center; gap:8px; padding:8px 12px; border-bottom:1px solid #f5f5f5; cursor:pointer; }
.refash-row:hover { background:#fafafa; }
.refash-row:last-child { border-bottom:none; }
.slot-tag { margin-inline-end:0; flex-shrink:0; }
.eq-name { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.eq-id { color:#999; font-size:12px; font-family:Consolas,monospace; flex-shrink:0; }
.running-spin { display:block; }
.running-panel { min-height: 200px; }
.current-banner { position:relative; display:flex; align-items:center; gap:14px; padding:18px 20px; margin-bottom:12px; border-radius:12px; background:linear-gradient(135deg,#fff7e6,#ffecd0); border:2px solid #ffc069; overflow:hidden; }
.current-pulse { position:absolute; left:0; top:0; width:100%; height:100%; background:radial-gradient(circle, rgba(250,140,22,0.18) 0%, transparent 70%); animation:currentPulse 1.6s ease-in-out infinite; pointer-events:none; }
@keyframes currentPulse { 0%,100%{opacity:0.35;} 50%{opacity:0.95;} }
.current-body { position:relative; z-index:1; flex:1; min-width:0; }
.current-label { font-size:13px; color:#d46b08; font-weight:700; margin-bottom:6px; letter-spacing:1px; }
.current-main { display:flex; align-items:center; gap:12px; }
.current-slot { font-weight:700; font-size:14px; }
.current-name { font-size:24px; font-weight:800; color:#d4380d; flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.current-id { font-size:20px; font-family:Consolas,monospace; color:#fa8c16; font-weight:800; flex-shrink:0; }
.exec-row.active { animation:rowGlow 1.4s ease-in-out infinite; }
@keyframes rowGlow { 0%,100%{ background:#fff7e6;} 50%{ background:#ffe7ba;} }
.exec-card { border:1px solid #e6f0ff; border-radius:10px; margin-bottom:10px; overflow:hidden; box-shadow:0 1px 2px rgba(0,0,0,0.03); }
.exec-head { display:flex; align-items:center; justify-content:space-between; padding:8px 12px; background:#f0f7ff; border-bottom:1px solid #e6f0ff; }
.exec-head-title { font-weight:600; color:#1a73e8; font-size:13px; }
.exec-head-count { font-size:12px; color:#666; }
.exec-list { overflow:hidden; }
.exec-row { display:flex; align-items:center; gap:8px; padding:7px 12px; border-bottom:1px solid #f5f7ff; }
.exec-row:last-child { border-bottom:none; }
.exec-no { width:22px; font-family:Consolas,monospace; font-size:12px; color:#b0b8c4; flex-shrink:0; }
.exec-row.done .exec-no { color:#52c41a; font-weight:700; }
.exec-row.todo { color:#c9cdd4; background:#fff; }
.exec-row.todo .exec-id { color:#d0d3d8; font-weight:400; }
.exec-row.active { border-left:3px solid #fa8c16; animation:rowGlow 1.4s ease-in-out infinite; }
@keyframes rowGlow { 0%,100%{ background:#fff7e6;} 50%{ background:#ffe7ba;} }
.exec-row.active .exec-no { color:#fa8c16; font-weight:700; }
.exec-row.active .exec-name { color:#d4380d; font-weight:700; }
.exec-row.skip { color:#c9cdd4; background:#fafafa; }
.exec-status { width:18px; display:inline-flex; justify-content:center; flex-shrink:0; }
.done-mark { color:#52c41a; font-weight:700; font-size:14px; }
.skip-mark { color:#c9cdd4; font-size:14px; }
.exec-dot { width:8px; height:8px; border-radius:50%; background:#e2e4e8; display:inline-block; }
.exec-name { flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; font-size:13px; }
.exec-row.done .exec-name { color:#237804; font-weight:600; }
.exec-id { color:#1a73e8; font-size:12px; font-family:Consolas,monospace; flex-shrink:0; font-weight:500; }
.exec-row.done .exec-id { color:#389e0d; font-weight:700; }
.exec-row.skip .exec-id { color:#c9cdd4; font-weight:400; }
.exec-row.done .exec-dot { display:none; }
.log-box { background:#0f172a; color:#d1d5db; font-family:Consolas,Menlo,monospace; font-size:12px; border-radius:8px; padding:10px 12px; max-height: 30vh; overflow:auto; margin-bottom:12px; }
.log-line { line-height:1.7; white-space:pre-wrap; word-break:break-all; }
.log-line.dim { color:#6b7280; }
.prompt-alert { margin-top: 4px; }
.refresh-alert { margin-bottom: 10px; }
.prompt-item { font-family:Consolas,monospace; font-size:12px; margin-bottom:4px; }
.prompt-hint { color:#d48806; font-size:12px; }
</style>

<template>
  <a-modal
    :open="open"
    :title="`武器幻化：${mod?.nickname || mod?.name || '独立幻化'}`"
    :closable="!running"
    :mask-closable="false"
    width="min(640px, 92vw)"
    centered
    :z-index="zIndex"
    @cancel="onCancel"
  >
    <!-- 三步流程条：校准 → 捕获并写入 → 刷新缓存 -->
    <a-steps v-if="running" :current="currentStep" size="small" class="flow-steps" style="margin-bottom:12px">
      <a-step title="确定武器槽" description="悬停武器槽+确认" />
      <a-step title="捕获并幻化" description="自动改写外观" />
      <a-step title="刷新缓存" description="重载 Mod 文件" />
    </a-steps>

    <!-- 未开始：选目标武器 -->
    <div v-if="!running && !finished">
      <div v-if="weaponPickOptions.length === 0" style="color:#888;padding:16px 0">
        未找到可用武器数据（weapon_parts.json 缺失），无法幻化。
      </div>
      <template v-else>
        <div class="tip">
          选择要幻化成的【目标武器】后点「开始幻化」。程序会打开捕获槽：你在游戏里用鼠标悬停要幻化的武器槽
          （不要动鼠标），切回本窗口回车确认，随后自动读取并改写外观。
        </div>
        <div class="weapon-select-row">
          <span class="weapon-select-label">目标武器：</span>
          <a-select
            v-model:value="targetWeapon"
            show-search
            :filter-option="filterWeapon"
            :options="weaponPickOptions"
            placeholder="搜索/选择目标武器"
            style="width: 320px"
          />
          <span class="prompt-pick-id" v-if="targetWeapon">ID {{ hex(parseInt(targetWeapon, 16)) }}</span>
        </div>
      </template>
    </div>

    <!-- 运行中：进度 + 交互提示 + 日志 -->
    <div v-else-if="running">
      <div class="running-panel">
        <div class="global-spin"><a-spin size="small" :spinning="true" /></div>
        <a-progress
          :percent="progressPercent"
          size="small"
          :stroke-color="{ '0%': '#1677ff', '100%': '#52c41a' }"
          style="margin-bottom:10px"
        />

        <!-- 步骤3: 刷新缓存 状态条 -->
        <a-alert
          v-if="flowStage === 2"
          class="refresh-alert"
          type="info"
          show-icon
          message="正在刷新游戏内 Mod 缓存…"
          description="已发 F10 重载 Mod 文件，稍候即可看到日志确认。"
        />

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
            <span class="prompt-hint">操作完成后【不要动鼠标】，用键盘【Alt+Tab】切回本窗口，然后【按回车】确认（或点击下方按钮）</span>
          </template>
        </a-alert>
      </div>
    </div>

    <!-- 完成通知 -->
    <a-result
      v-else
      :status="doneOk ? 'success' : 'error'"
      :title="doneOk ? '武器幻化完成' : '武器幻化失败'"
      :sub-title="doneMsg"
    >
      <template #extra>
        <a-button type="primary" @click="close">立即关闭</a-button>
        <span v-if="countdown > 0" class="auto-close-tip">{{ countdown }} 秒后自动关闭…</span>
      </template>
    </a-result>

    <template #footer>
      <template v-if="!running && !finished">
        <a-button @click="onCancel">取消</a-button>
        <a-button type="primary" :disabled="weaponPickOptions.length === 0 || !targetWeapon" @click="start">开始幻化</a-button>
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
import { GetWeaponParts } from '../../wailsjs/go/main/App'
import { RefashionWeapon, Confirm, Cancel } from '../../wailsjs/go/transformation/Ref'
import { EventsOn } from '../../wailsjs/runtime/runtime'

const props = defineProps({
  open: { type: Boolean, default: false },
  mod: { type: Object, default: null },
})
const emit = defineEmits(['update:open', 'done'])

const weapons = ref([])   // [{id,name}]
const zIndex = ref(1000)
const running = ref(false)
const finished = ref(false)
const doneOk = ref(false)
const doneMsg = ref('')
const logs = ref([])
const prompt = ref(null)
const keyConfirmPending = ref(false)
const progress = ref({ step: 0, total: 0 })
const flowStage = ref(0) // 0=校准 1=捕获并写入 2=刷新缓存
const targetWeapon = ref('')
const countdown = ref(0)
let countdownTimer = null

const currentStep = computed(() => {
  if (finished.value) return 3
  return flowStage.value
})

// 武器词典 → 下拉选项；优先展示该 Mod 占用的武器
const weaponPickOptions = computed(() => {
  const out = []
  const seen = new Set()
  for (const n of (props.mod?.parts?.['武器'] || [])) {
    const w = weapons.value.find(x => x.name === n)
    if (w && !seen.has(w.id)) { out.push(w); seen.add(w.id) }
  }
  for (const w of weapons.value) {
    if (!seen.has(w.id)) out.push(w)
  }
  return out.map(w => ({ value: String(w.id), label: w.name, tipId: String(w.id) }))
})

// 可搜索：按名称或 ID 过滤
function filterWeapon(input, option) {
  const kw = input.trim().toLowerCase()
  if (!kw) return true
  const name = String(option.label || '').toLowerCase()
  return name.includes(kw) || String(option.value).toLowerCase().includes(kw.replace(/^0x/i, ''))
}

const btnLabel = computed(() => (prompt.value ? '回车确认' : '继续'))

const progressPercent = computed(() => {
  const t = progress.value.total || 1
  if (!t) return 0
  return Math.min(100, Math.round((progress.value.step / t) * 100))
})

function hex(v) {
  return '0x' + (v != null ? Number(v).toString(16).toUpperCase().padStart(4, '0') : '----')
}

watch(() => props.open, async (open) => {
  if (!open) return
  reset()
  try {
    weapons.value = (await GetWeaponParts()) || []
  } catch (e) {
    message.error(`加载武器数据失败: ${e}`)
  }
  targetWeapon.value = weaponPickOptions.value[0]?.value || ''
  subscribe()
})

function reset() {
  clearCountdown()
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

// 完成后自动关闭倒计时
function startCountdown(sec) {
  clearCountdown()
  countdown.value = sec
  countdownTimer = setInterval(() => {
    countdown.value -= 1
    if (countdown.value <= 0) {
      clearCountdown()
      close()
    }
  }, 1000)
}
function clearCountdown() {
  if (countdownTimer) { clearInterval(countdownTimer); countdownTimer = null }
  countdown.value = 0
}

// 校准交互需要玩家「用游戏内方向键吸附槽位中心后，不动鼠标，直接按回车确认」
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

// ---- 事件订阅 ----
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
    startCountdown(5)
  }))
}
function unsubscribe() {
  for (const off of offs) off()
  offs.length = 0
}

async function start() {
  if (!targetWeapon.value) {
    message.warning('请先选择目标武器')
    return
  }
  logs.value = []
  prompt.value = null
  running.value = true
  try {
    // 目标武器随 ids 预先传入，后端捕获后直接改写
    await RefashionWeapon(0, [parseInt(targetWeapon.value, 16)])
  } catch (e) {
    running.value = false
    finished.value = true
    doneOk.value = false
    doneMsg.value = String(e)
  }
}

async function confirm() {
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
  clearCountdown()
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
.global-spin { position:absolute; top:16px; right:16px; z-index:2; }
.running-panel { min-height: 200px; position: relative; }
.current-banner { position:relative; display:flex; align-items:center; gap:14px; padding:18px 20px; margin-bottom:12px; border-radius:12px; background:linear-gradient(135deg,#fff7e6,#ffecd0); border:2px solid #ffc069; overflow:hidden; }
.current-pulse { position:absolute; left:0; top:0; width:100%; height:100%; background:radial-gradient(circle, rgba(250,140,22,0.18) 0%, transparent 70%); animation:currentPulse 1.6s ease-in-out infinite; pointer-events:none; }
@keyframes currentPulse { 0%,100%{opacity:0.35;} 50%{opacity:0.95;} }
.current-body { position:relative; z-index:1; flex:1; min-width:0; }
.current-label { font-size:13px; color:#d46b08; font-weight:700; margin-bottom:6px; letter-spacing:1px; }
.current-main { display:flex; align-items:center; gap:12px; }
.current-slot { font-weight:700; font-size:14px; }
.current-name { font-size:24px; font-weight:800; color:#d4380d; flex:1; min-width:0; overflow:hidden; text-overflow:ellipsis; white-space:nowrap; }
.current-id { font-size:20px; font-family:Consolas,monospace; color:#fa8c16; font-weight:800; flex-shrink:0; }
.log-box { background:#0f172a; color:#d1d5db; font-family:Consolas,Menlo,monospace; font-size:12px; border-radius:8px; padding:10px 12px; max-height: 30vh; overflow:auto; margin-bottom:12px; }
.log-line { line-height:1.7; white-space:pre-wrap; word-break:break-all; }
.log-line.dim { color:#6b7280; }
.prompt-alert { margin-top: 4px; }
.refresh-alert { margin-bottom: 10px; }
.prompt-item { font-family:Consolas,monospace; font-size:12px; margin-bottom:4px; }
.prompt-weapon-pick { display:flex; align-items:center; margin-bottom:6px; }
.weapon-select-row { display:flex; align-items:center; gap:8px; margin-top:10px; }
.weapon-select-row.mid { margin-top:14px; padding:12px 14px; background:#fffbe6; border:1px dashed #ffc069; border-radius:8px; }
.weapon-select-label { font-size:13px; color:#333; flex-shrink:0; }
.prompt-pick-label { font-size:13px; color:#333; flex-shrink:0; }
.prompt-pick-id { font-family:Consolas,monospace; font-size:12px; color:#1a73e8; }
.prompt-hint { color:#d48806; font-size:12px; }
.auto-close-tip { margin-left: 12px; color:#888; font-size:13px; }
</style>
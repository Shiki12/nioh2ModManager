<template>
  <div class="page author-tool">
    <a-alert message="作者工具：选择一个 mod 文件夹，识别封面图、填写占用资源后生成 mod.json 卡片文件，随 mod 一起分发。用户导入后无需再填写。" type="info" show-icon style="margin-bottom:14px" />

    <div class="tool-header">
      <a-space :size="10">
        <a-button type="primary" class="btn-blue" @click="selectFolder"><template #icon><FolderOpenOutlined /></template>选择 Mod 文件夹</a-button>
        <a-button class="btn-magenta" @click="batchGenerate"><template #icon><ExperimentOutlined /></template>批量识别 HDR 合集</a-button>
      </a-space>
      <div class="tool-header-tip">批量识别：选择一个包含多个 HDR 合集（NN_HDR ... Collection）的目录，自动解析各子 Mod 的 Read Me 并生成 mod.json。</div>
    </div>

    <a-spin v-if="folder && scanning" class="scan-panel" tip="正在扫描文件夹…">
      <LoadingOutlined spin class="scan-icon" />
    </a-spin>

    <div v-if="folder && !scanning" class="tool-body">
      <div class="tool-left">
        <div class="mod-folder-name">{{ folderName }}</div>
        <div class="mod-folder-path">{{ folder }}</div>

        <div class="section-title">图片 <span class="section-sub">点击缩略图设为封面，勾选"效果图"作为多张预览</span>
          <a-button size="small" class="btn-gold" style="margin-left:8px" @click="importImage"><template #icon><PictureOutlined /></template>导入图片到文件夹</a-button>
        </div>
        <div v-if="images.length" class="image-grid">
          <div v-for="img in images" :key="img" class="image-item" :class="{ selected: img === selectedCover }"
            :title="img" @click="selectedCover = img">
            <img :src="imageUrl(img)" loading="lazy" decoding="async" @error="e => e.target.style.visibility = 'hidden'" />
            <span v-if="img === selectedCover" class="img-badge img-cover">封面</span>
            <span class="img-badge img-preview" :class="{ active: previews.includes(img) }" @click.stop="togglePreview(img)">
              <CheckSquareOutlined v-if="previews.includes(img)" /> <BorderOutlined v-else /> 效果图
            </span>
            <div class="image-name">{{ img }}</div>
          </div>
        </div>
        <div v-else class="empty-tip">文件夹根目录未找到图片，未选择封面时卡片将显示占位图</div>

        <div class="section-title">昵称</div>
        <a-input v-model:value="nickname" placeholder="卡片显示名称（留空用文件夹名）" />

        <div class="section-title">占用资源 <span class="section-sub">可同时选服装部位和多个武器</span></div>
        <div class="parts-grid">
          <div v-for="slot in slots" :key="slot.name" class="parts-row">
            <a-tag class="parts-tag">{{ slot.name }}</a-tag>
            <a-select
              v-if="slot.name === '武器'"
              v-model:value="parts[slot.name]"
              :options="slotOptions(slot)"
              placeholder="选择占用的武器（可添加多个）"
              mode="multiple"
              allow-clear
              show-search
              option-filter-prop="label"
              class="parts-select"
            />
            <a-select
              v-else
              v-model:value="parts[slot.name]"
              :options="slotOptions(slot)"
              placeholder="未占用"
              allow-clear
              show-search
              option-filter-prop="label"
              class="parts-select"
            />
          </div>
        </div>
      </div>

      <div class="tool-right">
        <div class="section-title">卡片预览</div>
        <ModCard :mod="previewMod" />
        <div class="preview-tip">提示：卡片仅作预览，写入文件后以实际卡片为准</div>

        <div class="section-title">生成的 mod.json
          <a-button size="small" class="btn-cyan" style="margin-left:8px" @click="openJsonModal"><template #icon><EditOutlined /></template>查看/编辑</a-button>
          <span class="section-sub">也可点击下方内容</span>
        </div>
        <pre class="json-preview" title="点击编辑 mod.json" @click="openJsonModal">{{ previewJson }}</pre>
      </div>
    </div>

    <div class="tool-footer">
      <a-button v-if="folder" type="primary" size="large" class="btn-orange" :loading="saving" @click="saveCard">
        <template #icon><ExportOutlined /></template>导出 mod.json
      </a-button>
    </div>

    <!-- 弹窗：编辑 mod.json -->
    <a-modal :open="jsonModalVisible" title="编辑 mod.json" :footer="null" :width="560" @cancel="jsonModalVisible = false">
      <a-textarea v-model:value="editableJson" :rows="14" class="json-editor" spellcheck="false" />
      <a-space style="margin-top:12px;display:flex;justify-content:center">
        <a-button type="primary" class="btn-blue" @click="applyJsonOnly">应用修改</a-button>
        <a-button type="primary" class="btn-green" @click="applyJsonAndSave">应用并写入文件</a-button>
        <a-button class="btn-cancel" @click="jsonModalVisible = false">取消</a-button>
      </a-space>
    </a-modal>

    <!-- 弹窗：批量识别结果 -->
    <a-modal v-model:open="batchVisible" title="批量识别 HDR 合集结果" :footer="null" :width="760" :maskClosable="false">
      <a-spin :spinning="batchRunning">
        <template v-if="batchResult">
          <a-alert :message="`识别到 ${batchResult.total} 个 HDR 合集，成功生成 ${batchResult.generated} 个 mod.json`" type="success" show-icon style="margin-bottom:12px" />
          <div v-if="batchResult.errors.length" style="margin-bottom:12px">
            <div class="section-title">处理失败</div>
            <a-alert v-for="(e, i) in batchResult.errors" :key="i" :message="e" type="error" show-icon style="margin-bottom:6px" />
          </div>
          <div v-if="batchResult.pending.length" style="margin-bottom:12px">
            <div class="section-title">待人工确认的占用（未能自动匹配）</div>
            <a-table size="small" :data-source="batchResult.pending" :columns="pendingColumns" :pagination="false" :row-key="(r, i) => i" bordered />
          </div>
          <div v-if="batchResult.mods.length">
            <div class="section-title">已生成的 mod.json（含子 Mod 占用）</div>
            <a-table size="small" :data-source="batchResult.mods" :columns="modsColumns" :pagination="false" row-key="nickname" :expandable="modsExpand" bordered />
          </div>
        </template>
      </a-spin>
      <a-space style="margin-top:12px;display:flex;justify-content:center">
        <a-button type="primary" class="btn-blue" @click="batchVisible = false">完成</a-button>
      </a-space>
    </a-modal>
  </div>
</template>

<script setup>
import { ref, computed, inject, h } from 'vue'
import { message } from 'ant-design-vue'
import { FolderOpenOutlined, ExportOutlined, LoadingOutlined, PictureOutlined, EditOutlined, ExperimentOutlined, CheckSquareOutlined, BorderOutlined } from '@ant-design/icons-vue'
import { SelectDirectory, SelectImageFile, ListFolderImages, ReadModConfig, WriteModCard, AddCoverImage, GetArmorParts, GetWeaponParts, BatchGenerateModCards } from '../../wailsjs/go/main/App'
import ModCard from './ModCard.vue'

const addLog = inject('addLog', () => {})

const folder = ref('')
const folderName = ref('')
const scanning = ref(false)
const saving = ref(false)
const jsonModalVisible = ref(false)
const editableJson = ref('')
const batchVisible = ref(false)
const batchRunning = ref(false)
const batchResult = ref(null)
const images = ref([])
const selectedCover = ref('')
const previews = ref([])
const nickname = ref('')
const parts = ref({})

const armorSlots = ref([])
const weaponSlots = ref([])
async function loadResourceData() {
  try {
    const [slots, weapons] = await Promise.all([GetArmorParts(), GetWeaponParts()])
    armorSlots.value = slots || []
    weaponSlots.value = [{ name: '武器', parts: weapons || [] }]
  } catch (e) { addLog(`加载资源数据失败: ${e}`) }
}
loadResourceData()

// 全部可选槽位：服装部位 + 武器（可同时占用）
const slots = computed(() => [...armorSlots.value, ...weaponSlots.value])

/** 根据占用的资源推导分类：mixed=服装+武器 / weapon=仅武器 / armor=仅服装 */
function deriveCategory(parts) {
  let armor = false, weapon = false
  for (const k of Object.keys(parts || {})) { if (k === '武器') weapon = true; else armor = true }
  if (weapon && armor) return 'mixed'
  if (weapon) return 'weapon'
  return 'armor'
}

function folderPath(file) {
  if (!file) return ''
  return folder.value.replace(/[\\/]+$/, '') + '\\' + file
}
function imageUrl(file) {
  return '/localfile?file=' + encodeURIComponent(folderPath(file)) + '&w=160'
}
function slotOptions(slot) {
  const seen = new Set()
  const opts = []
  for (const p of slot.parts || []) {
    if (!p.name || seen.has(p.name)) continue
    seen.add(p.name)
    opts.push({ value: p.name, label: p.name })
  }
  return opts
}

/** 归一化占用资源为表单结构：服装部位取单值，武器部位为数组 */
function normalizeParts(parts) {
  const out = {}
  for (const [slot, vals] of Object.entries(parts || {})) {
    const list = Array.isArray(vals) ? vals : (vals ? [vals] : [])
    const clean = list.map(v => (typeof v === 'string' ? v.trim() : '')).filter(Boolean)
    if (!clean.length) continue
    if (slot === '武器') out[slot] = clean
    else out[slot] = clean[0]
  }
  return out
}

/** 输出为后端数组格式：服装部位 [单值]，武器部位 [多个值] */
function toArrayParts(parts) {
  const out = {}
  for (const [slot, v] of Object.entries(parts || {})) {
    const list = (Array.isArray(v) ? v : [v]).map(x => (typeof x === 'string' ? x.trim() : '')).filter(Boolean)
    if (list.length) out[slot] = list
  }
  return out
}

const previewMod = computed(() => ({
  name: folderName.value || 'Mod 名称',
  nickname: nickname.value,
  cover: selectedCover.value ? folderPath(selectedCover.value) : '',
  previews: previews.value.map(f => folderPath(f)),
  enabled: true,
  category: deriveCategory(parts.value),
  parts: { ...parts.value },
}))

/** 切换某张图片是否为效果图（多选） */
function togglePreview(img) {
  const i = previews.value.indexOf(img)
  if (i >= 0) previews.value.splice(i, 1)
  else previews.value.push(img)
}

// 将要写入的 mod.json 内容（键顺序与后端一致：服装部位在前、武器最后）
const previewJson = computed(() => {
  const cleaned = toArrayParts(parts.value)
  const ordered = {}
  for (const slot of slots.value.map(s => s.name)) {
    if (cleaned[slot]) ordered[slot] = cleaned[slot]
  }
  const out = {
    nickname: nickname.value,
    category: deriveCategory(ordered),
    cover: selectedCover.value,
  }
  if (previews.value.length) out.previews = [...previews.value]
  out.parts = ordered
  return JSON.stringify(out, null, 2)
})

async function selectFolder() {
  let dir
  try { dir = await SelectDirectory() } catch (e) { addLog(`选择文件夹失败: ${e}`); return }
  if (!dir) return
  folder.value = dir
  folderName.value = dir.replace(/[\\/]+$/, '').split(/[\\/]/).pop() || dir
  scanning.value = true
  images.value = []
  selectedCover.value = ''
  previews.value = []
  nickname.value = ''
  parts.value = {}
  try {
    const imgs = await ListFolderImages(dir)
    images.value = imgs || []
    const preferred = (imgs || []).find(i => /^(cover|preview|1\.)/i.test(i))
    selectedCover.value = preferred || (imgs || [])[0] || ''
    const cfg = await ReadModConfig(dir)
    if (cfg && cfg.configFound) {
      if (cfg.nickname) nickname.value = cfg.nickname
      if (cfg.cover && !images.value.includes(cfg.cover)) images.value.unshift(cfg.cover)
      if (cfg.cover) selectedCover.value = cfg.cover
      previews.value = (cfg.previews || []).filter(p => images.value.includes(p))
      parts.value = normalizeParts(cfg.parts)
      addLog(`作者工具：已载入已有 mod.json（${folderName.value}）`)
    }
  } catch (e) {
    addLog(`扫描文件夹失败: ${e}`)
  } finally {
    scanning.value = false
  }
}

/** 点击 JSON 预览：打开编辑弹窗（预填当前内容） */
function openJsonModal() {
  editableJson.value = previewJson.value
  jsonModalVisible.value = true
}

/** 应用编辑后的 JSON：解析并回填表单状态。成功返回 true，失败返回 false */
function applyJson() {
  let obj
  try { obj = JSON.parse(editableJson.value) } catch (e) { message.error('JSON 格式错误：' + e.message); return false }
  if (typeof obj !== 'object' || obj === null || Array.isArray(obj)) { message.error('JSON 应为对象'); return false }
  if (typeof obj.nickname === 'string') nickname.value = obj.nickname
  if (typeof obj.cover === 'string') selectedCover.value = obj.cover
  if (Array.isArray(obj.previews)) previews.value = obj.previews.filter(p => typeof p === 'string')
  if (obj.parts && typeof obj.parts === 'object' && !Array.isArray(obj.parts)) {
    parts.value = normalizeParts(obj.parts)
  }
  addLog('作者工具：已应用 JSON 编辑')
  message.success('已应用 JSON 修改')
  return true
}

/** 弹窗"应用修改"：只回填表单，不写入文件 */
function applyJsonOnly() {
  if (applyJson()) jsonModalVisible.value = false
}

/** 弹窗"应用并写入文件"：回填表单后直接生成 mod.json */
async function applyJsonAndSave() {
  if (!applyJson()) return
  jsonModalVisible.value = false
  await saveCard()
}

/** 批量识别：选择包含多个 HDR 合集的目录（或单个合集目录），自动生成各合集的 mod.json */
async function batchGenerate() {
  let dir
  try { dir = await SelectDirectory() } catch (e) { addLog(`选择目录失败: ${e}`); return }
  if (!dir) return
  batchVisible.value = true
  batchRunning.value = true
  batchResult.value = null
  try {
    const res = await BatchGenerateModCards(dir)
    batchResult.value = res
    addLog(`批量识别完成：识别 ${res.total} 个合集，生成 ${res.generated} 个 mod.json`)
    if (res.total === 0) {
      message.warning('所选目录下未找到 HDR 合集。需选择含多个合集（NN_HDR ... Collection）的上级目录，或直接选单个合集目录本身（需同时含 meshes/ 与 textures/ 子目录）')
    } else if (res.errors && res.errors.length) {
      message.warning(`识别完成但 ${res.errors.length} 个合集失败，详见日志`)
    }
  } catch (e) {
    addLog(`批量识别失败: ${e}`)
    message.error('批量识别失败，详见日志')
  } finally {
    batchRunning.value = false
  }
}

const pendingColumns = [
  { title: '合集', dataIndex: 'mod', ellipsis: true },
  { title: '子 Mod', dataIndex: 'subMod', ellipsis: true },
  { title: '槽位', dataIndex: 'slot', width: 70 },
  { title: '中文套装名', dataIndex: 'chinese', ellipsis: true },
  { title: '英文装备名', dataIndex: 'english', ellipsis: true },
]

function submodsRows(mod) {
  return (mod.submods || []).map(sm => ({ ...sm, key: sm.name }))
}
const modsColumns = [
  { title: '合集', dataIndex: 'nickname', ellipsis: true },
  { title: '子 Mod 数', dataIndex: 'submods', width: 90, customRender: ({ record }) => (record.submods || []).length },
]
const modsExpand = {
  expandedRowRender: (record) => h('div', { class: 'batch-submods' },
    (record.submods || []).map(sm => h('div', { key: sm.name, class: 'batch-submod' }, [
      h('div', { class: 'batch-submod-name' }, sm.name),
      h('div', { class: 'batch-submod-parts' },
        Object.entries(sm.parts || {}).map(([slot, vals]) => {
          const list = Array.isArray(vals) ? vals : [vals]
          return h('a-tag', { key: slot }, `${slot}：${list.filter(Boolean).join('、')}`)
        })),
    ]))),
}

async function importImage() {
  if (!folder.value) { message.warning('请先选择 Mod 文件夹'); return }
  let file
  try { file = await SelectImageFile() } catch (e) { addLog(`选择图片失败: ${e}`); return }
  if (!file) return
  try {
    const name = await AddCoverImage(folder.value, file)
    if (!images.value.includes(name)) images.value.unshift(name)
    selectedCover.value = name
    addLog(`作者工具：已导入封面图片 ${name}`)
    message.success('图片已复制到 mod 文件夹并设为封面')
  } catch (e) {
    addLog(`导入封面图片失败: ${e}`)
    message.error('导入失败，详见日志')
  }
}

async function saveCard() {
  if (!folder.value) return
  saving.value = true
  try {
    const cleaned = toArrayParts(parts.value)
    const res = await WriteModCard(folder.value, {
      nickname: nickname.value,
      cover: selectedCover.value,
      previews: [...previews.value],
      parts: cleaned,
    })
    addLog(`作者工具：已生成 mod.json → ${folder.value}`)
    if (res && res.nickname) nickname.value = res.nickname
    message.success('mod.json 已生成')
  } catch (e) {
    addLog(`生成 mod.json 失败: ${e}`)
    message.error('生成失败，详见日志')
  } finally {
    saving.value = false
  }
}
</script>

<style scoped>
.tool-header { margin-bottom: 14px; }
.scan-panel { display: flex; justify-content: center; padding: 48px 0; }
.scan-icon { font-size: 22px; color: #1a73e8; }
.tool-body { display: flex; gap: 24px; align-items: flex-start; }
.tool-left { flex: 1.4; min-width: 0; }
.tool-right { flex: 1; min-width: 260px; }
.mod-folder-name { font-size: 15px; font-weight: 600; color: #333; word-break: break-all; }
.mod-folder-path { font-size: 12px; color: #999; word-break: break-all; margin: 2px 0 14px; }
.section-title { font-size: 13px; font-weight: 600; color: #333; margin: 14px 0 8px; }
.section-sub { font-size: 12px; font-weight: 400; color: #999; }
.image-grid { display: grid; grid-template-columns: repeat(4, 1fr); gap: 8px; }
.image-item { border: 2px solid #eee; border-radius: 4px; overflow: hidden; cursor: pointer; background: #fafafa; position: relative; }
.image-item.selected { border-color: #1a73e8; }
.image-item img { width: 100%; height: 70px; object-fit: cover; display: block; }
.img-badge { position: absolute; font-size: 10px; line-height: 1; padding: 2px 4px; border-radius: 3px; }
.img-cover { top: 4px; left: 4px; background: #1a73e8; color: #fff; }
.img-preview { bottom: 22px; right: 4px; background: rgba(0,0,0,.55); color: #fff; cursor: pointer; display: inline-flex; align-items: center; gap: 2px; user-select: none; }
.img-preview:hover { background: rgba(26,115,232,.85); }
.img-preview.active { background: #52c41a; }
.image-name { font-size: 10px; color: #666; padding: 2px 4px; text-align: center; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.empty-tip { font-size: 12px; color: #999; padding: 8px 0; }
.parts-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 12px; }
.parts-row { display: flex; align-items: center; gap: 10px; min-width: 0; }
.parts-tag { width: 40px; text-align: center; margin-inline-end: 0; flex-shrink: 0; }
.parts-select { flex: 1; }
.preview-tip { font-size: 12px; color: #999; margin-top: 8px; }
.json-preview { margin: 0; padding: 10px 12px; background: #f6f8fa; border: 1px solid #e8e8e8; border-radius: 4px; font-size: 12px; line-height: 1.5; color: #333; max-height: 320px; overflow: auto; white-space: pre-wrap; word-break: break-all; cursor: pointer; }
.json-preview:hover { border-color: #1a73e8; }
.json-editor { font-family: Consolas, Monaco, 'Courier New', monospace; font-size: 12px; line-height: 1.6; }
.tool-footer { margin-top: 20px; padding-top: 16px; border-top: 1px solid #eee; display: flex; justify-content: center; }
.tool-header-tip { font-size: 12px; color: #999; margin-top: 6px; }
.batch-submods { display: flex; flex-direction: column; gap: 8px; }
.batch-submod-name { font-size: 12px; font-weight: 600; color: #333; }
.batch-submod-parts { display: flex; flex-wrap: wrap; gap: 4px; margin-top: 4px; }
</style>

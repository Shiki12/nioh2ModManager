<template>
  <div class="page">
    <div class="page-toolbar">
      <span class="toolbar-title">Mod 管理</span>
      <a-space :size="8">
        <a-button class="btn-cyan" @click="refreshMods"><template #icon><ReloadOutlined /></template>刷新列表</a-button>
        <a-button type="primary" class="btn-blue" @click="importMod"><template #icon><DownloadOutlined /></template>自动安装</a-button>
        <a-button type="primary" class="btn-orange" :loading="installingHdr" @click="importHdr"><template #icon><BulbOutlined /></template>安装 HDR Mod</a-button>
      </a-space>
    </div>

    <a-tabs>
      <a-tab-pane key="installed">
        <template #tab>
          <span>已安装 Mod <span class="count-badge">{{ installedMods.length }}</span></span>
        </template>
        <a-empty v-if="installedMods.length === 0" description="还没有已安装的 Mod" style="margin-top:80px" />
        <a-alert v-else-if="conflictCount > 0" type="warning" show-icon class="conflict-banner">
          <template #message>检测到 {{ conflictCount }} 个已启用 Mod 之间存在资源占用冲突（同时启用可能互相覆盖）</template>
          <template #description>
            <div class="conflict-list">
              <div v-for="info in conflicts" :key="info.modName" class="conflict-item">
                <span class="conflict-mod">{{ info.nickname || info.modName }}</span>
                <span class="conflict-vs">与</span>
                <span v-for="(c, idx) in info.conflicts" :key="idx" class="conflict-pair">
                  <template v-if="idx > 0">、</template>
                  <b>{{ c.nickname || c.modName }}</b>
                  <span class="conflict-res">（{{ c.slot }} → {{ c.value }}）</span>
                </span>
              </div>
            </div>
          </template>
        </a-alert>
        <div v-else class="filter-bar">
          <a-input v-model:value="instSearch" allow-clear placeholder="搜索 Mod 名称" style="max-width:220px">
            <template #prefix><SearchOutlined /></template>
          </a-input>
          <a-select v-model:value="instStatus" style="width:110px" :options="statusOptions" />
          <a-select v-model:value="instArmor" style="width:180px" :options="armorOptions" placeholder="按占用资源筛选" allow-clear show-search option-filter-prop="label" />
        </div>
        <a-empty v-if="filteredInstalled.length === 0" description="没有符合筛选条件的 Mod" style="margin-top:60px" />
        <div v-else class="mod-grid">
          <ModCard v-for="mod in filteredInstalled" :key="mod.name" :mod="mod" :conflict-info="conflictsMap[mod.name]"
            @toggle="toggleMod" @toggleRefresh="toggleModRefresh" @edit="openEditMod" />
        </div>
      </a-tab-pane>

      <a-tab-pane key="locks">
        <template #tab>
          <span>冲突列表 <span class="count-badge">{{ conflictCount }}</span></span>
        </template>
        <a-empty v-if="conflictGroups.length === 0" description="当前已启用 Mod 之间没有冲突" style="margin-top:80px" />
        <div v-else class="conflict-group-list">
          <a-card v-for="(g, gi) in conflictGroups" :key="gi" size="small" class="conflict-group-card" :bordered="false">
            <template #title>
              <span class="conflict-group-title">冲突组 {{ gi + 1 }}（{{ g.mods.length }} 个 Mod）</span>
            </template>
            <div class="conflict-group-mods">
              <span class="conflict-group-label">涉及 Mod：</span>
              <span v-for="(m, mi) in g.mods" :key="m.name" class="conflict-group-mod">
                <a-switch size="small" :checked="m.enabled" style="margin-right:6px" @change="() => onLockToggle(m)" />
                <b>{{ m.nickname || m.name }}</b><template v-if="mi < g.mods.length - 1">、</template>
              </span>
            </div>
            <div class="conflict-group-edges">
              <div v-for="(e, ei) in g.edges" :key="ei" class="conflict-edge">
                「{{ nickOf(e.a) }}」与「{{ nickOf(e.b) }}」冲突：{{ e.slot }} → {{ e.value }}
              </div>
            </div>
          </a-card>
        </div>
      </a-tab-pane>

      <a-tab-pane key="library">
        <template #tab>
          <span>Mod 库 <span class="count-badge">{{ libraryMods.length }}</span></span>
        </template>
        <a-input v-model:value="searchQuery" allow-clear placeholder="搜索 Mod 名称" class="lib-search">
          <template #prefix><SearchOutlined /></template>
        </a-input>
        <a-empty v-if="libraryMods.length === 0" description="托管目录中没有 Mod 文件，请先导入或更换托管目录" style="margin-top:80px" />
        <a-table v-else class="lib-table" :data-source="libraryMods" :columns="columns" :pagination="tablePagination" size="middle" row-key="name">
          <template #bodyCell="{ column, record }">
            <template v-if="column.key === 'name'">
              <span class="mod-name-cell" :class="{ 'name-missing': record.missing }">
                <FolderFilled class="folder-icon" :class="{ 'folder-missing': record.missing }" />
                <span class="mod-name-text" :title="record.name">{{ record.name }}</span>
              </span>
            </template>
            <template v-else-if="column.key === 'status'">
              <a-tag v-if="record.missing" color="error">文件缺失</a-tag>
              <a-tag v-else-if="record.installed && record.enabled" color="success">已启用</a-tag>
              <a-tag v-else-if="record.installed" color="blue">已安装</a-tag>
              <a-tag v-else>未安装</a-tag>
            </template>
            <template v-else-if="column.key === 'action'">
              <a-popconfirm v-if="record.missing" title="确定清除该 Mod 的记录吗？" ok-text="清除" cancel-text="取消" @confirm="removeRecord(record)">
                <a-button type="link" size="small" class="btn-link-red">清除记录</a-button>
              </a-popconfirm>
              <template v-else-if="record.installed">
                <a-popconfirm title="卸载后该 Mod 回到“未安装”，文件夹保留在 Mod 库，确定卸载吗？" ok-text="卸载" cancel-text="取消" @confirm="uninstallMod(record)">
                  <a-button type="link" size="small" class="btn-link-red">卸载</a-button>
                </a-popconfirm>
                <a-divider type="vertical" />
                <a-button type="link" size="small" class="btn-link-blue" @click.stop="openEditMod(record)">编辑</a-button>
              </template>
              <template v-else>
                <a-button type="link" size="small" class="btn-link-green" @click.stop="installMod(record)">安装</a-button>
                <a-divider type="vertical" />
                <a-button type="link" size="small" class="btn-link-blue" @click.stop="openEditMod(record)">编辑</a-button>
              </template>
            </template>
          </template>
        </a-table>
      </a-tab-pane>
    </a-tabs>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { message } from 'ant-design-vue'
import { ReloadOutlined, FolderFilled, SearchOutlined, DownloadOutlined, BulbOutlined } from '@ant-design/icons-vue'
import { SelectDirectory, InstallHdrMod } from '../../wailsjs/go/main/App'
import ModCard from './ModCard.vue'

const props = defineProps(['mods', 'conflicts', 'toggleMod', 'toggleModRefresh', 'toggleSubMod', 'toggleSubModRefresh', 'openEditMod', 'beginImportProgress', 'endImportProgress', 'refreshMods', 'installMod', 'importMod', 'uninstallMod', 'removeRecord'])
const installedMods = computed(() => props.mods.filter(m => m.installed && !m.missing))
const installingHdr = ref(false)

/** 冲突信息按 Mod 名称索引，供卡片展示当前 Mod 是否有冲突 */
const conflictsMap = computed(() => {
  const map = {}
  for (const c of props.conflicts || []) map[c.modName] = c
  return map
})
const conflictCount = computed(() => (props.conflicts || []).length)

/** Mod 名称 → Mod 对象索引（含启用状态与昵称） */
const nameToMod = computed(() => {
  const map = {}
  for (const m of props.mods || []) map[m.name] = m
  return map
})
function nickOf(name) {
  const m = nameToMod.value[name]
  return m?.nickname || name
}
/**
 * 冲突列表：把全部冲突按"连通分量"分组，只有真正互相冲突的 Mod 才归为一组。
 * 例如 A-B、C-D 各自独立冲突时，会得到两个冲突组，而不会混成一个。
 */
const conflictGroups = computed(() => {
  const parent = {}
  const find = (x) => { while (parent[x] !== x) { parent[x] = parent[parent[x]]; x = parent[x] } return x }
  const union = (a, b) => { parent[find(a)] = find(b) }
  const ensure = (name) => { if (!(name in parent)) parent[name] = name }
  const rawEdges = []
  for (const info of props.conflicts || []) {
    ensure(info.modName)
    for (const c of info.conflicts || []) {
      ensure(c.modName)
      union(info.modName, c.modName)
      rawEdges.push({ a: info.modName, b: c.modName, slot: c.slot, value: c.value })
    }
  }
  const seen = new Set()
  const edges = []
  for (const e of rawEdges) {
    const key = [e.a, e.b].sort().join('\u0000') + '|' + e.slot + '|' + e.value
    if (seen.has(key)) continue
    seen.add(key)
    edges.push(e)
  }
  const groups = {}
  const order = []
  for (const name in parent) {
    const r = find(name)
    if (!groups[r]) { groups[r] = { mods: [], edges: [] }; order.push(r) }
    groups[r].mods.push(name)
  }
  for (const e of edges) groups[find(e.a)].edges.push(e)
  return order.map(r => ({
    mods: groups[r].mods.map(name => nameToMod.value[name]).filter(Boolean),
    edges: groups[r].edges,
  }))
})
async function onLockToggle(record) {
  await props.toggleMod(record)
}

const instSearch = ref('')
const instStatus = ref('all')
const instArmor = ref(undefined)
const statusOptions = [
  { label: '全部状态', value: 'all' },
  { label: '已启用', value: 'enabled' },
  { label: '已禁用', value: 'disabled' },
]
const armorOptions = computed(() => {
  const set = new Set()
  for (const m of installedMods.value) {
    for (const vals of Object.values(m.parts || {})) {
      const list = Array.isArray(vals) ? vals : [vals]
      for (const v of list) if (v) set.add(v)
    }
  }
  return [...set].map(v => ({ label: v, value: v }))
})
const filteredInstalled = computed(() => {
  const q = instSearch.value.trim().toLowerCase()
  return installedMods.value.filter(m => {
    if (q && !(m.name || '').toLowerCase().includes(q) && !(m.nickname || '').toLowerCase().includes(q)) return false
    if (instStatus.value === 'enabled' && !m.enabled) return false
    if (instStatus.value === 'disabled' && m.enabled) return false
    if (instArmor.value && !Object.values(m.parts || {}).some(vals => (Array.isArray(vals) ? vals : [vals]).includes(instArmor.value))) return false
    return true
  })
})

const searchQuery = ref('')
const libraryMods = computed(() => {
  const q = searchQuery.value.trim().toLowerCase()
  if (!q) return props.mods
  return props.mods.filter(m => (m.name || '').toLowerCase().includes(q) || (m.nickname || '').toLowerCase().includes(q))
})

const columns = [
  { title: 'Mod 名称', key: 'name', ellipsis: true },
  { title: '状态', key: 'status', width: 90 },
  { title: '操作', key: 'action', width: 130 },
]

const tablePagination = {
  pageSize: 10,
  showSizeChanger: true,
  pageSizeOptions: ['10', '20', '50'],
  showTotal: (total) => `共 ${total} 个 Mod`,
}

// 安装 HDR 合集：选择合集目录 → 后端校验/识别生成 mod.json → 移入托管并按子 Mod 登记
// 安装期间打开"正在安装 Mod"进度弹窗（后端按步骤推送进度），按钮同步 loading
async function importHdr() {
  const dir = await SelectDirectory()
  if (!dir) return
  installingHdr.value = true
  props.beginImportProgress()
  try {
    const res = await InstallHdrMod(dir)
    message.success(`已安装 HDR 合集「${res.nickname || res.name}」（${res.submods?.length || 0} 个子 Mod）`)
    await props.refreshMods()
  } catch (e) {
    message.error(String(e))
  } finally {
    props.endImportProgress()
    installingHdr.value = false
  }
}
</script>

<style scoped>
.page-toolbar { display: flex; align-items: center; justify-content: space-between; margin-bottom: 8px; }
.toolbar-title { font-size: 15px; font-weight: 500; color: #333; }
.count-badge { display: inline-block; background: #1a73e8; color: #fff; font-size: 12px; padding: 1px 8px; border-radius: 10px; margin-left: 6px; min-width: 20px; text-align: center; }
.mod-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); gap: 12px; }
.filter-bar { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
.conflict-banner { margin-bottom: 12px; }
.conflict-list { margin-top: 4px; }
.conflict-item { line-height: 1.9; }
.conflict-mod { font-weight: 600; color: #d4380d; }
.conflict-vs { color: #888; margin: 0 6px; }
.conflict-pair { color: #555; }
.conflict-res { color: #999; font-size: 12px; margin-left: 2px; }
.lib-search { max-width: 320px; margin-bottom: 12px; }
.lib-table :deep(.ant-table) { border-radius: 6px; overflow: hidden; }
.lib-table :deep(.ant-table-thead > tr > th) { background: #fafafa; color: #666; font-weight: 600; }
.lib-table :deep(.ant-table-tbody > tr:hover > td) { background: #f6f9ff; }
.mod-name-cell { display: inline-flex; align-items: center; gap: 8px; max-width: 100%; }
.folder-icon { color: #faad14; font-size: 16px; flex-shrink: 0; }
.folder-missing { color: #d9d9d9; }
.mod-name-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.name-missing { color: #bbb; }
.lock-name { font-weight: 500; }
.conflict-group-list { display: flex; flex-direction: column; gap: 12px; }
.conflict-group-card { background: #fff7f0; border: 1px solid #ffd8bf; border-radius: 8px; }
.conflict-group-card :deep(.ant-card-head) { border-bottom: 1px dashed #ffd8bf; }
.conflict-group-title { color: #d4380d; font-weight: 600; }
.conflict-group-mods { display: flex; align-items: center; flex-wrap: wrap; gap: 4px 14px; line-height: 2; }
.conflict-group-label { color: #888; font-size: 12px; }
.conflict-group-mod { display: inline-flex; align-items: center; }
.conflict-group-mod b { color: #333; }
.conflict-group-edges { margin-top: 6px; padding-top: 6px; border-top: 1px dashed #ffd8bf; }
.conflict-edge { line-height: 1.9; color: #d4380d; font-size: 13px; }
</style>

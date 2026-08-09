<template>
  <div class="page">
    <div class="search-bar">
      <a-select v-model:value="searchMode" style="width:110px" :options="searchModeOptions" />
      <a-input v-model:value="searchText" allow-clear placeholder="搜索 Mod 名称" class="search-input">
        <template #prefix><SearchOutlined /></template>
      </a-input>
      <a-tag v-if="searchMode === 'online'" color="orange">线上搜索即将开放</a-tag>
    </div>

    <div class="filter-bar">
      <a-radio-group v-model:value="instStatus" button-style="solid">
        <a-radio-button value="all">全部</a-radio-button>
        <a-radio-button value="enabled">已启用</a-radio-button>
        <a-radio-button value="disabled">已禁用</a-radio-button>
      </a-radio-group>
    </div>

    <a-empty v-if="installedMods.length === 0" description="还没有已安装的 Mod" style="margin-top:80px" />
    <template v-else>
      <a-empty v-if="filteredInstalled.length === 0" description="没有符合筛选条件的 Mod" style="margin-top:60px" />
      <div v-else class="mod-grid">
        <ModCard v-for="mod in filteredInstalled" :key="mod.name" :mod="mod" :conflict-info="conflictsMap[mod.name]"
          @toggle="toggleMod" @toggleRefresh="toggleModRefresh" @edit="openEditMod" @refash="onRefash" @openfolder="openModFolder" />
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { SearchOutlined } from '@ant-design/icons-vue'
import ModCard from './ModCard.vue'
import { OpenDirectory } from '../../wailsjs/go/main/App'
import { message } from 'ant-design-vue'

const props = defineProps(['mods', 'conflicts', 'toggleMod', 'toggleModRefresh', 'toggleSubMod', 'toggleSubModRefresh', 'openEditMod', 'installMod', 'uninstallMod', 'removeRecord', 'onRefash'])
const installedMods = computed(() => props.mods.filter(m => m.installed && !m.missing))

/** 打开 Mod 所在文件夹 */
async function openModFolder(path) {
  if (!path) return
  try {
    await OpenDirectory(path)
  } catch (e) {
    message.error(`打开文件夹失败：${e || ''}`)
  }
}

/** 冲突信息按 Mod 名称索引，供卡片展示当前 Mod 是否有冲突 */
const conflictsMap = computed(() => {
  const map = {}
  for (const c of props.conflicts || []) map[c.modName] = c
  return map
})

const searchText = ref('')
const searchMode = ref('local')
const searchModeOptions = [
  { label: '本地搜索', value: 'local' },
  { label: '线上搜索', value: 'online' },
]
const instStatus = ref('all')
const filteredInstalled = computed(() => {
  const q = searchText.value.trim().toLowerCase()
  return installedMods.value.filter(m => {
    if (q && !(m.name || '').toLowerCase().includes(q) && !(m.nickname || '').toLowerCase().includes(q)) return false
    if (instStatus.value === 'enabled' && !m.enabled) return false
    if (instStatus.value === 'disabled' && m.enabled) return false
    return true
  })
})
</script>

<style scoped>
.search-bar { display: flex; align-items: center; gap: 12px; flex-wrap: wrap; margin-bottom: 14px; }
.search-input { flex: 1; min-width: 200px; }
.filter-bar { display: flex; align-items: center; gap: 8px; margin-bottom: 12px; flex-wrap: wrap; }
.filter-bar :deep(.ant-radio-group) { display: flex; gap: 8px; }
.filter-bar :deep(.ant-radio-button-wrapper) { border-radius: 6px; }
.filter-bar :deep(.ant-radio-button-wrapper:not(:first-child)::before) { display: none; }
.mod-grid { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); grid-auto-rows: 1fr; gap: 12px; align-items: stretch; }
</style>

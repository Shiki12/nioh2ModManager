<template>
  <div class="page">
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
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { FolderFilled, SearchOutlined } from '@ant-design/icons-vue'

const props = defineProps(['mods', 'installMod', 'uninstallMod', 'removeRecord', 'openEditMod'])

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

const tablePagination = ref({
  pageSize: 5,
  showSizeChanger: true,
  pageSizeOptions: ['5', '10', '20'],
  showTotal: (total) => `共 ${total} 个 Mod`,
})
</script>

<style scoped>
.lib-search { max-width: 320px; margin-bottom: 12px; }
.lib-table :deep(.ant-table) { border-radius: 6px; overflow: hidden; }
.lib-table :deep(.ant-table-thead > tr > th) { background: #fafafa; color: #666; font-weight: 600; }
.lib-table :deep(.ant-table-tbody > tr:hover > td) { background: #f6f9ff; }
.mod-name-cell { display: inline-flex; align-items: center; gap: 8px; max-width: 100%; }
.folder-icon { color: #faad14; font-size: 16px; flex-shrink: 0; }
.folder-missing { color: #d9d9d9; }
.mod-name-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.name-missing { color: #bbb; }
</style>
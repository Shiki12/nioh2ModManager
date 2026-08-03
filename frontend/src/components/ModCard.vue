<template>
  <a-card hoverable class="mod-card" :class="{ 'sub-card': isSub }" @click="$emit('edit', isSub ? parentMod : mod)">
    <template #cover>
      <div class="mod-cover" :class="{ disabled: !mod.enabled }">
        <img v-if="coverSrc" :src="coverSrc" :alt="mod.nickname || mod.name" @error="onImgError" />
        <div v-else class="mod-cover-placeholder"><FileImageOutlined style="font-size:32px;color:#ccc" /></div>
        <div class="mod-cover-overlay" @click.stop>
          <a-switch :checked="mod.enabled" size="small" @change="onToggle" />
          <a-tooltip title="放大查看效果图">
            <ZoomInOutlined class="cover-zoom" @click="openPreview(0)" />
          </a-tooltip>
        </div>
      </div>
    </template>
    <a-card-meta>
      <template #title><span class="mod-title"><a-tag v-if="isSub" color="cyan" size="small">子 Mod</a-tag>{{ mod.nickname || mod.name }}</span></template>
      <template #description>
        <div class="mod-meta">
          <a-tag :color="mod.enabled ? 'success' : 'default'" size="small">{{ mod.enabled ? '已启用' : '已禁用' }}</a-tag>
          <a-tag v-if="mod.submods && mod.submods.length" color="cyan" size="small">组合包 {{ mod.submods.length }} 个子 Mod</a-tag>
          <a-tag v-if="mod.category === 'weapon'" color="purple" size="small">武器</a-tag>
          <a-tag v-else-if="mod.category === 'mixed'" color="gold" size="small">混合</a-tag>
          <a-tooltip v-if="conflictText" :title="conflictText">
            <a-tag color="error" size="small">冲突</a-tag>
          </a-tooltip>
          <span class="mod-path" :title="mod.name">{{ mod.name }}</span>
        </div>
        <a-tooltip v-if="armorText" :title="armorText" placement="top">
          <div class="mod-armor">占用的服装：{{ armorText }}</div>
        </a-tooltip>
        <div v-if="weapons.length" class="mod-weapons">
          <span class="mod-weapons-label">武器</span>
          <a-tag v-for="w in weapons" :key="w" color="purple" size="small">{{ w }}</a-tag>
        </div>
        <div v-if="effectImages.length > 1" class="mod-thumbs" @click.stop>
          <img v-for="(img, i) in effectImages.slice(0, 6)" :key="img" :src="resolveUrl(img)" :alt="i" class="mod-thumb" @click="openPreview(i)" @error="onImgError" />
          <span v-if="effectImages.length > 6" class="mod-thumbs-more" @click="openPreview(6)">+{{ effectImages.length - 6 }}</span>
        </div>
      </template>
    </a-card-meta>
    <template #actions>
      <a-tooltip :title="isSub ? '查看父 Mod 信息' : '编辑'">
        <EditOutlined key="edit" @click.stop="$emit('edit', isSub ? parentMod : mod)" />
      </a-tooltip>
      <a-tooltip v-if="mod.enabled" :title="isSub ? '禁用子 Mod：删除链接并刷新游戏' : '禁用：删除符号链接并刷新游戏'">
        <MinusCircleOutlined key="disable" style="color:#ff4d4f;font-size:16px" @click.stop="onToggleRefresh" />
      </a-tooltip>
    </template>
  </a-card>

  <a-modal v-model:open="previewVisible" :footer="null" :title="mod.nickname || mod.name" width="min(720px, 92vw)" centered>
    <div class="preview-wrap">
      <img v-if="previewSrc" :src="previewSrc" :alt="mod.nickname || mod.name" class="preview-img" @error="onPreviewError" />
      <a-button v-if="effectImages.length > 1" class="preview-nav prev" shape="circle" @click="previewNav(-1)"><LeftOutlined /></a-button>
      <a-button v-if="effectImages.length > 1" class="preview-nav next" shape="circle" @click="previewNav(1)"><RightOutlined /></a-button>
    </div>
    <div v-if="effectImages.length > 1" class="preview-count">{{ previewIndex + 1 }} / {{ effectImages.length }}</div>
  </a-modal>
</template>

<script setup>
import { computed, ref } from 'vue'
import { FileImageOutlined, EditOutlined, MinusCircleOutlined, ZoomInOutlined, LeftOutlined, RightOutlined } from '@ant-design/icons-vue'
const props = defineProps({ mod: { type: Object, required: true }, isSub: { type: Boolean, default: false }, parentMod: { type: Object, default: null }, conflictInfo: { type: Object, default: null } })
const emit = defineEmits(['toggle', 'toggleRefresh', 'toggleSub', 'toggleSubRefresh', 'edit'])
const previewVisible = ref(false)
const previewIndex = ref(0)
const flatParts = computed(() => {
  const flat = {}
  for (const [k, v] of Object.entries(props.mod.parts || {})) {
    flat[k] = (Array.isArray(v) ? v : [v]).filter(Boolean)
  }
  return flat
})
const armorText = computed(() => Object.entries(flatParts.value).filter(([k]) => k !== '武器').flatMap(([, vals]) => vals).join('、'))
const weapons = computed(() => flatParts.value['武器'] || [])
const conflictText = computed(() => {
  const confs = props.conflictInfo?.conflicts
  if (!confs || !confs.length) return ''
  return confs.map(c => `与「${c.nickname || c.modName}」冲突：${c.slot} → ${c.value}`).join('；')
})
// 多张效果图：普通 Mod 取 mod.previews（缺省回退 cover）；
// 组合包父级取父级 + 各子 Mod 的效果图并集；子 Mod 卡片取自身效果图。
const effectImages = computed(() => {
  const imgs = []
  const push = p => { if (p && !imgs.includes(p)) imgs.push(p) }
  if (props.isSub) {
    ;(props.mod.previews || []).forEach(push)
    push(props.mod.cover)
  } else if (props.mod.submods && props.mod.submods.length) {
    ;(props.mod.previews || []).forEach(push)
    for (const sm of props.mod.submods || []) {
      if (sm.previews && sm.previews.length) sm.previews.forEach(push)
      else push(sm.cover)
    }
    push(props.mod.cover)
  } else {
    ;(props.mod.previews || []).forEach(push)
    push(props.mod.cover)
  }
  return imgs
})
const coverSrc = computed(() => (effectImages.value.length ? resolveUrl(effectImages.value[0]) : ''))
const previewSrc = computed(() => resolveUrl(effectImages.value[previewIndex.value] || props.mod.cover))
function onToggle() { if (props.isSub) emit('toggleSub', props.parentMod, props.mod); else emit('toggle', props.mod) }
function onToggleRefresh() { if (props.isSub) emit('toggleSubRefresh', props.parentMod, props.mod); else emit('toggleRefresh', props.mod) }
// 效果图：mod.json 指定的是相对文件名（随 mod 文件夹走）→ 走 /modfile；
// 旧数据/手动选择的是绝对路径 → 回退 /localfile
function resolveUrl(path) {
  if (!path) return ''
  if (/^[a-zA-Z]:[\\/]/.test(path) || path.startsWith('\\\\')) {
    return '/localfile?file=' + encodeURIComponent(path)
  }
  // 子 Mod 封面相对父 Mod（合集）目录解析
  const owner = props.isSub ? props.parentMod?.name : props.mod.name
  return '/modfile?mod=' + encodeURIComponent(owner || props.mod.name) + '&file=' + encodeURIComponent(path)
}
function onImgError(e) { e.target.style.display = 'none' }
function onPreviewError(e) { e.target.style.display = 'none' }
function openPreview(i) { previewIndex.value = i; previewVisible.value = true }
function previewNav(d) {
  const n = effectImages.value.length
  if (n) previewIndex.value = (previewIndex.value + d + n) % n
}
</script>

<style scoped>
.mod-card { border-radius: 6px !important; overflow: hidden; transition: box-shadow .2s, transform .2s; }
.mod-card :deep(.ant-card-body) { padding: 10px 12px !important; }
.mod-card:hover { box-shadow: 0 2px 8px rgba(0,0,0,.10); transform: translateY(-1px); }
.mod-cover { height: 110px; overflow: hidden; background: #e8e8e8; position: relative; }
.mod-cover img { width: 100%; height: 100%; object-fit: cover; }
.mod-cover-placeholder { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; background: #f5f5f5; }
.mod-cover.disabled img { opacity: .45; filter: grayscale(100%); }
.mod-cover-overlay { position: absolute; top: 4px; right: 4px; z-index: 2; display: flex; align-items: center; gap: 6px; }
.cover-zoom { color: #fff; background: rgba(0,0,0,.45); border-radius: 50%; padding: 4px; cursor: pointer; font-size: 14px; }
.cover-zoom:hover { background: #1a73e8; }
.mod-title { display: inline-flex; align-items: center; gap: 6px; overflow: hidden; }
.mod-meta { display: flex; align-items: center; justify-content: space-between; gap: 8px; }
.mod-path { font-size: 12px; color: #999; max-width: 120px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mod-armor { margin-top: 6px; font-size: 12px; color: #1a73e8; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.mod-weapons { margin-top: 6px; display: flex; align-items: center; gap: 4px; flex-wrap: wrap; }
.mod-weapons-label { font-size: 12px; color: #999; margin-right: 2px; flex-shrink: 0; }
.mod-thumbs { margin-top: 6px; display: flex; align-items: center; gap: 4px; }
.mod-thumb { width: 30px; height: 22px; object-fit: cover; border-radius: 3px; cursor: zoom-in; border: 1px solid #eee; }
.mod-thumb:hover { border-color: #1a73e8; }
.mod-thumbs-more { font-size: 11px; color: #1a73e8; cursor: pointer; padding: 0 4px; }
.preview-wrap { position: relative; }
.preview-img { width: 100%; display: block; }
.preview-nav { position: absolute; top: 50%; transform: translateY(-50%); z-index: 2; border-color: rgba(0,0,0,.2); background: rgba(0,0,0,.45); color: #fff; }
.preview-nav:hover { background: #1a73e8; color: #fff; }
.preview-nav.prev { left: 8px; }
.preview-nav.next { right: 8px; }
.preview-count { text-align: center; color: #999; font-size: 12px; margin-top: 8px; }
.mod-card :deep(.ant-card-actions) { background: #fafafa; }
.mod-card :deep(.ant-card-actions > li) { margin: 4px 0; }
.mod-card :deep(.ant-card-meta-title) { font-size: 14px; margin-bottom: 4px !important; }
</style>

<template>
  <a-card hoverable class="mod-card" :class="{ 'sub-card': isSub, 'mod-card-enabled': mod.enabled }" @click="$emit('edit', isSub ? parentMod : mod)">
    <template #cover>
      <div class="mod-cover" :class="{ disabled: !mod.enabled }">
        <img v-if="coverSrc" :src="coverSrc" :alt="mod.nickname || mod.name" loading="lazy" decoding="async" @error="onImgError" />
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
      <template #title>
        <a-tooltip :title="mod.nickname || mod.name" placement="topLeft">
          <span class="mod-title">
            <a-tag v-if="isSub" class="tag-sub" size="small">子 Mod</a-tag>
            <span class="mod-title-text">{{ mod.nickname || mod.name }}</span>
          </span>
        </a-tooltip>
      </template>
      <template #description>
        <div class="mod-tags">
          <a-tag :class="mod.enabled ? 'tag-enabled' : 'tag-disabled'" size="small">{{ mod.enabled ? '已启用' : '已禁用' }}</a-tag>
          <div v-if="mod.submods && mod.submods.length" class="mod-composite">
            <a-tag class="tag-type" size="small">组合包 {{ mod.submods.length }} 个子 Mod</a-tag>
            <a-tag class="tag-sub-count" size="small">{{ subEnabledCount }}/{{ mod.submods.length }} 已启用</a-tag>
          </div>
          <a-tag v-if="mod.category === 'weapon'" class="tag-type" size="small">武器</a-tag>
          <a-tag v-else-if="mod.category === 'mixed'" class="tag-type" size="small">混合</a-tag>
          <a-tooltip v-if="conflictText" :title="conflictText">
            <a-tag class="tag-warning" size="small">冲突</a-tag>
          </a-tooltip>
        </div>
        <div class="mod-folder" :title="mod.name">{{ mod.name }}</div>
        <a-tooltip v-if="armorText" :title="armorText" placement="top">
          <div class="mod-armor">占用的服装：{{ armorText }}</div>
        </a-tooltip>
        <div v-if="weapons.length" class="mod-weapons">
          <span class="mod-weapons-label">武器</span>
          <a-tooltip :title="weapons.join('、')" placement="top">
            <div class="mod-weapons-inner">
              <a-tag v-for="w in weapons.slice(0, maxWeapons)" :key="w" class="tag-type" size="small">{{ w }}</a-tag>
              <span v-if="weapons.length > maxWeapons" class="mod-weapons-more">+{{ weapons.length - maxWeapons }}</span>
            </div>
          </a-tooltip>
        </div>
        <div v-if="!hideThumbs && effectImages.length > 1" class="mod-thumbs" @click.stop>
          <img v-for="(img, i) in effectImages.slice(0, 6)" :key="img" :src="resolveUrl(img, 64)" :alt="i" class="mod-thumb" loading="lazy" decoding="async" @click="openPreview(i)" @error="onImgError" />
          <span v-if="effectImages.length > 6" class="mod-thumbs-more" @click="openPreview(6)">+{{ effectImages.length - 6 }}</span>
        </div>
      </template>
    </a-card-meta>
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
import { FileImageOutlined, ZoomInOutlined, LeftOutlined, RightOutlined } from '@ant-design/icons-vue'
const props = defineProps({ mod: { type: Object, required: true }, isSub: { type: Boolean, default: false }, parentMod: { type: Object, default: null }, conflictInfo: { type: Object, default: null }, hideThumbs: { type: Boolean, default: false } })
const emit = defineEmits(['toggle', 'toggleRefresh', 'toggleSub', 'toggleSubRefresh', 'edit'])
const maxWeapons = 3
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
const subEnabledCount = computed(() => (props.mod.submods || []).filter(s => s.enabled).length)
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
const coverSrc = computed(() => (effectImages.value.length ? resolveUrl(effectImages.value[0], 360) : ''))
const previewSrc = computed(() => resolveUrl(effectImages.value[previewIndex.value] || props.mod.cover))
function onToggle() { if (props.isSub) emit('toggleSub', props.parentMod, props.mod); else emit('toggle', props.mod) }
// 效果图：mod.json 指定的是相对文件名（随 mod 文件夹走）→ 走 /modfile；
// 旧数据/手动选择的是绝对路径 → 回退 /localfile
// dim 指定时请求服务端缩略图（?w=），仅用于小尺寸显示；缺省返回原图（全屏预览用）
function resolveUrl(path, dim) {
  if (!path) return ''
  if (/^[a-zA-Z]:[\\/]/.test(path) || path.startsWith('\\\\')) {
    const u = '/localfile?file=' + encodeURIComponent(path)
    return dim ? u + '&w=' + dim : u
  }
  // 子 Mod 封面相对父 Mod（合集）目录解析
  const owner = props.isSub ? props.parentMod?.name : props.mod.name
  const u = '/modfile?mod=' + encodeURIComponent(owner || props.mod.name) + '&file=' + encodeURIComponent(path)
  return dim ? u + '&w=' + dim : u
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
.mod-card { border-radius: 10px !important; overflow: hidden; height: 100%; display: flex; flex-direction: column; box-shadow: 0 4px 12px rgba(0,0,0,0.05); transition: box-shadow .2s, transform .2s; }
.mod-card :deep(.ant-card-body) { padding: 12px 14px !important; flex: 1 1 auto; display: flex; flex-direction: column; min-height: 0; }
.mod-card :deep(.ant-card-body > .ant-card-meta) { flex: 1 1 auto; display: flex; flex-direction: column; min-height: 0; }
.mod-card :deep(.ant-card-body > .ant-card-meta > .ant-card-meta-detail) { flex: 1 1 auto; display: flex; flex-direction: column; min-height: 0; }
.mod-card :deep(.ant-card-cover) { border-radius: 10px 10px 0 0; overflow: hidden; }
.mod-card :deep(.ant-card-cover img) { border-radius: 0; }
.mod-card:hover { box-shadow: 0 6px 18px rgba(0,0,0,0.10); transform: translateY(-2px); }
/* 启用状态高亮：主题色边框 + 微光 */
.mod-card-enabled { border-color: #1a73e8 !important; box-shadow: 0 0 0 1px rgba(26,115,232,.55), 0 0 14px rgba(26,115,232,.30) !important; }
.mod-card-enabled:hover { box-shadow: 0 0 0 1px rgba(26,115,232,.55), 0 0 16px rgba(26,115,232,.34) !important; }
.mod-cover { width: 100%; aspect-ratio: 16 / 9; overflow: hidden; background: #e5e7eb; position: relative; }
.mod-cover img { width: 100%; height: 100%; object-fit: cover; }
.mod-cover-placeholder { width: 100%; height: 100%; display: flex; align-items: center; justify-content: center; background: #f5f5f5; }
.mod-cover.disabled img { opacity: .45; filter: grayscale(100%); }
/* 顶部+底部渐变遮罩：保证浅色图片上标题区/控件清晰可见 */
.mod-cover::before { content: ''; position: absolute; left: 0; right: 0; top: 0; height: 45%; background: linear-gradient(to bottom, rgba(0,0,0,.45), transparent); pointer-events: none; z-index: 1; }
.mod-cover::after { content: ''; position: absolute; left: 0; right: 0; bottom: 0; height: 35%; background: linear-gradient(to top, rgba(0,0,0,.35), transparent); pointer-events: none; z-index: 1; }
.mod-cover-overlay { position: absolute; top: 4px; right: 4px; z-index: 2; display: flex; align-items: center; gap: 6px; background: rgba(0,0,0,.35); padding: 2px 5px; border-radius: 12px; }
.cover-zoom { color: #fff; background: rgba(0,0,0,.45); border-radius: 50%; padding: 4px; cursor: pointer; font-size: 14px; }
.cover-zoom:hover { background: #1a73e8; }
.mod-title { display: flex; align-items: center; gap: 6px; max-width: 100%; min-width: 0; }
.mod-title-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; min-width: 0; font-weight: 600; }
/* 胶囊标签：统一高度/字号 */
.mod-card :deep(.ant-tag) { border-radius: 999px; height: 20px; line-height: 18px; padding: 0 8px; font-size: 12px; margin-inline-end: 4px; border: none; }
.mod-card :deep(.ant-tag.tag-enabled) { background: #e6ffed !important; color: #389e0d !important; }
.mod-card :deep(.ant-tag.tag-disabled) { background: #f0f0f0 !important; color: #999 !important; }
.mod-card :deep(.ant-tag.tag-type) { background: #e6f4ff !important; color: #1a73e8 !important; }
.mod-card :deep(.ant-tag.tag-sub-count) { background: #f0f5ff !important; color: #597ef7 !important; }
.mod-card :deep(.ant-tag.tag-sub) { background: #e6fffb !important; color: #08979c !important; }
.mod-card :deep(.ant-tag.tag-warning) { background: #fff1f0 !important; color: #d9363e !important; }
.mod-tags { display: flex; flex-wrap: wrap; align-items: center; margin-top: 6px; }
.mod-composite { display: flex; flex-direction: column; gap: 2px; width: 100%; margin-top: 2px; }
.mod-folder { margin-top: 6px; font-size: 11px; color: #aaa; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
/* 占用服装：灰色辅助文本，最多 2 行省略（Tooltip 已绑定完整文本） */
.mod-armor { margin-top: 6px; font-size: 12px; color: #888; display: -webkit-box; -webkit-line-clamp: 2; -webkit-box-orient: vertical; overflow: hidden; }
.mod-weapons { margin-top: 6px; display: flex; align-items: center; gap: 4px; }
.mod-weapons-label { font-size: 12px; color: #999; margin-right: 2px; flex-shrink: 0; }
.mod-weapons-inner { display: inline-flex; align-items: center; gap: 4px; flex-wrap: nowrap; overflow: hidden; min-width: 0; }
.mod-weapons-more { font-size: 11px; color: #1a73e8; cursor: pointer; padding: 0 4px; flex-shrink: 0; }
.mod-thumbs { margin-top: auto; padding-top: 6px; display: flex; align-items: center; gap: 4px; }
.mod-thumb { width: 30px; height: 22px; object-fit: cover; border-radius: 4px; cursor: zoom-in; border: 1px solid #eee; }
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
.mod-card :deep(.ant-card-meta-title) { font-size: 14px; margin-bottom: 0 !important; }
</style>

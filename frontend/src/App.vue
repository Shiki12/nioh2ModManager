<template>
  <a-config-provider :theme="themeConfig">
    <a-layout class="app-layout">
      <AppSider :engine-status="engineStatus" @navigate="key => currentPage = key" @open-engine-modal="openEngineModal" />

      <a-layout class="main-layout">
        <a-layout-header class="app-header">
          <div class="header-left">
            <h2 class="page-title">{{ currentPageTitle }}</h2>
          </div>
          <div class="header-right">
            <a-space v-if="currentPage === 'mods' || currentPage === 'library'" :size="8" class="header-actions">
              <a-tooltip :title="gameRunning ? '刷新游戏 Mod' : '请先启动游戏'"><img :src="refreshImg" class="header-img-btn" :class="{ 'is-disabled': !gameRunning }" alt="刷新游戏 Mod" @click="gameRunning && refreshGame()" /></a-tooltip>
              <a-tooltip title="自动安装"><img :src="installImg" class="header-img-btn" alt="自动安装" @click="installMod" /></a-tooltip>
              <a-tooltip title="安装 HDR Mod"><img :src="installHdrImg" class="header-img-btn" :class="{ 'is-loading': installingHdr }" alt="安装 HDR Mod" @click="installHdr" /></a-tooltip>
              <a-tooltip title="刷新列表"><img :src="refreshListImg" class="header-img-btn" alt="刷新列表" @click="refreshMods" /></a-tooltip>
              <a-tooltip title="卡片工具"><img :src="cardToolImg" class="header-img-btn" alt="卡片工具" @click="cardToolVisible = true" /></a-tooltip>
              <a-tooltip :title="conflictCount > 0 ? '解决冲突' : '没有冲突'">
                <div class="conflict-img-wrap" @click="conflictVisible = true">
                  <img :src="conflictImg" class="header-img-btn" alt="解决冲突" />
                  <span class="conflict-badge" :class="{ 'no-conflict': conflictCount === 0, 'has-conflict': conflictCount > 0 }">{{ conflictCount }}</span>
                </div>
              </a-tooltip>
            </a-space>
            <a-button v-if="!gameRunning" type="primary" class="btn-green launch-btn" @click="launchGame"><template #icon><CaretRightOutlined /></template>启动游戏</a-button>
            <a-button v-else type="primary" danger class="launch-btn" @click="launchGame"><template #icon><CloseOutlined /></template>立即停止</a-button>
          </div>
        </a-layout-header>

        <a-layout-content class="app-content">
          <ModsPage
            v-if="currentPage === 'mods'"
            :mods="mods"
            :conflicts="conflicts"
            :toggleMod="toggleMod"
            :toggleModRefresh="toggleModRefresh"
            :toggleSubMod="toggleSubMod"
            :toggleSubModRefresh="toggleSubModRefresh"
            :openEditMod="openEditMod"
            :installMod="openInstallMod"
            :uninstallMod="uninstallMod"
            :removeRecord="removeRecord"
          />
          <SettingsPage
            v-else-if="currentPage === 'settings'"
            :settingsForm="settingsForm"
            :selectGameDir="selectGameDir"
            :searchGame="searchGame"
            :selectModsDir="selectModsDir"
            :saveAllSettings="saveAllSettings"
            :saveModsRepo="saveModsRepo"
          />
          <ModLibrary
            v-else-if="currentPage === 'library'"
            :mods="mods"
            :installMod="openInstallMod"
            :uninstallMod="uninstallMod"
            :removeRecord="removeRecord"
            :openEditMod="openEditMod"
          />
        </a-layout-content>

        <LogPanel />
      </a-layout>
    </a-layout>

    <!-- 弹窗：冲突管理 -->
    <a-modal v-model:open="conflictVisible" title="冲突管理" :footer="null" width="680px">
      <a-empty v-if="conflictGroups.length === 0" description="当前已启用 Mod 之间没有冲突" />
      <div v-else class="conflict-group-list">
        <a-card v-for="(g, gi) in conflictGroups" :key="gi" size="small" class="conflict-group-card" :bordered="false">
          <template #title>
            <span class="conflict-group-title">冲突组 {{ gi + 1 }}（{{ g.mods.length }} 个 Mod）</span>
          </template>
          <div class="conflict-group-mods">
            <span class="conflict-group-label">涉及 Mod：</span>
            <span v-for="(entry, mi) in g.mods" :key="entry.mod.name" class="conflict-group-mod">
              <template v-if="entry.subs.length">
                <b class="conflict-mod-title">{{ entry.mod.nickname || entry.mod.name }}</b>
                <span v-for="sub in entry.subs" :key="sub.name" class="conflict-sub-mod">
                  <a-switch size="small" :checked="sub.enabled" @change="() => toggleSubMod(entry.mod, sub)" />
                  {{ sub.name }}
                </span>
              </template>
              <template v-else>
                <a-switch size="small" :checked="entry.mod.enabled" @change="() => toggleMod(entry.mod)" />
                <b>{{ entry.mod.nickname || entry.mod.name }}</b>
              </template>
              <template v-if="mi < g.mods.length - 1">、</template>
            </span>
          </div>
          <div class="conflict-group-edges">
            <div v-for="(e, ei) in g.edges" :key="ei" class="conflict-edge">
              「{{ nickOf(e.a) }}」与「{{ nickOf(e.b) }}」冲突：{{ e.slot }} → {{ e.value }}
            </div>
          </div>
        </a-card>
      </div>
    </a-modal>

    <!-- 弹窗：卡片工具 -->
    <a-modal v-model:open="cardToolVisible" title="卡片工具" :footer="null" width="min(1120px, 96vw)"
      :body-style="{ padding: '20px' }">
      <AuthorTool />
    </a-modal>

    <!-- 弹窗：安装 Mod 引擎 -->
    <a-modal :open="engineModalVisible" title="安装 Mod 引擎" :closable="false" :maskClosable="false"
      :okText="engineInstalling ? '安装中...' : '立即安装'" :cancelText="'稍后再说'"
      :confirm-loading="engineInstalling" @ok="installEngine" @cancel="engineModalVisible = false"
      :ok-button-props="{ class: 'btn-blue' }" :cancel-button-props="{ class: 'btn-cancel' }">
      <a-space direction="vertical" style="width:100%">
        <a-alert message="检测到游戏根目录缺少 Mod 引擎，请选择安装方式" type="warning" show-icon />

        <div class="engine-mode">
          <div class="engine-mode-title">方式一：系统安装（自动）</div>
          <div style="color:#555;font-size:13px">
            点击"立即安装"，程序将自动完成 Mod 引擎的安装：
            <b style="word-break:break-all">{{ settingsForm.gameRoot }}</b>
          </div>
        </div>

        <div class="engine-mode engine-mode-manual">
          <div class="engine-mode-title">方式二：手动安装</div>
          <div style="color:#555;font-size:13px">
            复制下方引擎包路径，自行完成 Mod 引擎的安装：
          </div>
          <a-space style="width:100%;margin-top:6px">
            <a-input :value="engineZipPath" readonly style="flex:1" />
            <a-button class="btn-gold" @click="copyEnginePath"><CopyOutlined /> 复制</a-button>
            <a-button class="btn-purple" @click="openEngineZipFolder"><FolderOpenOutlined /> 打开所在文件夹</a-button>
          </a-space>
        </div>
      </a-space>
    </a-modal>

    <!-- 弹窗：游戏目录 -->
    <a-modal :open="needSetup" title="确认游戏目录" :closable="false" :maskClosable="false" okText="确认" cancelText="手动搜索" @ok="confirmGameRoot" @cancel="searchGame"
      :ok-button-props="{ class: 'btn-blue' }" :cancel-button-props="{ class: 'btn-gold' }">
      <a-space direction="vertical" style="width:100%">
        <a-alert message="未检测到游戏目录或需要确认" type="warning" show-icon />
        <a-space style="width:100%">
          <a-input v-model:value="settingsForm.gameRoot" placeholder="如: E:\SteamLibrary\steamapps\common\Nioh2" style="flex:1" />
          <a-button class="btn-cyan" @click="selectGameDir"><FolderOpenOutlined /> 选择目录</a-button>
        </a-space>
        <span style="color:#888;font-size:12px">点击"手动搜索"自动查找，或"选择目录"手动浏览</span>
      </a-space>
    </a-modal>

    <!-- 弹窗：Mod 托管目录 -->
    <a-modal :open="needModsRepoSetup && !needSetup" title="指定 Mod 托管目录" :closable="false" :maskClosable="false"
      :okText="detectingGameDir ? '检测中...' : '确认并扫描'"
      :ok-button-props="{ disabled: detectingGameDir || !gameDirConfirmed, class: 'btn-blue' }"
      :cancel-button-props="{ class: 'btn-cancel' }"
      @ok="confirmModsRepo" @cancel="needModsRepoSetup = false">
      <a-space direction="vertical" style="width:100%">
        <a-alert message="系统将自动核对游戏目录，请确认扫描结果" type="info" show-icon />

        <a-spin :spinning="detectingGameDir" tip="正在扫描游戏目录...">
          <Transition name="fade" mode="out-in">
            <div v-if="detectingGameDir" key="loading" class="detect-panel detect-loading">
              <LoadingOutlined spin class="detect-icon" />
              <span>正在自动扫描游戏目录...</span>
            </div>
            <a-alert v-else-if="detectedGameDirError" key="error" type="warning" show-icon message="未检测到游戏目录">
              <template #description>
                <div class="detect-path">{{ detectedGameDirError }}</div>
                <a-button size="small" class="btn-orange" style="margin-top:8px" @click="pickGameDirManual"><FolderOpenOutlined /> 手动选择游戏目录</a-button>
              </template>
            </a-alert>
            <div v-else-if="detectedGameRoot" key="ok" class="detect-panel detect-ok">
              <CheckCircleOutlined class="detect-icon detect-icon-ok" />
              <div class="detect-result">
                <div class="detect-result-title">系统扫描到游戏目录</div>
                <div class="detect-path">{{ detectedGameRoot }}</div>
                <a-checkbox v-model:checked="gameDirConfirmed" style="margin-top:6px">确认使用该游戏目录</a-checkbox>
              </div>
            </div>
          </Transition>
        </a-spin>

        <a-divider style="margin:4px 0" />

        <a-alert message="请指定存放 Mod 的目录，所有 Mod 文件夹将存放在该目录下" type="info" show-icon />
        <a-space style="width:100%">
          <a-input v-model:value="settingsForm.modsRepo" placeholder="如: F:\Mod" style="flex:1" />
          <a-button class="btn-cyan" @click="selectModsDir"><FolderOpenOutlined /> 选择目录</a-button>
        </a-space>
      </a-space>
    </a-modal>

    <!-- 弹窗：安装 Mod -->
    <a-modal :open="installModVisible" title="安装 Mod" :closable="false" :maskClosable="false" okText="确认安装" cancelText="取消"
      :confirm-loading="installRecognizing" @ok="installModConfirm" @cancel="installModVisible = false"
      :ok-button-props="{ class: 'btn-green' }" :cancel-button-props="{ class: 'btn-cancel' }">
      <a-space direction="vertical" style="width:100%">
        <div v-if="installRecognizing" class="recognize-panel">
          <LoadingOutlined spin class="recognize-icon" />
          <span>正在识别该 Mod 占用的服装/武器资源…</span>
        </div>
        <template v-else>
          <a-alert v-if="!installingSubMods.length"
            :message="installPrompt.text || '请填写该 Mod 对应的衣服名称（占用的装备资源），用于生成已安装卡片'"
            :type="installPrompt.type" show-icon />
          <a-alert v-else type="info" show-icon :message="`该 Mod 为组合包（HDR 合集），共 ${installingSubMods.length} 个子 Mod，安装后各子 Mod 将分别显示在卡片中`" />
          <div class="install-mod-name">{{ installingMod?.name }}</div>
          <template v-if="installingSubMods.length">
            <div class="form-section-title">子 Mod（组合包，共 {{ installingSubMods.length }} 个）</div>
            <div class="section-tip">
              组合包由多个子 Mod 组成，每个子 Mod 各自占用一套装备资源，父级不单独占用。安装后需先启用组合包，再单独启用子 Mod
            </div>
            <div class="edit-submods">
              <ModCard
                v-for="sub in installingSubMods"
                :key="sub.name"
                :mod="installSubModView(sub)"
                :is-sub="true"
                :parent-mod="installingMod"
                hide-thumbs
                @toggle-sub="() => toggleSubMod(installingMod, sub)"
                @toggle-sub-refresh="() => toggleSubModRefresh(installingMod, sub)"
                @edit="() => openSubEdit(sub)"
              />
            </div>
          </template>
          <div v-else class="parts-grid">
            <div v-for="slot in allSlots" :key="slot.name" class="parts-row">
              <a-tag class="parts-tag">{{ slot.name }}</a-tag>
              <a-select
                v-if="slot.name === '武器'"
                v-model:value="installParts[slot.name]"
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
                v-model:value="installParts[slot.name]"
                :options="slotOptions(slot)"
                placeholder="未占用"
                allow-clear
                show-search
                option-filter-prop="label"
                class="parts-select"
              />
            </div>
          </div>
        </template>
      </a-space>
    </a-modal>

    <!-- 弹窗：Mod 导入进度 -->
    <a-modal :open="importProgressVisible" :footer="null" :closable="false" :maskClosable="false" title="正在安装 Mod">
      <a-steps :current="importStep" size="small" style="margin-bottom:16px">
        <a-step title="移动文件夹到托管目录" />
        <a-step title="登记到 Mod 数据仓库" />
        <a-step title="识别占用资源（服装/武器）" />
      </a-steps>
      <a-progress v-if="importPercent > 0" :percent="importPercent" size="small" style="margin-bottom:8px" />
      <div style="color:#666;font-size:13px">{{ importMessage }}</div>
    </a-modal>

    <!-- 弹窗：Mod 编辑 -->
    <a-modal :open="editModVisible" title="编辑 Mod" :footer="null" :width="680" @cancel="editModVisible = false">
      <a-form layout="vertical" v-if="editingMod" class="part-form">
        <div class="form-section-title">基础信息</div>
        <a-form-item label="昵称">
          <a-input v-model:value="editNickname" placeholder="自定义显示名称" @pressEnter="saveModEdit" />
        </a-form-item>

        <div class="form-section-title">图片资源</div>
        <a-form-item label="封面图片">
          <div class="img-card" @click="selectModCover" :title="editCover || '点击选择封面图片'">
            <img v-if="editCoverUrl(128)" :src="editCoverUrl(128)" alt="封面预览" loading="lazy" decoding="async" @error="e => e.target.style.display = 'none'" />
            <div v-else class="img-card-empty">
              <PictureOutlined />
              <span>点击选择封面图</span>
            </div>
            <a-tooltip title="移除封面">
              <CloseOutlined v-if="editCover" class="img-card-remove" @click.stop="editCover = ''" />
            </a-tooltip>
          </div>
          <div v-if="editCover" class="img-card-name" :title="editCover">{{ editCover }}</div>
        </a-form-item>
        <a-form-item label="效果图（多张）">
          <div class="preview-list">
            <div v-for="(p, i) in editPreviews" :key="p" class="preview-item" @click="openEditPreview(i)">
              <img :src="editPreviewUrl(p, 96)" :alt="`效果图 ${i + 1}`" loading="lazy" decoding="async" @error="e => e.target.style.display = 'none'" />
              <a-tooltip title="设为封面"><StarOutlined class="preview-item-cover" :class="{ active: p === editCover }" @click.stop="setCover(p)" /></a-tooltip>
              <a-tooltip title="移除效果图"><CloseOutlined class="preview-item-remove" @click.stop="removeModPreview(p)" /></a-tooltip>
            </div>
            <div v-if="!editPreviews.length" class="preview-empty">暂无效果图，可点击下方按钮添加或自动扫描</div>
          </div>
          <a-space style="margin-top:8px">
            <a-button size="small" class="btn-blue" @click="selectModPreview"><PictureOutlined /> 添加效果图</a-button>
            <a-button size="small" @click="scanModPreviews"><ReloadOutlined /> 自动扫描</a-button>
          </a-space>
        </a-form-item>

        <template v-if="editingMod.submods && editingMod.submods.length">
          <div class="form-section-title">子 Mod（组合包，共 {{ editingMod.submods.length }} 个）</div>
          <div class="section-tip">
            组合包由多个子 Mod 组成，每个子 Mod 各自占用一套装备资源，父级不单独占用。点击子 Mod 卡片的编辑图标可手动填写/修改其占用
          </div>
          <div class="edit-submods">
            <ModCard
              v-for="sub in editingMod.submods"
              :key="sub.name"
              :mod="subModView(sub)"
              :is-sub="true"
              :parent-mod="editingMod"
              hide-thumbs
              @toggle-sub="() => toggleSubMod(editingMod, sub)"
              @toggle-sub-refresh="() => toggleSubModRefresh(editingMod, sub)"
              @edit="() => openSubEdit(sub)"
            />
          </div>
        </template>

        <template v-if="!isEditingComposite">
          <div class="form-section-title">装备占用配置</div>
          <div class="section-tip">
            选择该 Mod 替换的游戏资源：可同时占用服装部位和武器，用于检测与其他 Mod 的冲突
          </div>
          <div class="parts-grid">
            <div v-for="slot in allSlots" :key="slot.name" class="parts-row">
              <a-tag class="parts-tag">{{ slot.name }}</a-tag>
              <a-select
                v-if="slot.name === '武器'"
                v-model:value="editParts[slot.name]"
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
                v-model:value="editParts[slot.name]"
                :options="slotOptions(slot)"
                placeholder="未占用"
                allow-clear
                show-search
                option-filter-prop="label"
                class="parts-select"
              />
            </div>
          </div>
        </template>

        <a-space class="form-actions" :size="10">
          <a-button v-if="!isEditingComposite" :loading="generatingModJson" class="btn-cancel" @click="generateModJson"><FileTextOutlined /> 生成 mod.json</a-button>
          <a-button type="primary" class="btn-blue" @click="saveModEdit">保存</a-button>
          <a-button @click="editModVisible = false">取消</a-button>
        </a-space>
      </a-form>
    </a-modal>

    <!-- 弹窗：修改子 Mod 占用 -->
    <a-modal :open="subEditVisible" :title="`修改子 Mod 占用：${editingSub?.name || ''}`" :footer="null" :width="560" @cancel="subEditVisible = false">
      <div class="sub-cover-row">
        <div class="edit-cover-box" @click="selectSubCover" :title="editSubCover || '点击选择封面图'">
          <img v-if="subCoverUrl(128)" :src="subCoverUrl(128)" alt="子 Mod 封面" loading="lazy" decoding="async" @error="e => e.target.style.display = 'none'" />
          <div v-else class="edit-cover-placeholder">
            <PictureOutlined style="font-size:20px;color:#ccc" />
            <span>点选封面</span>
          </div>
        </div>
        <div class="sub-cover-tip">
          点击选择本地图片作为子 Mod 封面（会复制到子 Mod 目录）；也可在下方效果图中点星标设为封面
        </div>
      </div>
      <div style="color:#888;font-size:12px;margin-bottom:10px">
        该子 Mod 自动解析的装备资源可能不完整，可在此手动选择其占用的服装部位和武器
      </div>
      <div class="parts-grid">
        <div v-for="slot in allSlots" :key="slot.name" class="parts-row">
          <a-tag class="parts-tag">{{ slot.name }}</a-tag>
          <a-select
            v-if="slot.name === '武器'"
            v-model:value="editSubParts[slot.name]"
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
            v-model:value="editSubParts[slot.name]"
            :options="slotOptions(slot)"
            placeholder="未占用"
            allow-clear
            show-search
            option-filter-prop="label"
            class="parts-select"
          />
        </div>
      </div>
      <a-divider style="margin:12px 0">子 Mod 效果图</a-divider>
      <div class="preview-list">
        <div v-for="(p, i) in editSubPreviews" :key="p" class="preview-item" @click="openSubPreview(i)">
          <img :src="subPreviewUrl(p, 96)" :alt="`效果图 ${i + 1}`" loading="lazy" decoding="async" @error="e => e.target.style.display = 'none'" />
          <a-tooltip title="设为封面"><StarOutlined class="preview-item-cover" :class="{ active: p === editSubCover }" @click.stop="setSubCover(p)" /></a-tooltip>
          <a-tooltip title="移除效果图"><CloseOutlined class="preview-item-remove" @click.stop="removeSubPreview(p)" /></a-tooltip>
        </div>
        <div v-if="!editSubPreviews.length" class="preview-empty">暂无效果图，可点击下方按钮添加或自动扫描</div>
      </div>
      <a-space style="margin-top:8px">
        <a-button size="small" class="btn-blue" @click="addSubPreview"><PictureOutlined /> 添加效果图</a-button>
        <a-button size="small" @click="scanSubPreviews"><ReloadOutlined /> 自动扫描</a-button>
      </a-space>
      <a-space style="margin-top:16px;display:flex;justify-content:center;width:100%">
        <a-button type="primary" class="btn-blue" :loading="savingSubParts" @click="saveSubParts">保存占用</a-button>
        <a-button class="btn-cancel" @click="subEditVisible = false">取消</a-button>
      </a-space>
    </a-modal>

    <!-- 弹窗：效果图放大预览（编辑弹窗 / 子 Mod 弹窗共用） -->
    <a-modal v-model:open="previewModalVisible" :footer="null" :title="previewModalTitle" width="min(720px, 92vw)" centered>
      <div class="preview-wrap">
        <img v-if="previewModalSrc" :src="previewModalSrc" :alt="previewModalTitle" decoding="async" class="preview-img" @error="e => e.target.style.display = 'none'" />
        <a-button v-if="previewModalImages.length > 1" class="preview-nav prev" shape="circle" @click="previewModalNav(-1)"><LeftOutlined /></a-button>
        <a-button v-if="previewModalImages.length > 1" class="preview-nav next" shape="circle" @click="previewModalNav(1)"><RightOutlined /></a-button>
      </div>
      <div v-if="previewModalImages.length > 1" class="preview-count">{{ previewModalIndex + 1 }} / {{ previewModalImages.length }}</div>
    </a-modal>
  </a-config-provider>
</template>

<script setup>
import { ref, computed, provide, watch, onMounted, onUnmounted } from 'vue'
import { message } from 'ant-design-vue'
import refreshImg from './assets/images/refresh.png'
import refreshListImg from './assets/images/refresh-list.png'
import installImg from './assets/images/install.png'
import installHdrImg from './assets/images/install-hdr.png'
import conflictImg from './assets/images/conflict.png'
import cardToolImg from './assets/images/card-tool.png'
import { ReloadOutlined, FolderOpenOutlined, FileImageOutlined, CopyOutlined, LoadingOutlined, CheckCircleOutlined, CaretRightOutlined, PictureOutlined, CloseOutlined, FileTextOutlined, LeftOutlined, RightOutlined, StarOutlined } from '@ant-design/icons-vue'

import { SelectImageFile, SetModCover, SetModNickname, CheckModEngine, InstallModEngine, GetEnginePath, OpenDirectory, GetArmorParts, GetWeaponParts, SetModParts, SetSubModParts, GenerateSubModModJson, RemoveModRecord, UninstallMod, LaunchGame, StopGame, SelectDirectory, ImportMod, GetModConfig, AddModPreview, RemoveModPreview, RefreshModPreviews, GetSubModPreviews, AddSubModPreview, RemoveSubModPreview, SetSubModCover, InstallHdrMod, IsGameRunning } from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'
import AppSider from './components/AppSider.vue'
import ModsPage from './components/ModsPage.vue'
import ModLibrary from './components/ModLibrary.vue'
import SettingsPage from './components/SettingsPage.vue'
import AuthorTool from './components/AuthorTool.vue'
import LogPanel from './components/LogPanel.vue'
import ModCard from './components/ModCard.vue'

import { useLogger } from './composables/useLogger.js'
import { useModData } from './composables/useModData.js'
import { useModOperations } from './composables/useModOperations.js'

// ---- 主题（暗色科技风） ----
const themeConfig = {
  token: {
    colorPrimary: '#1a73e8', colorInfo: '#1a73e8', colorLink: '#1a73e8',
    borderRadius: 6,
    fontFamily: "'Segoe UI', 'Microsoft YaHei', 'PingFang SC', sans-serif",
  },
}

// ---- 导航 ----
const currentPage = ref('mods')
const navItems = [
  { key: 'mods', label: 'Mod 管理' },
  { key: 'library', label: 'Mod 库' },
  { key: 'settings', label: '设置' },
]
const currentPageTitle = computed(() => navItems.find(i => i.key === currentPage.value)?.label ?? '')

// ---- 日志（provide 给子组件） ----
const { logs, logBody, addLog, clearLogs, refreshLogs } = useLogger()
provide('addLog', addLog)
provide('logs', logs)
provide('logBody', logBody)
provide('clearLogs', clearLogs)

// ---- 数据 ----
const { settingsForm, mods, needSetup, needModsRepoSetup } = useModData(addLog)

// ---- Mod 引擎检测 ----
const engineStatus = ref(null)
const engineModalVisible = ref(false)
const engineInstalling = ref(false)
const engineZipPath = ref('')
async function loadEngineZipPath() {
  try { engineZipPath.value = await GetEnginePath() } catch (e) { addLog(`获取引擎包路径失败: ${e}`) }
}
async function copyEnginePath() {
  try { await navigator.clipboard.writeText(engineZipPath.value); addLog('引擎包路径已复制') }
  catch (e) { addLog(`复制失败: ${e}`) }
}
async function openEngineZipFolder() {
  const idx = engineZipPath.value.lastIndexOf('\\')
  const dir = idx === -1 ? engineZipPath.value : engineZipPath.value.substring(0, idx)
  try { await OpenDirectory(dir) } catch (e) { addLog(`打开文件夹失败: ${e}`) }
}
async function checkEngine() {
  try {
    engineStatus.value = await CheckModEngine()
    if (engineStatus.value && !engineStatus.value.present) {
      await loadEngineZipPath()
      engineModalVisible.value = true
    }
  } catch (e) { addLog(`引擎检测失败: ${e}`) }
}
async function openEngineModal() {
  await loadEngineZipPath()
  engineModalVisible.value = true
}
async function installEngine() {
  engineInstalling.value = true
  try {
    await InstallModEngine()
    engineModalVisible.value = false
    await checkEngine()
  } catch (_) {
    await refreshLogs()
  } finally {
    engineInstalling.value = false
  }
}
watch(() => settingsForm.gameRoot, () => { if (settingsForm.gameRoot) checkEngine() })
checkEngine()

// ---- 操作 ----
const {
  searchGame, confirmGameRoot, selectGameDir,
  confirmModsRepo, selectModsDir, refreshMods, loadMods,
  toggleMod, toggleModRefresh, toggleSubMod, toggleSubModRefresh, refreshGame,
  saveAllSettings, saveModsRepo,
  detectingGameDir, detectedGameRoot, detectedGameDirError, gameDirConfirmed,
  detectGameRootForConfirm, pickGameDirManual, installModRecord,
  conflicts, loadConflicts,
} = useModOperations(settingsForm, mods, needSetup, needModsRepoSetup, addLog, refreshLogs)

// 启动后加载一次冲突检测结果（数据文件已在后端加载）
onMounted(() => { loadConflicts() })

// ---- 游戏运行状态（用于禁用"刷新游戏 Mod"） ----
const gameRunning = ref(false)
let gamePollTimer = null
async function pollGameRunning() {
  try { gameRunning.value = await IsGameRunning() } catch (_) {}
}
pollGameRunning()
gamePollTimer = setInterval(pollGameRunning, 3000)
onUnmounted(() => { if (gamePollTimer) clearInterval(gamePollTimer) })

// ---- 冲突解决 ----
const conflictVisible = ref(false)
const cardToolVisible = ref(false)
const conflictCount = computed(() => (conflicts?.value?.length || 0))
const conflictNameToMod = computed(() => {
  const map = {}
  for (const m of mods.value || []) map[m.name] = m
  return map
})
function nickOf(name) {
  const m = conflictNameToMod.value[name]
  return m?.nickname || name
}

/** 组合包中实际占用冲突资源（slot=value）的已启用子 Mod 列表 */
function implicatedSubs(mod, edges) {
  if (!mod.submods || !mod.submods.length) return []
  const res = []
  for (const e of edges) {
    if (e.a !== mod.name && e.b !== mod.name) continue
    for (const sub of mod.submods) {
      if (!sub.enabled) continue
      const vals = sub.parts && sub.parts[e.slot]
      if (vals && vals.indexOf(e.value) !== -1 && res.indexOf(sub) === -1) res.push(sub)
    }
  }
  return res
}
const conflictGroups = computed(() => {
  // 全部使用 Map，避免 Mod 名为 constructor/prototype 等原型属性名时污染冲突
  const parent = new Map()
  const find = (x) => {
    if (!parent.has(x)) return x
    let r = x
    while (parent.get(r) !== r) r = parent.get(r)
    let c = x
    while (parent.get(c) !== r) { const next = parent.get(c); parent.set(c, r); c = next }
    return r
  }
  const ensure = (name) => { if (!parent.has(name)) parent.set(name, name) }
  const list = Array.isArray(conflicts.value) ? conflicts.value : []
  for (const info of list) {
    if (!info || typeof info.modName !== 'string' || !info.modName) continue
    ensure(info.modName)
    for (const c of Array.isArray(info.conflicts) ? info.conflicts : []) {
      if (!c || typeof c.modName !== 'string' || !c.modName) continue
      ensure(c.modName)
      parent.set(find(info.modName), find(c.modName))
    }
  }
  const groups = new Map()
  const order = []
  for (const name of parent.keys()) {
    const r = find(name)
    if (!groups.has(r)) { groups.set(r, { mods: [], edges: [] }); order.push(r) }
    groups.get(r).mods.push(name)
  }
  const seen = new Set()
  const edges = []
  for (const info of list) {
    if (!info || typeof info.modName !== 'string' || !info.modName) continue
    for (const c of Array.isArray(info.conflicts) ? info.conflicts : []) {
      if (!c || typeof c.modName !== 'string' || !c.modName) continue
      const key = [info.modName, c.modName].sort().join('\u0000') + '|' + (c.slot || '') + '|' + (c.value || '')
      if (seen.has(key)) continue
      seen.add(key)
      edges.push({ a: info.modName, b: c.modName, slot: c.slot, value: c.value })
    }
  }
  for (const e of edges) {
    const g = groups.get(find(e.a))
    if (g) g.edges.push(e)
  }
  return order.map(r => {
    const g = groups.get(r)
    return {
      mods: g.mods.map(name => {
        const mod = name != null ? conflictNameToMod.value[name] : null
        if (!mod) return null
        return { mod, subs: implicatedSubs(mod, g.edges) }
      }).filter(Boolean),
      edges: g.edges,
    }
  })
})

// 打开 Mod 托管目录弹窗时自动核对游戏目录（带加载动画）
watch(() => needModsRepoSetup.value && !needSetup.value, (open) => { if (open) detectGameRootForConfirm() }, { immediate: true })

// ---- 启动 / 停止游戏（依据游戏运行状态切换） ----
async function launchGame() {
  if (gameRunning.value) {
    try { await StopGame() } catch (e) { addLog(`停止游戏失败: ${e}`) }
  } else {
    try { await LaunchGame() } catch (e) { addLog(`启动游戏失败: ${e}`) }
  }
  await pollGameRunning()
  await refreshLogs()
}

// ---- 安装 HDR Mod（顶部工具栏） ----
const installingHdr = ref(false)
async function installHdr() {
  let dir
  try { dir = await SelectDirectory() } catch (e) { addLog(`选择目录失败: ${e}`); return }
  if (!dir) return
  installingHdr.value = true
  beginImportProgress()
  try {
    const res = await InstallHdrMod(dir)
    message.success(`已安装 HDR 合集「${res.nickname || res.name}」（${res.submods?.length || 0} 个子 Mod）`)
    await refreshMods()
  } catch (e) {
    message.error(String(e))
  } finally {
    endImportProgress()
    installingHdr.value = false
  }
}

// ---- 自动安装 Mod ----
const importProgressVisible = ref(false)
const importStep = ref(0)
const importPercent = ref(0)
const importMessage = ref('')

// 监听后端导入进度事件
EventsOn('modInstallProgress', (p) => {
  if (!p) return
  importMessage.value = p.message || ''
  importPercent.value = p.percent || 0
  importStep.value = Math.max(0, Math.min((p.step || 0) - 1, 2))
})

// 供子组件（HDR 一键安装等）打开/关闭"正在安装 Mod"进度弹窗
function beginImportProgress() {
  importProgressVisible.value = true
  importStep.value = 0
  importPercent.value = 0
  importMessage.value = '正在识别并准备安装…'
}
function endImportProgress() {
  importProgressVisible.value = false
}

async function installMod() {
  let folder
  try { folder = await SelectDirectory() } catch (e) { addLog(`选择目录失败: ${e}`); return }
  if (!folder) return
  let res
  importProgressVisible.value = true
  importStep.value = 0
  importPercent.value = 0
  importMessage.value = '正在准备导入…'
  try { res = await ImportMod(folder) } catch (e) {
    importProgressVisible.value = false
    addLog(`导入失败: ${e}`)
    return
  }
  importProgressVisible.value = false
  // 导入已登记进数据文件，直接读取展示
  await loadMods()
  const isComposite = (res.submods || []).length > 0
  const hasParts = res.configFound && Object.keys(res.parts || {}).length > 0
  if (isComposite) {
    // 组合包（HDR 合集）：ImportMod 已按子 Mod 登记，直接标记安装完成（与"安装 HDR Mod"行为一致）
    try {
      await installModRecord({ name: res.name }, res.parts)
      if (res.nickname) await SetModNickname(res.name, res.nickname)
      message.success(`已识别为组合包（HDR 合集），共 ${res.submods.length} 个子 Mod，安装完成`)
      addLog(`已自动安装 HDR 合集: ${res.name}`)
    } catch (e) { addLog(`自动安装失败: ${e}`) }
  } else if (hasParts) {
    // 有 mod.json 且能解析出有效部位 → 自动补全安装
    try {
      await installModRecord({ name: res.name }, res.parts)
      if (res.nickname) await SetModNickname(res.name, res.nickname)
      message.success(`已检测到 mod.json，已自动补全占用装备资源并完成安装`)
      addLog(`已自动安装（读取 mod.json）: ${res.name}`)
    } catch (e) { addLog(`自动安装失败: ${e}`) }
  } else {
    // 没有 mod.json 或解析不到有效部位 → 打开弹窗手动输入（已解析到的部位预填）
    addLog(res.configFound ? 'mod.json 无有效部位数据，请手动补充' : '未检测到 mod.json，请手动填写占用装备资源')
    openInstallMod({ name: res.name, parts: res.parts || {}, nickname: res.nickname || '', submods: res.submods || [] })
  }
}

// ---- Mod 安装弹窗 ----
const installModVisible = ref(false)
const installingMod = ref(null)
const installParts = ref({})
const installCover = ref('')
const pendingNickname = ref('')
const installPrompt = ref({ type: 'info', text: '' })
const installRecognizing = ref(false)

/** 打开安装弹窗：尝试读取 mod.json，有则自动补全占用装备资源并提示，无则提示手动填写 */
async function openInstallMod(mod) {
  installingMod.value = mod
  // 组合包（HDR 等）：父级 Parts 在卸载时被清空，但各子 Mod 的占用数据仍在数据文件中，
  // 用子 Mod 部位并集预填，避免重新安装时弹窗空白需手动重填
  installParts.value = normalizeParts({ ...unionSubModParts(mod.submods), ...(mod.parts || {}) })
  installCover.value = mod.cover || ''
  pendingNickname.value = mod.nickname || ''
  installPrompt.value = { type: 'info', text: '' }
  installModVisible.value = true
  // 后端识别期间显示加载态（按钮禁用 + 加载提示），避免无反应
  installRecognizing.value = true
  try {
    const cfg = await GetModConfig(mod.name)
    if (cfg && cfg.configFound) {
      if (cfg.nickname && !pendingNickname.value) pendingNickname.value = cfg.nickname
      if (cfg.cover && !installCover.value) installCover.value = cfg.cover
      installParts.value = normalizeParts({ ...unionSubModParts(cfg.submods), ...(cfg.parts || {}), ...unionSubModParts(mod.submods), ...(mod.parts || {}) })
      // 组合包（HDR 合集）：用后端识别出的子 Mod 补全弹窗，显示组合包形式而非单个 Mod 表单
      if (cfg.submods && cfg.submods.length) {
        installingMod.value = { ...installingMod.value, name: mod.name, submods: cfg.submods }
      }
      const hasAny = Object.keys(installParts.value).length > 0
      installPrompt.value = hasAny
        ? { type: 'success', text: '已检测到 mod.json，以下占用装备资源已自动补全，可确认或修改' }
        : { type: 'warning', text: '已检测到 mod.json，但未解析到有效部位数据，请手动填写' }
    } else {
      installPrompt.value = { type: 'info', text: '未检测到 mod.json，请手动填写该 Mod 占用的装备资源' }
    }
  } catch (_) {
    installPrompt.value = { type: 'info', text: '未检测到 mod.json，请手动填写该 Mod 占用的装备资源' }
  } finally {
    installRecognizing.value = false
  }
}
async function installModConfirm() {
  if (!installingMod.value) return
  try {
    // installModRecord 内部已读取数据文件刷新列表
    await installModRecord(installingMod.value, cleanParts(installParts.value))
    if (installCover.value && !installingMod.value.cover) {
      const saved = await SetModCover(installingMod.value.name, installCover.value)
      installingMod.value.cover = saved
      installCover.value = saved
    }
    if (pendingNickname.value) await SetModNickname(installingMod.value.name, pendingNickname.value)
    installModVisible.value = false
  } catch (e) { addLog(`安装失败: ${e}`) }
}

// ---- Mod 卸载（回到未安装，保留文件夹与记录） ----
async function uninstallMod(mod) {
  try {
    await UninstallMod(mod.name)
    await loadMods()
    await refreshLogs()
  } catch (e) { addLog(`卸载失败: ${e}`) }
}

// ---- Mod 记录清理（磁盘文件已缺失时清除记录） ----
async function removeRecord(mod) {
  try {
    await RemoveModRecord(mod.name)
    const idx = mods.value.findIndex(m => m.name === mod.name)
    if (idx !== -1) mods.value.splice(idx, 1)
    await refreshLogs()
  } catch (e) { addLog(`清除记录失败: ${e}`) }
}

// ---- Mod 编辑弹窗 ----
const editModVisible = ref(false)
const editingMod = ref(null)
const editNickname = ref('')
const editCover = ref('')
const editPreviews = ref([])
const armorSlots = ref([])
const weaponSlots = ref([])
const editParts = ref({})

// 全部可选槽位：服装部位 + 武器（可同时占用）
const allSlots = computed(() => [...armorSlots.value, ...weaponSlots.value])

/** 根据占用的资源推导分类：mixed=服装+武器 / weapon=仅武器 / armor=仅服装 */
function deriveCategory(parts) {
  let armor = false, weapon = false
  for (const k of Object.keys(parts || {})) { if (k === '武器') weapon = true; else armor = true }
  if (weapon && armor) return 'mixed'
  if (weapon) return 'weapon'
  return 'armor'
}

async function loadArmorSlots() {
  try {
    const slots = await GetArmorParts()
    armorSlots.value = slots || []
  } catch (e) { addLog(`加载装备资源失败: ${e}`) }
}
loadArmorSlots()

async function loadWeaponSlots() {
  try {
    const parts = await GetWeaponParts()
    // 武器无部位分组，统一归入"武器"槽位
    weaponSlots.value = [{ name: '武器', parts: parts || [] }]
  } catch (e) { addLog(`加载武器资源失败: ${e}`) }
}
loadWeaponSlots()

/** 把后端多值占用归一化为编辑弹窗表单：服装部位取单个值，武器部位为数组（可多选） */
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

/** 组合 Mod 各子 Mod 占用部位的并集（父级预填/冲突展示用） */
function unionSubModParts(submods) {
  const union = {}
  for (const sm of submods || []) {
    for (const [slot, vals] of Object.entries(sm.parts || {})) {
      const list = Array.isArray(vals) ? vals : (vals ? [vals] : [])
      for (const v of list) {
        const s = typeof v === 'string' ? v.trim() : ''
        if (!s) continue
        if (slot === '武器') {
          const arr = union[slot] || (union[slot] = [])
          if (!arr.includes(s)) arr.push(s)
        } else if (!union[slot]) {
          union[slot] = s
        }
      }
    }
  }
  return union
}

/** 过滤占用资源，仅保留有效槽位（服装部位 + 武器），输出为 部位 -> 数组（后端格式）。
 *  武器部位可含多个值，服装部位每个一个值。 */
function cleanParts(parts) {
  const keys = new Set(allSlots.value.map(s => s.name))
  const cleaned = {}
  for (const [slot, v] of Object.entries(parts || {})) {
    if (!keys.has(slot)) continue
    if (slot === '武器') {
      const list = (Array.isArray(v) ? v : [v]).map(x => (typeof x === 'string' ? x.trim() : '')).filter(Boolean)
      if (list.length) cleaned[slot] = list
    } else {
      const s = (typeof v === 'string' ? v : Array.isArray(v) ? v[0] : '').trim()
      if (s) cleaned[slot] = [s]
    }
  }
  return cleaned
}

// 编辑弹窗封面缩略图：相对文件名走 /modfile（随 mod 文件夹），绝对路径走 /localfile
// dim 指定时请求服务端缩略图，小尺寸显示用；全屏预览缺省取原图
function editCoverUrl(dim) {
  const cover = editCover.value
  if (!cover) return ''
  if (/^[a-zA-Z]:[\\/]/.test(cover) || cover.startsWith('\\\\')) {
    const u = '/localfile?file=' + encodeURIComponent(cover)
    return dim ? u + '&w=' + dim : u
  }
  const u = '/modfile?mod=' + encodeURIComponent(editingMod.value?.name || '') + '&file=' + encodeURIComponent(cover)
  return dim ? u + '&w=' + dim : u
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

function openEditMod(mod) {
  editingMod.value = mod
  editNickname.value = mod.nickname || ''
  editCover.value = mod.cover || ''
  editPreviews.value = (Array.isArray(mod.previews) ? mod.previews : []).slice()
  editParts.value = normalizeParts(mod.parts)
  subEditVisible.value = false
  editModVisible.value = true
}
// 组合包父级不占用：隐藏"占用装备资源"选择器
const isEditingComposite = computed(() => !!(editingMod.value?.submods && editingMod.value.submods.length))
// 编辑弹窗内子 Mod 卡片的展示视图（与外部 ModCard 结构一致）
function subModView(sub) {
  return {
    name: sub.name,
    nickname: sub.name,
    parts: sub.parts || {},
    enabled: sub.enabled,
    cover: sub.cover || '',
    previews: sub.previews || [],
    category: editingMod.value?.category || 'armor',
    submods: [],
  }
}
// 安装弹窗内组合包的子 Mod 列表
const installingSubMods = computed(() => (installingMod.value?.submods || []).filter(Boolean))
// 安装弹窗内子 Mod 卡片的展示视图（与编辑弹窗一致）
function installSubModView(sub) {
  const mod = installingMod.value || {}
  return {
    name: sub.name,
    nickname: sub.nickname || sub.name,
    parts: sub.parts || {},
    enabled: sub.enabled,
    cover: sub.cover || '',
    previews: sub.previews || [],
    category: mod.category || 'armor',
    submods: [],
  }
}
// 手动填写/修改子 Mod 占用：打开子 Mod 占用编辑弹窗
const editingSub = ref(null)
const subEditVisible = ref(false)
const editSubParts = ref({})
const editSubPreviews = ref([])
const editSubCover = ref('')
const savingSubParts = ref(false)
function openSubEdit(sub) {
  editingSub.value = sub
  editSubParts.value = normalizeParts(sub.parts)
  editSubPreviews.value = (Array.isArray(sub.previews) ? sub.previews : []).slice()
  editSubCover.value = sub.cover || ''
  subEditVisible.value = true
}
async function saveSubParts() {
  const mod = editingMod.value
  const sub = editingSub.value
  if (!mod || !sub) return
  savingSubParts.value = true
  try {
    const cleaned = cleanParts(editSubParts.value)
    await SetSubModParts(mod.name, sub.name, cleaned)
    sub.parts = cleaned
    const union = {}
    for (const sm of mod.submods) for (const [k, v] of Object.entries(sm.parts || {})) {
      const list = Array.isArray(v) ? v : (v ? [v] : [])
      union[k] = (union[k] || []).concat(list.filter(Boolean))
    }
    mod.parts = union
    mod.category = deriveCategory(union)
    subEditVisible.value = false
    message.success(`已更新子 Mod「${sub.name}」占用资源`)
    await refreshLogs()
  } catch (e) {
    message.error(String(e))
  } finally {
    savingSubParts.value = false
  }
}
// 手动生成/刷新合集 mod.json：将当前手动填写的全部子 Mod 占用写入合集目录
const generatingModJson = ref(false)
async function generateModJson() {
  const mod = editingMod.value
  if (!mod) return
  generatingModJson.value = true
  try {
    await GenerateSubModModJson(mod.name)
    message.success('已生成/更新 mod.json')
    await refreshLogs()
  } catch (e) {
    message.error(String(e))
  } finally {
    generatingModJson.value = false
  }
}
async function selectModCover() { try { const file = await SelectImageFile(); if (file) editCover.value = file } catch (_) {} }
// 将某张效果图设为 Mod 封面
async function setCover(p) {
  const mod = editingMod.value
  if (!mod) return
  try {
    const saved = await SetModCover(mod.name, p)
    editCover.value = saved || p
    mod.cover = saved || p
    message.success('已设为封面')
  } catch (e) { message.error(String(e)) }
}
// 子 Mod 封面缩略图：相对父 Mod 目录路径 → /modfile（owner 为父 Mod）
function subCoverUrl(dim) {
  const cover = editSubCover.value
  if (!cover) return ''
  if (/^[a-zA-Z]:[\\/]/.test(cover) || cover.startsWith('\\\\')) {
    const u = '/localfile?file=' + encodeURIComponent(cover)
    return dim ? u + '&w=' + dim : u
  }
  const u = '/modfile?mod=' + encodeURIComponent(editingMod.value?.name || '') + '&file=' + encodeURIComponent(cover)
  return dim ? u + '&w=' + dim : u
}
// 选择本地图片作为子 Mod 封面：复制进子 Mod 目录后设为封面
async function selectSubCover() {
  const mod = editingMod.value, sub = editingSub.value
  if (!mod || !sub) return
  try {
    const file = await SelectImageFile()
    if (!file) return
    const rel = await AddSubModPreview(mod.name, sub.name, file)
    await SetSubModCover(mod.name, sub.name, rel)
    editSubCover.value = rel
    sub.cover = rel
    if (!editSubPreviews.value.includes(rel)) {
      editSubPreviews.value.push(rel)
      sub.previews = editSubPreviews.value.slice()
    }
    message.success('已设置子 Mod 封面')
  } catch (e) { message.error(String(e)) }
}
// 将某张效果图设为子 Mod 封面
async function setSubCover(p) {
  const mod = editingMod.value, sub = editingSub.value
  if (!mod || !sub) return
  try {
    await SetSubModCover(mod.name, sub.name, p)
    editSubCover.value = p
    sub.cover = p
    message.success('已设为封面')
  } catch (e) { message.error(String(e)) }
}
// 效果图：相对文件名走 /modfile（随 Mod 文件夹），绝对路径走 /localfile
function editPreviewUrl(p, dim) {
  if (!p) return ''
  if (/^[a-zA-Z]:[\\/]/.test(p) || p.startsWith('\\\\')) {
    const u = '/localfile?file=' + encodeURIComponent(p)
    return dim ? u + '&w=' + dim : u
  }
  const u = '/modfile?mod=' + encodeURIComponent(editingMod.value?.name || '') + '&file=' + encodeURIComponent(p)
  return dim ? u + '&w=' + dim : u
}
function subPreviewUrl(p, dim) {
  if (!p) return ''
  if (/^[a-zA-Z]:[\\/]/.test(p) || p.startsWith('\\\\')) {
    const u = '/localfile?file=' + encodeURIComponent(p)
    return dim ? u + '&w=' + dim : u
  }
  const u = '/modfile?mod=' + encodeURIComponent(editingMod.value?.name || '') + '&file=' + encodeURIComponent(p)
  return dim ? u + '&w=' + dim : u
}
async function selectModPreview() {
  const mod = editingMod.value
  if (!mod) return
  try {
    const file = await SelectImageFile()
    if (!file) return
    const saved = await AddModPreview(mod.name, file)
    if (saved && !editPreviews.value.includes(saved)) {
      editPreviews.value.push(saved)
      mod.previews = editPreviews.value.slice()
    }
    message.success('已添加效果图')
  } catch (e) { message.error(String(e)) }
}
async function removeModPreview(file) {
  const mod = editingMod.value
  if (!mod) return
  try {
    await RemoveModPreview(mod.name, file)
    editPreviews.value = editPreviews.value.filter(p => p !== file)
    mod.previews = editPreviews.value.slice()
  } catch (e) { message.error(String(e)) }
}
async function scanModPreviews() {
  const mod = editingMod.value
  if (!mod) return
  try {
    const list = await RefreshModPreviews(mod.name)
    editPreviews.value = (Array.isArray(list) ? list : []).slice()
    mod.previews = editPreviews.value.slice()
    message.success('已自动扫描效果图')
  } catch (e) { message.error(String(e)) }
}
async function addSubPreview() {
  const mod = editingMod.value, sub = editingSub.value
  if (!mod || !sub) return
  try {
    const file = await SelectImageFile()
    if (!file) return
    const saved = await AddSubModPreview(mod.name, sub.name, file)
    if (saved && !editSubPreviews.value.includes(saved)) {
      editSubPreviews.value.push(saved)
      sub.previews = editSubPreviews.value.slice()
    }
    message.success('已添加子 Mod 效果图')
  } catch (e) { message.error(String(e)) }
}
async function removeSubPreview(file) {
  const mod = editingMod.value, sub = editingSub.value
  if (!mod || !sub) return
  try {
    await RemoveSubModPreview(mod.name, sub.name, file)
    editSubPreviews.value = editSubPreviews.value.filter(p => p !== file)
    sub.previews = editSubPreviews.value.slice()
  } catch (e) { message.error(String(e)) }
}
async function scanSubPreviews() {
  const mod = editingMod.value, sub = editingSub.value
  if (!mod || !sub) return
  try {
    await RefreshModPreviews(mod.name)
    const subs = await GetSubModPreviews(mod.name, sub.name)
    editSubPreviews.value = (Array.isArray(subs) ? subs : []).slice()
    sub.previews = editSubPreviews.value.slice()
    message.success('已自动扫描子 Mod 效果图')
  } catch (e) { message.error(String(e)) }
}
// 效果图放大预览（编辑弹窗 / 子 Mod 弹窗共用）
const previewModalVisible = ref(false)
const previewModalImages = ref([])
const previewModalIndex = ref(0)
const previewModalTitle = ref('效果图预览')
const previewModalSrc = computed(() => previewModalImages.value[previewModalIndex.value] || '')
function openEditPreview(i) {
  previewModalImages.value = editPreviews.value.map(p => editPreviewUrl(p)).filter(Boolean)
  previewModalIndex.value = Math.min(i, previewModalImages.value.length - 1)
  previewModalTitle.value = `${editingMod.value?.nickname || editingMod.value?.name || ''} 效果图`
  previewModalVisible.value = true
}
function openSubPreview(i) {
  previewModalImages.value = editSubPreviews.value.map(p => subPreviewUrl(p)).filter(Boolean)
  previewModalIndex.value = Math.min(i, previewModalImages.value.length - 1)
  previewModalTitle.value = `${editingSub.value?.name || ''} 效果图`
  previewModalVisible.value = true
}
function previewModalNav(d) {
  const n = previewModalImages.value.length
  if (n) previewModalIndex.value = (previewModalIndex.value + d + n) % n
}
async function saveModEdit() {
  if (!editingMod.value) return
  const mod = editingMod.value
  if (editNickname.value !== (mod.nickname || '')) { await SetModNickname(mod.name, editNickname.value || mod.name); mod.nickname = editNickname.value }
  if (editCover.value !== (mod.cover || '')) {
    const saved = await SetModCover(mod.name, editCover.value)
    mod.cover = saved
    editCover.value = saved
  }
  const cleaned = cleanParts(editParts.value)
  const prev = mod.parts || {}
  const changed = JSON.stringify(cleaned) !== JSON.stringify(prev)
  if (changed) {
    await SetModParts(mod.name, cleaned)
    mod.parts = cleaned
    mod.category = deriveCategory(cleaned)
  }
  editModVisible.value = false
  await refreshLogs()
}
</script>

<style>html, body { background: #f5f5f5; margin:0; padding:0; }</style>

<style scoped>
.app-layout { height: 100vh; overflow: hidden; font-family: 'Segoe UI','Microsoft YaHei','PingFang SC',sans-serif; background: #f5f5f5; }
.main-layout { background: #f5f5f5 !important; }
.app-header { display: flex; align-items: center; justify-content: space-between; height: 56px !important; line-height: 56px !important; padding: 0 16px !important; background: #fff !important; border-bottom: 1px solid #f0f0f0; }
.page-title { margin: 0; font-size: 17px; font-weight: 600; color: #333; flex-shrink: 0; }
.header-left { display: flex; align-items: center; gap: 24px; min-width: 0; overflow: hidden; }
.header-right { display: flex; align-items: center; gap: 16px; }
.header-actions { display: flex; align-items: center; white-space: nowrap; }
.header-icon { display: inline-flex; align-items: center; justify-content: center; width: 28px; height: 28px; border-radius: 6px; color: #555; font-size: 16px; cursor: pointer; vertical-align: middle; transition: color .15s, background .15s; }
.header-icon:hover { color: #1a73e8; background: #e8f1fe; }
.header-icon.is-loading { color: #1a73e8; }
.header-img-btn { display: block; width: 44px; height: 44px; padding: 7px; box-sizing: border-box; object-fit: contain; background: #f5f5f5; border-radius: 9px; cursor: pointer; transition: transform .15s, background .15s, box-shadow .15s; }
.header-img-btn:hover { transform: scale(1.08); background: #e8f1fe; box-shadow: 0 0 0 2px rgba(26,115,232,.18); }
.header-img-btn.is-loading { opacity: .5; }
.header-img-btn.is-disabled { opacity: .4; filter: grayscale(1); cursor: not-allowed; pointer-events: none; }
.launch-btn { flex-shrink: 0; margin-left: 16px; height: 44px; padding: 0 20px; font-size: 16px; display: inline-flex; align-items: center; gap: 6px; }
.conflict-img-wrap { position: relative; cursor: pointer; }
.conflict-badge { position: absolute; top: -4px; right: -4px; min-width: 20px; height: 20px; line-height: 20px; padding: 0 6px; box-sizing: border-box; border-radius: 11px; background: #f5222d; color: #fff; font-size: 12px; font-weight: 600; text-align: center; box-shadow: 0 1px 3px rgba(0,0,0,.3); z-index: 2; }
.conflict-badge.no-conflict { background: #9ca3af; min-width: 20px; height: 20px; line-height: 20px; font-size: 12px; }
.conflict-badge.has-conflict { background: #f5222d; min-width: 24px; height: 24px; line-height: 24px; padding: 0 7px; border-radius: 12px; font-size: 14px; animation: conflict-pulse 1.6s ease-in-out infinite; }
@keyframes conflict-pulse { 0%, 100% { box-shadow: 0 1px 3px rgba(0,0,0,.3); } 50% { box-shadow: 0 1px 8px rgba(245,34,45,.75); } }
.conflict-group-list { display: flex; flex-direction: column; gap: 12px; }
.conflict-group-card { background: #fff7f0; border: 1px solid #ffd8bf; border-radius: 8px; }
.conflict-group-card :deep(.ant-card-head) { border-bottom: 1px dashed #ffd8bf; }
.conflict-group-title { color: #d4380d; font-weight: 600; }
.conflict-group-mods { display: flex; align-items: center; flex-wrap: wrap; gap: 4px 14px; line-height: 2; }
.conflict-group-label { color: #888; font-size: 12px; }
.conflict-group-mod { display: inline-flex; align-items: center; }
.conflict-group-mod b { color: #333; }
.conflict-mod-title { color: #d4380d; font-weight: 600; margin-right: 4px; }
.conflict-sub-mod { display: inline-flex; align-items: center; gap: 4px; margin-left: 12px; color: #555; font-size: 13px; }
.conflict-group-edges { margin-top: 6px; padding-top: 6px; border-top: 1px dashed #ffd8bf; }
.conflict-edge { line-height: 1.9; color: #d4380d; font-size: 13px; }
.launch-btn { flex-shrink: 0; margin-left: 16px; height: 40px; padding: 0 18px; font-size: 15px; display: inline-flex; align-items: center; gap: 6px; }
.app-content { flex: 1; overflow-y: auto; background: #f5f5f5; }
.app-content > :deep(.page) { padding: 20px 24px; animation: fadeIn .2s ease; }
.engine-mode { width: 100%; padding: 10px 12px; border: 1px solid #d9d9d9; border-radius: 8px; background: #fff; }
.engine-mode-title { font-size: 13px; font-weight: 600; color: #333; margin-bottom: 4px; }
.engine-mode-manual { border-style: dashed; }
.parts-grid { display: grid; grid-template-columns: 1fr 1fr; gap: 8px 12px; }
.edit-submods { display: grid; grid-template-columns: repeat(auto-fill, minmax(180px, 1fr)); grid-auto-rows: 1fr; gap: 12px; }
.edit-cover-box { width: 80px; height: 80px; border: 1px dashed #d9d9d9; border-radius: 8px; display: flex; align-items: center; justify-content: center; overflow: hidden; cursor: pointer; background: #fafafa; }
.edit-cover-box:hover { border-color: #1a73e8; }
.edit-cover-box img { width: 100%; height: 100%; object-fit: cover; }
.edit-cover-placeholder { display: flex; flex-direction: column; align-items: center; gap: 4px; color: #999; font-size: 11px; }
.edit-cover-placeholder .anticon { font-size: 20px !important; }
.edit-cover-path { display: flex; align-items: center; gap: 4px; font-size: 12px; color: #888; max-width: 100%; }
.edit-cover-path-text { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.preview-list { display: flex; flex-wrap: wrap; gap: 8px; }
.preview-item { position: relative; width: 72px; height: 72px; border-radius: 8px; overflow: hidden; border: 1px solid #d9d9d9; cursor: zoom-in; }
.preview-item:hover { border-color: #1a73e8; }
.preview-item img { width: 100%; height: 100%; object-fit: cover; display: block; }
.preview-item-cover { position: absolute; bottom: 3px; left: 3px; color: #fff; background: rgba(0,0,0,.5); width: 18px; height: 18px; display: flex; align-items: center; justify-content: center; border-radius: 50%; font-size: 10px; cursor: pointer; z-index: 1; }
.preview-item-cover:hover { background: #faad14; }
.preview-item-cover.active { background: #faad14; color: #fff; }
/* X 删除按钮：深色半透明小圆底，白色 X，悬浮右上角，hover 高亮 */
.preview-item-remove { position: absolute; top: 3px; right: 3px; width: 18px; height: 18px; display: flex; align-items: center; justify-content: center; background: rgba(0,0,0,.5); color: #fff; border-radius: 50%; font-size: 10px; cursor: pointer; box-shadow: 0 1px 3px rgba(0,0,0,.25); z-index: 1; }
.preview-item-remove:hover { background: #f5222d; color: #fff; }
.preview-empty { width: 100%; padding: 10px 0; font-size: 12px; color: #999; border: 1px dashed #d9d9d9; border-radius: 8px; text-align: center; }
.sub-cover-row { display: flex; align-items: center; gap: 12px; margin-bottom: 12px; }
.sub-cover-tip { flex: 1; font-size: 12px; color: #888; line-height: 1.6; }
.preview-wrap { position: relative; }
.preview-img { width: 100%; display: block; }
.preview-nav { position: absolute; top: 50%; transform: translateY(-50%); z-index: 2; border-color: rgba(255,255,255,.2); background: rgba(0,0,0,.5); color: #fff; }
.preview-nav:hover { background: #1a73e8; color: #fff; }
.preview-nav.prev { left: 8px; }
.preview-nav.next { right: 8px; }
.preview-count { text-align: center; color: #888; font-size: 12px; margin-top: 8px; }
.category-radio { margin-bottom: 4px; }
.category-radio .ant-radio-button-wrapper { font-size: 12px; }
.parts-grid { display: grid; grid-template-columns: repeat(2, minmax(0, 1fr)); gap: 8px 16px; }
.parts-row { display: flex; align-items: center; gap: 8px; min-width: 0; }
.parts-tag { width: 44px; text-align: center; margin-inline-end: 0; flex-shrink: 0; }
.parts-select { flex: 1 1 0; min-width: 0; max-width: 100%; }
/* 弹窗分组标题：左侧加粗，下方微弱分割线 */
.form-section-title { font-size: 15px; font-weight: 600; color: #333; padding-bottom: 10px; margin: 22px 0 16px; border-bottom: 1px solid #f0f0f0; }
.section-tip { color: #888; font-size: 12px; margin-bottom: 10px; }
.form-actions { display: flex; justify-content: flex-end; width: 100%; margin-top: 26px; margin-bottom: 2px; }
/* 封面/效果图图片卡片 */
.img-card { position: relative; width: 84px; height: 84px; border-radius: 8px; overflow: hidden; border: 1px solid #d9d9d9; cursor: pointer; background: #fafafa; }
.img-card img { width: 100%; height: 100%; object-fit: cover; display: block; }
.img-card-empty { width: 100%; height: 100%; display: flex; flex-direction: column; align-items: center; justify-content: center; gap: 4px; color: #999; font-size: 12px; }
.img-card-empty .anticon { font-size: 22px; color: #999; }
.img-card-remove { position: absolute; top: 4px; right: 4px; width: 16px; height: 16px; display: flex; align-items: center; justify-content: center; background: rgba(0,0,0,.5); color: #fff; border-radius: 50%; font-size: 10px; cursor: pointer; z-index: 1; }
.img-card-remove:hover { background: #f5222d; }
.img-card-name { margin-top: 8px; font-size: 12px; color: #888; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 100%; }
/* 表单留白：标签↔输入、区块↔区块 上下间距 */
.part-form :deep(.ant-form-item) { margin-bottom: 22px; }
.part-form :deep(.ant-form-item-label) { padding-bottom: 6px; }
.part-form :deep(.ant-form-item-label > label) { font-weight: 500; color: #333; }
/* 装备下拉框统一高度（含 Weapon 多选标签输入框） */
.part-form :deep(.parts-select .ant-select-selector) { height: 32px; align-items: center; }
.part-form :deep(.ant-select-multiple .ant-select-selector) { height: 32px; align-items: center; overflow: hidden; }
.install-mod-name { font-weight: 600; color: #333; word-break: break-all; }
.detect-panel { display: flex; align-items: flex-start; gap: 10px; padding: 12px 14px; border: 1px solid #d9d9d9; border-radius: 8px; background: #fff; }
.recognize-panel { display: flex; align-items: center; gap: 10px; padding: 12px 14px; border: 1px solid #91caff; border-radius: 8px; background: #e6f4ff; color: #0958d9; font-size: 13px; }
.recognize-icon { font-size: 16px; }
.detect-loading { align-items: center; color: #888; }
.detect-icon { font-size: 20px; color: #1a73e8; }
.detect-icon-ok { color: #52c41a; }
.detect-result { flex: 1; }
.detect-result-title { font-weight: 600; color: #333; }
.detect-path { font-size: 13px; color: #888; word-break: break-all; margin-top: 4px; }
.detect-ok { border-color: #b7eb8f; background: #f6ffed; }
.fade-enter-active, .fade-leave-active { transition: opacity .25s ease, transform .25s ease; }
.fade-enter-from { opacity: 0; transform: translateY(6px); }
.fade-leave-to { opacity: 0; transform: translateY(-6px); }
@keyframes fadeIn { from{opacity:0;transform:translateY(4px)} to{opacity:1;transform:translateY(0)} }
</style>

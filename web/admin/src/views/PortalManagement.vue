<template>
  <div class="portal-management">
    <!-- Top Bar -->
    <div class="editor-topbar">
      <div class="topbar-left">
        <span class="portal-name">{{ ed.form.name || t('portal.defaultName') }}</span>
        <el-tag v-if="ed.hasUnsavedChanges" type="warning" size="small">{{ t('portal.unsaved') }}</el-tag>
        <el-tag v-else type="success" size="small">{{ t('portal.saved') }}</el-tag>
      </div>
      <div class="topbar-right">
        <el-button-group>
          <el-button :type="ed.editorMode === 'builder' ? 'primary' : 'default'" size="small" @click="ed.editorMode = 'builder'">{{ t('portal.builder') }}</el-button>
          <el-button :type="ed.editorMode === 'lang' ? 'primary' : 'default'" size="small" @click="ed.editorMode = 'lang'">{{ t('portal.lang') }}</el-button>
          <el-button :type="ed.editorMode === 'settings' ? 'primary' : 'default'" size="small" @click="ed.editorMode = 'settings'">{{ t('portal.settings') }}</el-button>
          <el-button :type="ed.editorMode === 'code' ? 'primary' : 'default'" size="small" @click="ed.editorMode = 'code'">{{ t('portal.templateEdit') }}</el-button>
        </el-button-group>
        <el-button size="small" :icon="View" @click="ed.handlePreview">{{ t('portal.preview') }}</el-button>
        <el-button type="primary" size="small" :icon="Check" :loading="ed.saving" @click="ed.handleSave">{{ t('common.save') }}</el-button>
        <el-dropdown trigger="click" @command="ed.handleCacheCommand">
          <el-button size="small">{{ t('portal.cache') }}<el-icon class="el-icon--right"><ArrowDown /></el-icon></el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="status">{{ t('portal.cacheStatus') }}{{ ed.cacheStatus.cached ? t('portal.cached') : t('portal.notCached') }}</el-dropdown-item>
              <el-dropdown-item command="rebuild" divided>{{ t('portal.rebuildCache') }}</el-dropdown-item>
              <el-dropdown-item command="clear">{{ t('portal.clearCache') }}</el-dropdown-item>
              <el-dropdown-item command="dev" divided>{{ ed.cacheStatus.dev_mode ? t('portal.switchToProduction') : t('portal.switchToDev') }}</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
    </div>

    <div v-loading="ed.loading" class="editor-body">
      <!-- Builder Mode (Three-Column) -->
      <div v-if="ed.editorMode === 'builder'" class="builder-layout">
        <BuilderStructurePanel />
        <BuilderCanvas />
        <ModulePropsPanel />
      </div>

      <!-- Lang Editor Mode -->
      <LangEditorPanel v-if="ed.editorMode === 'lang'" />

      <!-- Settings Mode -->
      <SettingsPanel v-if="ed.editorMode === 'settings'" />

      <!-- Template Code Mode -->
      <CodeModePanel v-if="ed.editorMode === 'code'" />
    </div>

    <!-- Navigation Dialog -->
    <NavDialog />
  </div>
</template>

<script setup>
import { provide } from 'vue'
import { useI18n } from 'vue-i18n'
import { View, Check, ArrowDown } from '@element-plus/icons-vue'
import { usePortalEditor } from './portal/usePortalEditor'
import BuilderStructurePanel from './portal/components/BuilderStructurePanel.vue'
import BuilderCanvas from './portal/components/BuilderCanvas.vue'
import ModulePropsPanel from './portal/components/ModulePropsPanel.vue'
import LangEditorPanel from './portal/components/LangEditorPanel.vue'
import SettingsPanel from './portal/components/SettingsPanel.vue'
import CodeModePanel from './portal/components/CodeModePanel.vue'
import NavDialog from './portal/components/NavDialog.vue'

const { t } = useI18n()

// Shared editor store — all state and actions live in the composable;
// child panels inject('portalEditor') and use ed.xxx (refs unwrap on access).
const ed = usePortalEditor()
provide('portalEditor', ed)
</script>

<style scoped>
.portal-management { height: 100%; display: flex; flex-direction: column; overflow: hidden; }

.editor-topbar {
  display: flex; justify-content: space-between; align-items: center;
  padding: 8px 16px; background: #fff; border-bottom: 1px solid #e4e7ed;
  flex-shrink: 0; box-shadow: 0 1px 4px rgba(0, 0, 0, 0.04);
}
.topbar-left { display: flex; align-items: center; gap: 8px; }
.portal-name { font-size: 16px; font-weight: 600; color: #303133; }
.topbar-right { display: flex; align-items: center; gap: 8px; }

.editor-body { flex: 1; overflow: hidden; position: relative; min-height: 0; }
.editor-body > div { height: 100%; min-height: 0; }

.builder-layout { display: flex; height: 100%; min-height: 0; background: #f0f2f5; }

@media (max-width: 1100px) {
  .builder-layout { flex-direction: column; }
}
</style>

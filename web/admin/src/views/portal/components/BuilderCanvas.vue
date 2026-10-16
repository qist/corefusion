<template>
  <div class="col-canvas">
    <!-- Region Editor Toolbar (when a region is selected) -->
    <div v-if="ed.selectedRegion" class="canvas-toolbar">
      <div class="canvas-toolbar-left">
        <el-radio-group v-model="ed.regionViewMode" size="small">
          <el-radio-button value="preview">{{ t('portal.visualPreview') }}</el-radio-button>
          <el-radio-button value="html">HTML</el-radio-button>
          <el-radio-button value="css">CSS</el-radio-button>
          <el-radio-button value="js">JS</el-radio-button>
        </el-radio-group>
      </div>
      <div class="canvas-toolbar-right">
        <el-radio-group v-if="ed.regionViewMode === 'preview'" v-model="ed.previewDevice" size="small">
          <el-radio-button value="desktop">{{ t('portal.deviceDesktop') }}</el-radio-button>
          <el-radio-button value="tablet">{{ t('portal.deviceTablet') }}</el-radio-button>
          <el-radio-button value="mobile">{{ t('portal.deviceMobile') }}</el-radio-button>
        </el-radio-group>
      </div>
    </div>

    <!-- Region Preview Mode (iframe) -->
    <div v-if="ed.selectedRegion && ed.regionViewMode === 'preview'" class="canvas-scroll">
      <iframe :src="ed.previewUrl" class="preview-iframe" />
    </div>

    <!-- Region HTML/CSS/JS Editor — binds directly to the region object so
         edits actually persist (previously edited a throwaway computed copy) -->
    <div v-if="ed.selectedRegion && ed.regionViewMode !== 'preview'" class="canvas-code-editor">
      <CodeEditor
        :key="'region-' + ed.selectedRegionType + '-' + ed.regionViewMode"
        v-model="ed.selectedRegion[ed.regionViewMode]"
        :language="ed.regionViewMode === 'html' ? 'html' : ed.regionViewMode === 'css' ? 'css' : 'javascript'"
        @change="ed.onCodeChange"
      />
    </div>

    <!-- Module Toolbar: view mode toggle + device buttons -->
    <div v-if="ed.selectedModule" class="canvas-toolbar">
      <div class="canvas-toolbar-left">
        <el-radio-group v-model="ed.centerViewMode" size="small">
          <el-radio-button v-if="ed.hasContentEditor" value="editor">{{ t('portal.contentEditor') }}</el-radio-button>
          <el-radio-button value="preview">{{ t('portal.visualPreview') }}</el-radio-button>
          <el-radio-button value="code">{{ t('portal.moduleCode') }}</el-radio-button>
        </el-radio-group>
      </div>
      <div class="canvas-toolbar-right">
        <el-radio-group v-if="ed.centerViewMode === 'preview' || ed.centerViewMode === 'editor'" v-model="ed.previewDevice" size="small">
          <el-radio-button value="desktop">{{ t('portal.deviceDesktop') }}</el-radio-button>
          <el-radio-button value="tablet">{{ t('portal.deviceTablet') }}</el-radio-button>
          <el-radio-button value="mobile">{{ t('portal.deviceMobile') }}</el-radio-button>
        </el-radio-group>
      </div>
    </div>

    <!-- Content Editor Mode (markdown, rich_text, notice, custom_html, freeform) -->
    <div v-if="ed.selectedModule && ed.centerViewMode === 'editor' && ed.hasContentEditor" class="canvas-code-editor">
      <ModuleContentEditor :key="'content-' + ed.selectedModuleId + '-' + ed.selectedModule.type" />
    </div>

    <!-- 全屏模块代码编辑（HTML / CSS / JS）——与右侧属性面板共用同一编辑器 -->
    <div v-if="ed.selectedModule && ed.centerViewMode === 'code'" class="canvas-code-editor">
      <ModuleCodeEditor fullscreen />
    </div>

    <!-- Visual Preview Mode — embeds the real portal page via iframe so what
         you see is exactly what users get. This is the single, canonical
         preview (the old duplicate "Page Preview" mode has been removed). -->
    <div v-if="!ed.selectedModule || ed.centerViewMode === 'preview'" class="canvas-scroll">
      <div class="preview-toolbar">
        <el-button text size="small" :icon="Refresh" @click="ed.refreshPreview">{{ t('portal.refreshPreview') }}</el-button>
        <span class="preview-hint">{{ t('portal.previewHint') }}</span>
      </div>
      <div class="preview-iframe-wrap" :class="'device-' + ed.previewDevice">
        <iframe
          :key="ed.previewKey"
          :src="ed.pagePreviewUrl"
          class="preview-iframe"
          @load="ed.onPreviewLoad"
        />
      </div>
    </div>
  </div>
</template>

<script setup>
import { inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, Refresh } from '@element-plus/icons-vue'
import CodeEditor from '../../../components/common/CodeEditor.vue'
import ModuleContentEditor from './ModuleContentEditor.vue'
import ModuleCodeEditor from './ModuleCodeEditor.vue'

const { t } = useI18n()
const ed = inject('portalEditor')
</script>

<style scoped>
.col-canvas {
  flex: 1;
  background: #f0f2f5;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  min-height: 0;
  min-width: 0;
}

.canvas-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding: 8px 16px;
  background: #fff;
  border-bottom: 1px solid #e4e7ed;
  flex-shrink: 0;
}
.canvas-toolbar-left, .canvas-toolbar-right { display: flex; align-items: center; gap: 8px; }

.canvas-code-editor { flex: 1; display: flex; flex-direction: column; overflow: hidden; padding: 12px; background: #fff; min-height: 0; }
.canvas-scroll { flex: 1; overflow-y: auto; padding: 20px; display: flex; justify-content: center; min-height: 0; }
.canvas-scroll:has(.preview-iframe) { display: flex; flex-direction: column; }

.preview-iframe { width: 100%; height: 100%; border: none; background: #f5f7fa; }

/* ===== Visual Preview (iframe) ===== */
.preview-toolbar {
  display: flex; align-items: center; gap: 8px;
  padding: 6px 12px; background: #fff; border-bottom: 1px solid #e4e7ed;
  flex-shrink: 0;
}
.preview-hint { font-size: 12px; color: #909399; }
.preview-iframe-wrap {
  flex: 1; display: flex; justify-content: center; overflow: hidden;
  background: #f0f2f5; padding: 16px;
}
.preview-iframe-wrap .preview-iframe {
  border-radius: 8px; box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08);
  background: #fff;
}
.preview-iframe-wrap.device-desktop .preview-iframe { width: 100%; max-width: 1200px; }
.preview-iframe-wrap.device-tablet .preview-iframe { width: 768px; }
.preview-iframe-wrap.device-mobile .preview-iframe { width: 375px; }

@media (max-width: 1100px) {
  .col-canvas { flex: 1; min-height: 300px; }
}
</style>

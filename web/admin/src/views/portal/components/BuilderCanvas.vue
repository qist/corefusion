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
          <el-radio-button value="html">HTML</el-radio-button>
          <el-radio-button value="css">CSS</el-radio-button>
          <el-radio-button value="js">JS</el-radio-button>
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

    <!-- CSS Editor Mode (available for ALL module types) -->
    <div v-if="ed.selectedModule && ed.centerViewMode === 'css'" class="canvas-css-editor">
      <div class="css-editor-info">
        <el-alert type="info" :closable="false" show-icon style="margin-bottom: 8px; border-radius: 6px">
          <span style="font-size: 12px">
            {{ t('portal.cssHint1') }}<b>{{ ed.selectedModule.title || ed.typeLabel(ed.selectedModule.type) }}</b>{{ t('portal.cssHint2') }}
          </span>
        </el-alert>
      </div>
      <div class="css-editor-split">
        <div class="css-editor-pane">
          <CodeEditor v-model="ed.selectedModule.style.custom_css" language="css" @change="ed.onCodeChange" />
        </div>
        <div class="css-preview-pane" :class="'device-' + ed.previewDevice">
          <div class="portal-preview" :style="ed.previewStyles">
            <div class="preview-header">
              <div class="preview-header-left">
                <span class="preview-site-name">{{ ed.config.site_name || 'CoreFusion' }}</span>
              </div>
            </div>
            <div class="preview-modules">
              <div :id="'preview-mod-' + ed.selectedModule.id" class="module-wrap-preview">
                <style v-if="ed.selectedModule.data?.css" v-html="ed.selectedModule.data.css"></style>
                <style v-if="ed.selectedModule.style?.custom_css" v-html="ed.scopeCss(ed.selectedModule.style.custom_css, ed.selectedModule.id)"></style>
                <component :is="ed.dynamicModuleComponent" v-if="ed.dynamicModuleComponent" />
                <div v-else class="preview-module" style="text-align:center;padding:40px;color:#909399">
                  <p>{{ t('portal.noCustomCode') }}</p>
                  <p style="font-size:12px;margin-top:8px">{{ t('portal.addCodeHint') }}</p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </div>
    </div>

    <!-- HTML Editor Mode -->
    <div v-if="ed.selectedModule && ed.centerViewMode === 'html'" class="canvas-code-editor">
      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 8px">
        <span style="font-size: 12px">{{ t('portal.htmlEditorHint') }}</span>
      </el-alert>
      <CodeEditor v-model="ed.selectedModule.data.html" language="html" @change="ed.onCodeChange" />
    </div>

    <!-- JS Editor Mode -->
    <div v-if="ed.selectedModule && ed.centerViewMode === 'js'" class="canvas-code-editor">
      <CodeEditor v-model="ed.selectedModule.data.js" language="javascript" @change="ed.onCodeChange" />
      <CtxApiRef style="padding: 8px 12px" />
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
import CtxApiRef from './CtxApiRef.vue'

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

/* ===== CSS Editor Mode ===== */
.canvas-css-editor { flex: 1; display: flex; flex-direction: column; overflow: hidden; padding: 12px; }
.css-editor-info { flex-shrink: 0; }
.css-editor-split { flex: 1; display: flex; min-height: 0; gap: 8px; overflow: hidden; }
.css-editor-pane { flex: 1; min-width: 0; overflow: hidden; }
.css-preview-pane { flex: 1; overflow-y: auto; background: #f5f7fa; border-radius: 6px; }
.css-preview-pane.device-tablet .portal-preview { max-width: 768px; margin: 0 auto; }
.css-preview-pane.device-mobile .portal-preview { max-width: 375px; margin: 0 auto; }
.module-wrap-preview { padding: 16px; }

/* ===== Visual Preview ===== */
.portal-preview {
  background: var(--pv-bg); border-radius: 16px; overflow: hidden;
  font-family: var(--pv-font); color: var(--pv-text);
  box-shadow: 0 4px 20px rgba(0, 0, 0, 0.08); min-height: 600px;
  display: flex; flex-direction: column;
}
.portal-preview.device-desktop { width: 100%; max-width: 800px; }
.portal-preview.device-tablet { width: 768px; }
.portal-preview.device-mobile { width: 375px; }

.preview-header {
  display: flex; justify-content: space-between; align-items: center;
  padding: 12px 20px; background: var(--pv-card-bg);
  border-bottom: 1px solid rgba(0, 0, 0, 0.06);
}
.preview-header-left { display: flex; align-items: center; gap: 8px; }
.preview-site-name { font-size: 16px; font-weight: 600; }
.preview-header-right { display: flex; gap: 12px; }
.preview-nav-item { font-size: 13px; color: #6c7293; cursor: pointer; }
.preview-nav-item:hover { color: var(--pv-primary); }

.preview-modules { flex: 1; padding: 16px; overflow-y: auto; }
.preview-module { margin-bottom: 16px; padding: 20px; background: var(--pv-card-bg); border-radius: 12px; }

.preview-page-title {
  display: flex; align-items: center; gap: 8px;
  padding: 12px 16px; background: var(--pv-card-bg);
  border-bottom: 2px solid var(--pv-primary);
  flex-shrink: 0;
}
.page-title-icon { font-size: 20px; }
.page-title-text { font-size: 18px; font-weight: 700; color: var(--pv-text); }

.preview-empty { text-align: center; padding: 40px; color: #909399; }

.preview-bottom-nav {
  display: flex; justify-content: space-around; padding: 8px 0;
  background: var(--pv-card-bg); border-top: 1px solid rgba(0, 0, 0, 0.06);
}
.preview-bottom-nav-item { display: flex; flex-direction: column; align-items: center; gap: 2px; }
.preview-bottom-nav-item .nav-icon { font-size: 18px; }
.preview-bottom-nav-item .nav-label { font-size: 11px; color: #909399; }

@media (max-width: 1100px) {
  .col-canvas { flex: 1; min-height: 300px; }
}
</style>

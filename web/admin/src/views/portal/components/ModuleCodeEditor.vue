<template>
  <div class="module-code-editor" :class="{ 'is-fullscreen': fullscreen }">
    <div class="mce-tabs">
      <el-radio-group v-model="ed.moduleCodeTab" size="small">
        <el-radio-button value="html">HTML</el-radio-button>
        <el-radio-button value="css">CSS</el-radio-button>
        <el-radio-button value="js">JS</el-radio-button>
      </el-radio-group>
      <span class="mce-hint">{{ t('portal.moduleCodeHint') }}</span>
    </div>
    <div class="mce-body">
      <CodeEditor
        v-if="ed.moduleCodeTab === 'html'"
        :key="'mce-html-' + ed.selectedModuleId"
        v-model="ed.selectedModule.data.html"
        language="html"
        @change="ed.onCodeChange"
      />
      <CodeEditor
        v-else-if="ed.moduleCodeTab === 'css'"
        :key="'mce-css-' + ed.selectedModuleId"
        v-model="ed.selectedModule.data.css"
        language="css"
        @change="ed.onCodeChange"
      />
      <template v-else>
        <CodeEditor
          :key="'mce-js-' + ed.selectedModuleId"
          v-model="ed.selectedModule.data.js"
          language="javascript"
          @change="ed.onCodeChange"
        />
        <CtxApiRef />
      </template>
    </div>
  </div>
</template>

<script setup>
import { inject } from 'vue'
import { useI18n } from 'vue-i18n'
import CodeEditor from '../../../components/common/CodeEditor.vue'
import CtxApiRef from './CtxApiRef.vue'

defineProps({ fullscreen: { type: Boolean, default: false } })

const { t } = useI18n()
const ed = inject('portalEditor')
</script>

<style scoped>
.module-code-editor { display: flex; flex-direction: column; min-height: 0; }
.mce-tabs { display: flex; align-items: center; justify-content: space-between; gap: 8px; margin-bottom: 6px; flex-shrink: 0; }
.mce-hint { font-size: 11px; color: #909399; }
.mce-body { height: 300px; display: flex; flex-direction: column; min-height: 0; }
.mce-body > * { min-height: 0; }
.module-code-editor.is-fullscreen { flex: 1; min-height: 0; height: 100%; }
.module-code-editor.is-fullscreen .mce-body { flex: 1; height: auto; }
</style>

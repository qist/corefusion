<template>
  <div class="code-mode">
    <el-alert type="info" :closable="false" show-icon style="margin-bottom: 0; border-radius: 0">
      <span style="font-size: 13px">
        <b>{{ t('portal.codeModeTitle') }}</b> — {{ t('portal.codeModeHint') }}
        <br/>{{ t('portal.htmlVarsLabel') }}：<code v-pre>{{.PortalName}}</code> <code v-pre>{{.LogoURL}}</code> <code v-pre>{{.RegistrationEnabled}}</code>
        <code v-pre>{{registrationEnabledText}}</code> &nbsp;|&nbsp; {{ t('portal.cssVarsLabel') }}：<code v-pre>{{.PrimaryColor}}</code>
        <code v-pre>{{.BackgroundColor}}</code> <code v-pre>{{.FontFamily}}</code> {{ t('portal.etc') }}
      </span>
    </el-alert>
    <div class="code-tabs-bar">
      <el-radio-group v-model="ed.codeTab" size="small">
        <el-radio-button value="html">{{ t('portal.htmlTemplate') }}</el-radio-button>
        <el-radio-button value="css">{{ t('portal.cssStyles') }}</el-radio-button>
        <el-radio-button value="js">{{ t('portal.javascript') }}</el-radio-button>
        <el-radio-button value="advanced">{{ t('portal.resetAndExport') }}</el-radio-button>
      </el-radio-group>
    </div>
    <div v-if="ed.codeTab !== 'advanced'" class="code-editor-area">
      <CodeEditor
        v-if="ed.codeTab === 'html'"
        v-model="ed.form.html_content"
        language="html"
        :minimap="true"
        @change="ed.onCodeChange"
      />
      <CodeEditor
        v-if="ed.codeTab === 'css'"
        v-model="ed.form.css_content"
        language="css"
        :minimap="true"
        @change="ed.onCodeChange"
      />
      <CodeEditor
        v-if="ed.codeTab === 'js'"
        v-model="ed.form.js_content"
        language="javascript"
        :minimap="true"
        @change="ed.onCodeChange"
      />
    </div>
    <div v-else class="code-advanced-pane">
      <el-alert type="warning" :closable="false" show-icon :title="t('portal.resetToDefault')" style="margin-bottom: 16px">
        {{ t('portal.resetWarning') }}
      </el-alert>
      <el-button type="danger" @click="ed.handleReset">{{ t('portal.resetToDefaultBtn') }}</el-button>
    </div>
  </div>
</template>

<script setup>
import { inject } from 'vue'
import { useI18n } from 'vue-i18n'
import CodeEditor from '../../../components/common/CodeEditor.vue'

const { t } = useI18n()
const ed = inject('portalEditor')
</script>

<style scoped>
.code-mode { display: flex; flex-direction: column; height: 100%; overflow: hidden; }
.code-tabs-bar {
  display: flex; align-items: center; gap: 8px; padding: 8px 16px;
  background: #fff; border-bottom: 1px solid #e4e7ed; flex-shrink: 0;
}
.code-editor-area { flex: 1; min-height: 0; padding: 8px; }
.code-editor-area > * { height: 100%; }
.code-advanced-pane { padding: 20px; overflow-y: auto; }
</style>

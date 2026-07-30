<template>
  <div v-if="ed.selectedModule" class="content-editor">
    <!-- Markdown Editor -->
    <template v-if="ed.selectedModule.type === 'markdown'">
      <div class="md-editor-wrap md-editor-full">
        <div class="md-editor-toolbar">
          <button v-for="btn in mdToolbarButtons" :key="btn.type" :title="t(btn.titleKey)" @click="insertMd(btn.type)">
            <span v-if="btn.text" v-html="btn.text"></span><template v-else>{{ btn.label }}</template>
          </button>
        </div>
        <div class="md-split md-split-full">
          <div class="md-editor-pane">
            <CodeEditor
              v-model="ed.selectedModule.data.markdown"
              language="markdown"
              @mount="onMdEditorMount"
              @change="ed.onCodeChange"
            />
          </div>
          <div class="md-preview-pane md-preview-content" :class="'device-' + ed.previewDevice" v-html="ed.renderMd(ed.selectedModule.data.markdown || '')"></div>
        </div>
      </div>
    </template>

    <!-- Notice Editor -->
    <template v-else-if="ed.selectedModule.type === 'notice'">
      <div class="notice-editor-wrap">
        <el-form-item :label="t('portal.noticeTitle')" style="margin-bottom: 8px">
          <el-input v-model="ed.selectedModule.data.title" :placeholder="t('portal.noticeTitle')" />
        </el-form-item>
        <div class="md-editor-wrap md-editor-full">
          <div class="md-editor-toolbar">
            <button :title="t('portal.mdHeading')" @click="insertMd('heading')">H</button>
            <button :title="t('portal.mdBold')" @click="insertMd('bold')"><b>B</b></button>
            <button :title="t('portal.mdItalic')" @click="insertMd('italic')"><i>I</i></button>
            <button :title="t('portal.mdLink')" @click="insertMd('link')">🔗</button>
            <button :title="t('portal.mdCode')" @click="insertMd('code')">&lt;/&gt;</button>
          </div>
          <div class="md-split md-split-full">
            <div class="md-editor-pane">
              <CodeEditor
                v-model="ed.selectedModule.data.markdown"
                language="markdown"
                @mount="onMdEditorMount"
                @change="ed.onCodeChange"
              />
            </div>
            <div class="md-preview-pane md-preview-content" :class="'device-' + ed.previewDevice" v-html="ed.renderMd(ed.selectedModule.data.markdown || '')"></div>
          </div>
        </div>
      </div>
    </template>

    <!-- Rich Text Editor -->
    <template v-else-if="ed.selectedModule.type === 'rich_text'">
      <div class="rte-editor-wrap">
        <div class="rte-toolbar">
          <button @click="execCmd('formatBlock', '<h2>')" :title="t('portal.rteHeading1')">H2</button>
          <button @click="execCmd('formatBlock', '<h3>')" :title="t('portal.rteHeading2')">H3</button>
          <button @click="execCmd('formatBlock', '<p>')" :title="t('portal.rteParagraph')">P</button>
          <span class="rte-divider"></span>
          <button @click="execCmd('bold')" :title="t('portal.mdBold')"><b>B</b></button>
          <button @click="execCmd('italic')" :title="t('portal.mdItalic')"><i>I</i></button>
          <button @click="execCmd('underline')" :title="t('portal.rteUnderline')"><u>U</u></button>
          <button @click="execCmd('strikeThrough')" :title="t('portal.mdStrikethrough')"><s>S</s></button>
          <span class="rte-divider"></span>
          <button @click="execCmd('insertUnorderedList')" :title="t('portal.mdUnorderedList')">•</button>
          <button @click="execCmd('insertOrderedList')" :title="t('portal.mdOrderedList')">1.</button>
          <button @click="execCmd('formatBlock', '<blockquote>')" :title="t('portal.mdQuote')">❝</button>
          <span class="rte-divider"></span>
          <button @click="insertLink" :title="t('portal.rteHyperlink')">🔗</button>
          <button @click="execCmd('removeFormat')" :title="t('portal.rteClearFormat')">✕</button>
        </div>
        <div class="rte-split">
          <!-- key forces a fresh element per module; content set imperatively to
               avoid the v-html + contenteditable cursor-reset bug -->
          <div
            :key="'rte-' + ed.selectedModuleId"
            ref="richTextEditRef"
            class="rte-content rte-content-full"
            contenteditable="true"
            @input="onRichTextInput"
          ></div>
          <div class="rte-preview md-preview-content" :class="'device-' + ed.previewDevice" v-html="ed.sanitizeHtml(ed.selectedModule.data.html || '')"></div>
        </div>
      </div>
    </template>

    <!-- Custom HTML Editor -->
    <template v-else-if="ed.selectedModule.type === 'custom_html'">
      <div class="html-editor-wrap">
        <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 8px">
          <span style="font-size: 12px">{{ t('portal.customHtmlWarning') }}</span>
        </el-alert>
        <div class="html-split">
          <div class="html-editor-pane">
            <CodeEditor v-model="ed.selectedModule.data.html" language="html" @change="ed.onCodeChange" />
          </div>
          <div class="html-preview md-preview-content" :class="'device-' + ed.previewDevice" v-html="ed.sanitizeHtml(ed.selectedModule.data.html || '')"></div>
        </div>
      </div>
    </template>

    <!-- Freeform Editor (HTML + CSS + JS) -->
    <template v-else-if="ed.selectedModule.type === 'freeform'">
      <div class="freeform-editor-wrap">
        <el-tabs v-model="ed.freeformTab" type="border-card" class="freeform-tabs">
          <el-tab-pane :label="t('portal.freeformHtmlTab')" name="html">
            <div class="freeform-editor-pane">
              <CodeEditor v-model="ed.selectedModule.data.html" language="html" @change="ed.onCodeChange" />
            </div>
          </el-tab-pane>
          <el-tab-pane label="CSS" name="css">
            <div class="freeform-editor-pane">
              <CodeEditor v-model="ed.selectedModule.data.css" language="css" @change="ed.onCodeChange" />
            </div>
          </el-tab-pane>
          <el-tab-pane :label="t('portal.freeformJsTab')" name="js">
            <div class="freeform-editor-pane">
              <CodeEditor v-model="ed.selectedModule.data.js" language="javascript" @change="ed.onCodeChange" />
            </div>
            <CtxApiRef />
          </el-tab-pane>
        </el-tabs>
        <el-alert type="info" :closable="false" show-icon style="margin-top: 8px">
          <span style="font-size: 12px">{{ t('portal.jsHint') }}</span>
        </el-alert>
      </div>
    </template>
  </div>
</template>

<script setup>
import { ref, inject, watch, nextTick } from 'vue'
import { useI18n } from 'vue-i18n'
import CodeEditor from '../../../components/common/CodeEditor.vue'
import CtxApiRef from './CtxApiRef.vue'

const { t } = useI18n()
const ed = inject('portalEditor')

// ---- Markdown toolbar ----
const mdToolbarButtons = [
  { type: 'heading', label: 'H', titleKey: 'portal.mdHeading' },
  { type: 'bold', text: '<b>B</b>', titleKey: 'portal.mdBold' },
  { type: 'italic', text: '<i>I</i>', titleKey: 'portal.mdItalic' },
  { type: 'strikethrough', text: '<s>S</s>', titleKey: 'portal.mdStrikethrough' },
  { type: 'ul', label: '•', titleKey: 'portal.mdUnorderedList' },
  { type: 'ol', label: '1.', titleKey: 'portal.mdOrderedList' },
  { type: 'checklist', label: '☑', titleKey: 'portal.mdChecklist' },
  { type: 'link', label: '🔗', titleKey: 'portal.mdLink' },
  { type: 'image', label: '🖼', titleKey: 'portal.mdImage' },
  { type: 'code', label: '</>', titleKey: 'portal.mdCode' },
  { type: 'codeblock', label: '{ }', titleKey: 'portal.mdCodeBlock' },
  { type: 'quote', label: '❝', titleKey: 'portal.mdQuote' },
  { type: 'table', label: '⊞', titleKey: 'portal.mdTable' },
  { type: 'hr', label: '—', titleKey: 'portal.mdHr' },
]

const mdEditorRef = ref(null)
function onMdEditorMount(editor) {
  mdEditorRef.value = editor
}

// Single insertion helper shared by markdown & notice editors (previously two
// near-identical functions). Falls back to the current selection text.
function insertMd(type) {
  const editor = mdEditorRef.value
  if (!editor || !ed.selectedModule) return
  const model = editor.getModel()
  if (!model) return
  const selection = editor.getSelection()
  const sel = model.getValueInRange(selection)
  let insert = ''
  switch (type) {
    case 'heading': insert = `## ${sel || t('portal.mdInsert.title')}`; break
    case 'bold': insert = `**${sel || t('portal.mdInsert.bold')}**`; break
    case 'italic': insert = `*${sel || t('portal.mdInsert.italic')}*`; break
    case 'strikethrough': insert = `~~${sel || t('portal.mdInsert.strike')}~~`; break
    case 'ul': insert = `- ${sel || t('portal.mdInsert.listItem')}`; break
    case 'ol': insert = `1. ${sel || t('portal.mdInsert.listItem')}`; break
    case 'checklist': insert = `- [ ] ${sel || t('portal.mdInsert.todo')}`; break
    case 'link': insert = `[${sel || t('portal.mdInsert.linkText')}](https://example.com)`; break
    case 'image': insert = `![${sel || t('portal.mdInsert.imageDesc')}](https://...)`; break
    case 'code': insert = `\`${sel || t('portal.mdInsert.code')}\``; break
    case 'codeblock': insert = `\`\`\`\n${sel || t('portal.mdInsert.codeblock')}\n\`\`\``; break
    case 'quote': insert = `> ${sel || t('portal.mdInsert.quote')}`; break
    // Fixed: previously inserted the literal text "t('portal.mdInsert.table')"
    case 'table': insert = t('portal.mdInsert.table'); break
    case 'hr': insert = `---`; break
  }
  editor.executeEdits('toolbar', [{ range: selection, text: insert, forceMoveMarkers: true }])
  editor.focus?.()
  ed.onCodeChange()
}

// ---- Rich text editor ----
const richTextEditRef = ref(null)

// Seed the editable element whenever it is (re)created — v-html is deliberately
// NOT used here because re-rendering a contenteditable resets the caret.
watch(richTextEditRef, (el) => {
  if (el && ed.selectedModule) {
    nextTick(() => { el.innerHTML = ed.selectedModule?.data?.html || '' })
  }
}, { flush: 'post' })

function execCmd(cmd, val) {
  document.execCommand(cmd, false, val)
  syncRichText()
}
function insertLink() {
  const url = window.prompt(t('portal.enterLinkUrl'))
  if (url) { document.execCommand('createLink', false, url); syncRichText() }
}
function onRichTextInput() { syncRichText() }
function syncRichText() {
  if (richTextEditRef.value && ed.selectedModule) {
    ed.selectedModule.data.html = richTextEditRef.value.innerHTML
    ed.onCodeChange()
  }
}
</script>

<style scoped>
.content-editor { flex: 1; display: flex; flex-direction: column; overflow: hidden; min-height: 0; }
.content-editor > * { flex: 1; display: flex; flex-direction: column; overflow: hidden; min-height: 0; }

/* ===== Markdown Editor ===== */
.md-editor-wrap { border: 1px solid #dcdfe6; border-radius: 6px; overflow: hidden; }
.md-editor-toolbar {
  display: flex; flex-wrap: wrap; gap: 2px; padding: 6px 8px;
  background: #fafafa; border-bottom: 1px solid #e4e7ed;
}
.md-editor-toolbar button {
  padding: 4px 8px; border: none; background: none; cursor: pointer;
  font-size: 13px; border-radius: 4px; transition: background 0.15s; color: #606266;
}
.md-editor-toolbar button:hover { background: #e4e7ed; }
.md-split { display: flex; min-height: 240px; flex: 1; min-height: 0; }
.md-editor-pane { width: 50%; min-width: 0; overflow: hidden; }
.md-preview-pane { width: 50%; padding: 12px; border-left: 1px solid #e4e7ed; overflow-y: auto; background: #fff; }

/* ===== Rich Text Editor ===== */
.rte-toolbar { display: flex; flex-wrap: wrap; gap: 2px; padding: 6px 8px; background: #f5f7fa; border-bottom: 1px solid #dcdfe6; }
.rte-toolbar button { padding: 4px 8px; border: none; background: none; cursor: pointer; font-size: 13px; border-radius: 4px; transition: background 0.2s; }
.rte-toolbar button:hover { background: #dcdfe6; }
.rte-divider { width: 1px; background: #dcdfe6; margin: 2px 4px; }
.rte-split { flex: 1; display: flex; min-height: 0; overflow: hidden; }
.rte-content { padding: 12px; min-height: 120px; font-size: 14px; line-height: 1.6; outline: none; }
.rte-content:focus { outline: 2px solid #409eff; }
.rte-content-full { flex: 1; overflow-y: auto; }
.rte-preview { flex: 1; overflow-y: auto; border-left: 1px solid #e4e7ed; background: #fff; }

/* ===== Custom HTML Editor ===== */
.html-split { flex: 1; display: flex; min-height: 0; overflow: hidden; }
.html-editor-pane { flex: 1; min-width: 0; border: 1px solid #dcdfe6; border-radius: 6px 0 0 6px; overflow: hidden; }
.html-preview {
  flex: 1; padding: 12px; border: 1px solid #dcdfe6; border-left: none; border-radius: 0 6px 6px 0;
  overflow-y: auto; background: #fff;
}

/* ===== Freeform Editor ===== */
.freeform-editor-wrap { display: flex; flex-direction: column; }
.freeform-tabs { flex: 1; display: flex; flex-direction: column; overflow: hidden; }
.freeform-tabs :deep(.el-tabs__content) { flex: 1; overflow: hidden; }
.freeform-tabs :deep(.el-tab-pane) { height: 100%; display: flex; flex-direction: column; }
.freeform-editor-pane { flex: 1; overflow: hidden; min-height: 300px; }

/* ===== Shared preview content ===== */
.md-preview-content { font-size: 14px; line-height: 1.7; }
.md-preview-content :deep(h1) { font-size: 22px; font-weight: 700; margin: 16px 0 8px; padding-bottom: 6px; border-bottom: 1px solid rgba(0, 0, 0, 0.08); }
.md-preview-content :deep(h2) { font-size: 18px; font-weight: 700; margin: 14px 0 6px; padding-bottom: 4px; border-bottom: 1px solid rgba(0, 0, 0, 0.06); }
.md-preview-content :deep(h3) { font-size: 16px; font-weight: 600; margin: 12px 0 4px; }
.md-preview-content :deep(h4) { font-size: 14px; font-weight: 600; margin: 10px 0 4px; }
.md-preview-content :deep(p) { margin: 6px 0; }
.md-preview-content :deep(a) { color: var(--pv-primary, #409eff); text-decoration: none; }
.md-preview-content :deep(a:hover) { text-decoration: underline; }
.md-preview-content :deep(strong) { font-weight: 600; }
.md-preview-content :deep(blockquote) { border-left: 3px solid var(--pv-primary, #409eff); padding: 4px 12px; margin: 8px 0; color: #6c7293; background: var(--pv-bg, #f5f7fa); border-radius: 0 6px 6px 0; }
.md-preview-content :deep(code) { background: rgba(0, 0, 0, 0.06); padding: 2px 6px; border-radius: 4px; font-size: 13px; font-family: 'SF Mono', 'Fira Code', Consolas, monospace; }
.md-preview-content :deep(pre) { background: #1e1e2e; color: #cdd6f4; padding: 14px; border-radius: 8px; overflow-x: auto; margin: 10px 0; }
.md-preview-content :deep(pre code) { background: none; padding: 0; color: inherit; }
.md-preview-content :deep(ul), .md-preview-content :deep(ol) { padding-left: 20px; margin: 6px 0; }
.md-preview-content :deep(li) { margin: 3px 0; }
.md-preview-content :deep(table) { border-collapse: collapse; width: 100%; margin: 10px 0; font-size: 13px; }
.md-preview-content :deep(th), .md-preview-content :deep(td) { border: 1px solid rgba(0, 0, 0, 0.1); padding: 6px 10px; text-align: left; }
.md-preview-content :deep(th) { background: var(--pv-bg, #f5f7fa); font-weight: 600; }
.md-preview-content :deep(hr) { border: none; border-top: 1px solid rgba(0, 0, 0, 0.1); margin: 16px 0; }
.md-preview-content :deep(img) { max-width: 100%; border-radius: 8px; }
.md-preview-content :deep(input[type="checkbox"]) { margin-right: 6px; }

/* Device width for preview panes */
.md-preview-pane.device-tablet, .rte-preview.device-tablet, .html-preview.device-tablet { max-width: 768px; margin: 0 auto; }
.md-preview-pane.device-mobile, .rte-preview.device-mobile, .html-preview.device-mobile { max-width: 375px; margin: 0 auto; }
</style>

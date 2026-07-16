<template>
  <div class="code-editor-container" :class="{ 'theme-dark': isDark }">
    <vue-monaco-editor
      :value="modelValue"
      :language="language"
      :theme="isDark ? 'vs-dark' : 'vs'"
      :options="mergedOptions"
      :height="height"
      @mount="handleMount"
      @change="handleChange"
    />
  </div>
</template>

<script setup>
import { computed, shallowRef } from 'vue'
import { VueMonacoEditor } from '@guolao/vue-monaco-editor'

const props = defineProps({
  modelValue: { type: String, default: '' },
  language: { type: String, default: 'plaintext' },
  height: { type: [String, Number], default: '100%' },
  isDark: { type: Boolean, default: false },
  readOnly: { type: Boolean, default: false },
  minimap: { type: Boolean, default: false },
  wordWrap: { type: Boolean, default: true },
  fontSize: { type: Number, default: 13 },
  lineNumbers: { type: Boolean, default: true },
  options: { type: Object, default: () => ({}) },
})

const emit = defineEmits(['update:modelValue', 'change', 'mount'])

const editorRef = shallowRef(null)

const mergedOptions = computed(() => ({
  automaticLayout: true,
  fontSize: props.fontSize,
  minimap: { enabled: props.minimap },
  scrollBeyondLastLine: false,
  wordWrap: props.wordWrap ? 'on' : 'off',
  lineNumbers: props.lineNumbers ? 'on' : 'off',
  readOnly: props.readOnly,
  tabSize: 2,
  insertSpaces: true,
  roundedSelection: true,
  scrollbar: {
    verticalScrollbarSize: 10,
    horizontalScrollbarSize: 10,
    useShadows: false,
  },
  padding: { top: 12, bottom: 12 },
  renderWhitespace: 'selection',
  renderLineHighlight: 'all',
  smoothScrolling: true,
  cursorBlinking: 'smooth',
  cursorSmoothCaretAnimation: 'on',
  ...props.options,
}))

function handleMount(editor, monaco) {
  editorRef.value = editor
  emit('mount', editor, monaco)
}

function handleChange(value) {
  emit('update:modelValue', value)
  emit('change', value)
}

defineExpose({
  getEditor: () => editorRef.value,
  focus: () => editorRef.value?.focus(),
})
</script>

<style scoped>
.code-editor-container {
  width: 100%;
  height: 100%;
  overflow: hidden;
  border-radius: 6px;
  border: 1px solid #dcdfe6;
}
.code-editor-container.theme-dark {
  border-color: #383838;
}
.code-editor-container :deep(.monaco-editor) {
  border-radius: 6px;
}
.code-editor-container :deep(.monaco-editor .overflow-guard) {
  border-radius: 6px;
}
</style>

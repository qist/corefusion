<template>
  <img
    v-if="src"
    class="lang-flag"
    :src="src"
    :alt="alt || code"
    :title="alt || code"
    loading="lazy"
    decoding="async"
  />
  <span v-else class="lang-flag lang-flag-fallback" :title="alt || code">{{ fallbackText }}</span>
</template>

<script setup>
import { computed } from 'vue'

// 国旗用本地 PNG 而不是 emoji：Windows 桌面浏览器（Chrome / Edge）不渲染
// 国旗 emoji（🇨🇳 会退化成两个区域指示字母 "CN"），只有 iOS / Android 才显示。
// 这里通过 Vite 打包到 assets 目录，构建时会按运行时 base 生成 URL，
// 子路径部署（webPath）也能正确加载。
const props = defineProps({
  code: { type: String, default: '' }, // 两位国家代码：cn / us / ru ...
  alt: { type: String, default: '' },
})

const modules = import.meta.glob('../assets/flags/*.png', {
  eager: true,
  query: '?url',
  import: 'default',
})

const flagMap = {}
for (const [path, url] of Object.entries(modules)) {
  const m = path.match(/([a-z]{2})\.png$/)
  if (m && typeof url === 'string') flagMap[m[1]] = url
}

const src = computed(() => flagMap[(props.code || '').toLowerCase()] || '')
const fallbackText = computed(() => (props.code || '?').toUpperCase().slice(0, 2))
</script>

<style scoped>
.lang-flag {
  display: inline-block;
  width: 20px;
  height: 14px;
  object-fit: cover;
  border-radius: 2px;
  vertical-align: -2px;
  box-shadow: 0 0 0 1px rgba(0, 0, 0, 0.08);
}

.lang-flag-fallback {
  font-size: 9px;
  line-height: 14px;
  text-align: center;
  color: #909399;
  background: #f5f7fa;
  vertical-align: -2px;
}
</style>

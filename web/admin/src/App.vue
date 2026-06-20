<template>
  <el-config-provider :locale="elLocale">
    <router-view />
  </el-config-provider>
</template>

<script setup>
import { computed, watch, ref } from 'vue'
import zhCn from 'element-plus/es/locale/lang/zh-cn'
import enUs from 'element-plus/es/locale/lang/en'
import i18n from './locales'

const lang = ref(i18n.global.locale.value)

watch(() => i18n.global.locale.value, (v) => { lang.value = v })

const elLocale = computed(() => lang.value === 'en' ? enUs : zhCn)
</script>

<style>
* {
  margin: 0;
  padding: 0;
  box-sizing: border-box;
}
body {
  font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
}
</style>

<template>
  <div class="lang-mode">
    <div class="lang-toolbar">
      <el-input v-model="ed.langSearch" :placeholder="t('portal.searchLangKey')" :prefix-icon="Search" clearable size="small" style="width: 320px" />
      <el-button text size="small" :icon="Plus" @click="ed.addLangKey">{{ t('portal.addLangEntry') }}</el-button>
      <el-button text size="small" @click="ed.exportLang">{{ t('portal.exportJson') }}</el-button>
      <el-upload :show-file-list="false" :before-upload="ed.importLang" accept=".json">
        <el-button text size="small">{{ t('portal.importJson') }}</el-button>
      </el-upload>
      <span class="lang-count">{{ t('portal.langEntryCount', { count: ed.filteredLangEntries.length }) }}</span>
    </div>
    <!-- Locale tabs -->
    <div class="lang-locales-bar">
      <div
        v-for="loc in ed.langAvailableLocales"
        :key="loc.code"
        class="lang-locale-tab"
        :class="{ active: ed.langLocale === loc.code }"
        @click="ed.switchLangLocale(loc.code)"
      >
        <span>{{ loc.flag }} {{ loc.label }}</span>
        <el-icon v-if="loc.code === ed.langPack.default" class="locale-default-badge" :title="t('portal.defaultLang')">★</el-icon>
        <el-icon v-if="loc.code !== ed.langPack.default" class="locale-remove-btn" @click.stop="ed.removeLocale(loc.code)"><Close /></el-icon>
      </div>
      <el-button text size="small" :icon="Plus" @click="ed.addLocale">{{ t('portal.addLocale') }}</el-button>
      <el-select v-model="ed.langPack.default" size="small" style="width: 160px; margin-left: auto;" :placeholder="t('portal.defaultLang')">
        <el-option v-for="loc in ed.langAvailableLocales" :key="loc.code" :label="loc.flag + ' ' + loc.label" :value="loc.code" />
      </el-select>
    </div>
    <div class="lang-table-wrap">
      <table class="lang-table">
        <thead>
          <tr>
            <th style="width: 30%">Key</th>
            <th>{{ ed.langAvailableLocales.find(l => l.code === ed.langLocale)?.flag }} {{ ed.langAvailableLocales.find(l => l.code === ed.langLocale)?.label }} {{ t('portal.langContent') }}</th>
            <th style="width: 80px">{{ t('common.actions') }}</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="entry in ed.filteredLangEntries" :key="entry.key">
            <td class="lang-key-cell">
              <span class="lang-key-prefix" :class="'prefix-' + entry.prefix">{{ entry.prefix }}</span>
              <span class="lang-key-name">{{ entry.key }}</span>
            </td>
            <td class="lang-value-cell">
              <el-input v-model="ed.langData[entry.key]" size="small" :placeholder="entry.key" @input="ed.onCodeChange" />
            </td>
            <td class="lang-action-cell">
              <el-button text size="small" type="danger" :icon="Delete" @click="ed.deleteLangKey(entry.key)" />
            </td>
          </tr>
        </tbody>
      </table>
      <div v-if="ed.filteredLangEntries.length === 0" class="lang-empty">
        <span style="font-size: 40px">📝</span>
        <p>{{ t('portal.noLangData') }}</p>
        <el-button text size="small" :icon="Plus" @click="ed.addLangKey">{{ t('portal.addFirstLangEntry') }}</el-button>
      </div>
    </div>
  </div>
</template>

<script setup>
import { inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, Delete, Search, Close } from '@element-plus/icons-vue'

const { t } = useI18n()
const ed = inject('portalEditor')
</script>

<style scoped>
.lang-mode { height: 100%; width: 100%; display: flex; flex-direction: column; overflow: hidden; position: relative; }
.lang-toolbar {
  display: flex; align-items: center; gap: 8px;
  padding: 8px 16px; background: #fff; border-bottom: 1px solid #e4e7ed;
}
.lang-count { margin-left: auto; font-size: 12px; color: #909399; }
.lang-locales-bar {
  display: flex; align-items: center; gap: 4px; flex-wrap: nowrap;
  padding: 6px 16px; background: #fafbfc; border-bottom: 1px solid #e4e7ed;
  overflow-x: auto; overflow-y: hidden;
  scrollbar-width: thin;
}
.lang-locales-bar::-webkit-scrollbar { height: 6px; }
.lang-locales-bar::-webkit-scrollbar-thumb { background: #c0c4cc; border-radius: 3px; }
.lang-locales-bar::-webkit-scrollbar-track { background: transparent; }
.lang-locale-tab {
  display: inline-flex; align-items: center; gap: 4px;
  padding: 4px 12px; border-radius: 6px; cursor: pointer;
  font-size: 13px; color: #606266; border: 1px solid transparent;
  transition: all 0.15s;
}
.lang-locale-tab:hover { background: #ecf5ff; color: #409eff; }
.lang-locale-tab.active { background: #409eff; color: #fff; border-color: #409eff; }
.locale-default-badge { color: #f7ba2a; font-size: 12px; }
.lang-locale-tab.active .locale-default-badge { color: #ffd700; }
.locale-remove-btn { color: #c0c4cc; font-size: 12px; margin-left: 2px; }
.locale-remove-btn:hover { color: #f56c6c; }
.lang-table-wrap { flex: 1; overflow: auto; padding: 0 16px 16px; min-height: 0; }
.lang-table { width: 100%; border-collapse: collapse; font-size: 13px; margin-top: 8px; table-layout: fixed; }
.lang-table th {
  text-align: left; padding: 8px 12px; background: #f5f7fa;
  border-bottom: 2px solid #e4e7ed; font-weight: 600; color: #606266;
  position: sticky; top: 0; z-index: 1;
}
.lang-table td { padding: 6px 12px; border-bottom: 1px solid #ebeef5; vertical-align: middle; }
.lang-key-cell { display: flex; align-items: center; gap: 6px; overflow: hidden; }
.lang-key-prefix {
  font-size: 10px; font-weight: 700; padding: 2px 6px; border-radius: 10px;
  text-transform: uppercase; letter-spacing: 0.3px; flex-shrink: 0;
}
.lang-key-name { font-family: 'SF Mono', Monaco, monospace; font-size: 12px; color: #606266; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; flex: 1; min-width: 0; }
.prefix-nav { background: #e6f7ff; color: #1890ff; }
.prefix-common { background: #f6ffed; color: #52c41a; }
.prefix-auth { background: #fff7e6; color: #fa8c16; }
.prefix-account { background: #f9f0ff; color: #722ed1; }
.prefix-traffic { background: #e6fffb; color: #13c2c2; }
.prefix-sub { background: #fff0f6; color: #eb2f96; }
.prefix-plan { background: #fffbe6; color: #faad14; }
.prefix-balance { background: #f0f5ff; color: #2f54eb; }
.prefix-order { background: #fff2e8; color: #fa541c; }
.prefix-node { background: #e8f4f8; color: #1890ff; }
.prefix-tutorial { background: #f0f9eb; color: #67c23a; }
.prefix-client { background: #ecf5ff; color: #409eff; }
.prefix-article { background: #fef0f0; color: #f56c6c; }
.prefix-announcement { background: #fdf6ec; color: #e6a23c; }
.prefix-referral { background: #f0f9eb; color: #67c23a; }
.prefix-ticket { background: #e8f4f8; color: #1890ff; }
.prefix-security { background: #fef0f0; color: #f56c6c; }
.prefix-log { background: #f4f4f5; color: #909399; }
.prefix-charge { background: #fff7e6; color: #fa8c16; }
.prefix-shared { background: #f9f0ff; color: #722ed1; }
.prefix-streaming { background: #e6f7ff; color: #1890ff; }
.prefix-theme { background: #f4f4f5; color: #909399; }
.prefix-error { background: #fef0f0; color: #f56c6c; }
.prefix-toast { background: #f0f9eb; color: #67c23a; }
.prefix-hero { background: #fff0f6; color: #eb2f96; }
.lang-value-cell { min-width: 200px; }
.lang-action-cell { text-align: center; }
.lang-empty {
  display: flex; flex-direction: column; align-items: center; justify-content: center;
  padding: 60px 20px; color: #909399;
}
.lang-empty p { margin: 8px 0; font-size: 14px; }
</style>

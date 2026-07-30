<template>
  <div class="col-structure">
    <!-- System Pages Section (Regions) -->
    <div class="structure-section">
      <div class="section-header">
        <span class="section-title">{{ t('portal.systemPages') }}</span>
      </div>
      <div class="nav-list">
        <div class="nav-item-row" :class="{ selected: ed.selectedRegionType === 'auth' }" @click="ed.selectRegion('auth')">
          <div class="nav-item-info">
            <span class="nav-icon-emoji">🔐</span>
            <span class="nav-label-text">{{ t('portal.loginPage') }}</span>
          </div>
        </div>
      </div>
    </div>

    <!-- Page Navigation Section -->
    <div class="structure-section">
      <div class="section-header">
        <span class="section-title">{{ t('portal.pageNav') }}</span>
        <el-button text size="small" :icon="Plus" @click="ed.openNavDialog()" />
      </div>
      <div class="nav-list">
        <div
          v-for="(nav, idx) in ed.navigation"
          :key="nav.id"
          class="nav-item-row"
          :class="{ selected: ed.currentPageNavId === nav.id, disabled: !nav.enabled }"
          @click="ed.selectNav(nav.id)"
        >
          <div class="nav-item-info">
            <span class="nav-icon-emoji">{{ nav.icon || '🔗' }}</span>
            <span class="nav-label-text">{{ nav.label }}</span>
            <el-tag size="small" type="info" effect="plain">{{ t('portal.moduleCount', { count: ed.currentPageModuleCount(nav.target) }) }}</el-tag>
          </div>
          <div class="nav-item-actions" @click.stop>
            <el-button text size="small" :icon="ArrowUp" @click="ed.moveItem(ed.navigation, idx, -1)" :disabled="idx === 0" />
            <el-button text size="small" :icon="ArrowDown" @click="ed.moveItem(ed.navigation, idx, 1)" :disabled="idx === ed.navigation.length - 1" />
            <el-switch v-model="nav.enabled" size="small" @change="ed.onCodeChange" />
            <el-button text size="small" type="danger" :icon="Delete" @click="ed.removeNav(idx)" />
          </div>
        </div>
        <div v-if="ed.navigation.length === 0" class="empty-hint" @click="ed.openNavDialog()">
          {{ t('portal.addNav') }}
        </div>
      </div>
    </div>

    <!-- Current Page Modules Section -->
    <div class="structure-section structure-modules">
      <div class="section-header">
        <span class="section-title">
          {{ ed.currentNav ? t('portal.navModules', { name: ed.currentNav.label }) : t('portal.contentModules') }}
        </span>
        <el-dropdown trigger="click" :disabled="!ed.currentNav" @command="ed.addModule">
          <el-button text size="small" :icon="Plus" :disabled="!ed.currentNav">{{ t('portal.addModule') }}</el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <template v-for="(group, gi) in ed.addModuleGroups" :key="group.key">
                <el-dropdown-item disabled :divided="gi > 0" class="module-group-header">— {{ t(group.labelKey) }} —</el-dropdown-item>
                <el-dropdown-item v-for="item in group.types" :key="item.type" :command="item.type">
                  {{ item.icon }} {{ t(item.labelKey) }}
                </el-dropdown-item>
              </template>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </div>
      <div class="module-list">
        <div
          v-for="(mod, idx) in ed.currentPageModules"
          :key="mod.id"
          class="module-item-row"
          :class="{ selected: ed.selectedModuleId === mod.id, disabled: !mod.visible }"
          @click="ed.selectModule(mod.id)"
        >
          <div class="module-item-info">
            <span class="module-type-badge" :class="'badge-' + mod.type">{{ ed.typeLabel(mod.type) }}</span>
            <span class="module-title-text">{{ mod.title || t('portal.noTitle') }}</span>
          </div>
          <div class="module-item-actions" @click.stop>
            <el-button text size="small" :icon="ArrowUp" @click="ed.movePageModule(idx, -1)" :disabled="idx === 0" />
            <el-button text size="small" :icon="ArrowDown" @click="ed.movePageModule(idx, 1)" :disabled="idx === ed.currentPageModules.length - 1" />
            <el-switch v-model="mod.visible" size="small" @change="ed.onCodeChange" />
            <el-button text size="small" :icon="CopyDocument" @click="ed.duplicateModuleByIdx(idx)" />
            <el-button text size="small" type="danger" :icon="Delete" @click="ed.removeModuleByIdx(idx)" />
          </div>
        </div>
        <div v-if="!ed.currentNav" class="empty-hint no-click">
          {{ t('portal.clickNavHint') }}
        </div>
        <div v-else-if="ed.currentPageModules.length === 0" class="empty-hint" @click="ed.addModule('hero')">
          {{ t('portal.addFirstModule', { name: ed.currentNav.label }) }}
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { inject } from 'vue'
import { useI18n } from 'vue-i18n'
import { Plus, Delete, ArrowUp, ArrowDown, CopyDocument } from '@element-plus/icons-vue'

const { t } = useI18n()
const ed = inject('portalEditor')
</script>

<style scoped>
.col-structure {
  width: 320px;
  background: #fff;
  overflow-y: auto;
  display: flex;
  flex-direction: column;
  border-right: 1px solid #e8e8e8;
  flex-shrink: 0;
  padding: 10px;
  gap: 10px;
  min-height: 0;
}

.structure-section:last-child { flex: 1; display: flex; flex-direction: column; }

.section-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 8px;
  padding: 0 4px;
}
.section-title { font-size: 11px; font-weight: 600; color: #909399; text-transform: uppercase; letter-spacing: 0.5px; }

.nav-list { display: flex; flex-direction: column; gap: 4px; max-height: 180px; overflow-y: auto; overflow-x: hidden; }
.nav-item-row {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 8px;
  border-radius: 6px;
  cursor: pointer;
  background: #f8f9fa;
  border: 1px solid transparent;
  transition: all 0.2s ease;
}
.nav-item-row:hover { background: #fff; border-color: #d9d9d9; box-shadow: 0 2px 8px rgba(0, 0, 0, 0.06); }
.nav-item-row.selected { background: #e6f7ff; border-color: #1890ff; box-shadow: 0 2px 8px rgba(24, 144, 255, 0.15); }
.nav-item-row.disabled { opacity: 0.5; }

.nav-icon-emoji { font-size: 12px; flex-shrink: 0; }
.nav-item-info { display: flex; align-items: center; gap: 4px; flex: 1; overflow: hidden; min-width: 0; }
.nav-label-text {
  font-size: 11px;
  color: #333;
  font-weight: 500;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  min-width: 0;
}
.nav-item-info .el-tag { flex-shrink: 0; font-size: 10px; padding: 0 4px; }

.nav-item-actions { display: flex; align-items: center; gap: 2px; flex-shrink: 0; }
.nav-item-actions .el-button { padding: 4px; }

.structure-modules { flex: 1; display: flex; flex-direction: column; overflow: hidden; margin-top: 8px; min-height: 200px; }
.module-list { flex: 1; overflow-y: auto; display: flex; flex-direction: column; gap: 6px; }
.module-item-row {
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 8px;
  border-radius: 6px;
  cursor: pointer;
  background: #fafafa;
  border: 1px solid #f0f0f0;
  transition: all 0.2s;
}
.module-item-row:hover { background: #f0f5ff; border-color: #d6e4ff; }
.module-item-row.selected { background: #e6f7ff; border-color: #1890ff; }
.module-item-row.disabled { opacity: 0.5; }

.module-item-info { display: flex; align-items: center; gap: 6px; flex: 1; overflow: hidden; min-width: 0; }
.module-type-badge {
  display: inline-flex;
  align-items: center;
  padding: 1px 4px;
  border-radius: 3px;
  font-size: 10px;
  font-weight: 500;
  flex-shrink: 0;
  background: #f4f4f5;
  color: #606266;
}
.module-title-text {
  font-size: 11px;
  color: #333;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  flex: 1;
  min-width: 0;
}
.module-item-actions { display: flex; align-items: center; gap: 1px; flex-shrink: 0; }
.module-item-actions .el-button { padding: 4px; }

/* Badge colors (single source — previously duplicated with conflicting values) */
.badge-hero { background: #e8f5e9; color: #4caf50; }
.badge-markdown { background: #e3f2fd; color: #1976d2; }
.badge-rich_text { background: #e8eaf6; color: #5c6bc0; }
.badge-notice { background: #fff3e0; color: #ff9800; }
.badge-freeform { background: #e8f5e9; color: #2e7d32; }
.badge-feature_cards { background: #f3e5f5; color: #9c27b0; }
.badge-tutorial_cards { background: #fffde7; color: #f9a825; }
.badge-article_list { background: #eceff1; color: #546e7a; }
.badge-account_stats { background: #e0f7fa; color: #00bcd4; }
.badge-subscription { background: #fce4ec; color: #e91e63; }
.badge-custom_html { background: #efebe9; color: #795548; }
.badge-account_info { background: #e8f5e9; color: #4caf50; }
.badge-traffic_ring { background: #e3f2fd; color: #1976d2; }
.badge-subscription_detail { background: #fce4ec; color: #e91e63; }
.badge-client_download { background: #e8eaf6; color: #3f51b5; }
.badge-streaming_status { background: #f3e5f5; color: #8e24aa; }
.badge-security_settings { background: #fff3e0; color: #ff9800; }
.badge-log_packages { background: #f3e5f5; color: #9c27b0; }
.badge-shared_accounts { background: #fff8e1; color: #ffb300; }

.empty-hint {
  text-align: center;
  padding: 16px;
  color: #909399;
  font-size: 13px;
  cursor: pointer;
  border: 1px dashed #dcdfe6;
  border-radius: 6px;
}
.empty-hint:hover { color: #409eff; border-color: #409eff; }
.empty-hint.no-click { cursor: default; }
.empty-hint.no-click:hover { color: #909399; border-color: #dcdfe6; }

.module-group-header { font-size: 11px; color: #909399; font-weight: 600; }

@media (max-width: 1400px) {
  .col-structure { width: 200px; }
}
@media (max-width: 1100px) {
  .col-structure { width: 100%; max-height: 250px; }
}
</style>

<template>
  <el-dialog v-model="ed.navDialogVisible" :title="t('portal.addNavPage')" width="500px">
    <el-form label-position="top" size="small">
      <el-form-item :label="t('portal.pageName')"><el-input v-model="ed.newNav.label" :placeholder="t('portal.pageNamePlaceholder')" /></el-form-item>
      <el-form-item :label="t('portal.iconOptional')"><el-input v-model="ed.newNav.icon" :placeholder="t('portal.iconPlaceholder2')" /></el-form-item>
      <el-form-item :label="t('portal.externalLink')"><el-input v-model="ed.newNav.href" :placeholder="t('portal.externalLinkPlaceholder2')" /></el-form-item>
    </el-form>
    <div style="font-size: 13px; color: #909399; margin-top: 8px">{{ t('portal.addNavHint') }}</div>
    <template #footer>
      <el-button @click="ed.navDialogVisible = false">{{ t('common.cancel') }}</el-button>
      <el-button type="primary" @click="ed.confirmAddNav">{{ t('common.create') }}</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { inject } from 'vue'
import { useI18n } from 'vue-i18n'

const { t } = useI18n()
const ed = inject('portalEditor')
</script>

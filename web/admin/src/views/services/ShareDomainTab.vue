<template>
  <div>
    <!-- Direct Domains -->
    <el-card style="margin-bottom:16px">
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span style="font-weight:600">{{ t('service.shareDomain.direct') }}</span>
          <el-button size="small" type="primary" @click="addItem('direct')">+ {{ t('service.shareDomain.add') }}</el-button>
        </div>
      </template>
      <div class="hint" style="margin-bottom:12px">{{ t('service.shareDomain.directHint') }}</div>
      <div v-for="item in directItems" :key="item.id" class="domain-item">
        <div style="display:flex;gap:8px;align-items:center">
          <el-switch v-model="item.enabled" @change="updateItem(item)" />
          <el-input v-model="item.value" :placeholder="t('service.shareDomain.domainOrIP')" style="flex:1" @change="updateItem(item)" />
          <el-input-number v-model="item.port" :min="1" :max="65535" style="width:120px" @change="updateItem(item)" />
          <el-button size="small" type="danger" text @click="deleteItem(item)">X</el-button>
        </div>
      </div>
      <div v-if="!directItems.length" style="text-align:center;color:#909399;padding:16px">{{ t('common.noData') }}</div>
    </el-card>

    <!-- CF Priority Domains -->
    <el-card>
      <template #header>
        <div style="display:flex;justify-content:space-between;align-items:center">
          <span style="font-weight:600">{{ t('service.shareDomain.cfDomain') }}</span>
          <div style="display:flex;gap:8px">
            <el-button size="small" @click="seedDefaults">{{ t('service.shareDomain.seed') }}</el-button>
            <el-button size="small" type="danger" @click="clearAll('cf_domain')">{{ t('service.shareDomain.clearAll') }}</el-button>
            <el-button size="small" type="primary" @click="addItem('cf_domain')">+ {{ t('service.shareDomain.add') }}</el-button>
          </div>
        </div>
      </template>
      <div class="hint" style="margin-bottom:12px">{{ t('service.shareDomain.cfDomainHint') }}</div>
      <div v-for="item in cfDomainItems" :key="item.id" class="domain-item">
        <div style="display:flex;gap:8px;align-items:center;flex-wrap:wrap">
          <el-switch v-model="item.enabled" @change="updateItem(item)" />
          <el-input v-model="item.value" :placeholder="t('service.shareDomain.domain')" style="flex:1;min-width:200px" @change="updateItem(item)" />
          <div style="display:flex;gap:4px;align-items:center;flex-wrap:wrap">
            <span style="font-size:12px;color:#909399">HTTP:</span>
            <el-checkbox v-for="p in cfHttpPorts" :key="p" :model-value="item.ports.includes(p)" @change="(v) => togglePort(item, p, v)" size="small">
              {{ p }}
            </el-checkbox>
            <span style="font-size:12px;color:#909399;margin-left:8px">HTTPS:</span>
            <el-checkbox v-for="p in cfHttpsPorts" :key="p" :model-value="item.ports.includes(p)" @change="(v) => togglePort(item, p, v)" size="small">
              {{ p }}
            </el-checkbox>
          </div>
          <el-button size="small" type="danger" text @click="deleteItem(item)">X</el-button>
        </div>
      </div>
      <div v-if="!cfDomainItems.length" style="text-align:center;color:#909399;padding:16px">{{ t('common.noData') }}</div>
    </el-card>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import client from '../../api/client'

const { t } = useI18n()
const items = ref([])
const loading = ref(false)

// CF supported ports
const cfHttpPorts = [80, 8080, 8880, 2052, 2082, 2086, 2095]
const cfHttpsPorts = [443, 8443, 2053, 2083, 2087, 2096]

const directItems = computed(() => items.value.filter(i => i.type === 'direct'))
const cfDomainItems = computed(() => items.value.filter(i => i.type === 'cf_domain'))

async function fetchItems() {
  loading.value = true
  try {
    const data = await client.get('/share-domains')
    items.value = Array.isArray(data) ? data.map(item => ({
      ...item,
      ports: item.ports ? JSON.parse(item.ports) : [item.port || 443]
    })) : []
  } catch {
    items.value = []
  } finally {
    loading.value = false
  }
}

async function addItem(type) {
  const defaultPorts = type === 'cf_domain' ? [80, 443] : []
  try {
    const newItem = await client.post('/share-domains', {
      type,
      value: '',
      port: defaultPorts[0] || 443,
      ports: JSON.stringify(defaultPorts)
    })
    items.value.push({ ...newItem, ports: defaultPorts })
  } catch (e) {
    ElMessage.error(t('common.failed'))
  }
}

async function updateItem(item) {
  try {
    await client.put(`/share-domains/${item.id}`, {
      value: item.value,
      port: item.ports && item.ports.length ? item.ports[0] : 443,
      ports: JSON.stringify(item.ports || []),
      enabled: item.enabled
    })
  } catch (e) {
    ElMessage.error(t('common.failed'))
  }
}

function togglePort(item, port, checked) {
  if (!item.ports) item.ports = []
  if (checked) {
    if (!item.ports.includes(port)) item.ports.push(port)
  } else {
    item.ports = item.ports.filter(p => p !== port)
  }
  updateItem(item)
}

async function deleteItem(item) {
  try {
    await ElMessageBox.confirm(t('common.confirm'), t('common.confirm'))
    await client.delete(`/share-domains/${item.id}`)
    items.value = items.value.filter(i => i.id !== item.id)
    ElMessage.success(t('common.success'))
  } catch {}
}

async function clearAll(type) {
  try {
    await ElMessageBox.confirm(t('service.shareDomain.clearAllConfirm'), t('common.confirm'))
    await client.delete(`/share-domains/all?type=${type}`)
    items.value = items.value.filter(i => i.type !== type)
    ElMessage.success(t('common.success'))
  } catch {}
}

async function seedDefaults() {
  try {
    await client.post('/share-domains/seed')
    await fetchItems()
    ElMessage.success(t('common.success'))
  } catch (e) {
    ElMessage.error(t('common.failed'))
  }
}

onMounted(fetchItems)
</script>

<style scoped>
.domain-item {
  padding: 8px 0;
  border-bottom: 1px solid var(--el-border-color-lighter);
}
.domain-item:last-child {
  border-bottom: none;
}
.hint {
  font-size: 12px;
  color: #909399;
}
</style>

<template>
  <div>
    <div style="display:flex;justify-content:space-between;margin-bottom:20px">
      <h2>{{ t('certificate.title') }}</h2>
      <div>
        <el-button @click="showEmailDialog">{{ t('certificate.manageEmails') }}</el-button>
        <el-button @click="showChannelDialog">{{ t('certificate.manageChannels') }}</el-button>
        <el-button @click="showUploadDialog">{{ t('certificate.upload') }}</el-button>
        <el-button type="primary" @click="showIssueDialog">{{ t('certificate.issue') }}</el-button>
      </div>
    </div>

    <!-- Certificate List -->
    <el-card>
      <el-table :data="certificates" v-loading="loading" stripe>
        <el-table-column prop="name" :label="t('certificate.certName')" width="160" />
        <el-table-column :label="t('certificate.domain')" min-width="300">
          <template #default="{row}">
            <div v-if="row.domains" style="display:flex;flex-wrap:wrap;gap:2px">
              <el-tag v-for="d in parseDomains(row.domains)" :key="d" size="small" :type="d.startsWith('*.')?'warning':''">{{ d }}</el-tag>
            </div>
            <span v-else>{{ row.domain }}</span>
          </template>
        </el-table-column>
        <el-table-column :label="t('certificate.keyType')" width="120">
          <template #default="{row}">
            <el-tag v-if="row.key_type" :type="row.key_type==='ecc'?'success':'primary'" size="small">
              {{ row.key_type.toUpperCase() }} {{ row.key_bits }}
            </el-tag>
            <span v-else>-</span>
          </template>
        </el-table-column>
        <el-table-column prop="issuer" :label="t('certificate.issuer')" width="120" />
        <el-table-column :label="t('common.status')" width="80">
          <template #default="{row}">
            <el-tag :type="row.status==='active'?'success':'danger'" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('certificate.expiresAt')" width="180">
          <template #default="{row}">{{ formatDate(row.expires_at) }}</template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="340">
          <template #default="{row}">
            <el-button size="small" type="primary" plain :loading="row._distributing" @click="handleDistribute(row)">{{ t('certificate.distribute') }}</el-button>
            <el-button size="small" type="warning" plain @click="showMigrateDialog(row)">{{ t('certificate.migrate') }}</el-button>
            <el-button v-if="row.provider_type!=='upload'" size="small" @click="handleRenew(row)" :loading="row._renewing">{{ t('certificate.renew') }}</el-button>
            <el-button size="small" type="danger" @click="handleDelete(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <PaginationBar v-model:page="page" v-model:page-size="pageSize" :total="total" @change="fetchCertificates" />
    </el-card>

    <!-- Issue Certificate Dialog -->
    <el-dialog v-model="issueVisible" :title="t('certificate.issue')" width="700">
      <el-form :model="issueForm" label-width="120px">
        <el-form-item :label="t('certificate.email')">
          <el-select v-model="issueForm.email_id" style="width:100%" :placeholder="t('certificate.selectEmail')">
            <el-option v-for="e in emails" :key="e.id" :label="`${e.email} (${e.ca_provider})`" :value="e.id" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('certificate.caProvider')">
          <el-select v-model="issueForm.ca_provider" style="width:100%">
            <el-option label="Let's Encrypt" value="lets_encrypt" />
            <el-option label="ZeroSSL" value="zerossl" />
            <el-option label="Google Public CA" value="google" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('certificate.certTarget')">
          <el-radio-group v-model="issueForm.cert_target" @change="onCertTargetChange">
            <el-radio value="domain">{{ t('certificate.domainCert') }}</el-radio>
            <el-radio value="ip" :disabled="issueForm.ca_provider!=='lets_encrypt'">{{ t('certificate.ipCert') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="issueForm.cert_target==='domain'" :label="t('certificate.challengeType')">
          <el-radio-group v-model="issueForm.challenge_type">
            <el-radio value="http-01">HTTP-01</el-radio>
            <el-radio value="dns-01">DNS-01</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="issueForm.cert_target==='domain' && issueForm.challenge_type==='dns-01'" :label="t('certificate.dnsChannel')">
          <el-select v-model="issueForm.dns_channel_id" style="width:100%" :placeholder="t('certificate.selectChannel')">
            <el-option v-for="ch in channels" :key="ch.id" :label="`${ch.name} (${ch.provider})`" :value="ch.id" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="issueForm.cert_target==='domain'" :label="t('certificate.domains')">
          <el-input v-model="issueForm.domains_text" type="textarea" :rows="4" placeholder="example.com&#10;*.example.com&#10;sub.example.com" />
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('certificate.domainsHint') }}</div>
        </el-form-item>
        <el-form-item v-if="issueForm.cert_target==='ip'" :label="t('certificate.ips')">
          <div style="display:flex;gap:8px">
            <el-input v-model="issueForm.ips_text" type="textarea" :rows="3" placeholder="1.2.3.4&#10;2001:db8::1" style="flex:1" />
            <el-button @click="handleDetectIP" :loading="detectingIP">{{ t('certificate.detectIP') }}</el-button>
          </div>
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('certificate.ipsHint') }}</div>
        </el-form-item>
        <el-form-item :label="t('certificate.keyType')">
          <el-select v-model="issueForm.key_type" style="width:50%" @change="onKeyTypeChange">
            <el-option label="ECC" value="ecc" />
            <el-option label="RSA" value="rsa" />
          </el-select>
          <el-select v-model="issueForm.key_bits" style="width:50%">
            <el-option v-for="b in keyBitsOptions" :key="b" :label="b" :value="b" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('certificate.dualKey')">
          <el-checkbox v-model="issueForm.dual_key">{{ t('certificate.dualKeyHint') }}</el-checkbox>
        </el-form-item>
        <el-form-item v-if="issueForm.ca_provider!=='lets_encrypt'" :label="'EAB-KID'">
          <el-input v-model="issueForm.eab_key_id" :placeholder="t('certificate.eabKeyIDPlaceholder')" />
        </el-form-item>
        <el-form-item v-if="issueForm.ca_provider!=='lets_encrypt'" :label="'EAB-HMAC-Key'">
          <el-input v-model="issueForm.eab_mac_key" :placeholder="t('certificate.eabMACKeyPlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('certificate.autoRenew')">
          <el-switch v-model="issueForm.auto_renew" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="issueVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="issuing" @click="handleIssue">{{ t('certificate.issue') }}</el-button>
      </template>
    </el-dialog>

    <!-- DNS Channel Management Dialog -->
    <el-dialog v-model="channelVisible" :title="t('certificate.manageChannels')" width="700">
      <div style="margin-bottom:16px">
        <el-button type="primary" size="small" @click="showAddChannel">{{ t('certificate.addChannel') }}</el-button>
      </div>
      <el-table :data="channels" stripe>
        <el-table-column prop="name" :label="t('common.name')" width="150" />
        <el-table-column prop="provider" :label="t('certificate.dnsProvider')" width="120" />
        <el-table-column :label="t('certificate.envVars')" min-width="200">
          <template #default="{row}">
            <div v-if="row.env_vars" style="display:flex;flex-wrap:wrap;gap:4px;max-width:100%;overflow:hidden">
              <el-tag v-for="(v, k) in row.env_vars" :key="k" size="small" type="info">
                {{ k }}: {{ v }}
              </el-tag>
            </div>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="120">
          <template #default="{row}">
            <el-button size="small" @click="showEditChannel(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" type="danger" @click="handleDeleteChannel(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- Add/Edit Channel Dialog -->
    <el-dialog v-model="channelFormVisible" :title="editingChannel ? t('common.edit') : t('certificate.addChannel')" width="600">
      <el-form :model="channelForm" label-width="140px">
        <el-form-item :label="t('common.name')">
          <el-input v-model="channelForm.name" />
        </el-form-item>
        <el-form-item :label="t('certificate.dnsProvider')">
          <el-select v-model="channelForm.provider" style="width:100%" @change="onProviderChange">
            <el-option v-for="(fields, p) in providerOptions" :key="p" :label="providerLabel(p)" :value="p" />
          </el-select>
        </el-form-item>
        <el-divider v-if="currentProviderFields.length > 0" />
        <el-form-item v-for="field in currentProviderFields" :key="field" :label="shortLabel(field)">
          <el-input v-model="channelForm.env_vars[field]" :placeholder="field" show-password style="width:100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="channelFormVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="channelSaving" @click="handleSaveChannel">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Email Management Dialog -->
    <el-dialog v-model="emailVisible" :title="t('certificate.manageEmails')" width="700">
      <div style="margin-bottom:16px">
        <el-button type="primary" size="small" @click="showAddEmail">{{ t('certificate.addEmail') }}</el-button>
      </div>
      <el-table :data="emails" stripe>
        <el-table-column prop="email" :label="t('certificate.email')" />
        <el-table-column :label="t('certificate.default')" width="100">
          <template #default="{row}">
            <el-tag v-if="row.is_default" type="success" size="small">{{ t('common.yes') }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column :label="t('common.actions')" width="120">
          <template #default="{row}">
            <el-button size="small" @click="showEditEmail(row)">{{ t('common.edit') }}</el-button>
            <el-button size="small" type="danger" @click="handleDeleteEmail(row)">{{ t('common.delete') }}</el-button>
          </template>
        </el-table-column>
      </el-table>
    </el-dialog>

    <!-- Add/Edit Email Dialog -->
    <el-dialog v-model="emailFormVisible" :title="editingEmail ? t('common.edit') : t('certificate.addEmail')" width="500">
      <el-form :model="emailForm" label-width="80px">
        <el-form-item :label="t('certificate.email')">
          <el-input v-model="emailForm.email" placeholder="admin@example.com" />
        </el-form-item>
        <el-form-item :label="t('certificate.default')">
          <el-switch v-model="emailForm.is_default" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="emailFormVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="emailSaving" @click="handleSaveEmail">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Upload Certificate Dialog -->
    <el-dialog v-model="uploadVisible" :title="t('certificate.upload')" width="650">
      <el-form :model="uploadForm" label-width="100px">
        <el-form-item :label="t('certificate.certName')">
          <el-input v-model="uploadForm.name" :placeholder="t('certificate.certNamePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('certificate.domain')">
          <el-input v-model="uploadForm.domain" :placeholder="t('certificate.domainPlaceholder')" />
          <div style="font-size:11px;color:#999;margin-top:4px">{{ t('certificate.domainAutoDetect') }}</div>
        </el-form-item>
        <el-form-item :label="t('certificate.uploadMethod')">
          <el-radio-group v-model="uploadForm.method">
            <el-radio value="paste">{{ t('certificate.pasteContent') }}</el-radio>
            <el-radio value="file">{{ t('certificate.uploadFile') }}</el-radio>
          </el-radio-group>
        </el-form-item>
        <template v-if="uploadForm.method==='paste'">
          <el-form-item :label="t('certificate.fullchain')">
            <el-input v-model="uploadForm.cert_content" type="textarea" :rows="5" placeholder="-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----&#10;-----BEGIN CERTIFICATE-----&#10;...&#10;-----END CERTIFICATE-----" />
            <div style="font-size:11px;color:#999;margin-top:4px">{{ t('certificate.fullchainHint') }}</div>
          </el-form-item>
          <el-form-item label="Key PEM">
            <el-input v-model="uploadForm.key_content" type="textarea" :rows="3" placeholder="-----BEGIN PRIVATE KEY-----&#10;...&#10;-----END PRIVATE KEY-----" />
          </el-form-item>
        </template>
        <template v-else>
          <el-form-item :label="t('certificate.fullchain')">
            <el-upload
              drag
              :auto-upload="false"
              :limit="1"
              :on-change="(f) => handleFileRead(f, 'cert')"
              :on-remove="() => uploadForm.cert_content = ''"
              accept=".pem,.crt,.cer,.txt"
            >
              <el-icon style="font-size:40px;color:#c0c4cc"><UploadFilled /></el-icon>
              <div style="color:#999;font-size:13px">{{ t('certificate.dragOrClick') }}</div>
              <template #tip>
                <div style="font-size:11px;color:#999">{{ t('certificate.fullchainHint') }}</div>
              </template>
            </el-upload>
            <div v-if="uploadForm.cert_content" style="margin-top:8px">
              <el-tag type="success" size="small">{{ t('certificate.fileLoaded') }} ({{ uploadForm.cert_content.length }} bytes)</el-tag>
            </div>
          </el-form-item>
          <el-form-item label="Key">
            <el-upload
              drag
              :auto-upload="false"
              :limit="1"
              :on-change="(f) => handleFileRead(f, 'key')"
              :on-remove="() => uploadForm.key_content = ''"
              accept=".pem,.key,.txt"
            >
              <el-icon style="font-size:40px;color:#c0c4cc"><UploadFilled /></el-icon>
              <div style="color:#999;font-size:13px">{{ t('certificate.dragOrClick') }}</div>
            </el-upload>
            <div v-if="uploadForm.key_content" style="margin-top:8px">
              <el-tag type="success" size="small">{{ t('certificate.fileLoaded') }} ({{ uploadForm.key_content.length }} bytes)</el-tag>
            </div>
          </el-form-item>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="uploadVisible=false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="uploading" @click="handleUpload">{{ t('certificate.upload') }}</el-button>
      </template>
    </el-dialog>

    <!-- Migrate Certificate Dialog -->
    <el-dialog v-model="migrateVisible" :title="t('certificate.migrateTitle')" width="600">
      <el-alert type="warning" :closable="false" style="margin-bottom:16px">
        {{ t('certificate.migrateWarning') }}
      </el-alert>
      <el-descriptions :column="1" border style="margin-bottom:16px">
        <el-descriptions-item :label="t('certificate.oldCert')">
          <span v-if="migrateRow"><el-tag type="warning">{{ migrateRow.domain }}</el-tag> (ID: {{ migrateRow.id }}, {{ migrateRow.issuer }})</span>
        </el-descriptions-item>
      </el-descriptions>
      <el-form label-width="120px">
        <el-form-item :label="t('certificate.newCert')">
          <el-select v-model="migrateTargetId" style="width:100%" :placeholder="t('certificate.selectNewCert')" filterable>
            <el-option v-for="c in migrateCandidates" :key="c.id" :label="`${c.domain} (${c.issuer})`" :value="c.id" />
          </el-select>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="migrateVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="migrating" @click="handleMigrate">{{ t('certificate.migrate') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { UploadFilled } from '@element-plus/icons-vue'
import { useI18n } from 'vue-i18n'
import { useQueryState } from '../composables/useQueryState'
import PaginationBar from '../components/PaginationBar.vue'
import client from '../api/client'

const { t } = useI18n()

// Certificates
const certificates = ref([])
const loading = ref(false)
const page = useQueryState('certPage', 1, { type: 'number' })
const pageSize = useQueryState('certPageSize', 20, { type: 'number' })
const total = ref(0)

// Issue
const issueVisible = ref(false)
const issuing = ref(false)
const detectingIP = ref(false)
const issueForm = ref({
  email_id: null,
  ca_provider: 'lets_encrypt',
  challenge_type: 'dns-01',
  dns_channel_id: null,
  domains_text: '',
  ips_text: '',
  key_type: 'ecc',
  key_bits: 256,
  dual_key: false,
  eab_key_id: '',
  eab_mac_key: '',
  auto_renew: true
})

// DNS Channels
const channels = ref([])
const channelVisible = ref(false)
const channelFormVisible = ref(false)
const channelSaving = ref(false)
const editingChannel = ref(null)
const channelForm = ref({ name: '', provider: 'cloudflare', env_vars: {} })
const providerOptions = ref({})

// Emails
const emails = ref([])
const emailVisible = ref(false)
const emailFormVisible = ref(false)
const emailSaving = ref(false)
const editingEmail = ref(null)
const emailForm = ref({ email: '', ca_provider: 'lets_encrypt', eab_key_id: '', eab_mac_key: '', is_default: false })

const keyBitsOptions = computed(() => {
  if (issueForm.value.key_type === 'ecc') return [384, 256]
  return [8192, 4096, 2048]
})

const currentProviderFields = computed(() => {
  return providerOptions.value[channelForm.value.provider] || []
})

const providerLabels = {
  cloudflare: 'Cloudflare',
  alidns: 'Alibaba Cloud DNS',
  huawei: 'Huawei Cloud DNS',
  dnspod: 'DNSPod',
  route53: 'AWS Route 53',
  google: 'Google Cloud DNS',
  godaddy: 'GoDaddy',
  namecheap: 'Namecheap'
}

function providerLabel(p) { return providerLabels[p] || p }

function parseDomains(domains) {
  try { return JSON.parse(domains) } catch { return [domains] }
}

function formatDate(d) {
  if (!d || d === '0001-01-01T00:00:00Z') return '-'
  return new Date(d).toLocaleString()
}

// --- Certificates ---
async function fetchCertificates() {
  loading.value = true
  try {
    const data = await client.get('/certificates', { params: { limit: pageSize.value, offset: (page.value - 1) * pageSize.value } })
    certificates.value = (data.items || []).map(c => ({ ...c, _renewing: false, _distributing: false }))
    total.value = data.total || 0
  } finally { loading.value = false }
}

function showIssueDialog() {
  issueForm.value = { email_id: null, ca_provider: 'lets_encrypt', cert_target: 'domain', challenge_type: 'dns-01', dns_channel_id: null, domains_text: '', ips_text: '', key_type: 'ecc', key_bits: 256, dual_key: false, eab_key_id: '', eab_mac_key: '', auto_renew: true }
  issueVisible.value = true
}

async function handleDetectIP() {
  detectingIP.value = true
  try {
    const data = await client.get('/certificates/detect-ip')
    const ips = []
    if (data.ipv4) ips.push(data.ipv4)
    if (data.ipv6) ips.push(data.ipv6)
    issueForm.value.ips_text = ips.join('\n')
    ElMessage.success(t('common.success'))
  } catch (e) {} finally { detectingIP.value = false }
}

async function handleIssue() {
  const isIP = issueForm.value.cert_target === 'ip'
  const domains = isIP ? [] : issueForm.value.domains_text.split('\n').map(d => d.trim()).filter(Boolean)
  const ips = isIP ? issueForm.value.ips_text.split('\n').map(i => i.trim()).filter(Boolean) : []
  if (!isIP && domains.length === 0) { ElMessage.error(t('certificate.domainsRequired')); return }
  if (isIP && ips.length === 0) { ElMessage.error(t('certificate.ipsRequired')); return }

  // Check wildcard domain with HTTP-01
  const challengeType = isIP ? 'http-01' : issueForm.value.challenge_type
  if (challengeType === 'http-01' && !isIP) {
    const wildcards = domains.filter(d => d.startsWith('*.'))
    if (wildcards.length > 0) {
      ElMessage.error(t('certificate.wildcardNotAllowed'))
      return
    }
  }

  // Check port 80 for HTTP-01
  if (challengeType === 'http-01') {
    try {
      const portCheck = await client.get('/certificates/check-port', { params: { port: '80' } })
      if (portCheck.in_use) {
        await ElMessageBox.confirm(
          t('certificate.port80InUse', { process: portCheck.process }),
          t('common.confirm'),
          { confirmButtonText: t('certificate.continueAnyway'), cancelButtonText: t('common.cancel'), type: 'warning' }
        )
      }
    } catch (e) {
      // Port check failed, continue anyway
    }
  }
  const payload = {
    domains,
    ips,
    ca_provider: issueForm.value.ca_provider,
    challenge_type: isIP ? 'http-01' : issueForm.value.challenge_type,
    key_type: issueForm.value.key_type,
    key_bits: issueForm.value.key_bits,
    dns_channel_id: isIP ? 0 : (issueForm.value.dns_channel_id || 0),
    email_id: issueForm.value.email_id || 0,
    eab_key_id: issueForm.value.eab_key_id,
    eab_mac_key: issueForm.value.eab_mac_key,
    auto_renew: issueForm.value.auto_renew
  }
  issuing.value = true
  try {
    await client.post('/certificates/issue', payload)
    ElMessage.success(t('common.success'))
    issueVisible.value = false
    fetchCertificates()
    if (issueForm.value.dual_key) {
      const p2 = { ...payload, key_type: payload.key_type === 'ecc' ? 'rsa' : 'ecc', key_bits: payload.key_type === 'ecc' ? 2048 : 256 }
      await client.post('/certificates/issue', p2)
      fetchCertificates()
    }
  } catch (e) {} finally { issuing.value = false }
}

async function handleRenew(row) {
  row._renewing = true
  try { await client.post(`/certificates/${row.id}/renew`); ElMessage.success(t('common.success')); fetchCertificates() } catch (e) {} finally { row._renewing = false }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(t('certificate.deleteConfirm', { domain: row.domain || row.name }), t('common.confirm'))
  await client.delete(`/certificates/${row.id}`); ElMessage.success(t('common.success')); fetchCertificates()
}

async function handleDistribute(row) {
  row._distributing = true
  try {
    await client.post('/node-servers/distribute-cert', { certificate_id: row.id })
    ElMessage.success(t('certificate.distributeSuccess'))
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('certificate.distributeFailed'))
  } finally {
    row._distributing = false
  }
}

// --- Migrate Certificate ---
const migrateVisible = ref(false)
const migrateRow = ref(null)
const migrateTargetId = ref(null)
const migrating = ref(false)
const migrateCandidates = ref([])

function showMigrateDialog(row) {
  migrateRow.value = row
  migrateTargetId.value = null
  migrateCandidates.value = certificates.value.filter(c => c.id !== row.id)
  migrateVisible.value = true
}

async function handleMigrate() {
  if (!migrateTargetId.value) {
    ElMessage.warning(t('certificate.selectNewCert'))
    return
  }
  await ElMessageBox.confirm(
    t('certificate.migrateConfirm', { old: migrateRow.value.domain, new: migrateCandidates.value.find(c => c.id === migrateTargetId.value)?.domain }),
    t('common.confirm'),
    { confirmButtonText: t('common.confirm'), cancelButtonText: t('common.cancel'), type: 'warning' }
  )
  migrating.value = true
  try {
    const data = await client.post('/node-servers/migrate-cert', {
      old_cert_id: migrateRow.value.id,
      new_cert_id: migrateTargetId.value
    })
    ElMessage.success(t('certificate.migrateSuccess'))
    migrateVisible.value = false
    fetchCertificates()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('certificate.migrateFailed'))
  } finally {
    migrating.value = false
  }
}

// --- DNS Channels ---
async function fetchChannels() { try { channels.value = await client.get('/cert-channels') || [] } catch (e) {} }
async function fetchProviders() { try { providerOptions.value = await client.get('/cert-channels/providers') || {} } catch (e) {} }
function showChannelDialog() { channelVisible.value = true; fetchChannels() }
function showAddChannel() { editingChannel.value = null; channelForm.value = { name: '', provider: 'cloudflare', env_vars: {} }; channelFormVisible.value = true }
function showEditChannel(row) { editingChannel.value = row; channelForm.value = { name: row.name, provider: row.provider, env_vars: {} }; channelFormVisible.value = true }
function onProviderChange() { channelForm.value.env_vars = {} }

function onKeyTypeChange(val) {
  issueForm.value.key_bits = val === 'ecc' ? 384 : 8192
}

function onCertTargetChange(val) {
  if (val === 'ip') {
    issueForm.value.challenge_type = 'http-01'
    issueForm.value.domains_text = ''
  } else {
    issueForm.value.ips_text = ''
  }
}

function shortLabel(field) {
  // Shorten env var names for display
  const map = {
    'CLOUDFLARE_EMAIL': 'Email',
    'CLOUDFLARE_API_KEY': 'API Key',
    'ALICLOUD_ACCESS_KEY': 'Access Key',
    'ALICLOUD_SECRET_KEY': 'Secret Key',
    'HWC_ACCESS_KEY': 'Access Key',
    'HWC_SECRET_KEY': 'Secret Key',
    'HWC_REGION': 'Region',
    'HWC_PROJECT_ID': 'Project ID',
    'DNSPOD_API_KEY': 'API Key',
    'AWS_ACCESS_KEY_ID': 'Access Key ID',
    'AWS_SECRET_ACCESS_KEY': 'Secret Access Key',
    'AWS_REGION': 'Region',
    'GCE_PROJECT': 'Project',
    'GCE_SERVICE_ACCOUNT_FILE': 'Service Account File',
    'GODADDY_API_KEY': 'API Key',
    'GODADDY_API_SECRET': 'API Secret',
    'NAMECHEAP_API_USER': 'API User',
    'NAMECHEAP_API_KEY': 'API Key'
  }
  return map[field] || field
}
async function handleSaveChannel() {
  channelSaving.value = true
  try {
    if (editingChannel.value) { await client.put(`/cert-channels/${editingChannel.value.id}`, channelForm.value) }
    else { await client.post('/cert-channels', channelForm.value) }
    ElMessage.success(t('common.success')); channelFormVisible.value = false; fetchChannels()
  } finally { channelSaving.value = false }
}
async function handleDeleteChannel(row) { await ElMessageBox.confirm(t('certificate.deleteChannelConfirm', { name: row.name }), t('common.confirm')); await client.delete(`/cert-channels/${row.id}`); ElMessage.success(t('common.success')); fetchChannels() }

// --- Emails ---
async function fetchEmails() { try { emails.value = await client.get('/acme-accounts') || [] } catch (e) {} }
function showEmailDialog() { emailVisible.value = true; fetchEmails() }
function showAddEmail() { editingEmail.value = null; emailForm.value = { email: '', is_default: false }; emailFormVisible.value = true }
function showEditEmail(row) { editingEmail.value = row; emailForm.value = { email: row.email, is_default: row.is_default }; emailFormVisible.value = true }
async function handleSaveEmail() {
  emailSaving.value = true
  try {
    if (editingEmail.value) { await client.put(`/acme-accounts/${editingEmail.value.id}`, emailForm.value) }
    else { await client.post('/acme-accounts', emailForm.value) }
    ElMessage.success(t('common.success')); emailFormVisible.value = false; fetchEmails()
  } finally { emailSaving.value = false }
}
async function handleDeleteEmail(row) { await ElMessageBox.confirm(t('certificate.deleteEmailConfirm', { email: row.email }), t('common.confirm')); await client.delete(`/acme-accounts/${row.id}`); ElMessage.success(t('common.success')); fetchEmails() }

// --- Upload ---
const uploadVisible = ref(false)
const uploading = ref(false)
const uploadForm = ref({ domain: '', method: 'content', cert_content: '', key_content: '', fullchain_content: '', cert_path: '', key_path: '', fullchain_path: '' })

function showUploadDialog() {
  uploadForm.value = { name: '', domain: '', method: 'paste', cert_content: '', key_content: '' }
  uploadVisible.value = true
}

function handleFileRead(file, type) {
  const reader = new FileReader()
  reader.onload = (e) => {
    const content = e.target.result
    if (type === 'cert') {
      uploadForm.value.cert_content = content
    } else {
      uploadForm.value.key_content = content
    }
  }
  reader.readAsText(file.raw)
}

async function handleUpload() {
  if (!uploadForm.value.cert_content || !uploadForm.value.key_content) { ElMessage.error(t('certificate.certAndKeyRequired')); return }
  uploading.value = true
  try {
    await client.post('/certificates/upload', {
      name: uploadForm.value.name,
      domain: uploadForm.value.domain,
      cert_content: uploadForm.value.cert_content,
      key_content: uploadForm.value.key_content
    })
    ElMessage.success(t('common.success'))
    uploadVisible.value = false
    fetchCertificates()
  } catch (e) {} finally { uploading.value = false }
}

onMounted(() => { fetchCertificates(); fetchChannels(); fetchEmails(); fetchProviders() })
</script>

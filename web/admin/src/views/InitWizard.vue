<template>
  <div class="init-container">
    <el-card class="init-card">
      <h2 style="text-align:center;margin-bottom:6px">{{ t('init.title') }}</h2>
      <p style="text-align:center;color:#909399;margin-bottom:20px">{{ t('init.subtitle') }}</p>

      <el-alert v-if="fatal" :title="fatal" type="error" :closable="false" style="margin-bottom:16px" />

      <el-result v-if="done" icon="success" :title="t('init.finishSuccess')" :sub-title="t('init.restartHint')">
        <template #extra>
          <el-descriptions :column="1" border size="small" style="max-width:520px;margin:0 auto;text-align:left">
            <el-descriptions-item :label="t('init.username')">{{ summary.admin_username }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.password')">{{ summary.admin_password }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.accessPanel')">
              {{ summary.access && summary.access.panel }}
              <el-tag v-if="isInsecure(summary.access && summary.access.panel)" size="small" type="warning">{{ t('init.notSecure') }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item :label="t('init.accessSubscription')">
              {{ summary.access && summary.access.subscription }}
              <el-tag v-if="isInsecure(summary.access && summary.access.subscription)" size="small" type="warning">{{ t('init.notSecure') }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item :label="t('init.accessPortal')">
              {{ summary.access && summary.access.portal }}
              <el-tag v-if="isInsecure(summary.access && summary.access.portal)" size="small" type="warning">{{ t('init.notSecure') }}</el-tag>
            </el-descriptions-item>
          </el-descriptions>
        </template>
      </el-result>

      <template v-else-if="!fatal">
        <el-steps :active="step" finish-status="success" align-center style="margin-bottom:24px">
          <el-step :title="t('init.stepDB')" />
          <el-step :title="t('init.stepDataDir')" />
          <el-step :title="t('init.stepEnv')" />
          <el-step :title="t('init.stepCore')" />
          <el-step :title="t('init.stepProxy')" />
          <el-step :title="t('init.stepSites')" />
          <el-step :title="t('init.stepHttps')" />
          <el-step :title="t('init.stepSummary')" />
        </el-steps>

        <!-- 1. 数据库 -->
        <div v-show="step === 0">
          <el-form label-position="top">
            <el-form-item :label="t('init.dbType')">
              <el-radio-group v-model="form.db_type">
                <el-radio value="sqlite">{{ t('init.dbSqlite') }}</el-radio>
                <el-radio value="mysql">{{ t('init.dbMySQL') }}</el-radio>
                <el-radio value="postgres">{{ t('init.dbPostgres') }}</el-radio>
              </el-radio-group>
            </el-form-item>
            <template v-if="form.db_type === 'sqlite'">
              <el-form-item :label="t('init.dbSqlitePath')">
                <el-input v-model="form.db_sqlite_path" :placeholder="t('init.dbSqliteHint')" />
              </el-form-item>
            </template>
            <template v-else>
              <el-form-item :label="t('init.dbHost')"><el-input v-model="form.db_host" /></el-form-item>
              <el-form-item :label="t('init.dbPort')"><el-input v-model="form.db_port" /></el-form-item>
              <el-form-item :label="t('init.dbUser')"><el-input v-model="form.db_user" /></el-form-item>
              <el-form-item :label="t('init.dbPassword')"><el-input v-model="form.db_password" show-password /></el-form-item>
              <el-form-item :label="t('init.dbName')"><el-input v-model="form.db_name" /></el-form-item>
              <el-form-item v-if="form.db_type === 'postgres'" :label="t('init.dbSSLMode')">
                <el-select v-model="form.db_sslmode" style="width:100%">
                  <el-option label="disable" value="disable" />
                  <el-option label="require" value="require" />
                  <el-option label="verify-full" value="verify-full" />
                </el-select>
              </el-form-item>
            </template>
            <el-form-item v-if="form.db_type !== 'sqlite'">
              <el-button :loading="testing" @click="testConn">{{ t('init.testConn') }}</el-button>
            </el-form-item>
            <div style="font-size:12px;color:#909399">{{ t('init.dbHint') }}</div>
          </el-form>
        </div>

        <!-- 2. 数据目录 -->
        <div v-show="step === 1">
          <el-form label-position="top">
            <el-form-item :label="t('init.dataDir')"><el-input v-model="form.data_dir" /></el-form-item>
            <div style="font-size:12px;color:#909399">{{ t('init.dataDirHint') }}</div>
          </el-form>
        </div>

        <!-- 3. 加密密钥 -->
        <div v-show="step === 2">
          <el-form label-position="top">
            <el-form-item :label="t('init.jwtSecret')">
              <el-input v-model="form.jwt_secret"><template #append><el-button @click="regenSecrets">{{ t('init.regenerate') }}</el-button></template></el-input>
            </el-form-item>
            <el-form-item :label="t('init.secretKey')"><el-input v-model="form.secret_encryption_key" /></el-form-item>
            <div style="font-size:12px;color:#909399">{{ t('init.envHint') }}</div>
          </el-form>
        </div>

        <!-- 4. 核心安装 -->
        <div v-show="step === 3">
          <el-form label-position="top">
            <el-form-item :label="t('init.core')">
              <el-checkbox-group v-model="form.cores">
                <el-checkbox v-for="c in cores" :key="c" :value="c">{{ coreLabel(c) }}</el-checkbox>
              </el-checkbox-group>
            </el-form-item>
            <div style="font-size:12px;color:#909399">{{ t('init.coreHint') }}</div>
          </el-form>
        </div>

        <!-- 5. 代理 -->
        <div v-show="step === 4">
          <el-form label-position="top">
            <el-form-item :label="t('init.proxy')">
              <el-radio-group v-model="form.proxy_type">
                <el-radio value="none">{{ t('init.proxyNone') }}</el-radio>
                <el-radio value="nginx">{{ t('init.proxyNginx') }}</el-radio>
                <el-radio value="caddy">{{ t('init.proxyCaddy') }}</el-radio>
              </el-radio-group>
            </el-form-item>
            <div style="font-size:12px;color:#909399">{{ t('init.proxyHint') }}</div>
          </el-form>
        </div>

        <!-- 6. 面板 / 订阅 / 门户 + 域名 -->
        <div v-show="step === 5">
          <el-form label-position="top">
            <el-form-item :label="t('init.listenAddrAuto')">
              <el-input :model-value="listenAddrResolved" disabled />
            </el-form-item>
            <div style="font-size:12px;color:#909399;margin-bottom:12px">{{ t('init.listenAddrHint') }}</div>
            <el-form-item :label="t('init.listenPort')">
              <el-input v-model="form.listen_port"><template #append><el-button @click="roll(1, 'listen_port')">🎲</el-button></template></el-input>
            </el-form-item>
            <el-form-item :label="t('init.subscriptionPort')">
              <el-input v-model="form.subscription_port"><template #append><el-button @click="roll(1, 'subscription_port')">🎲</el-button></template></el-input>
            </el-form-item>
            <el-form-item :label="t('init.portalPort')">
              <el-input v-model="form.portal_port"><template #append><el-button @click="roll(1, 'portal_port')">🎲</el-button></template></el-input>
            </el-form-item>
            <el-form-item :label="t('init.webPath')">
              <el-input v-model="form.web_path"><template #append><el-button @click="genPath">🎲</el-button></template></el-input>
            </el-form-item>
            <div style="font-size:12px;color:#909399;margin-bottom:12px">{{ t('init.siteHint') }}</div>
            <el-divider />
            <el-form-item :label="t('init.username')">
              <el-input v-model="form.username"><template #append><el-button @click="genUser">{{ t('init.randomUser') }}</el-button></template></el-input>
            </el-form-item>
            <el-form-item :label="t('init.password')">
              <el-input v-model="form.password" show-password><template #append><el-button @click="genPass">{{ t('init.randomPass') }}</el-button></template></el-input>
            </el-form-item>
            <el-form-item :label="t('init.confirmPassword')"><el-input v-model="form.confirm" show-password /></el-form-item>
            <div style="font-size:12px" :style="{ color: strengthColor }">{{ t('init.strength') }}: {{ strengthText }}</div>
            <el-divider />
            <el-form-item :label="t('init.accessMode')">
              <el-radio-group v-model="form.access_mode">
                <el-radio value="domain">{{ t('init.accessModeDomain') }}</el-radio>
                <el-radio value="ip">{{ t('init.accessModeIP') }}</el-radio>
              </el-radio-group>
            </el-form-item>
            <template v-if="form.access_mode === 'domain'">
              <el-form-item :label="t('init.panelDomain')"><el-input v-model="form.panel_domain" placeholder="panel.example.com" /></el-form-item>
              <el-form-item :label="t('init.subscriptionDomain')"><el-input v-model="form.subscription_domain" placeholder="sub.example.com" /></el-form-item>
              <el-form-item :label="t('init.portalDomain')"><el-input v-model="form.portal_domain" placeholder="portal.example.com" /></el-form-item>
              <div style="font-size:12px;color:#909399">{{ t('init.domainHint') }}</div>
            </template>
            <el-alert v-else type="info" :closable="false" show-icon :title="t('init.accessModeIPHint')" style="margin-bottom:12px" />
            <el-form-item :label="t('init.defaultCurrency')">
              <el-select v-model="form.default_currency" filterable style="width:100%">
                <el-option v-for="c in currencies" :key="c.code" :label="c.code + ' ' + c.symbol" :value="c.code" />
              </el-select>
            </el-form-item>
            <el-divider v-if="form.proxy_type !== 'none' && form.access_mode === 'domain'" />
            <template v-if="form.proxy_type !== 'none' && form.access_mode === 'domain'">
              <el-form-item :label="t('init.addToProxy')">
                <el-checkbox v-model="form.add_panel">{{ t('init.addPanel') }}</el-checkbox>
                <el-checkbox v-model="form.add_subscription">{{ t('init.addSubscription') }}</el-checkbox>
                <el-checkbox v-model="form.add_portal">{{ t('init.addPortal') }}</el-checkbox>
              </el-form-item>
              <div style="font-size:12px;color:#909399">{{ t('init.addToProxyHint') }}</div>
            </template>
          </el-form>
        </div>

        <!-- 7. HTTPS -->
        <div v-show="step === 6">
          <el-form label-position="top">
            <el-alert v-if="form.tls_mode === 'none'" type="warning" :closable="false" show-icon
              :title="t('init.insecureWarn')" style="margin-bottom:12px" />
            <el-form-item :label="t('init.httpsMode')">
              <el-radio-group v-model="form.tls_mode">
                <el-radio value="none">{{ t('init.httpsNone') }}</el-radio>
                <el-radio v-if="form.access_mode !== 'ip'" value="upload">{{ t('init.httpsUpload') }}</el-radio>
                <el-radio value="acme">{{ t('init.httpsAcme') }}</el-radio>
              </el-radio-group>
            </el-form-item>
            <template v-if="form.tls_mode === 'acme'">
              <template v-if="form.access_mode === 'ip'">
                <el-alert type="info" :closable="false" show-icon :title="t('init.ipCertNote')" style="margin-bottom:12px" />
                <el-form-item :label="t('init.email')"><el-input v-model="form.email" /></el-form-item>
                <el-form-item :label="t('init.acmeProvider')"><el-tag type="success">Let's Encrypt</el-tag></el-form-item>
                <div style="font-size:12px;color:#909399">{{ t('init.ipCertHint') }}</div>
                <div style="margin-top:8px;display:flex;align-items:center;gap:8px;flex-wrap:wrap">
                  <el-tag v-for="ip in detectedIPs" :key="ip" size="small" type="success">{{ ip }}</el-tag>
                  <span v-if="!detecting && !detectedIPs.length" style="font-size:12px;color:#e6a23c">{{ t('init.noPublicIP') }}</span>
                  <el-button size="small" type="primary" plain :loading="detecting" @click="detectIP">{{ t('init.detectIP') }}</el-button>
                </div>
              </template>
              <template v-else>
              <el-form-item :label="t('init.email')"><el-input v-model="form.email" /></el-form-item>
              <el-form-item :label="t('init.acmeProvider')">
                <el-select v-model="form.acme_provider" style="width:100%">
                  <el-option label="Let's Encrypt" value="lets_encrypt" />
                  <el-option label="Let's Encrypt (Staging)" value="lets_encrypt_staging" />
                  <el-option label="ZeroSSL" value="zerossl" />
                  <el-option label="Google Public CA" value="google" />
                </el-select>
              </el-form-item>
              <el-form-item :label="t('init.certTarget')">
                <el-radio-group v-model="certTarget">
                  <el-radio value="domain">{{ t('init.certTargetDomain') }}</el-radio>
                  <el-radio value="ip" :disabled="!form.acme_provider.startsWith('lets_encrypt')">{{ t('init.certTargetIP') }}</el-radio>
                </el-radio-group>
              </el-form-item>
              <template v-if="!form.ip_cert">
                <el-form-item :label="t('init.challenge')">
                  <el-radio-group v-model="form.challenge">
                    <el-radio value="http-01">{{ t('init.challengeHttp') }}</el-radio>
                    <el-radio value="dns-01">{{ t('init.challengeDns') }}</el-radio>
                  </el-radio-group>
                </el-form-item>
                <el-form-item :label="t('init.panelDomain')"><el-input v-model="form.panel_domain" placeholder="panel.example.com" /></el-form-item>
                <el-form-item :label="t('init.subscriptionDomain')"><el-input v-model="form.subscription_domain" placeholder="sub.example.com" /></el-form-item>
                <el-form-item :label="t('init.portalDomain')"><el-input v-model="form.portal_domain" placeholder="portal.example.com" /></el-form-item>
                <el-form-item :label="t('init.domainsExtra')">
                  <el-input v-model="form.domains" :placeholder="t('init.domainsExtraPlaceholder')" />
                  <div style="font-size:12px;color:#909399;margin-top:4px">{{ t('init.domainsHint') }}</div>
                </el-form-item>
              </template>
              <el-form-item :label="t('init.keyType')">
                <el-select v-model="form.key_type" style="width:100%">
                  <el-option label="ECC" value="ecc" />
                  <el-option label="RSA" value="rsa" />
                </el-select>
              </el-form-item>
              <el-form-item v-if="form.key_type === 'ecc'" :label="t('init.keyBits')">
                <el-select v-model.number="form.key_bits" style="width:100%">
                  <el-option label="256" :value="256" />
                  <el-option label="384" :value="384" />
                </el-select>
              </el-form-item>
              <el-form-item v-else :label="t('init.keyBits')">
                <el-select v-model.number="form.key_bits" style="width:100%">
                  <el-option label="2048" :value="2048" />
                  <el-option label="3072" :value="3072" />
                  <el-option label="4096" :value="4096" />
                </el-select>
              </el-form-item>
              <template v-if="form.ip_cert">
                <div style="font-size:12px;color:#909399">{{ t('init.ipCertHint') }}</div>
                <div style="margin-top:8px;display:flex;align-items:center;gap:8px;flex-wrap:wrap">
                  <el-tag v-for="ip in detectedIPs" :key="ip" size="small" type="success">{{ ip }}</el-tag>
                  <span v-if="!detecting && !detectedIPs.length" style="font-size:12px;color:#e6a23c">{{ t('init.noPublicIP') }}</span>
                  <el-button size="small" type="primary" plain :loading="detecting" @click="detectIP">{{ t('init.detectIP') }}</el-button>
                </div>
              </template>
              <template v-if="!form.ip_cert && form.challenge === 'dns-01'">
                <el-form-item :label="t('init.dnsProvider')">
                  <el-select v-model="form.dns_provider" style="width:100%">
                    <el-option v-for="(v, k) in dnsProviders" :key="k" :label="k" :value="k" />
                  </el-select>
                </el-form-item>
                <el-form-item v-for="k in dnsEnvKeys" :key="k" :label="k">
                  <el-input v-model="form.dns_env[k]" show-password />
                </el-form-item>
              </template>
              <template v-if="form.acme_provider === 'google'">
                <div style="font-size:12px;margin-bottom:8px">
                  {{ t('init.googleEABHint') }}
                  <a href="https://cloud.google.com/certificate-manager/public-ca" target="_blank" rel="noopener">{{ t('init.googleDocLink') }}</a>
                </div>
                <el-form-item label="EAB Key ID"><el-input v-model="form.eab_key_id" /></el-form-item>
                <el-form-item label="EAB HMAC Key"><el-input v-model="form.eab_mac_key" show-password /></el-form-item>
                <el-divider>{{ t('init.orAutoEAB') }}</el-divider>
                <el-form-item label="GCP Project ID"><el-input v-model="form.project_id" /></el-form-item>
                <el-form-item label="Service Account JSON"><el-input v-model="form.service_account" type="textarea" :rows="4" /></el-form-item>
              </template>
              <div style="font-size:12px;color:#909399">{{ t('init.acmeHint') }}</div>
              </template>
            </template>
            <template v-if="form.tls_mode === 'upload'">
              <el-form-item :label="t('init.certPem')"><el-input v-model="form.cert_pem" type="textarea" :rows="5" /></el-form-item>
              <el-form-item :label="t('init.keyPem')"><el-input v-model="form.key_pem" type="textarea" :rows="5" /></el-form-item>
            </template>
          </el-form>
        </div>

        <!-- 8. 配置清单 -->
        <div v-show="step === 7">
          <h3 style="margin:0 0 12px">{{ t('init.summaryTitle') }}</h3>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item :label="t('init.dbType')">{{ summary.db_type }} {{ summary.db_dsn }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.dataDir')">{{ summary.data_dir }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.core')">{{ (summary.cores || []).join(', ') || '-' }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.proxy')">{{ summary.proxy_type }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.defaultCurrency')">{{ summary.default_currency }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.listenAddrAuto')">{{ summary.listen_addr }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.listenPort')">{{ summary.listen_port }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.subscriptionPort')">{{ summary.subscription_port }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.portalPort')">{{ summary.portal_port }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.webPath')">{{ summary.web_path }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.username')">{{ summary.admin_username }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.password')">{{ summary.admin_password }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.panelDomain')">{{ summary.panel_domain || '-' }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.subscriptionDomain')">{{ summary.subscription_domain || '-' }}</el-descriptions-item>
            <el-descriptions-item :label="t('init.portalDomain')">{{ summary.portal_domain || '-' }}</el-descriptions-item>
          </el-descriptions>

          <h4 style="margin:14px 0 8px">.env</h4>
          <el-table :data="(summary.env || [])" size="small" border>
            <el-table-column prop="k" label="KEY" width="240" />
            <el-table-column prop="v" label="VALUE" />
          </el-table>
          <el-button style="margin-top:10px" @click="downloadEnv">{{ t('init.downloadEnv') }}</el-button>

          <h4 style="margin:14px 0 8px">{{ t('init.accessTitle') }}</h4>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item :label="t('init.accessPanel')">
              {{ summary.access && summary.access.panel }}
              <el-tag v-if="isInsecure(summary.access && summary.access.panel)" size="small" type="warning">{{ t('init.notSecure') }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item :label="t('init.accessSubscription')">
              {{ summary.access && summary.access.subscription }}
              <el-tag v-if="isInsecure(summary.access && summary.access.subscription)" size="small" type="warning">{{ t('init.notSecure') }}</el-tag>
            </el-descriptions-item>
            <el-descriptions-item :label="t('init.accessPortal')">
              {{ summary.access && summary.access.portal }}
              <el-tag v-if="isInsecure(summary.access && summary.access.portal)" size="small" type="warning">{{ t('init.notSecure') }}</el-tag>
            </el-descriptions-item>
          </el-descriptions>
          <div style="font-size:12px;color:#909399;margin-top:10px">{{ t('init.summaryHint') }}</div>
        </div>

        <div style="display:flex;justify-content:space-between;margin-top:16px">
          <el-button :disabled="step === 0 || busy" @click="step--">{{ t('init.prev') }}</el-button>
          <el-button v-if="step < 7" type="primary" :loading="busy" @click="next">{{ t('init.next') }}</el-button>
          <el-button v-else type="primary" :loading="busy" @click="finish">{{ t('init.finish') }}</el-button>
        </div>
      </template>
    </el-card>
  </div>
</template>

<script setup>
import { reactive, ref, computed, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { CURRENCIES } from '../utils/currency'
import {
  getInitStatus, randomPorts, saveDB, testDB, saveDataDir, saveEnv, saveCore, saveProxy,
  saveSites, saveAdmin, saveHttps, getSummary, complete, initToken, envDownloadUrl, getDNSProviders, getPublicIP
} from '../api/init'

const { t } = useI18n()
const router = useRouter()

const step = ref(0)
const busy = ref(false)
const testing = ref(false)
const done = ref(false)
const fatal = ref('')
const cores = ref(['xray', 'singbox'])
const dnsProviders = ref({})
const currencies = CURRENCIES
const summary = ref({})
const envUrl = ref('')
const detecting = ref(false)
const detectedIPs = ref([])

const form = reactive({
  db_type: 'sqlite',
  db_host: '',
  db_port: '3306',
  db_user: '',
  db_password: '',
  db_name: 'corefusion',
  db_sslmode: 'disable',
  db_sqlite_path: '',
  data_dir: '',
  jwt_secret: '',
  secret_encryption_key: '',
  cores: [],
  proxy_type: 'none',
  listen_port: '',
  subscription_port: '',
  portal_port: '',
  web_path: '/',
  username: '',
  password: '',
  confirm: '',
  panel_domain: '',
  subscription_domain: '',
  portal_domain: '',
  default_currency: 'CNY',
  add_panel: true,
  add_subscription: true,
  add_portal: true,
  access_mode: 'domain',
  tls_mode: 'none',
  domains: '',
  email: '',
  acme_provider: 'lets_encrypt',
  challenge: 'http-01',
  key_type: 'ecc',
  key_bits: 256,
  ip_cert: false,
  dns_provider: '',
  dns_env: {},
  eab_key_id: '',
  eab_mac_key: '',
  project_id: '',
  service_account: '',
  cert_pem: '',
  key_pem: ''
})

const listenAddrResolved = computed(() => {
  // 面板被「一键添加到反向代理」→ 代理对外，面板只监听回环；否则配置了证书就监听 0.0.0.0。
  const viaProxy = (form.proxy_type === 'nginx' || form.proxy_type === 'caddy') && form.add_panel
  if (viaProxy) return '127.0.0.1'
  return form.tls_mode && form.tls_mode !== 'none' ? '0.0.0.0' : '127.0.0.1'
})

// 证书目标：配置域名 / 使用 IP（映射到 ip_cert）。
const certTarget = computed({
  get: () => (form.ip_cert ? 'ip' : 'domain'),
  set: (v) => { form.ip_cert = v === 'ip' }
})

// 切换数据库类型时把端口/库名带成该类型的常用默认值（用户没改过才覆盖）
const dbPortTouched = ref(false)
watch(() => form.db_type, (t) => {
  if (!dbPortTouched.value) form.db_port = t === 'postgres' ? '5432' : '3306'
})
watch(() => form.db_port, () => { dbPortTouched.value = true })

// 切换密钥类型时把长度带成该类型的默认值（ecc 256 / rsa 2048）
watch(() => form.key_type, (kt) => { form.key_bits = kt === 'rsa' ? 2048 : 256 })
// IP 证书只能用 HTTP-01（LE 的 IP 证书走 http-01/TLS-ALPN）；反之选 dns-01 时取消 IP 证书。
// 勾选 IP 证书时自动获取本机公网 IPv4/IPv6（与证书页同一套）。
watch(() => form.ip_cert, (v) => { if (v) { form.challenge = 'http-01'; detectIP() } })
watch(() => form.challenge, (c) => { if (c === 'dns-01') form.ip_cert = false })
// 只有 Let's Encrypt 支持 IP 证书：切到其它 CA 时退回域名证书。
watch(() => form.acme_provider, (p) => { if (!String(p).startsWith('lets_encrypt')) form.ip_cert = false })
// 访问方式：域名 → 填面板/订阅/门户域名；IP → 隐藏域名，证书固定为 Let's Encrypt 的 IP 证书。
watch(() => form.access_mode, (m) => {
  if (m === 'ip') {
    form.tls_mode = 'acme'
    form.acme_provider = 'lets_encrypt'
    form.ip_cert = true
    form.panel_domain = ''
    form.subscription_domain = ''
    form.portal_domain = ''
    form.domains = ''
    form.add_panel = false
    form.add_subscription = false
    form.add_portal = false
  } else {
    form.ip_cert = false
  }
})

const dnsEnvKeys = computed(() => form.dns_provider ? (dnsProviders.value[form.dns_provider] || []) : [])
watch(() => form.dns_provider, (k) => {
  if (!k) return
  ;(dnsProviders.value[k] || []).forEach((e) => { if (!form.dns_env[e]) form.dns_env[e] = '' })
})

function coreLabel(c) {
  if (c === 'xray') return t('init.coreXray')
  if (c === 'singbox') return t('init.coreSingbox')
  return c
}
function randStr(n, charset) {
  const c = charset || 'abcdefghijklmnopqrstuvwxyz0123456789'
  const a = new Uint32Array(n)
  crypto.getRandomValues(a)
  let s = ''
  for (let i = 0; i < n; i++) s += c[a[i] % c.length]
  return s
}
function genUser() { form.username = 'cf' + randStr(6) }
function genPass() {
  const u = 'ABCDEFGHJKLMNPQRSTUVWXYZ', l = 'abcdefghijkmnpqrstuvwxyz', d = '23456789', sp = '!@#%^*-_=+'
  const all = u + l + d + sp
  const pick = (set) => set[Math.floor(Math.random() * set.length)]
  const s = pick(u) + pick(l) + pick(d) + pick(sp) + randStr(12, all)
  form.password = s
  form.confirm = s
}
function genPath() { form.web_path = '/' + randStr(12) }
async function roll(n, field) {
  try {
    const r = await randomPorts(n)
    const ports = r && r.ports ? r.ports : []
    if (ports.length) form[field] = String(ports[0])
  } catch (e) { ElMessage.error(errMsg(e)) }
}

const strength = computed(() => {
  const p = form.password || ''
  let s = 0
  if (p.length >= 8) s++
  if (/[A-Z]/.test(p)) s++
  if (/[a-z]/.test(p)) s++
  if (/[0-9]/.test(p)) s++
  if (/[^A-Za-z0-9]/.test(p)) s++
  return s
})
const strengthText = computed(() => ({ 0: '弱', 1: '弱', 2: '一般', 3: '中', 4: '强', 5: '很强' }[strength.value] || '弱'))
const strengthColor = computed(() => (strength.value >= 4 ? '#67c23a' : strength.value >= 2 ? '#e6a23c' : '#f56c6c'))

function errMsg(e) { return e?.response?.data?.message || e?.message || 'error' }
// 访问地址是 http（未配置证书 / 由代理 HTTP 提供）→ 标记“不安全”。
function isInsecure(url) { return String(url || '').startsWith('http://') }

async function regenSecrets() {
  try {
    const r = await saveEnv({ auto_generate: true })
    if (r) { form.jwt_secret = r.jwt_secret || form.jwt_secret; form.secret_encryption_key = r.secret_encryption_key || form.secret_encryption_key }
    ElMessage.success(t('common.success'))
  } catch (e) { ElMessage.error(errMsg(e)) }
}

async function next() {
  busy.value = true
  try {
    if (step.value === 0) {
      await saveDB(dbPayload())
    } else if (step.value === 1) {
      if (!form.data_dir) { ElMessage.error(t('init.required')); return }
      await saveDataDir({ data_dir: form.data_dir })
    } else if (step.value === 2) {
      const r = await saveEnv({ jwt_secret: form.jwt_secret, secret_encryption_key: form.secret_encryption_key, auto_generate: false })
      if (r) { form.jwt_secret = r.jwt_secret || form.jwt_secret; form.secret_encryption_key = r.secret_encryption_key || form.secret_encryption_key }
    } else if (step.value === 3) {
      await saveCore({ cores: form.cores })
    } else if (step.value === 4) {
      await saveProxy({ proxy_type: form.proxy_type })
    } else if (step.value === 5) {
      if (!form.username) { ElMessage.error(t('init.required')); return }
      if (form.password !== form.confirm) { ElMessage.error(t('init.passwordMismatch')); return }
      if (!form.panel_domain && !form.subscription_domain && !form.portal_domain) { /* none required together */ }
      if (!form.portal_domain && form.portal_port !== '0') { /* portal domain validated below */ }
      await saveAdmin({ username: form.username, password: form.password })
      await saveSites(sitesPayload())
    } else if (step.value === 6) {
      // HTTPS 步的面板/订阅/门户域名可能与第 6 步不同，先落草稿再签发（后端签发会自动带上这些域名）。
      await saveSites(sitesPayload())
      const ipMode = form.access_mode === 'ip'
      if (!ipMode && form.tls_mode === 'acme' && !form.ip_cert) {
        const hasDomain = [form.panel_domain, form.subscription_domain, form.portal_domain, form.domains].some((x) => String(x || '').trim())
        if (!hasDomain) { ElMessage.error(t('init.domainsRequired')); return }
      }
      await saveHttps({
        mode: form.tls_mode,
        domains: ipMode ? [] : splitList(form.domains),
        email: form.email,
        provider: ipMode ? 'lets_encrypt' : form.acme_provider,
        challenge: ipMode ? 'http-01' : form.challenge,
        key_type: form.key_type,
        key_bits: form.key_type === 'ecc' ? (form.key_bits || 256) : (form.key_bits || 2048),
        ip_cert: ipMode ? true : form.ip_cert,
        dns_provider: !ipMode && form.challenge === 'dns-01' ? form.dns_provider : '',
        dns_env: !ipMode && form.challenge === 'dns-01' ? form.dns_env : null,
        eab_key_id: form.eab_key_id,
        eab_mac_key: form.eab_mac_key,
        project_id: form.project_id,
        service_account: form.service_account,
        cert_pem: form.cert_pem,
        key_pem: form.key_pem
      })
    }
    if (step.value === 6) {
      await loadSummary()
    }
    step.value++
  } catch (e) {
    ElMessage.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

// 自动获取本机公网 IP（IP 证书用，与证书页「自动检测」同一套接口）。
async function detectIP() {
  detecting.value = true
  try {
    const r = await getPublicIP()
    const ips = (r && r.ips && r.ips.length ? r.ips : [r && r.ipv4, r && r.ipv6]) || []
    detectedIPs.value = ips.filter(Boolean)
    if (!detectedIPs.value.length) ElMessage.warning(t('init.noPublicIP'))
  } catch (e) { ElMessage.error(errMsg(e)) } finally { detecting.value = false }
}

// 测试连接：用表单当前值直接测（不保存草稿）；库不存在会自动创建。
async function testConn() {
  testing.value = true
  try {
    const r = await testDB(dbPayload())
    if (r && r.db_created) {
      ElMessage.success((r.message || 'ok') + ' — ' + t('init.dbCreated'))
    } else {
      ElMessage.success(r?.message || t('common.success'))
    }
  } catch (e) { ElMessage.error(errMsg(e)) } finally { testing.value = false }
}

function dbPayload() {
  return {
    db_type: form.db_type,
    db_host: form.db_host,
    db_port: form.db_port,
    db_user: form.db_user,
    db_password: form.db_password,
    db_name: form.db_name,
    db_sslmode: form.db_sslmode,
    db_sqlite_path: form.db_sqlite_path
  }
}
function sitesPayload() {
  // IP 访问方式不涉及域名，也不做反向代理站点。
  const ipMode = form.access_mode === 'ip'
  return {
    listen_port: form.listen_port,
    subscription_port: form.subscription_port,
    portal_port: form.portal_port,
    web_path: form.web_path,
    panel_domain: ipMode ? '' : form.panel_domain,
    subscription_domain: ipMode ? '' : form.subscription_domain,
    portal_domain: ipMode ? '' : form.portal_domain,
    default_currency: form.default_currency,
    add_panel: ipMode ? false : form.add_panel,
    add_subscription: ipMode ? false : form.add_subscription,
    add_portal: ipMode ? false : form.add_portal
  }
}
function splitList(s) { return String(s || '').split(',').map((x) => x.trim()).filter(Boolean) }

async function loadSummary() {
  try {
    const s = await getSummary()
    if (s && Array.isArray(s.env)) s.env = s.env.map((e) => ({ k: e[0], v: e[1] }))
    summary.value = s || {}
  } catch (e) { ElMessage.error(errMsg(e)) }
}
function downloadEnv() {
  envUrl.value = envDownloadUrl()
  window.location.href = envUrl.value
}

async function finish() {
  busy.value = true
  try {
    await complete({})
    done.value = true
    const base = form.web_path && form.web_path !== '/' ? form.web_path : ''
    setTimeout(() => { window.location.href = base + '/login' }, 5000)
  } catch (e) {
    ElMessage.error(errMsg(e))
  } finally {
    busy.value = false
  }
}

onMounted(async () => {
  if (!initToken()) { fatal.value = t('init.tokenMissing'); return }
  try {
    const s = await getInitStatus()
    if (s && s.supported_cores) cores.value = s.supported_cores
    getDNSProviders().then((r) => { dnsProviders.value = (r && r.providers) || {} }).catch(() => {})
    const d = (s && s.draft) || {}
    const keys = Object.keys(form)
    keys.forEach((k) => { if (d[k] !== undefined && d[k] !== null) form[k] = d[k] })
    // 草稿里 ACME 细节用的是 tls_* / acme_* 字段名，这里映射回表单字段（刷新后可恢复）
    if (d.tls_challenge) form.challenge = d.tls_challenge
    if (d.tls_key_type) form.key_type = d.tls_key_type
    if (d.tls_key_bits) form.key_bits = d.tls_key_bits
    if (d.tls_email) form.email = d.tls_email
    if (d.acme_provider) form.acme_provider = d.acme_provider
    if (d.acme_dns_provider) form.dns_provider = d.acme_dns_provider
    // 草稿里没有 access_mode：若已获取的 SAN 全是 IP，则视为 IP 访问方式（刷新恢复）。
    const td = Array.isArray(d.tls_domains) ? d.tls_domains.filter(Boolean) : []
    const looksIP = (x) => /^(\d{1,3}\.){3}\d{1,3}$/.test(String(x)) || String(x).includes(':')
    if (td.length && td.every(looksIP)) { form.access_mode = 'ip'; form.ip_cert = true }
    if (!form.username) genUser()
    if (!form.password) genPass()
  } catch (e) {
    const st = e?.response?.status
    if (st === 404) {
      router.replace('/login')
    } else {
      fatal.value = errMsg(e)
    }
  }
})
</script>

<style scoped>
.init-container {
  min-height: 100vh;
  display: flex;
  align-items: flex-start;
  justify-content: center;
  background: #f5f7fa;
  padding: 20px;
}
.init-card {
  width: 100%;
  max-width: 720px;
}
</style>

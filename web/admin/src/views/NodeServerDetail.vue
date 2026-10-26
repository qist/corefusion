<template>
  <div>
    <!-- Header -->
    <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:16px">
      <div style="display:flex;align-items:center;gap:12px">
        <el-button text @click="goBack"><el-icon><ArrowLeft /></el-icon> {{ t('common.back') }}</el-button>
        <span style="font-size:18px;font-weight:600">{{ server?.name || '...' }}</span>
        <el-tag v-if="server" size="small" :type="server.is_local ? 'warning' : 'success'">
          {{ server.is_local ? t('nodeServer.localNode') : t('nodeServer.remoteNode') }}
        </el-tag>
        <el-tag v-if="server" size="small" :type="statusTag(server.agent_status)">
          <span :class="['status-dot', server.agent_status]"></span>
          {{ statusLabel(server.agent_status) }}
        </el-tag>
      </div>
      <el-button size="small" @click="loadServer"><el-icon><Refresh /></el-icon> {{ t('common.refresh') }}</el-button>
    </div>

    <!-- Server Info Bar -->
    <el-card v-if="server" shadow="hover" style="margin-bottom:16px" body-style="padding:12px 20px">
      <div style="display:flex;gap:32px;font-size:13px;flex-wrap:wrap">
        <div><span style="color:#909399">{{ t('nodeServer.hostname') }}: </span>{{ server.hostname }}</div>
        <div><span style="color:#909399">{{ t('nodeServer.region') }}: </span>{{ server.region || '-' }}</div>
        <div><span style="color:#909399">{{ t('nodeServer.nodeType') }}: </span>{{ nodeTypeLabel(server.node_type) }}</div>
        <div v-if="server.agent_version"><span style="color:#909399">{{ t('nodeServer.agentVersion') }}: </span>{{ server.agent_version }}</div>
        <div v-if="server.last_heartbeat_at"><span style="color:#909399">{{ t('nodeServer.lastHeartbeat') }}: </span>{{ formatTime(server.last_heartbeat_at) }}</div>
      </div>
    </el-card>

    <!-- Tabs -->
    <el-tabs v-model="activeTab" type="border-card">
      <!-- Overview Tab -->
      <el-tab-pane :label="t('nodeServer.overview')" name="overview">
        <el-descriptions v-if="server" :column="isMobile ? 1 : 3" border>
          <el-descriptions-item :label="t('common.name')">{{ server.name }}</el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.hostname')">{{ server.hostname }}</el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.region')">{{ server.region || '-' }}</el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.nodeType')">{{ nodeTypeLabel(server.node_type) }}</el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.agentStatus')">
            <el-tag size="small" :type="statusTag(server.agent_status)">{{ statusLabel(server.agent_status) }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.agentVersion')">{{ server.agent_version || '-' }}</el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.lastHeartbeat')">{{ server.last_heartbeat_at ? formatTime(server.last_heartbeat_at) : '-' }}</el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.certStatus')">
            <template v-if="server.is_local">-</template>
            <template v-else-if="server.agent_cert_expires_at">
              <span>{{ formatTime(server.agent_cert_expires_at) }}</span>
              <el-tag v-if="server.agent_cert_invalid" size="small" type="danger" style="margin-left:6px">
                {{ t('nodeServer.certExpired') }}
              </el-tag>
              <el-tag v-else-if="agentCertExpiringSoon(server.agent_cert_expires_at)" size="small" type="warning" style="margin-left:6px">
                {{ t('nodeServer.certExpiringSoon') }}
              </el-tag>
            </template>
            <el-tag v-else size="small" type="info">{{ t('nodeServer.certNotIssued') }}</el-tag>
          </el-descriptions-item>
          <el-descriptions-item :label="t('nodeServer.isLocal')">
            <el-tag size="small" :type="server.is_local ? 'warning' : 'info'">{{ server.is_local ? t('common.yes') : t('common.no') }}</el-tag>
          </el-descriptions-item>
        </el-descriptions>

        <!--
          证书失效恢复入口：只在 Agent 证书过期/未签发时出现。
          证书正常时不显示——此时续签由面板自动推送，不需要（也不应该）重新引导。
        -->
        <el-alert
          v-if="server && !server.is_local && server.agent_cert_invalid"
          type="error"
          :closable="false"
          show-icon
          style="margin-top:16px"
        >
          <template #title>{{ t('nodeServer.certInvalidTitle') }}</template>
          <div style="line-height:1.9">
            {{ t('nodeServer.certInvalidHint') }}
            <div style="margin-top:8px">
              <el-button v-if="$can('admin')" size="small" type="danger" :loading="regenLoading" @click="regenerateBootstrapToken">
                {{ t('nodeServer.regenerateToken') }}
              </el-button>
            </div>
          </div>
        </el-alert>

        <!-- Agent Update Section -->
        <el-card v-if="server && !server.is_local" shadow="never" style="margin-top:16px">
          <template #header>
            <div style="display:flex;align-items:center;gap:8px">
              <el-icon><Upload /></el-icon>
              <span style="font-weight:600">{{ t('nodeServer.agentUpdate') }}</span>
            </div>
          </template>

          <!-- Current vs Panel Version -->
          <div style="display:flex;gap:24px;align-items:center;margin-bottom:16px;flex-wrap:wrap">
            <div>
              <span style="color:#909399;font-size:13px">{{ t('nodeServer.currentAgentVersion') }}: </span>
              <el-tag size="small" type="info">{{ server.agent_version || 'unknown' }}</el-tag>
            </div>
            <div v-if="hasLocalAgent">
              <span style="color:#909399;font-size:13px">{{ t('nodeServer.panelLocalVersion') }}: </span>
              <el-tag size="small" :type="server.agent_version === panelAgentVersion.version ? 'success' : 'warning'">
                {{ panelAgentVersion.version }}
              </el-tag>
              <el-button
                size="small" type="primary" style="margin-left:8px"
                :disabled="server.agent_version === panelAgentVersion.version || updating"
                :loading="updating"
                @click="pushUpdateFromPanel"
              >
                {{ t('nodeServer.updateFromPanel') }}
              </el-button>
            </div>
            <div v-else-if="panelVersionLoading">
              <el-text type="info" size="small">{{ t('nodeServer.loadingPanelVersion') }}</el-text>
            </div>
            <div v-else>
              <el-text type="info" size="small">{{ t('nodeServer.noAgentBinary') }}</el-text>
            </div>
          </div>

          <template v-if="githubReleases.length > 0 || releasesLoading">
          <el-divider content-position="center">
            {{ hasLocalAgent ? t('nodeServer.orFromGitHub') : t('nodeServer.updateFromGithub') }}
          </el-divider>

          <!-- GitHub Releases -->
          <div v-loading="releasesLoading">
            <div v-if="githubReleases.length === 0 && !releasesLoading" style="text-align:center;padding:20px;color:#909399">
              {{ t('nodeServer.noReleases') }}
            </div>
            <el-radio-group v-model="selectedRelease" style="display:flex;flex-wrap:wrap;gap:8px;margin-bottom:12px">
              <el-radio-button
                v-for="rel in githubReleases" :key="rel.version"
                :value="rel.version"
              >
                {{ rel.version }}
                <el-tag v-if="rel.prerelease" size="small" type="warning" style="margin-left:4px">pre</el-tag>
              </el-radio-button>
            </el-radio-group>

            <div v-if="selectedRelease" style="margin-bottom:12px">
              <div style="font-size:13px;color:#909399;margin-bottom:4px">{{ t('nodeServer.availableAssets') }}</div>
              <div style="display:flex;gap:6px;flex-wrap:wrap;margin-bottom:4px">
                <el-tag v-for="asset in selectedReleaseAssets" :key="asset.name" size="small" type="info">
                  {{ asset.os }}/{{ asset.arch }}
                </el-tag>
              </div>
              <div style="font-size:11px;color:#999">{{ t('nodeServer.githubAssetAutoHint') }}</div>
            </div>

            <el-button
              size="small" type="primary"
              :disabled="!selectedRelease || updating"
              :loading="updating"
              @click="pushUpdateFromGitHub"
            >
              {{ t('nodeServer.updateFromGitHub') }}
            </el-button>
          </div>
          </template>

          <!-- Update Status -->
          <div v-if="updateTask" style="margin-top:16px;padding:12px;background:#f5f7fa;border-radius:4px">
            <div style="display:flex;align-items:center;gap:8px">
              <span style="font-size:13px;font-weight:600">{{ t('nodeServer.updateStatus') }}:</span>
              <el-tag size="small" :type="updateStatusTag(updateTask.status)">{{ updateTask.status }}</el-tag>
              <span style="font-size:13px;color:#909399">{{ updateTask.from_version }} → {{ updateTask.to_version }}</span>
            </div>
            <div v-if="updateTask.error" style="margin-top:4px;font-size:12px;color:#f56c6c">{{ updateTask.error }}</div>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- Protocol Config Tab -->
      <el-tab-pane :label="t('menu.protoCenter')" name="protocols">
        <ProtocolCenter :server-id="serverId" />
      </el-tab-pane>

      <!-- Proxy Management Tab -->
      <el-tab-pane :label="t('menu.proxy')" name="proxy">
        <ProxyManagement :server-id="serverId" :is-local="server?.is_local || false" />
      </el-tab-pane>

      <!-- Core Process Tab (merged: binary management + process control + guardian) -->
      <el-tab-pane :label="t('service.tabs.process')" name="process">
        <CoreProcessTab :server-id="serverId" />
      </el-tab-pane>

      <!-- Core Traffic Tab (per node: monthly trend + realtime rate) -->
      <el-tab-pane :label="t('dashboard.coreTraffic')" name="traffic">
        <CoreTrafficTab :server-id="serverId" />
      </el-tab-pane>

      <!-- WARP Tab (per node) -->
      <el-tab-pane :label="t('service.tabs.warp')" name="warp">
        <WarpTab :server-id="serverId" />
      </el-tab-pane>

      <!-- Direct Domains Tab (per node) -->
      <el-tab-pane :label="t('nodeServer.directDomains')" name="shareDomain">
        <ShareDomainTab :server-id="serverId" />
      </el-tab-pane>

      <!-- Certificates Tab -->
      <el-tab-pane :label="t('rateLimit.tabLabel')" name="rateLimit">
        <RateLimitTab :server-id="serverId" />
      </el-tab-pane>
      <el-tab-pane :label="t('menu.certificates')" name="certs">
        <div style="padding:20px">
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px">
            <span style="font-size:14px;font-weight:600">{{ t('nodeServer.certList') }}</span>
            <div style="display:flex;gap:8px">
              <el-button size="small" @click="showCertBindDialog = true">{{ t('nodeServer.bindCert') }}</el-button>
              <el-button size="small" type="primary" @click="loadCerts"><el-icon><Refresh /></el-icon> {{ t('common.refresh') }}</el-button>
            </div>
          </div>
          <el-alert type="info" :closable="false" style="margin-bottom:12px;font-size:12px">
            {{ t('nodeServer.certAutoDetectHint') }}
          </el-alert>
          <el-table v-if="!isMobile" :data="certPagedList" v-loading="certLoading" stripe size="small">
            <el-table-column prop="id" label="ID" width="60" />
            <el-table-column prop="domain" :label="t('nodeServer.certDomain')" min-width="140" />
            <el-table-column :label="t('nodeServer.certSource')" min-width="180">
              <template #default="{row}">
                <div style="display:flex;flex-wrap:wrap;gap:2px">
                  <el-tag v-for="src in (row.sources || [])" :key="src" size="small" :type="sourceTagType(src)">{{ sourceLabel(src) }}</el-tag>
                </div>
              </template>
            </el-table-column>
            <el-table-column prop="issuer" label="Issuer" width="140" show-overflow-tooltip />
            <el-table-column :label="t('nodeServer.certProvider')" width="90">
              <template #default="{row}">
                <el-tag size="small" type="info">{{ row.provider_type || '-' }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('nodeServer.certAutoRenew')" width="80">
              <template #default="{row}">
                <el-tag size="small" :type="row.auto_renew ? 'success' : 'info'">{{ row.auto_renew ? t('common.yes') : t('common.no') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('nodeServer.certExpiry')" width="170">
              <template #default="{row}">
                <el-tag v-if="row.expires_at" size="small" :type="certExpiringSoon(row.expires_at) ? 'warning' : 'success'">
                  {{ formatTime(row.expires_at) }}
                </el-tag>
                <span v-else>-</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('nodeServer.certPath')" width="200" show-overflow-tooltip>
              <template #default="{row}">
                <span style="font-size:12px;color:#909399">{{ row.cert_path || '-' }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="120">
              <template #default="{row}">
                <el-button size="small" type="primary" plain :loading="syncingCertId === row.id" @click="syncCertToNode(row)">
                  {{ t('nodeServer.syncCert') }}
                </el-button>
              </template>
            </el-table-column>
          </el-table>
          <MobileCards
            v-else
            :rows="certPagedList"
            title-field="domain"
            :fields="certCardFields"
          >
            <template #actions="{ row }">
              <el-button size="small" type="primary" plain :loading="syncingCertId === row.id" @click="syncCertToNode(row)">
                {{ t('nodeServer.syncCert') }}
              </el-button>
            </template>
          </MobileCards>
          <PaginationBar v-model:page="certPage" v-model:page-size="certSize" :total="certTotal" />
          <div v-if="certificates.length === 0 && !certLoading" style="text-align:center;padding:40px;color:#909399">
            {{ t('nodeServer.noCerts') }}
          </div>
        </div>

        <!-- Bind Certificate Dialog -->
        <el-dialog v-model="showCertBindDialog" :title="t('nodeServer.bindCert')" width="600">
          <el-form label-width="120px">
            <el-form-item :label="t('nodeServer.selectCert')">
              <el-select v-model="certBindId" style="width:100%" :placeholder="t('nodeServer.selectCertPlaceholder')" filterable>
                <el-option v-for="c in allCerts" :key="c.id" :label="`${c.domain} (${(c.key_type||'').toUpperCase()}${c.key_bits?' '+c.key_bits:''} · ${c.issuer})`" :value="c.id" />
              </el-select>
            </el-form-item>
          </el-form>
          <template #footer>
            <el-button @click="showCertBindDialog = false">{{ t('common.cancel') }}</el-button>
            <el-button type="primary" :loading="certBinding" @click="handleBindCert">{{ t('common.confirm') }}</el-button>
          </template>
        </el-dialog>
      </el-tab-pane>

      <el-tab-pane :label="t('onlineUsers.tabLabel')" name="online">
        <div style="padding:12px">
          <div style="display:flex;justify-content:space-between;align-items:center;margin-bottom:12px;flex-wrap:wrap;gap:8px">
            <div style="display:flex;gap:12px;flex-wrap:wrap">
              <el-tag type="success" effect="dark">{{ t('onlineUsers.totalSubscriptions') }} {{ onlineData?.online_subscriptions || 0 }}</el-tag>
              <el-tag type="info" effect="dark">{{ t('onlineUsers.totalDevices') }} {{ onlineData?.online_devices || 0 }}</el-tag>
              <el-tag effect="plain">{{ t('onlineUsers.totalUp') }} {{ formatRate(onlineData?.total_uplink_rate || 0) }}</el-tag>
              <el-tag effect="plain">{{ t('onlineUsers.totalDown') }} {{ formatRate(onlineData?.total_downlink_rate || 0) }}</el-tag>
            </div>
            <div style="display:flex;gap:8px;align-items:center">
              <el-input v-model="onlineKeyword" :placeholder="t('onlineUsers.search')" clearable size="small" style="width:200px" @input="onOnlineKeywordInput" @clear="onOnlineFilterChange">
                <template #prefix><el-icon><Search /></el-icon></template>
              </el-input>
              <span style="font-size:12px;color:#909399" v-if="onlineData?.updated_at">{{ t('onlineUsers.updatedAt') }} {{ formatAgo(onlineData.updated_at) }}</span>
              <el-button size="small" :loading="onlineLoading" @click="loadOnlineUsers">
                <el-icon><Refresh /></el-icon> {{ t('common.refresh') }}
              </el-button>
            </div>
          </div>

          <el-alert v-if="onlineError" type="error" :closable="false" style="margin-bottom:12px">{{ onlineError }}</el-alert>

          <el-table v-if="!isMobile" :data="onlineData?.users || []" v-loading="onlineLoading" row-key="subscription_id" border stripe size="small">
            <el-table-column type="expand">
              <template #default="{row}">
                <el-table :data="row.sessions || []" size="mini" border>
                  <el-table-column prop="ip" label="IP" min-width="150" />
                  <el-table-column :label="t('onlineUsers.lastActive')" min-width="110">
                    <template #default="{row:s}">{{ formatAgo(s.last_seen) }}</template>
                  </el-table-column>
                  <el-table-column :label="t('onlineUsers.duration')" min-width="110">
                    <template #default="{row:s}">{{ formatDuration((s.last_seen || 0) - (s.connected_since || 0)) }}</template>
                  </el-table-column>
                </el-table>
              </template>
            </el-table-column>
            <el-table-column :label="t('onlineUsers.subscription')" min-width="240">
              <template #default="{row}">
                <div>{{ row.email }}</div>
                <div style="font-size:12px;color:#909399">ID: {{ row.subscription_id }}</div>
              </template>
            </el-table-column>
            <el-table-column :label="t('onlineUsers.clientPlan')" min-width="170" show-overflow-tooltip>
              <template #default="{row}">
                <div>{{ row.client_name || row.email }}</div>
                <div v-if="row.plan_name" style="font-size:12px;color:#909399">{{ row.plan_name }}</div>
              </template>
            </el-table-column>
            <el-table-column :label="t('onlineUsers.devices')" width="90" align="center">
              <template #default="{row}">{{ (row.sessions || []).length }}</template>
            </el-table-column>
            <el-table-column :label="t('onlineUsers.speedLimit')" width="140" align="center">
              <template #default="{row}">
                <span v-if="(row.speed_limit_down_mbps || 0) > 0 || (row.speed_limit_up_mbps || 0) > 0">
                  ↓{{ row.speed_limit_down_mbps || 0 }} / ↑{{ row.speed_limit_up_mbps || 0 }}
                  <div style="font-size:11px;color:#909399">Mbps</div>
                </span>
                <span v-else style="color:#c0c4cc;font-size:12px">{{ t('onlineUsers.unlimited') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('onlineUsers.upRate')" width="120">
              <template #default="{row}">{{ formatRate(row.uplink_rate) }}</template>
            </el-table-column>
            <el-table-column :label="t('onlineUsers.downRate')" width="120">
              <template #default="{row}">{{ formatRate(row.downlink_rate) }}</template>
            </el-table-column>
            <el-table-column :label="t('onlineUsers.totalUp')" width="100">
              <template #default="{row}">{{ formatBytes(row.uplink_bytes) }}</template>
            </el-table-column>
            <el-table-column :label="t('onlineUsers.totalDown')" width="100">
              <template #default="{row}">{{ formatBytes(row.downlink_bytes) }}</template>
            </el-table-column>
            <el-table-column :label="t('onlineUsers.lastActive')" width="100">
              <template #default="{row}">{{ formatAgo(row.last_seen) }}</template>
            </el-table-column>
            <el-table-column :label="t('onlineUsers.duration')" width="100">
              <template #default="{row}">{{ formatDuration((row.last_seen || 0) - (row.first_seen || 0)) }}</template>
            </el-table-column>
          </el-table>
          <MobileCards
            v-else
            :rows="onlineData?.users || []"
            title-field="email"
            :fields="onlineCardFields"
          />

          <el-empty v-if="!onlineLoading && (!onlineData?.users || onlineData.users.length === 0)" :description="t('onlineUsers.noOnline')" style="margin-top:20px" />

          <div v-if="(onlineData?.filtered_total || 0) > 0" style="display:flex;justify-content:flex-end;margin-top:14px">
            <el-pagination
              v-model:current-page="onlinePage"
              v-model:page-size="onlinePageSize"
              :total="onlineData?.filtered_total || 0"
              :page-sizes="[20, 50, 100]"
              layout="total, sizes, prev, pager, next"
              background
              small
              @size-change="onOnlineSizeChange"
              @current-change="onOnlinePageChange"
            />
          </div>
        </div>
      </el-tab-pane>
    </el-tabs>

    <!-- 重新生成 Bootstrap Token（仅证书失效时可用）-->
    <el-dialog v-model="regenVisible" :title="t('nodeServer.regenerateToken')" width="680px">
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom:12px">
        <template #title>{{ t('nodeServer.regenerateOnceWarn') }}</template>
      </el-alert>
      <el-alert v-if="isInsecureContext" type="error" :closable="false" show-icon style="margin-bottom:12px">
        <template #title>{{ t('nodeServer.insecureTokenWarning') }}</template>
      </el-alert>
      <el-form label-width="90px">
        <el-form-item :label="t('nodeServer.bootstrapToken')">
          <el-input v-model="regenToken" readonly>
            <template #append>
              <el-button @click="copyText(regenToken)">{{ t('nodeServer.copyCmd') }}</el-button>
            </template>
          </el-input>
        </el-form-item>
        <el-form-item :label="t('nodeServer.tokenExpiresAt')">
          <span>{{ regenExpiresAt ? formatTime(regenExpiresAt) : '-' }}</span>
        </el-form-item>
        <el-form-item :label="t('nodeServer.recoverySteps')">
          <el-input v-model="regenCmd" type="textarea" :rows="7" readonly />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button size="small" @click="copyText(regenCmd)">{{ t('nodeServer.copyCmd') }}</el-button>
        <el-button size="small" type="primary" @click="regenVisible = false">{{ t('common.confirm') }}</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useI18n } from 'vue-i18n'
import { ElMessage } from 'element-plus'
import { ArrowLeft, Refresh, Upload, Search } from '@element-plus/icons-vue'
import client from '../api/client'
import { useClientPagination } from '../composables/useClientPagination'
import { useIsMobile } from '../composables/useIsMobile'
import PaginationBar from '../components/PaginationBar.vue'
import MobileCards from '../components/MobileCards.vue'
import { getAgentVersion, getAgentReleases, updateAgent, getPendingAgentUpdates, distributeCertificate, associateCertificate, generateBootstrapToken } from '../api/nodeServer'
import ProtocolCenter from './protocols/ProtocolCenter.vue'
import ProxyManagement from './ProxyManagement.vue'
import CoreProcessTab from './node-server/CoreProcessTab.vue'
import WarpTab from './services/WarpTab.vue'
import ShareDomainTab from './services/ShareDomainTab.vue'
import RateLimitTab from './services/RateLimitTab.vue'
import CoreTrafficTab from './node-server/CoreTrafficTab.vue'
import { useQueryState } from '../composables/useQueryState'

const { t } = useI18n()
const route = useRoute()
const router = useRouter()

const serverId = parseInt(route.params.id) || 0
const activeTab = useQueryState('tab', 'overview')
const server = ref(null)
const certificates = ref([])

const { paged: certPagedList, page: certPage, pageSize: certSize, total: certTotal } = useClientPagination(certificates, { pageKey: 'certPage', pageSizeKey: 'certSize' })
const certLoading = ref(false)

// Agent update state
const panelAgentVersion = ref(null)
const panelVersionLoading = ref(false)
const githubReleases = ref([])
const releasesLoading = ref(false)
const selectedRelease = ref('')
const updating = ref(false)

// 面板本地「有二进制且有版本号」才允许走本地推送；VERSION 缺失时后端返回 unknown。
const hasLocalAgent = computed(() => {
  const v = panelAgentVersion.value?.version
  return !!v && v !== 'unknown'
})
const updateTask = ref(null)
let pollTimer = null

// Cert sync
const syncingCertId = ref(null)

// Cert bind dialog
const showCertBindDialog = ref(false)
const certBindId = ref(null)
const certBinding = ref(false)
const allCerts = ref([])

const nodeTypeOptions = {
  standalone: t('nodeServer.nodeTypeStandalone'),
  primary: t('nodeServer.nodeTypePrimary'),
  mirror: t('nodeServer.nodeTypeMirror'),
  chain_entry: t('nodeServer.nodeTypeChainEntry'),
  chain_exit: t('nodeServer.nodeTypeChainExit'),
}

const statusLabels = {
  online: t('nodeServer.statusOnline'),
  offline: t('nodeServer.statusOffline'),
  degraded: t('nodeServer.statusDegraded'),
  pending: t('nodeServer.statusPending'),
}

const selectedReleaseAssets = computed(() => {
  if (!selectedRelease.value) return []
  const rel = githubReleases.value.find(r => r.version === selectedRelease.value)
  return rel?.assets || []
})

function nodeTypeLabel(nt) { return nodeTypeOptions[nt] || nt || '-' }
function statusLabel(s) { return statusLabels[s] || s || '-' }
function statusTag(s) {
  if (s === 'online') return 'success'
  if (s === 'degraded') return 'warning'
  if (s === 'pending') return 'info'
  return 'danger'
}
function updateStatusTag(s) {
  if (s === 'done') return 'success'
  if (s === 'failed') return 'danger'
  if (s === 'pending') return 'info'
  return 'warning'
}
function formatTime(ts) {
  if (!ts || ts === '0001-01-01T00:00:00Z') return '-'
  return new Date(ts).toLocaleString()
}

function goBack() { router.push('/node-servers') }

function certExpiringSoon(ts) {
  if (!ts) return false
  const days = (new Date(ts).getTime() - Date.now()) / 86400000
  return days < 30
}

// Agent mTLS 证书：面板在到期前 3 天开始自动续签，所以用同一个窗口做"即将过期"提示
function agentCertExpiringSoon(ts) {
  if (!ts) return false
  const days = (new Date(ts).getTime() - Date.now()) / 86400000
  return days < 3
}

// ── 证书失效后的重新引导（仅当 agent_cert_invalid 为真时才提供入口）──
const regenVisible = ref(false)
const regenLoading = ref(false)
const regenToken = ref('')
const regenCmd = ref('')
const regenExpiresAt = ref(null)
// 非 HTTPS 时令牌会明文传输（引导令牌可换到 Agent 证书），必须提示
const isInsecureContext = computed(() => window.location.protocol !== 'https:' && !['localhost', '127.0.0.1'].includes(window.location.hostname))

async function regenerateBootstrapToken() {
  regenLoading.value = true
  try {
    const res = await generateBootstrapToken(serverId)
    const token = res?.token || ''
    if (!token) {
      ElMessage.error(t('nodeServer.cannotGenerateDeployToken'))
      return
    }
    regenToken.value = token
    regenExpiresAt.value = res?.expires_at || null
    regenCmd.value = buildRecoveryCmd(token)
    regenVisible.value = true
    // 令牌生成后 bootstrap_required 会变成 true，刷新一次让页面状态跟上
    loadServer()
  } catch (e) {
    // 后端会拒发（证书仍有效/本地面板节点），把具体原因显示出来，别吞成通用错误
    ElMessage.error(e?.response?.data?.message || t('nodeServer.generateDeployFailed'))
  } finally {
    regenLoading.value = false
  }
}

// 恢复步骤：只替换 bootstrap.token，不动其它配置（配置里的 host/port 仍然有效）
function buildRecoveryCmd(token) {
  return [
    '# 1) 把新的 Bootstrap Token 写进 Agent 配置（只替换 token 这一行）',
    `sed -i 's|^[[:space:]]*token:.*|  token: "${token}"|' /opt/corefusion-agent/config.yaml`,
    '',
    '# 2) 重启 Agent（证书失效时它会每 5 分钟自动重试引导，无需手工再等）',
    'systemctl restart corefusion-agent || (pkill -9 -x corefusion-agent; sleep 1; cd /opt/corefusion-agent && setsid nohup ./corefusion-agent -config config.yaml >> /tmp/agent.log 2>&1 &)',
    '',
    '# 3) 看结果：出现 "bootstrapping with panel" / "connected to panel" 即恢复',
    'journalctl -u corefusion-agent -n 20 --no-pager | grep -iE "bootstrap|mTLS|connected to panel"',
  ].join('\n')
}

function copyText(text) {
  if (!text) return
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text)
      .then(() => ElMessage.success(t('nodeServer.copied')))
      .catch(() => fallbackCopyText(text))
    return
  }
  fallbackCopyText(text)
}

function fallbackCopyText(text) {
  const ta = document.createElement('textarea')
  ta.value = text
  ta.style.position = 'fixed'
  ta.style.left = '-9999px'
  document.body.appendChild(ta)
  ta.select()
  try {
    document.execCommand('copy')
    ElMessage.success(t('nodeServer.copied'))
  } catch (e) {
    ElMessage.error(t('common.failed'))
  }
  document.body.removeChild(ta)
}

async function syncCertToNode(cert) {
  syncingCertId.value = cert.id
  try {
    await distributeCertificate(serverId, cert.id)
    ElMessage.success(t('nodeServer.syncCertSuccess'))
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('nodeServer.syncCertFailed'))
  } finally {
    syncingCertId.value = null
  }
}

async function loadServer() {
  try {
    server.value = await client.get(`/node-servers/${serverId}`)
  } catch (e) {
    ElMessage.error(t('common.loadFailed'))
  }
}

async function loadCerts() {
  certLoading.value = true
  try {
    const data = await client.get(`/node-servers/${serverId}/certificates`)
    certificates.value = data?.certificates || []
  } catch {
    certificates.value = []
  } finally {
    certLoading.value = false
  }
}

async function loadAllCerts() {
  try {
    const data = await client.get('/certificates', { params: { limit: 100, offset: 0 } })
    allCerts.value = data?.items || []
  } catch {
    allCerts.value = []
  }
}

function sourceLabel(src) {
  if (src === 'node-level') return t('nodeServer.certSrcNodeLevel')
  if (src.startsWith('xray:')) return 'Xray: ' + src.substring(5)
  if (src.startsWith('caddy:')) return 'Caddy: ' + src.substring(7)
  if (src.startsWith('nginx:')) return 'Nginx: ' + src.substring(6)
  if (src.startsWith('proxy:')) return 'Proxy: ' + src.substring(6)
  return src
}

function sourceTagType(src) {
  if (src === 'node-level') return 'primary'
  if (src.startsWith('xray:')) return 'success'
  if (src.startsWith('caddy:')) return 'warning'
  if (src.startsWith('nginx:')) return 'danger'
  return 'info'
}

async function handleBindCert() {
  if (!certBindId.value) {
    ElMessage.warning(t('nodeServer.selectCertPlaceholder'))
    return
  }
  certBinding.value = true
  try {
    await associateCertificate(serverId, certBindId.value)
    ElMessage.success(t('common.success'))
    showCertBindDialog.value = false
    certBindId.value = null
    loadCerts()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || t('common.failed'))
  } finally {
    certBinding.value = false
  }
}

async function loadPanelAgentVersion() {
  panelVersionLoading.value = true
  try {
    panelAgentVersion.value = await getAgentVersion()
  } catch {
    panelAgentVersion.value = null
  } finally {
    panelVersionLoading.value = false
  }
}

async function loadGithubReleases() {
  releasesLoading.value = true
  try {
    const data = await getAgentReleases(20)
    githubReleases.value = data?.releases || []
  } catch {
    githubReleases.value = []
  } finally {
    releasesLoading.value = false
  }
}

async function pushUpdateFromPanel() {
  if (!hasLocalAgent.value) return
  updating.value = true
  try {
    await updateAgent(serverId, {
      version: panelAgentVersion.value.version,
      download_url: '',
      checksum: '',
      source: 'panel',
    })
    ElMessage.success('Update pushed')
    startPolling()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || 'Update failed')
  } finally {
    updating.value = false
  }
}

async function pushUpdateFromGitHub() {
  if (!selectedRelease.value) return
  updating.value = true
  try {
    await updateAgent(serverId, {
      version: selectedRelease.value,
      download_url: '',
      checksum: '',
      source: 'github',
    })
    ElMessage.success('Update pushed')
    startPolling()
  } catch (e) {
    ElMessage.error(e?.response?.data?.message || 'Update failed')
  } finally {
    updating.value = false
  }
}

function startPolling() {
  stopPolling()
  pollTimer = setInterval(async () => {
    try {
      const data = await getPendingAgentUpdates()
      const tasks = Array.isArray(data) ? data : (data?.updates || [])
      const task = tasks.find(t => t.server_id === serverId)
      if (task) {
        updateTask.value = task
        if (task.status === 'done' || task.status === 'failed') {
          stopPolling()
          loadServer()
        }
      } else {
        // No pending task — check if server version changed
        await loadServer()
        if (server.value?.agent_version) {
          stopPolling()
        }
      }
    } catch {
      // ignore polling errors
    }
  }, 3000)
}

function stopPolling() {
  if (pollTimer) {
    clearInterval(pollTimer)
    pollTimer = null
  }
}

const onlineData = ref(null)
const onlineLoading = ref(false)
const onlineError = ref('')
const onlineKeyword = ref('')
const onlinePage = ref(1)
const onlinePageSize = ref(20)
let onlineSearchDebounce = null

function onOnlineFilterChange() {
  onlinePage.value = 1
  loadOnlineUsers()
}
function onOnlineKeywordInput() {
  clearTimeout(onlineSearchDebounce)
  onlineSearchDebounce = setTimeout(onOnlineFilterChange, 400)
}
function onOnlinePageChange(p) { onlinePage.value = p; loadOnlineUsers() }
function onOnlineSizeChange(s) { onlinePageSize.value = s; onlinePage.value = 1; loadOnlineUsers() }

async function loadOnlineUsers() {
  onlineLoading.value = true
  onlineError.value = ''
  try {
    const params = {
      limit: onlinePageSize.value,
      offset: (onlinePage.value - 1) * onlinePageSize.value,
    }
    const kw = (onlineKeyword.value || '').trim()
    if (kw) params.keyword = kw
    const data = await client.get(`/node-servers/${serverId}/online-users`, { params })
    onlineData.value = data
  } catch (e) {
    onlineError.value = e?.response?.data?.message || t('onlineUsers.loadFailed')
  } finally {
    onlineLoading.value = false
  }
}

function formatRate(bps) {
  if (!bps || bps <= 0) return '0 B/s'
  if (bps < 1024) return bps + ' B/s'
  if (bps < 1024 * 1024) return (bps / 1024).toFixed(1) + ' KB/s'
  return (bps / 1024 / 1024).toFixed(2) + ' MB/s'
}
function formatBytes(bytes) {
  if (!bytes || bytes <= 0) return '0 B'
  if (bytes < 1024) return bytes + ' B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}
function formatDuration(sec) {
  if (!sec || sec <= 0) return '0s'
  const h = Math.floor(sec / 3600)
  const m = Math.floor((sec % 3600) / 60)
  const s = Math.floor(sec % 60)
  const pad = (n) => String(n).padStart(2, '0')
  return (h > 0 ? h + ':' : '') + pad(m) + ':' + pad(s)
}
function formatAgo(ts) {
  if (!ts) return '-'
  const diff = Math.max(0, Math.floor(Date.now() / 1000) - ts)
  if (diff < 60) return diff + ' 秒前'
  if (diff < 3600) return Math.floor(diff / 60) + ' 分钟前'
  return Math.floor(diff / 3600) + ' 小时前'
}

let onlineTimer = null
function startOnlinePolling() {
  loadOnlineUsers()
  onlineTimer = setInterval(loadOnlineUsers, 15000)
}
function stopOnlinePolling() {
  if (onlineTimer) { clearInterval(onlineTimer); onlineTimer = null }
}

const { isMobile } = useIsMobile()
// 手机端卡片字段（与桌面表格列对应；证书表）
const certCardFields = computed(() => [
  { label: 'ID', text: (row) => (row.id ?? '-') },
  { label: t('nodeServer.certSource'), text: (row) => (row.sources && row.sources.length) ? row.sources.map(sourceLabel).join(' / ') : '-' },
  { label: 'Issuer', text: (row) => row.issuer || '-' },
  { label: t('nodeServer.certProvider'), text: (row) => row.provider_type || '-' },
  { label: t('nodeServer.certAutoRenew'), text: (row) => (row.auto_renew ? t('common.yes') : t('common.no')) },
  { label: t('nodeServer.certExpiry'), text: (row) => (row.expires_at ? formatTime(row.expires_at) : '-') },
  { label: t('nodeServer.certPath'), text: (row) => row.cert_path || '-' },
])
// 在线用户卡片（桌面端展开列的会话明细在手机端不展示，设备数保留）
const onlineCardFields = computed(() => [
  { label: t('onlineUsers.subscription'), text: (row) => (row.subscription_id != null ? 'ID: ' + row.subscription_id : '-') },
  { label: t('onlineUsers.clientPlan'), text: (row) => (row.client_name || row.email || '-') + (row.plan_name ? ' / ' + row.plan_name : '') },
  { label: t('onlineUsers.devices'), text: (row) => String((row.sessions || []).length) },
  { label: t('onlineUsers.speedLimit'), text: (row) => ((row.speed_limit_down_mbps || 0) > 0 || (row.speed_limit_up_mbps || 0) > 0) ? '↓' + (row.speed_limit_down_mbps || 0) + ' / ↑' + (row.speed_limit_up_mbps || 0) + ' Mbps' : t('onlineUsers.unlimited') },
  { label: t('onlineUsers.upRate'), text: (row) => formatRate(row.uplink_rate) },
  { label: t('onlineUsers.downRate'), text: (row) => formatRate(row.downlink_rate) },
  { label: t('onlineUsers.totalUp'), text: (row) => formatBytes(row.uplink_bytes) },
  { label: t('onlineUsers.totalDown'), text: (row) => formatBytes(row.downlink_bytes) },
  { label: t('onlineUsers.lastActive'), text: (row) => formatAgo(row.last_seen) },
  { label: t('onlineUsers.duration'), text: (row) => formatDuration((row.last_seen || 0) - (row.first_seen || 0)) },
])

onMounted(() => {
  loadServer()
  loadCerts()
  loadAllCerts()
  loadPanelAgentVersion()
  loadGithubReleases()
  startOnlinePolling()
})

onUnmounted(() => {
  stopPolling()
  stopOnlinePolling()
  clearTimeout(onlineSearchDebounce)
})
</script>

<style scoped>
.status-dot {
  display: inline-block;
  width: 6px;
  height: 6px;
  border-radius: 50%;
  margin-right: 4px;
}
.status-dot.online { background: #67c23a; }
.status-dot.offline { background: #f56c6c; }
.status-dot.degraded { background: #e6a23c; }
.status-dot.pending { background: #909399; }
</style>

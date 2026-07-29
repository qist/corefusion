<template>
  <div class="log-manage-page">
    <!-- Global Node Selector (affects Overview, Output Config, Policies tabs) -->
    <div class="node-selector-bar">
      <span class="node-selector-label">
        <el-icon><Monitor /></el-icon>
        {{ t('logManage.selectNode') }}:
      </span>
      <el-select
        v-model="selectedServerId"
        style="width:220px"
        @change="onSelectedServerChange"
      >
        <el-option :value="1" :label="t('logManage.localNode')">
          <el-icon><Monitor /></el-icon> {{ t('logManage.localNode') }}
        </el-option>
        <el-option
          v-for="s in nodeServers"
          :key="s.id"
          :value="s.id"
          :label="s.name"
        >
          <span>{{ s.name }}</span>
          <el-tag v-if="s.agent_status !== 'online'" size="small" type="danger" style="margin-left:8px">{{ t('logManage.nodeOffline') }}</el-tag>
        </el-option>
      </el-select>
      <el-tag v-if="selectedServerId > 1" type="warning" size="small" style="margin-left:8px">
        {{ selectedServerName }}
      </el-tag>
    </div>

    <el-tabs v-model="activeTab" type="border-card" class="log-tabs">
      <!-- ═══════════════ Tab 1: Overview ═══════════════ -->
      <el-tab-pane :label="t('logManage.tabOverview')" name="overview">
        <!-- Status Cards -->
        <el-row :gutter="16" style="margin-bottom:20px">
          <el-col :span="6">
            <el-card shadow="hover" class="stat-card-wrap">
              <div class="stat-card">
                <div class="stat-icon stat-icon-blue"><el-icon><Document /></el-icon></div>
                <div class="stat-body">
                  <div class="stat-value">{{ status.total_policies }}</div>
                  <div class="stat-label">{{ t('logManage.totalPolicies') }}</div>
                </div>
              </div>
            </el-card>
          </el-col>
          <el-col :span="6">
            <el-card shadow="hover" class="stat-card-wrap">
              <div class="stat-card">
                <div class="stat-icon stat-icon-green"><el-icon><CircleCheck /></el-icon></div>
                <div class="stat-body">
                  <div class="stat-value text-success">{{ status.enabled_count }}</div>
                  <div class="stat-label">{{ t('logManage.enabledPolicies') }}</div>
                </div>
              </div>
            </el-card>
          </el-col>
          <el-col :span="6">
            <el-card shadow="hover" class="stat-card-wrap">
              <div class="stat-card">
                <div class="stat-icon stat-icon-primary"><el-icon><Timer /></el-icon></div>
                <div class="stat-body">
                  <div class="stat-value text-primary">{{ status.scheduled_count }}</div>
                  <div class="stat-label">{{ t('logManage.scheduledJobs') }}</div>
                </div>
              </div>
            </el-card>
          </el-col>
          <el-col :span="6">
            <el-card shadow="hover" class="stat-card-wrap">
              <div class="stat-card">
                <div class="stat-icon stat-icon-orange"><el-icon><DataLine /></el-icon></div>
                <div class="stat-body">
                  <div class="stat-value text-warning">{{ status.total_jobs }}</div>
                  <div class="stat-label">{{ t('logManage.totalRuns') }}</div>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>

        <!-- Output Mode Quick View -->
        <el-row :gutter="16">
          <el-col :span="12">
            <el-card shadow="hover">
              <template #header>
                <div class="card-header-flex">
                  <span>{{ t('logManage.coreLogOutput') }}</span>
                  <el-button text size="small" @click="activeTab = 'output'">{{ t('common.edit') }}</el-button>
                </div>
              </template>
              <div class="mode-list">
                <div v-for="item in coreOutputList" :key="item.type" class="mode-item">
                  <span class="mode-label">{{ item.label }}</span>
                  <el-tag :type="modeTagType(item.cfg.mode)" size="small">{{ modeLabel(item.cfg.mode) }}</el-tag>
                </div>
              </div>
            </el-card>
          </el-col>
          <el-col :span="12">
            <el-card shadow="hover">
              <template #header>
                <div class="card-header-flex">
                  <span>{{ t('logManage.proxyLogOutput') }}</span>
                  <el-button text size="small" @click="activeTab = 'output'">{{ t('common.edit') }}</el-button>
                </div>
              </template>
              <div class="mode-list">
                <div v-for="item in proxyOutputList" :key="item.type" class="mode-item">
                  <span class="mode-label">{{ item.label }}</span>
                  <el-tag :type="modeTagType(item.cfg.mode)" size="small">{{ modeLabel(item.cfg.mode) }}</el-tag>
                </div>
              </div>
            </el-card>
          </el-col>
        </el-row>

        <!-- Quick Actions -->
        <el-card shadow="hover" style="margin-top:16px">
          <template #header><span>{{ t('common.actions') }}</span></template>
          <div style="display:flex;gap:12px;flex-wrap:wrap">
            <el-button @click="handleDiscover" :loading="discovering">
              <el-icon><Search /></el-icon>{{ t('logManage.discover') }}
            </el-button>
            <el-button type="success" @click="handleSync" :loading="syncing">
              <el-icon><Refresh /></el-icon>{{ t('logManage.syncPolicies') }}
            </el-button>
            <el-button type="primary" @click="showCreate">
              <el-icon><Plus /></el-icon>{{ t('logManage.create') }}
            </el-button>
            <el-button @click="activeTab = 'viewer'">
              <el-icon><View /></el-icon>{{ t('logManage.tabLogViewer') }}
            </el-button>
          </div>
        </el-card>
      </el-tab-pane>

      <!-- ═══════════════ Tab 2: Output Config ═══════════════ -->
      <el-tab-pane :label="t('logManage.tabOutputConfig')" name="output">
        <!-- Core Log Output Config -->
        <el-card shadow="hover" style="margin-bottom:16px">
          <template #header>
            <div class="card-header-flex">
              <div>
                <span style="font-weight:600">{{ t('logManage.coreLogOutput') }}</span>
                <span class="header-hint">{{ t('logManage.coreLogOutputHint') }}</span>
              </div>
              <el-button type="primary" size="small" :loading="savingCoreOutput" :disabled="fetchingOutput" @click="handleSaveCoreOutput">
                {{ t('logManage.saveOutputConfig') }}
              </el-button>
            </div>
          </template>
          <el-table :data="coreOutputList" stripe>
            <el-table-column :label="t('common.name')" width="140">
              <template #default="{row}">
                <span style="font-weight:600">{{ row.label }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('logManage.mode')" width="200">
              <template #default="{row}">
                <el-tag :type="modeTagType(row.cfg.mode)" size="default" effect="dark">{{ modeLabel(row.cfg.mode) }}</el-tag>
                <span style="margin-left:8px;color:#909399;font-size:12px">{{ t('logManage.modeControlledByProtocol') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('logManage.capturePath')" min-width="200">
              <template #default="{row}">
                <el-input
                  v-model="row.cfg.capture_path"
                  size="small"
                  :placeholder="row.cfg.mode === 'stdout' ? `logs/${row.type}-stdout.log` : `logs/${row.type}.log`"
                  :disabled="row.cfg.mode !== 'stdout'"
                  style="max-width:300px"
                />
              </template>
            </el-table-column>
            <el-table-column :label="t('logManage.managed')" width="100">
              <template #default="{row}">
                <el-switch v-model="row.cfg.managed" :disabled="row.cfg.mode !== 'stdout'" />
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <!-- Proxy Log Output Config -->
        <el-card shadow="hover" style="margin-bottom:16px">
          <template #header>
            <div class="card-header-flex">
              <div>
                <span style="font-weight:600">{{ t('logManage.proxyLogOutput') }}</span>
                <span class="header-hint">{{ t('logManage.proxyLogOutputHint') }}</span>
              </div>
              <el-button type="primary" size="small" :loading="savingProxyOutput" :disabled="fetchingOutput" @click="handleSaveProxyOutput">
                {{ t('logManage.saveOutputConfig') }}
              </el-button>
            </div>
          </template>
          <el-table :data="proxyOutputList" stripe>
            <el-table-column :label="t('common.name')" width="140">
              <template #default="{row}">
                <span style="font-weight:600">{{ row.label }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('logManage.mode')" width="200">
              <template #default="{row}">
                <el-tag :type="modeTagType(row.cfg.mode)" size="default" effect="dark">{{ modeLabel(row.cfg.mode) }}</el-tag>
                <span style="margin-left:8px;color:#909399;font-size:12px">{{ t('logManage.modeControlledByProtocol') }}</span>
              </template>
            </el-table-column>
            <el-table-column :label="t('logManage.capturePath')" min-width="200">
              <template #default="{row}">
                <el-input
                  v-model="row.cfg.capture_path"
                  size="small"
                  :placeholder="row.cfg.mode === 'stdout' ? `logs/${row.type}-stdout.log` : `logs/${row.type}-access.log`"
                  :disabled="row.cfg.mode !== 'stdout'"
                  style="max-width:300px"
                />
              </template>
            </el-table-column>
            <el-table-column :label="t('logManage.managed')" width="100">
              <template #default="{row}">
                <el-switch v-model="row.cfg.managed" :disabled="row.cfg.mode !== 'stdout'" />
              </template>
            </el-table-column>
          </el-table>
        </el-card>

        <!-- Writer Status (local + remote) -->
        <el-card shadow="hover">
          <template #header>
            <div class="card-header-flex">
              <span style="font-weight:600">LogWriter {{ t('common.status') }}</span>
              <el-button text size="small" @click="fetchWriterStatuses">
                <el-icon><Refresh /></el-icon>{{ t('common.refresh') }}
              </el-button>
            </div>
          </template>
          <el-table :data="writerStatuses" stripe size="small" style="width:100%">
            <el-table-column prop="proxy_type" :label="t('common.type')" width="100">
              <template #default="{row}">
                <el-tag size="small" :type="isCoreType(row.proxy_type) ? 'info' : 'success'">{{ row.proxy_type }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('common.status')" width="100">
              <template #default="{row}">
                <el-tag v-if="row.mode === 'off'" type="info" size="small">{{ t('logManage.modeOff') }}</el-tag>
                <el-tag v-else-if="row.mode === 'file'" type="info" size="small">{{ t('logManage.modeFile') }}</el-tag>
                <el-tag v-else-if="row.active && !row.paused" type="success" size="small">{{ t('logManage.writerActive') }}</el-tag>
                <el-tag v-else-if="row.paused" type="warning" size="small">{{ t('logManage.writerPaused') }}</el-tag>
                <el-tag v-else type="info" size="small">{{ t('common.disabled') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('logManage.mode')" width="90">
              <template #default="{row}">
                <el-tag :type="modeTagType(row.mode)" size="small">{{ modeLabel(row.mode) }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('logManage.managed')" width="100">
              <template #default="{row}">
                <el-tag :type="row.managed ? 'success' : 'info'" size="small">
                  {{ row.managed ? t('common.yes') : t('common.no') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="file_path" :label="t('logManage.filePath')" min-width="160" show-overflow-tooltip>
              <template #default="{row}">
                {{ row.mode === 'off' ? '-' : row.file_path }}
              </template>
            </el-table-column>
            <el-table-column prop="max_size_mb" :label="t('logManage.maxSizeMB')" width="90">
              <template #default="{row}">{{ row.max_size_mb || 100 }} MB</template>
            </el-table-column>
            <el-table-column prop="retain_days" :label="t('logManage.retainDays')" width="80" />
            <el-table-column prop="compress" :label="t('logManage.compress')" width="70">
              <template #default="{row}">
                <el-tag :type="row.compress ? 'warning' : 'info'" size="small">{{ row.compress ? t('common.yes') : t('common.no') }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="220" fixed="right">
              <template #default="{row}">
                <el-button size="small" type="primary" @click="showStdoutViewer(row.proxy_type)">
                  <el-icon><View /></el-icon>{{ t('logManage.viewRealtime') }}
                </el-button>
                <el-button size="small" type="warning" @click="showWriterConfig(row.proxy_type)">
                  <el-icon><Setting /></el-icon>{{ t('logManage.writerConfig') }}
                </el-button>
                <el-button size="small" @click="showWriterFiles(row.proxy_type)">
                  <el-icon><FolderOpened /></el-icon>
                </el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- ═══════════════ Tab 3: Rotation Policies ═══════════════ -->
      <el-tab-pane :label="t('logManage.tabPolicies')" name="policies">
        <!-- Header -->
        <div class="tab-header">
          <div style="display:flex;gap:10px">
            <el-button @click="handleDiscover" :loading="discovering">
              <el-icon><Search /></el-icon>{{ t('logManage.discover') }}
            </el-button>
            <el-button type="success" @click="handleSync" :loading="syncing">
              <el-icon><Refresh /></el-icon>{{ t('logManage.syncPolicies') }}
            </el-button>
            <el-button type="primary" @click="showCreate">
              <el-icon><Plus /></el-icon>{{ t('logManage.create') }}
            </el-button>
          </div>
        </div>

        <!-- Discovered Logs (collapsible) -->
        <el-collapse-transition>
          <el-card v-if="discoveredLogs.length > 0" style="margin-bottom:16px">
            <template #header>
              <div class="card-header-flex">
                <span>{{ t('logManage.discoveredLogs') }} ({{ discoveredLogs.length }})</span>
                <el-button text size="small" @click="discoveredLogs = []">{{ t('common.close') }}</el-button>
              </div>
            </template>
            <el-table :data="discoveredLogs" size="small" stripe>
              <el-table-column prop="source_type" :label="t('logManage.sourceType')" width="100">
                <template #default="{row}">
                  <el-tag :type="sourceTagType(row.source_type)" size="small">{{ row.source_type }}</el-tag>
                </template>
              </el-table-column>
              <el-table-column prop="domain" :label="t('logManage.domain')" width="180" show-overflow-tooltip>
                <template #default="{row}">{{ row.domain || '-' }}</template>
              </el-table-column>
              <el-table-column prop="log_path" :label="t('logManage.logPath')" />
              <el-table-column prop="log_format" :label="t('logManage.logFormat')" width="80">
                <template #default="{row}">{{ row.log_format || '-' }}</template>
              </el-table-column>
              <el-table-column :label="t('common.actions')" width="160" align="center">
                <template #default="{row}">
                  <el-button
                    v-if="!row.has_policy"
                    type="primary"
                    size="small"
                    :loading="addingPath === row.log_path"
                    @click="handleAddFromDiscover(row)"
                  >
                    <el-icon><Plus /></el-icon>{{ t('logManage.addToPolicy') }}
                  </el-button>
                  <el-tag v-else type="success" size="small">
                    <el-icon><CircleCheck /></el-icon>{{ t('logManage.alreadyInPolicy') }}
                  </el-tag>
                </template>
              </el-table-column>
            </el-table>
          </el-card>
        </el-collapse-transition>

        <!-- Policies Table -->
        <el-card>
          <el-table :data="policies" v-loading="loading" stripe>
            <el-table-column prop="id" :label="t('common.id')" width="60" />
            <el-table-column :label="t('logManage.nodeLabel')" width="100">
              <template #default>
                <el-tag size="small" :type="selectedServerId > 1 ? 'warning' : 'info'">{{ selectedServerName }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="name" :label="t('common.name')" min-width="120" show-overflow-tooltip />
            <el-table-column prop="source_type" :label="t('logManage.sourceType')" width="100">
              <template #default="{row}">
                <el-tag :type="sourceTagType(row.source_type)" size="small">{{ row.source_type }}</el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="log_path" :label="t('logManage.logPath')" min-width="180" show-overflow-tooltip />
            <el-table-column prop="enabled" :label="t('common.status')" width="70">
              <template #default="{row}">
                <el-tag :type="row.enabled ? 'success' : 'danger'" size="small">
                  {{ row.enabled ? t('common.enabled') : t('common.disabled') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column prop="max_size" :label="t('logManage.maxSizeMB')" width="90">
              <template #default="{row}">
                {{ row.max_size ? formatBytes(row.max_size) : t('common.unlimited') }}
              </template>
            </el-table-column>
            <el-table-column prop="retain_days" :label="t('logManage.retainDays')" width="80" />
            <el-table-column prop="compress" :label="t('logManage.compress')" width="70">
              <template #default="{row}">
                <el-tag :type="row.compress ? 'warning' : 'info'" size="small">
                  {{ row.compress ? t('common.yes') : t('common.no') }}
                </el-tag>
              </template>
            </el-table-column>
            <el-table-column :label="t('common.actions')" width="320" fixed="right">
              <template #default="{row}">
                <el-button size="small" type="success" @click="showLogViewer(row)">
                  <el-icon><View /></el-icon>{{ t('logManage.viewLog') }}
                </el-button>
                <el-button size="small" type="primary" @click="handleRotate(row)" :loading="rotatingId === row.id">
                  <el-icon><RefreshRight /></el-icon>{{ t('logManage.rotate') }}
                </el-button>
                <el-button size="small" @click="showJobs(row)">{{ t('logManage.history') }}</el-button>
                <el-button size="small" @click="showEdit(row)">{{ t('common.edit') }}</el-button>
                <el-button size="small" type="danger" @click="handleDelete(row)">{{ t('common.delete') }}</el-button>
              </template>
            </el-table-column>
          </el-table>
        </el-card>
      </el-tab-pane>

      <!-- ═══════════════ Tab 4: Log Viewer ═══════════════ -->
      <el-tab-pane :label="t('logManage.tabLogViewer')" name="viewer">
        <el-card shadow="hover">
          <!-- Node + Log Type Selector -->
          <div class="viewer-toolbar">
            <div class="viewer-selector">
              <el-select
                v-model="viewerNodeId"
                :placeholder="t('logManage.selectNode')"
                style="width:220px"
                @change="onViewerNodeChange"
              >
                <el-option :value="0" :label="t('logManage.localNode')">
                  <el-icon><Monitor /></el-icon> {{ t('logManage.localNode') }}
                </el-option>
                <el-option
                  v-for="s in nodeServers"
                  :key="s.id"
                  :value="s.id"
                  :label="s.name"
                  :disabled="s.agent_status !== 'online'"
                >
                  <span>{{ s.name }}</span>
                  <el-tag v-if="s.agent_status !== 'online'" size="small" type="danger" style="margin-left:8px">{{ t('logManage.nodeOffline') }}</el-tag>
                </el-option>
              </el-select>

              <el-select
                v-model="viewerLogType"
                :placeholder="t('logManage.selectLogType')"
                style="width:200px;margin-left:12px"
                :disabled="!viewerNodeId && viewerNodeId !== 0"
                @change="onViewerLogTypeChange"
              >
                <el-option
                  v-for="lt in viewerLogTypes"
                  :key="lt.type"
                  :value="lt.type"
                  :label="lt.label"
                >
                  <el-tag :type="sourceTagType(lt.type)" size="small" style="margin-right:6px">{{ lt.type }}</el-tag>
                  {{ lt.label }}
                </el-option>
              </el-select>

              <el-select
                v-if="viewerLogFiles.length > 0"
                v-model="viewerLogPath"
                :placeholder="t('logManage.selectLogFile')"
                style="width:280px;margin-left:12px"
                clearable
                @change="loadViewerLog"
              >
                <el-option
                  v-for="f in viewerLogFiles"
                  :key="f.log_path"
                  :value="f.log_path"
                  :label="f.domain ? `${f.domain} (${f.log_path})` : f.log_path"
                />
              </el-select>

              <el-input-number
                v-model="viewerLines"
                :min="50"
                :max="5000"
                :step="100"
                style="width:140px;margin-left:12px"
              />

              <el-button type="primary" style="margin-left:12px" :loading="viewerLoading" @click="loadViewerLog">
                <el-icon><Search /></el-icon>{{ t('common.search') }}
              </el-button>

              <el-button
                v-if="viewerLogType"
                type="warning"
                style="margin-left:8px"
                :loading="viewerRotating"
                @click="handleRotateNodeLog"
              >
                <el-icon><RefreshRight /></el-icon>{{ t('logManage.rotateNodeLog') }}
              </el-button>
            </div>

            <!-- Stream status -->
            <div class="viewer-status">
              <el-tag :type="logStreamConnected ? 'success' : 'info'" size="small">
                {{ logStreamConnected ? t('logManage.liveStreaming') : t('logManage.disconnected') }}
              </el-tag>
              <el-tag size="small" type="warning">{{ t('logManage.lineCount', { count: logLines.length }) }}</el-tag>
              <el-button size="small" :type="logAutoScroll ? 'primary' : 'default'" @click="logAutoScroll = !logAutoScroll">
                {{ logAutoScroll ? t('logManage.autoScrollOn') : t('logManage.autoScrollOff') }}
              </el-button>
              <el-button size="small" @click="clearLogLines">{{ t('logManage.clearLog') }}</el-button>
            </div>
          </div>

          <!-- Log Display -->
          <div ref="logContainerRef" class="log-container" @scroll="onLogScroll">
            <div v-for="(line, i) in logLines" :key="i" class="log-line" :class="logLineClass(line)">{{ line }}</div>
            <div v-if="logLines.length === 0 && !viewerLoading" class="log-empty">{{ t('logManage.noLogContent') }}</div>
            <div v-if="viewerLoading" class="log-empty">{{ t('common.loading') }}</div>
          </div>
        </el-card>
      </el-tab-pane>
    </el-tabs>

    <!-- ═══════════════ Dialogs ═══════════════ -->

    <!-- Create/Edit Policy Dialog -->
    <el-dialog v-model="dialogVisible" :title="editing ? t('logManage.edit') : t('logManage.create')" width="580">
      <el-form :model="form" label-width="120px">
        <el-form-item :label="t('common.name')" required>
          <el-input v-model="form.name" :placeholder="t('logManage.namePlaceholder')" />
        </el-form-item>
        <el-form-item :label="t('logManage.sourceType')" required>
          <el-select v-model="form.source_type" style="width:100%">
            <el-option label="Xray" value="xray" />
            <el-option label="sing-box" value="singbox" />
            <el-option label="Hysteria2" value="hysteria2" />
            <el-option label="TUIC" value="tuic" />
            <el-option label="Nginx" value="nginx" />
            <el-option label="Caddy" value="caddy" />
            <el-option label="Custom" value="custom" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('logManage.logPath')" required>
          <el-select
            v-model="form.log_path"
            filterable
            allow-create
            default-first-option
            :placeholder="t('logManage.logPathPlaceholder')"
            style="width:100%"
            @focus="loadDiscoveredOptions"
            @change="onLogPathSelect"
          >
            <el-option-group v-for="grp in groupedDiscoveredOptions" :key="grp.label" :label="grp.label">
              <el-option
                v-for="item in grp.items"
                :key="item.log_path"
                :label="item.log_path"
                :value="item.log_path"
              >
                <span style="float:left">{{ item.domain || item.source_type }}</span>
                <span style="float:right;color:#8492a6;font-size:12px">{{ item.log_path }}</span>
              </el-option>
            </el-option-group>
          </el-select>
        </el-form-item>
        <el-form-item :label="t('common.status')">
          <el-switch v-model="form.enabled" :active-text="t('common.enabled')" :inactive-text="t('common.disabled')" />
        </el-form-item>
        <el-form-item :label="t('logManage.rotateCron')">
          <el-select v-model="form.rotate_cron" style="width:100%">
            <el-option :label="t('logManage.cronDaily')" value="0 0 * * *" />
            <el-option :label="t('logManage.cronDaily3am')" value="0 3 * * *" />
            <el-option :label="t('logManage.cronWeekly')" value="0 0 * * 0" />
            <el-option :label="t('logManage.cronEvery12h')" value="0 */12 * * *" />
            <el-option :label="t('logManage.cronEvery6h')" value="0 */6 * * *" />
          </el-select>
        </el-form-item>
        <el-form-item :label="t('logManage.maxSizeMB')">
          <el-input-number v-model="form.max_size_mb" :min="0" :max="102400" />
          <span class="form-hint">{{ t('common.unlimited') }} (0)</span>
        </el-form-item>
        <el-form-item :label="t('logManage.retainDays')">
          <el-input-number v-model="form.retain_days" :min="1" :max="365" />
        </el-form-item>
        <el-form-item :label="t('logManage.compress')">
          <el-switch v-model="form.compress" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="dialogVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="saving" @click="handleSave">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Job History Dialog -->
    <el-dialog v-model="jobsVisible" :title="t('logManage.jobHistory')" width="760">
      <el-table :data="jobs" v-loading="jobsLoading" stripe size="small">
        <el-table-column prop="id" :label="t('common.id')" width="60" />
        <el-table-column prop="status" :label="t('common.status')" width="80">
          <template #default="{row}">
            <el-tag :type="row.status === 'success' ? 'success' : 'danger'" size="small">{{ row.status }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column prop="rotated_files" :label="t('logManage.rotatedFiles')" width="100" />
        <el-table-column prop="cleaned_files" :label="t('logManage.cleanedFiles')" width="100" />
        <el-table-column prop="freed_bytes" :label="t('logManage.freedSpace')" width="100">
          <template #default="{row}">{{ formatBytes(row.freed_bytes) }}</template>
        </el-table-column>
        <el-table-column prop="started_at" :label="t('logManage.startedAt')" min-width="160">
          <template #default="{row}">{{ formatTime(row.started_at) }}</template>
        </el-table-column>
        <el-table-column prop="error_message" :label="t('logManage.error')" min-width="150" show-overflow-tooltip />
      </el-table>
    </el-dialog>

    <!-- Stdout Viewer Dialog (for Writer tab) -->
    <el-dialog
      v-model="stdoutViewerVisible"
      :title="stdoutViewerType.toUpperCase() + ' ' + t('logManage.realtimeLog')"
      width="82%"
      top="5vh"
      :close-on-click-modal="false"
      @close="closeLogViewer"
    >
      <div class="log-toolbar">
        <el-tag :type="logStreamConnected ? 'success' : 'info'" size="small">
          {{ logStreamConnected ? t('logManage.liveStreaming') : t('logManage.disconnected') }}
        </el-tag>
        <el-tag size="small" type="warning">{{ t('logManage.lineCount', { count: logLines.length }) }}</el-tag>
        <div style="flex:1"></div>
        <el-button size="small" :type="logAutoScroll ? 'primary' : 'default'" @click="logAutoScroll = !logAutoScroll">
          {{ logAutoScroll ? t('logManage.autoScrollOn') : t('logManage.autoScrollOff') }}
        </el-button>
        <el-button size="small" @click="clearLogLines">{{ t('logManage.clearLog') }}</el-button>
      </div>
      <div ref="logContainerRef" class="log-container" @scroll="onLogScroll">
        <div v-for="(line, i) in logLines" :key="i" class="log-line" :class="logLineClass(line)">{{ line }}</div>
        <div v-if="logLines.length === 0 && !viewerLoading" class="log-empty">{{ t('logManage.noLogContent') }}</div>
      </div>
    </el-dialog>

    <!-- Writer Config Dialog -->
    <el-dialog v-model="writerConfigVisible" :title="writerConfigType.toUpperCase() + ' ' + t('logManage.writerConfig')" width="550">
      <el-form label-width="120px" size="small">
        <el-form-item :label="t('logManage.managed')">
          <el-switch
            :model-value="getOutputManaged(writerConfigType)"
            @change="(val) => onWriterManagedToggle(writerConfigType, val)"
            :disabled="!getOutputManagedEditable(writerConfigType)"
          />
          <span class="form-hint">{{ t('logManage.writerEnabledHint') }}</span>
        </el-form-item>
        <el-divider content-position="left">{{ t('logManage.fileSettings') }}</el-divider>
        <el-form-item :label="t('logManage.filePath')">
          <el-input v-model="writerForm.file_path" :placeholder="`logs/${writerConfigType}-stdout.log`" />
        </el-form-item>
        <el-form-item :label="t('logManage.maxSizeMB')">
          <el-input-number v-model="writerForm.max_size_mb" :min="1" :max="10240" />
          <span class="form-hint">MB</span>
        </el-form-item>
        <el-divider content-position="left">{{ t('logManage.retention') }}</el-divider>
        <el-form-item :label="t('logManage.retainDays')">
          <el-input-number v-model="writerForm.retain_days" :min="1" :max="365" />
        </el-form-item>
        <el-form-item :label="t('logManage.compress')">
          <el-switch v-model="writerForm.compress" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="writerConfigVisible = false">{{ t('common.cancel') }}</el-button>
        <el-button type="primary" :loading="writerConfigSaving" @click="handleSaveWriterConfig">{{ t('common.save') }}</el-button>
      </template>
    </el-dialog>

    <!-- Writer Files Dialog -->
    <el-dialog v-model="writerFilesVisible" :title="writerConfigType.toUpperCase() + ' ' + t('logManage.writerFiles')" width="660">
      <el-table :data="writerFiles" stripe size="small">
        <el-table-column prop="name" :label="t('common.name')" min-width="200" show-overflow-tooltip />
        <el-table-column prop="size" :label="t('logManage.maxSizeMB')" width="120">
          <template #default="{row}">{{ formatBytes(row.size) }}</template>
        </el-table-column>
        <el-table-column prop="modified" :label="t('logManage.startedAt')" width="180">
          <template #default="{row}">{{ formatTime(row.modified) }}</template>
        </el-table-column>
      </el-table>
      <div v-if="writerFiles.length === 0" class="log-empty">{{ t('logManage.writerNoFiles') }}</div>
    </el-dialog>

    <!-- Log Viewer Dialog (file-based, from policy table) -->
    <el-dialog
      v-model="logViewerVisible"
      :title="t('logManage.viewLog') + ' - ' + logViewerPath"
      width="82%"
      top="5vh"
      :close-on-click-modal="false"
      @close="closeLogViewer"
    >
      <div class="log-toolbar">
        <el-tag :type="logStreamConnected ? 'success' : 'info'" size="small">
          {{ logStreamConnected ? t('logManage.liveStreaming') : t('logManage.disconnected') }}
        </el-tag>
        <el-tag size="small" type="warning">{{ t('logManage.lineCount', { count: logLines.length }) }}</el-tag>
        <div style="flex:1"></div>
        <el-button size="small" :type="logAutoScroll ? 'primary' : 'default'" @click="logAutoScroll = !logAutoScroll">
          {{ logAutoScroll ? t('logManage.autoScrollOn') : t('logManage.autoScrollOff') }}
        </el-button>
        <el-button size="small" @click="clearLogLines">{{ t('logManage.clearLog') }}</el-button>
      </div>
      <div ref="logContainerRef" class="log-container" @scroll="onLogScroll">
        <div v-for="(line, i) in logLines" :key="i" class="log-line" :class="logLineClass(line)">{{ line }}</div>
        <div v-if="logLines.length === 0 && !viewerLoading" class="log-empty">{{ t('logManage.noLogContent') }}</div>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onUnmounted, nextTick, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useI18n } from 'vue-i18n'
import { useRoute, useRouter } from 'vue-router'
import {
  Plus, Refresh, Search, RefreshRight, View, CircleCheck, Setting,
  Document, Timer, DataLine, Monitor, FolderOpened,
} from '@element-plus/icons-vue'
import * as logApi from '../api/logManage'
import { listNodeServers } from '../api/nodeServer'

const { t } = useI18n()

// ──────────── Active Tab (persisted via URL query) ────────────
const route = useRoute()
const router = useRouter()
const activeTab = ref(typeof route.query.tab === 'string' ? route.query.tab : 'overview')

// ──────────── Global Node Selector (persisted via URL query) ────────────
const selectedServerId = ref(
  route.query.node !== undefined && route.query.node !== ''
    ? Number(route.query.node)
    : 1
)

// Keep URL query (?tab= / ?node=) in sync so a refresh restores the current view
watch([activeTab, selectedServerId], ([tab, node]) => {
  const q = { ...route.query }
  if (tab && tab !== 'overview') q.tab = tab
  else delete q.tab
  if (node && node !== 1) q.node = String(node)
  else delete q.node
  router.replace({ query: q }).catch(() => {})
})
const selectedServerName = computed(() => {
  if (selectedServerId.value <= 1) return t('logManage.localNode')
  const s = nodeServers.value.find(s => s.id === selectedServerId.value)
  return s ? s.name : t('logManage.localNode')
})

function onSelectedServerChange() {
  // Reset output lists to defaults immediately to prevent stale data
  // from the previous node being saved to the new node
  coreOutputList.value = makeDefaultCoreOutputList()
  proxyOutputList.value = makeDefaultProxyOutputList()
  // Re-fetch all data for the selected node
  fetchStatus()
  fetchData()
  fetchCoreLogOutput()
  fetchProxyLogOutput()
  fetchWriterStatuses()
}

// ──────────── Status ────────────
const status = ref({ total_policies: 0, enabled_count: 0, scheduled_count: 0, total_jobs: 0 })

// ──────────── Policies ────────────
const policies = ref([])
const loading = ref(false)
const discoveredLogs = ref([])
const discovering = ref(false)
const syncing = ref(false)
const rotatingId = ref(null)
const discoveredOptions = ref([])
const addingPath = ref(null)

// ──────────── Dialog state ────────────
const dialogVisible = ref(false)
const editing = ref(null)
const saving = ref(false)
const form = ref({
  name: '', source_type: 'nginx', log_path: '', enabled: true,
  rotate_cron: '0 0 * * *', retain_days: 30, compress: false, max_size_mb: 0,
})

const jobsVisible = ref(false)
const jobsLoading = ref(false)
const jobs = ref([])

// ──────────── Output Config ────────────
const CORE_TYPES = [
  { type: 'xray', label: 'Xray' },
  { type: 'singbox', label: 'sing-box' },
  { type: 'hysteria2', label: 'Hysteria 2' },
  { type: 'tuic', label: 'TUIC' },
]
const PROXY_TYPES = [
  { type: 'nginx', label: 'Nginx' },
  { type: 'caddy', label: 'Caddy' },
]
function makeDefaultCoreOutputList() {
  return CORE_TYPES.map(c => ({ ...c, cfg: { mode: 'file', managed: false, capture_path: '' } }))
}
function makeDefaultProxyOutputList() {
  return PROXY_TYPES.map(p => ({ ...p, cfg: { mode: 'file', managed: false, capture_path: '' } }))
}
const coreOutputList = ref(makeDefaultCoreOutputList())
const proxyOutputList = ref(makeDefaultProxyOutputList())
const savingCoreOutput = ref(false)
const savingProxyOutput = ref(false)
const fetchingOutput = ref(false)
let coreOutputFetchToken = 0
let proxyOutputFetchToken = 0

// ──────────── Writer ────────────
const writerStatuses = ref([])
const writerConfigVisible = ref(false)
const writerConfigType = ref('')
const writerConfigSaving = ref(false)
const writerForm = ref({ file_path: '', retain_days: 30, compress: false, max_size_mb: 100 })
const writerFilesVisible = ref(false)
const writerFiles = ref([])

// ──────────── Log Viewer ────────────
const nodeServers = ref([])
const viewerNodeId = ref(0)
const viewerLogType = ref('')
const viewerLogTypes = ref([])
const viewerLogPath = ref('')
const viewerLogFiles = ref([])
const viewerLines = ref(500)
const viewerLoading = ref(false)
const viewerRotating = ref(false)

// ──────────── Log Display (shared) ────────────
const logLines = ref([])
const logAutoScroll = ref(true)
const logStreamConnected = ref(false)
const logContainerRef = ref(null)
let eventSource = null

// ──────────── Dialog viewers ────────────
const logViewerVisible = ref(false)
const logViewerPath = ref('')
const stdoutViewerVisible = ref(false)
const stdoutViewerType = ref('')

// ═══════════════ Computed ═══════════════

const groupedDiscoveredOptions = computed(() => {
  const groups = {}
  for (const d of discoveredOptions.value) {
    const key = d.source_type || 'custom'
    if (!groups[key]) groups[key] = { label: key, items: [] }
    groups[key].items.push(d)
  }
  return Object.values(groups)
})

// ═══════════════ Helpers ═══════════════

function isCoreType(type) {
  return ['xray', 'singbox', 'hysteria2', 'tuic'].includes(type)
}

function modeLabel(mode) {
  if (mode === 'file') return t('logManage.modeFile')
  if (mode === 'stdout') return t('logManage.modeStdout')
  return t('logManage.modeOff')
}

function modeTagType(mode) {
  if (mode === 'file') return 'success'
  if (mode === 'stdout') return 'warning'
  return 'info'
}

function sourceTagType(source) {
  if (['xray', 'singbox', 'hysteria2', 'tuic'].includes(source)) return 'info'
  if (source === 'nginx') return 'success'
  if (source === 'caddy') return 'primary'
  return ''
}

function logLineClass(line) {
  if (/error|fatal|panic/i.test(line)) return 'log-error'
  if (/warn/i.test(line)) return 'log-warn'
  return ''
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  const units = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let val = bytes
  while (val >= 1024 && i < units.length - 1) { val /= 1024; i++ }
  return `${val.toFixed(1)} ${units[i]}`
}

function formatTime(ts) {
  if (!ts) return ''
  return new Date(ts).toLocaleString()
}

// ═══════════════ Data Fetching ═══════════════

async function fetchStatus() {
  try {
    const r = await logApi.getStatus(selectedServerId.value)
    status.value = r || status.value
  } catch { /* noop */ }
}

async function fetchData() {
  loading.value = true
  try {
    const r = await logApi.listPolicies(selectedServerId.value)
    policies.value = Array.isArray(r) ? r : []
  } finally {
    loading.value = false
  }
}

async function fetchNodeServers() {
  try {
    const r = await listNodeServers()
    nodeServers.value = (r?.items || r || []).filter(s => s.id !== 1)
  } catch { /* noop */ }
}

async function fetchCoreLogOutput() {
  const token = ++coreOutputFetchToken
  fetchingOutput.value = true
  try {
    const r = await logApi.getCoreLogOutput(selectedServerId.value)
    // Discard stale response if user has already switched to another node
    if (token !== coreOutputFetchToken) return
    if (r) {
      for (const item of coreOutputList.value) {
        if (r[item.type]) item.cfg = { ...item.cfg, ...r[item.type] }
      }
    }
  } catch { /* noop */ } finally {
    if (token === coreOutputFetchToken) fetchingOutput.value = false
  }
}

async function fetchProxyLogOutput() {
  const token = ++proxyOutputFetchToken
  fetchingOutput.value = true
  try {
    const r = await logApi.getProxyLogOutput(selectedServerId.value)
    // Discard stale response if user has already switched to another node
    if (token !== proxyOutputFetchToken) return
    if (r) {
      for (const item of proxyOutputList.value) {
        if (r[item.type]) item.cfg = { ...item.cfg, ...r[item.type] }
      }
    }
  } catch { /* noop */ } finally {
    if (token === proxyOutputFetchToken) fetchingOutput.value = false
  }
}

async function fetchWriterStatuses() {
  try {
    const r = await logApi.getWriters(selectedServerId.value)
    writerStatuses.value = Array.isArray(r) ? r : []
  } catch { /* noop */ }
}

// ═══════════════ Output Config Save ═══════════════

async function handleSaveCoreOutput() {
  if (fetchingOutput.value) {
    ElMessage.warning(t('logManage.dataLoading') || 'Data is still loading, please wait...')
    return
  }
  savingCoreOutput.value = true
  try {
    const data = {}
    for (const item of coreOutputList.value) {
      data[item.type] = item.cfg
    }
    await logApi.saveCoreLogOutput(data, selectedServerId.value)

    ElMessage.success(t('logManage.outputConfigSaved'))
    await fetchWriterStatuses()
  } finally {
    savingCoreOutput.value = false
  }
}

async function handleSaveProxyOutput() {
  if (fetchingOutput.value) {
    ElMessage.warning(t('logManage.dataLoading') || 'Data is still loading, please wait...')
    return
  }
  savingProxyOutput.value = true
  try {
    const data = {}
    for (const item of proxyOutputList.value) {
      data[item.type] = item.cfg
    }
    await logApi.saveProxyLogOutput(data, selectedServerId.value)

    ElMessage.success(t('logManage.outputConfigSaved'))
    await fetchWriterStatuses()
  } finally {
    savingProxyOutput.value = false
  }
}

// ═══════════════ Policy CRUD ═══════════════

function showCreate() {
  editing.value = null
  form.value = {
    name: '', source_type: 'nginx', log_path: '', enabled: true,
    rotate_cron: '0 0 * * *', retain_days: 30, compress: false, max_size_mb: 0,
  }
  dialogVisible.value = true
  loadDiscoveredOptions()
}

function showEdit(row) {
  editing.value = row
  form.value = {
    name: row.name,
    source_type: row.source_type,
    log_path: row.log_path,
    enabled: row.enabled,
    rotate_cron: row.rotate_cron,
    retain_days: row.retain_days,
    compress: row.compress,
    max_size_mb: row.max_size ? Math.round(row.max_size / 1024 / 1024) : 0,
  }
  dialogVisible.value = true
  loadDiscoveredOptions()
}

async function loadDiscoveredOptions() {
  try {
    const r = await logApi.discoverLogs(selectedServerId.value)
    discoveredOptions.value = Array.isArray(r) ? r : []
  } catch {
    discoveredOptions.value = []
  }
}

function onLogPathSelect(val) {
  const found = discoveredOptions.value.find(d => d.log_path === val)
  if (found) {
    form.value.source_type = found.source_type
    if (!form.value.name) {
      form.value.name = found.domain || found.log_path.split('/').pop()
    }
  }
}

async function handleSave() {
  if (!form.value.name || !form.value.log_path) {
    ElMessage.warning(t('logManage.fillRequired'))
    return
  }
  saving.value = true
  try {
    const payload = { ...form.value, server_id: selectedServerId.value }
    // Convert max_size_mb to bytes
    payload.max_size = payload.max_size_mb ? payload.max_size_mb * 1024 * 1024 : 0
    delete payload.max_size_mb
    if (editing.value) {
      await logApi.updatePolicy(editing.value.id, payload)
    } else {
      await logApi.createPolicy(payload)
    }
    ElMessage.success(t('common.success'))
    dialogVisible.value = false
    fetchData()
    fetchStatus()
  } finally {
    saving.value = false
  }
}

async function handleDelete(row) {
  await ElMessageBox.confirm(t('logManage.deleteConfirm', { name: row.name }), t('common.confirm'))
  await logApi.deletePolicy(row.id)
  ElMessage.success(t('common.success'))
  fetchData()
  fetchStatus()
}

async function handleRotate(row) {
  rotatingId.value = row.id
  try {
    await logApi.rotatePolicy(row.id)
    ElMessage.success(t('logManage.rotateSuccess'))
    fetchStatus()
  } finally {
    rotatingId.value = null
  }
}

async function handleDiscover() {
  discovering.value = true
  try {
    const r = await logApi.discoverLogs(selectedServerId.value)
    const logs = Array.isArray(r) ? r : []
    const policyPaths = new Set(policies.value.map(p => p.log_path))
    for (const log of logs) {
      if (policyPaths.has(log.log_path)) log.has_policy = true
    }
    discoveredLogs.value = logs
    ElMessage.success(t('logManage.discoverDone', { count: discoveredLogs.value.length }))
  } finally {
    discovering.value = false
  }
}

async function handleSync() {
  syncing.value = true
  try {
    const r = await logApi.syncPolicies(selectedServerId.value)
    ElMessage.success(t('logManage.syncDone', { count: r?.created || 0 }))
    fetchData()
    fetchStatus()
  } finally {
    syncing.value = false
  }
}

async function handleAddFromDiscover(row) {
  addingPath.value = row.log_path
  try {
    const name = row.domain || row.log_path.split('/').pop().replace('.log', '')
    await logApi.createPolicy({
      name, source_type: row.source_type, log_path: row.log_path,
      enabled: true, rotate_cron: '0 0 * * *', retain_days: 30, compress: false, max_size: 0,
      server_id: selectedServerId.value,
    })
    ElMessage.success(t('logManage.policyCreated', { name }))
    row.has_policy = true
    fetchData()
    fetchStatus()
  } catch (e) {
    if (e?.response?.data?.message?.includes('UNIQUE') || e?.message?.includes('exists')) {
      row.has_policy = true
    }
  } finally {
    addingPath.value = null
  }
}

async function showJobs(row) {
  jobsVisible.value = true
  jobsLoading.value = true
  try {
    const r = await logApi.getPolicyJobs(row.id)
    jobs.value = r?.items || []
  } finally {
    jobsLoading.value = false
  }
}

// ═══════════════ Log Viewer (Tab 4) ═══════════════

async function onViewerNodeChange() {
  viewerLogType.value = ''
  viewerLogTypes.value = []
  viewerLogPath.value = ''
  viewerLogFiles.value = []
  logLines.value = []
  closeLogStream()
  if (viewerNodeId.value === 0) {
    // Local node: show all known types
    viewerLogTypes.value = [
      ...CORE_TYPES.map(c => ({ type: c.type, label: c.label })),
      ...PROXY_TYPES.map(p => ({ type: p.type, label: p.label })),
    ]
  } else {
    // Remote node: fetch available log types
    try {
      const r = await logApi.listNodeLogs(viewerNodeId.value)
      const types = r?.log_types || r?.types || (Array.isArray(r) ? r : [])
      viewerLogTypes.value = types.map(t => {
        if (typeof t === 'string') return { type: t, label: t }
        return t
      })
    } catch {
      viewerLogTypes.value = []
      ElMessage.warning(t('logManage.nodeLogError'))
    }
  }
}

function onViewerLogTypeChange() {
  viewerLogPath.value = ''
  viewerLogFiles.value = []
  logLines.value = []
  closeLogStream()
  loadViewerLogFiles()
  loadViewerLog()
}

async function loadViewerLogFiles() {
  if (!viewerLogType.value) return
  viewerLogFiles.value = []
  try {
    const r = await logApi.discoverLogs(viewerNodeId.value || 1)
    const all = Array.isArray(r) ? r : []
    viewerLogFiles.value = all.filter(d => d.source_type === viewerLogType.value)
  } catch {
    viewerLogFiles.value = []
  }
}

async function loadViewerLog() {
  if (!viewerLogType.value) return
  viewerLoading.value = true
  logLines.value = []
  logAutoScroll.value = true
  closeLogStream()

  try {
    if (viewerNodeId.value === 0) {
      // Local node
      const type = viewerLogType.value
      const logPath = viewerLogPath.value
      let gotData = false

      if (logPath) {
        // Specific file selected: read via node API with log_path
        try {
          const r = await logApi.getNodeLog(1, type, viewerLines.value, logPath)
          if (r?.lines && r.lines.length > 0) {
            const firstLine = r.lines[0] || ''
            if (!firstLine.includes('[日志文件不存在]')) {
              logLines.value = r.lines
              gotData = true
            }
          }
        } catch { /* noop */ }
      } else {
        // No specific file: try policy-based file read first, then stdout
        const policy = policies.value.find(p => p.source_type === type)
        // 1. Try policy-based file read first
        if (policy) {
          try {
            const pr = await logApi.getPolicyLog(policy.id, viewerLines.value)
            if (pr?.lines && pr.lines.length > 0) {
              const firstLine = pr.lines[0] || ''
              if (!firstLine.includes('[日志文件不存在]') && !firstLine.includes('[未运行')) {
                logLines.value = pr.lines
                gotData = true
              }
            }
          } catch { /* noop */ }
        }
        // 2. If no file data, try stdout broker
        if (!gotData) {
          try {
            const r = isCoreType(type)
              ? await logApi.getCoreStdoutLog(type, viewerLines.value)
              : await logApi.getProxyStdoutLog(type, viewerLines.value)
            if (r?.lines && r.lines.length > 0) {
              const realLines = r.lines.filter(l => !l.includes('[未运行'))
              if (realLines.length > 0) {
                logLines.value = realLines
                gotData = true
              }
            }
          } catch { /* noop */ }
        }
      }
      // 3. If still no data, show informative message
      if (!gotData) {
        logLines.value = [t('logManage.noLogContent')]
      }
    } else {
      // Remote node
      const r = await logApi.getNodeLog(viewerNodeId.value, viewerLogType.value, viewerLines.value, viewerLogPath.value)
      logLines.value = r?.lines || [t('logManage.noLogContent')]
    }
    await nextTick()
    scrollToBottom()
    // Start SSE
    startViewerStream()
  } catch {
    logLines.value = [`[Error: ${t('logManage.nodeLogError')}]`]
  } finally {
    viewerLoading.value = false
  }
}

function startViewerStream() {
  closeLogStream()
  const type = viewerLogType.value
  if (!type) return
  const logPath = viewerLogPath.value
  let path
  if (viewerNodeId.value === 0) {
    if (logPath) {
      // Local: stream from specific file via node API
      path = `/log-manage/nodes/1/logs/${type}/stream`
    } else {
      // Local: stream from stdout broker
      if (isCoreType(type)) {
        path = `/log-manage/core/${type}/stdout-stream`
      } else {
        path = `/log-manage/proxy/${type}/stdout-stream`
      }
    }
  } else {
    // Remote node stream
    path = `/log-manage/nodes/${viewerNodeId.value}/logs/${type}/stream`
  }
  const params = {}
  if (logPath) params.log_path = logPath
  const url = logApi.buildSSEUrl(path, params)
  eventSource = logApi.createSSE(url)

  eventSource.addEventListener('log', (e) => {
    logLines.value.push(e.data)
    if (logLines.value.length > 5000) logLines.value = logLines.value.slice(-3000)
    if (logAutoScroll.value) nextTick(scrollToBottom)
  })
  eventSource.onopen = () => { logStreamConnected.value = true }
  eventSource.onerror = () => {
    logStreamConnected.value = false
    setTimeout(() => {
      if (logViewerVisible.value || stdoutViewerVisible.value || activeTab.value === 'viewer') {
        startViewerStream()
      }
    }, 3000)
  }
}

async function handleRotateNodeLog() {
  if (!viewerLogType.value) return
  viewerRotating.value = true
  try {
    if (viewerNodeId.value === 0) {
      // Local: find policy and rotate
      const policy = policies.value.find(p => p.source_type === viewerLogType.value)
      if (policy) {
        await logApi.rotatePolicy(policy.id)
        ElMessage.success(t('logManage.rotateSuccess'))
      }
    } else {
      // Remote
      await logApi.rotateNodeLog(viewerNodeId.value, viewerLogType.value, viewerLogPath.value)
      ElMessage.success(t('logManage.nodeLogRotateSuccess'))
    }
    fetchStatus()
  } catch {
    ElMessage.error(t('logManage.nodeLogRotateFailed'))
  } finally {
    viewerRotating.value = false
  }
}

// ═══════════════ Stdout Viewer (from Writer tab) ═══════════════

function showStdoutViewer(type) {
  stdoutViewerType.value = type
  stdoutViewerVisible.value = true
  loadStdoutLog(type)
}

async function loadStdoutLog(type) {
  viewerLoading.value = true
  logLines.value = []
  logAutoScroll.value = true
  try {
    const r = isCoreType(type)
      ? await logApi.getCoreStdoutLog(type, 500, selectedServerId.value)
      : await logApi.getProxyStdoutLog(type, 500, selectedServerId.value)
    // Filter out placeholder messages
    const lines = (r?.lines || []).filter(l => !l.includes('[未运行') && !l.includes('未产生标准输出'))
    logLines.value = lines.length > 0 ? lines : [t('logManage.noLogContent') || '暂无日志内容']
    await nextTick()
    scrollToBottom()
    startStdoutStream(type)
  } catch {
    logLines.value = ['[Error loading stdout log]']
  } finally {
    viewerLoading.value = false
  }
}

function startStdoutStream(type) {
  closeLogStream()
  const url = isCoreType(type)
    ? logApi.buildCoreStdoutStreamUrl(type, selectedServerId.value)
    : logApi.buildProxyStdoutStreamUrl(type, selectedServerId.value)
  eventSource = logApi.createSSE(url)
  eventSource.addEventListener('log', (e) => {
    logLines.value.push(e.data)
    if (logLines.value.length > 5000) logLines.value = logLines.value.slice(-3000)
    if (logAutoScroll.value) nextTick(scrollToBottom)
  })
  eventSource.onopen = () => { logStreamConnected.value = true }
  eventSource.onerror = () => {
    logStreamConnected.value = false
    setTimeout(() => { if (stdoutViewerVisible.value) startStdoutStream(type) }, 3000)
  }
}

// ═══════════════ Writer Management ═══════════════

function getWriterStatus(type) {
  return writerStatuses.value.find(w => w.proxy_type === type)
}

function showWriterConfig(type) {
  writerConfigType.value = type
  writerConfigVisible.value = true
  const s = getWriterStatus(type)
  writerForm.value = {
    file_path: s?.file_path || `logs/${type}-stdout.log`,
    retain_days: s?.retain_days || 30,
    compress: s?.compress || false,
    max_size_mb: s?.max_size_mb || 100,
  }
}

// ──────────── Managed ↔ Writer linkage ────────────

// Get the managed state from the output config for a given type
function getOutputManaged(type) {
  const list = isCoreType(type) ? coreOutputList.value : proxyOutputList.value
  const item = list.find(i => i.type === type)
  return item ? item.cfg.managed : false
}

// Check if the managed switch is editable (only when mode === 'stdout')
function getOutputManagedEditable(type) {
  const list = isCoreType(type) ? coreOutputList.value : proxyOutputList.value
  const item = list.find(i => i.type === type)
  return item ? item.cfg.mode === 'stdout' : false
}

// Toggle managed state from the Writer Config dialog
async function onWriterManagedToggle(type, val) {
  const list = isCoreType(type) ? coreOutputList.value : proxyOutputList.value
  const item = list.find(i => i.type === type)
  if (!item) return
  item.cfg.managed = val
  // For local node: save the output config which will start/stop the writer via StartWriterManaged
  if (selectedServerId.value <= 1) {
    try {
      if (isCoreType(type)) {
        const data = {}
        for (const it of coreOutputList.value) data[it.type] = it.cfg
        await logApi.saveCoreLogOutput(data, selectedServerId.value)
      } else {
        const data = {}
        for (const it of proxyOutputList.value) data[it.type] = it.cfg
        await logApi.saveProxyLogOutput(data, selectedServerId.value)
      }
      await fetchWriterStatuses()
    } catch { /* noop */ }
  }
}

async function handleSaveWriterConfig() {
  writerConfigSaving.value = true
  try {
    await logApi.updateWriterConfig(writerConfigType.value, writerForm.value, selectedServerId.value)
    ElMessage.success(t('common.success'))
    writerConfigVisible.value = false
    await fetchWriterStatuses()
  } finally {
    writerConfigSaving.value = false
  }
}

async function showWriterFiles(type) {
  writerConfigType.value = type
  writerFilesVisible.value = true
  writerFiles.value = []
  try {
    const r = await logApi.getWriterFiles(type, selectedServerId.value)
    writerFiles.value = Array.isArray(r) ? r : (r?.files || [])
  } catch { /* noop */ }
}

// ═══════════════ Log Viewer (Policy-based dialog) ═══════════════

async function showLogViewer(row) {
  logViewerPath.value = row.log_path
  logLines.value = []
  logAutoScroll.value = true
  logViewerVisible.value = true
  viewerLoading.value = true
  try {
    const r = await logApi.getPolicyLog(row.id, 500)
    logLines.value = r?.lines || []
    await nextTick()
    scrollToBottom()
  } catch (e) {
    logLines.value = [`[Error loading log: ${e.message || e}]`]
  } finally {
    viewerLoading.value = false
  }
  // Start SSE for this policy
  closeLogStream()
  const url = logApi.buildSSEUrl(`/log-manage/policies/${row.id}/log-stream`, { lines: 0 })
  eventSource = logApi.createSSE(url)
  eventSource.addEventListener('log', (e) => {
    const line = e.data
    // Skip placeholder messages from SSE
    if (line && !line.includes('[无可用日志源]')) {
      logLines.value.push(line)
      if (logLines.value.length > 5000) logLines.value = logLines.value.slice(-3000)
      if (logAutoScroll.value) nextTick(scrollToBottom)
    }
  })
  eventSource.onopen = () => { logStreamConnected.value = true }
  eventSource.onerror = () => {
    logStreamConnected.value = false
    setTimeout(() => {
      if (logViewerVisible.value) {
        closeLogStream()
        const url2 = logApi.buildSSEUrl(`/log-manage/policies/${row.id}/log-stream`, { lines: 0 })
        eventSource = logApi.createSSE(url2)
        eventSource.addEventListener('log', (e) => {
          const line = e.data
          if (line && !line.includes('[无可用日志源]')) {
            logLines.value.push(line)
            if (logLines.value.length > 5000) logLines.value = logLines.value.slice(-3000)
            if (logAutoScroll.value) nextTick(scrollToBottom)
          }
        })
        eventSource.onopen = () => { logStreamConnected.value = true }
        eventSource.onerror = () => { logStreamConnected.value = false }
      }
    }, 3000)
  }
}

// ═══════════════ Log Display Utilities ═══════════════

function closeLogStream() {
  if (eventSource) {
    eventSource.close()
    eventSource = null
    logStreamConnected.value = false
  }
}

function closeLogViewer() {
  closeLogStream()
  logLines.value = []
  logViewerVisible.value = false
  stdoutViewerVisible.value = false
}

function clearLogLines() {
  logLines.value = []
}

function scrollToBottom() {
  if (logContainerRef.value) {
    logContainerRef.value.scrollTop = logContainerRef.value.scrollHeight
  }
}

function onLogScroll() {
  if (!logContainerRef.value) return
  const { scrollTop, scrollHeight, clientHeight } = logContainerRef.value
  logAutoScroll.value = scrollHeight - scrollTop - clientHeight < 50
}

// ═══════════════ Lifecycle ═══════════════

// Auto-initialize viewer when tab is first opened
watch(activeTab, (val) => {
  if (val === 'viewer' && viewerLogTypes.value.length === 0) {
    onViewerNodeChange()
  }
})

onMounted(() => {
  fetchData()
  fetchStatus()
  fetchCoreLogOutput()
  fetchProxyLogOutput()
  fetchWriterStatuses()
  fetchNodeServers()
})

onUnmounted(() => {
  closeLogStream()
})
</script>

<style scoped>
.log-manage-page {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.node-selector-bar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 12px;
  padding: 8px 12px;
  background: #f5f7fa;
  border-radius: 6px;
  flex-shrink: 0;
}

.log-tabs {
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

.log-tabs :deep(.el-tabs__content) {
  height: calc(100% - 40px);
  overflow-y: auto;
}

.log-tabs :deep(.el-tab-pane) {
  height: auto;
  min-height: 100%;
}

.node-selector-label {
  font-size: 14px;
  font-weight: 600;
  color: #606266;
  display: flex;
  align-items: center;
  gap: 4px;
}

.log-tabs {
  height: 100%;
}

.stat-card-wrap {
  margin-bottom: 4px;
}

.stat-card {
  display: flex;
  align-items: center;
  gap: 16px;
  padding: 4px 0;
}

.stat-icon {
  width: 56px;
  height: 56px;
  border-radius: 12px;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 28px;
  color: #fff;
  flex-shrink: 0;
}
.stat-icon-blue   { background: linear-gradient(135deg, #409eff, #36cfc9); }
.stat-icon-green  { background: linear-gradient(135deg, #67c23a, #52c41a); }
.stat-icon-primary{ background: linear-gradient(135deg, #722ed1, #9254de); }
.stat-icon-orange { background: linear-gradient(135deg, #e6a23c, #fa8c16); }

.stat-body {
  flex: 1;
}

.stat-value {
  font-size: 28px;
  font-weight: 700;
  line-height: 1.2;
}
.stat-label {
  font-size: 13px;
  color: #909399;
  margin-top: 4px;
}
.text-success { color: #67c23a; }
.text-primary { color: #409eff; }
.text-warning { color: #e6a23c; }

.card-header-flex {
  display: flex;
  justify-content: space-between;
  align-items: center;
}

.header-hint {
  font-size: 12px;
  color: #909399;
  margin-left: 8px;
  font-weight: 400;
}

.tab-header {
  display: flex;
  justify-content: flex-end;
  margin-bottom: 16px;
}

.mode-list {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.mode-item {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 4px 0;
}

.mode-label {
  font-weight: 500;
}

.form-hint {
  margin-left: 8px;
  font-size: 12px;
  color: #909399;
}

.viewer-toolbar {
  display: flex;
  justify-content: space-between;
  align-items: flex-start;
  flex-wrap: wrap;
  gap: 12px;
  margin-bottom: 16px;
}

.viewer-selector {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
}

.viewer-status {
  display: flex;
  align-items: center;
  gap: 8px;
}

.log-toolbar {
  display: flex;
  align-items: center;
  gap: 8px;
  margin-bottom: 10px;
  padding: 6px 0;
}

.log-container {
  background: #1e1e1e;
  color: #d4d4d4;
  font-family: 'Consolas', 'Monaco', 'Courier New', monospace;
  font-size: 13px;
  line-height: 1.5;
  height: 60vh;
  overflow-y: auto;
  padding: 12px;
  border-radius: 6px;
  white-space: pre-wrap;
  word-break: break-all;
}

.log-line {
  padding: 0 4px;
  min-height: 20px;
}
.log-line:hover {
  background: rgba(255,255,255,0.05);
}
.log-error {
  color: #f56c6c;
}
.log-warn {
  color: #e6a23c;
}

.log-empty {
  color: #666;
  text-align: center;
  padding: 40px 0;
}
</style>

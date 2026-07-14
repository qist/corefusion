<template>
  <div class="portal-management">
    <!-- Header -->
    <el-card class="header-card">
      <div class="site-header">
        <div class="site-info">
          <h2>订阅用户门户站点</h2>
          <p>配置订阅用户自助门户站点的外观和功能，访问路径: <el-tag size="small" type="info">/p</el-tag></p>
        </div>
        <div class="site-actions">
          <el-button @click="handlePreview" :icon="View">预览</el-button>
          <el-button type="primary" @click="handleSave" :loading="saving" :icon="Check">保存</el-button>
        </div>
      </div>
    </el-card>

    <div v-loading="loading" class="portal-body">
      <el-tabs v-model="activeTab" type="border-card">
        <!-- Basic Settings -->
        <el-tab-pane label="基本设置" name="basic">
          <el-form :model="form" label-width="140px" style="max-width: 700px; padding: 20px 0">
            <el-form-item label="站点名称">
              <el-input v-model="form.name" placeholder="我的订阅站" />
            </el-form-item>
            <el-form-item label="站点描述">
              <el-input v-model="form.description" type="textarea" :rows="2" placeholder="站点描述" />
            </el-form-item>
            <el-form-item label="开放注册">
              <el-switch v-model="registrationEnabled" />
              <div class="form-hint">开启后用户可通过 Portal 页面自主注册账号</div>
            </el-form-item>
            <el-form-item label="门户访问路径">
              <el-input value="/p" disabled />
              <div class="form-hint">订阅用户门户页面的访问路径</div>
            </el-form-item>
            <el-form-item label="门户端口">
              <el-input v-model="portalPort" placeholder="留空使用面板端口" />
              <div class="form-hint">设置独立端口后，门户可在该端口根路径访问</div>
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- Appearance -->
        <el-tab-pane label="外观配置" name="appearance">
          <el-form :model="config" label-width="140px" style="max-width: 700px; padding: 20px 0">
            <el-form-item label="站点标题">
              <el-input v-model="config.site_name" placeholder="CoreFusion" />
            </el-form-item>
            <el-form-item label="Logo URL">
              <el-input v-model="config.logo_url" placeholder="https://example.com/logo.png" />
            </el-form-item>
            <el-form-item label="主色调">
              <div class="color-row">
                <el-color-picker v-model="config.primary_color" />
                <el-input v-model="config.primary_color" style="width: 120px" />
              </div>
            </el-form-item>
            <el-form-item label="背景色">
              <div class="color-row">
                <el-color-picker v-model="config.background_color" />
                <el-input v-model="config.background_color" style="width: 120px" />
              </div>
            </el-form-item>
            <el-form-item label="卡片背景色">
              <div class="color-row">
                <el-color-picker v-model="config.card_background_color" />
                <el-input v-model="config.card_background_color" style="width: 120px" />
              </div>
            </el-form-item>
            <el-form-item label="文字颜色">
              <div class="color-row">
                <el-color-picker v-model="config.text_color" />
                <el-input v-model="config.text_color" style="width: 120px" />
              </div>
            </el-form-item>
            <el-form-item label="强调色">
              <div class="color-row">
                <el-color-picker v-model="config.accent_color" />
                <el-input v-model="config.accent_color" style="width: 120px" />
              </div>
            </el-form-item>
            <el-form-item label="字体">
              <el-input v-model="config.font_family" placeholder="-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif" />
            </el-form-item>
          </el-form>
        </el-tab-pane>

        <!-- API Reference -->
        <el-tab-pane label="API 参考" name="api">
          <div style="padding: 20px 0; max-width: 800px">
            <el-alert type="info" :closable="false" show-icon style="margin-bottom: 20px">
              <template #title>Portal 页面通过以下 API 与后端交互</template>
            </el-alert>

            <h4 class="api-section-title">公开接口（无需认证）</h4>
            <el-table :data="publicApis" border size="small" style="margin-bottom: 24px">
              <el-table-column prop="method" label="方法" width="80" />
              <el-table-column prop="path" label="路径" />
              <el-table-column prop="desc" label="说明" />
            </el-table>

            <h4 class="api-section-title">认证接口（需 Bearer Token）</h4>
            <el-table :data="authApis" border size="small" style="margin-bottom: 24px">
              <el-table-column prop="method" label="方法" width="80" />
              <el-table-column prop="path" label="路径" />
              <el-table-column prop="desc" label="说明" />
            </el-table>

            <h4 class="api-section-title">日志包接口（需 Bearer Token）</h4>
            <el-table :data="logApis" border size="small">
              <el-table-column prop="method" label="方法" width="80" />
              <el-table-column prop="path" label="路径" />
              <el-table-column prop="desc" label="说明" />
            </el-table>
          </div>
        </el-tab-pane>

        <!-- HTML -->
        <el-tab-pane label="HTML 模板" name="html">
          <div class="editor-hint">
            <el-alert type="info" :closable="false" show-icon>
              HTML 模板支持变量替换: <code v-pre>{{.SiteName}}</code> <code v-pre>{{.LogoURL}}</code> <code v-pre>{{.RegistrationEnabled}}</code> <code v-pre>{{registrationEnabledText}}</code>
            </el-alert>
          </div>
          <el-input v-model="form.html_content" type="textarea" :rows="24" style="font-family: 'SF Mono', 'Fira Code', Consolas, monospace; font-size: 12px" />
        </el-tab-pane>

        <!-- CSS -->
        <el-tab-pane label="CSS 样式" name="css">
          <div class="editor-hint">
            <el-alert type="info" :closable="false" show-icon>
              CSS 支持变量替换: <code v-pre>{{.PrimaryColor}}</code> <code v-pre>{{.BackgroundColor}}</code> <code v-pre>{{.CardBackgroundColor}}</code> <code v-pre>{{.TextColor}}</code> <code v-pre>{{.AccentColor}}</code> <code v-pre>{{.FontFamily}}</code>
            </el-alert>
          </div>
          <el-input v-model="form.css_content" type="textarea" :rows="24" style="font-family: 'SF Mono', 'Fira Code', Consolas, monospace; font-size: 12px" />
        </el-tab-pane>

        <!-- JavaScript -->
        <el-tab-pane label="JavaScript" name="js">
          <div class="editor-hint">
            <el-alert type="info" :closable="false" show-icon>
              JS 中可通过 <code>window.PORTAL_CONFIG</code> 访问配置对象 (site_name, logo_url, primary_color, registration_enabled 等)
            </el-alert>
          </div>
          <el-input v-model="form.js_content" type="textarea" :rows="24" style="font-family: 'SF Mono', 'Fira Code', Consolas, monospace; font-size: 12px" />
        </el-tab-pane>

        <!-- Advanced -->
        <el-tab-pane label="高级" name="advanced">
          <div style="padding: 20px 0; max-width: 600px">
            <el-alert type="warning" :closable="false" show-icon title="重置为默认模板" style="margin-bottom: 16px">
              此操作将覆盖当前所有自定义配置，恢复为系统内置默认模板。此操作不可撤销。
            </el-alert>
            <el-button type="danger" @click="handleReset">重置为默认模板</el-button>
          </div>
        </el-tab-pane>
      </el-tabs>
    </div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { View, Check } from '@element-plus/icons-vue'
import { getActivePortal, updateActivePortal, resetPortal } from '../api/portal'

const loading = ref(false)
const saving = ref(false)
const activeTab = ref('basic')
const registrationEnabled = ref(false)
const portalPort = ref('')

const form = reactive({
  name: '',
  description: '',
  html_content: '',
  css_content: '',
  js_content: '',
})

const config = reactive({
  site_name: 'CoreFusion',
  logo_url: '',
  primary_color: '#667eea',
  background_color: '#f0f2f5',
  card_background_color: '#ffffff',
  text_color: '#303133',
  accent_color: '#67c23a',
  font_family: "-apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif",
})

// API reference data
const publicApis = [
  { method: 'GET', path: '/api/v1/public/settings', desc: '获取公开设置（注册开关等）' },
  { method: 'GET', path: '/api/v1/public/subscription-endpoints', desc: '获取订阅端点列表' },
  { method: 'POST', path: '/api/v1/client-auth/register', desc: '注册新用户' },
  { method: 'POST', path: '/api/v1/client-auth/login', desc: '登录（返回 access_token + refresh_token）' },
  { method: 'POST', path: '/api/v1/client-auth/refresh', desc: '使用 refresh_token 刷新 access_token' },
]

const authApis = [
  { method: 'GET', path: '/api/v1/client-auth/me', desc: '获取当前用户信息（含流量、订阅状态）' },
  { method: 'POST', path: '/api/v1/client-auth/rotate-token', desc: '轮换订阅 Token' },
  { method: 'PUT', path: '/api/v1/client-auth/settings', desc: '更新安全设置（IP 白名单、设备限制）' },
  { method: 'POST', path: '/api/v1/client-auth/change-password', desc: '修改密码' },
]

const logApis = [
  { method: 'GET', path: '/api/v1/client-auth/logs/types', desc: '获取可用日志类型和限制' },
  { method: 'POST', path: '/api/v1/client-auth/logs/package', desc: '创建日志打包任务' },
  { method: 'GET', path: '/api/v1/client-auth/logs/packages', desc: '获取日志包列表' },
  { method: 'GET', path: '/api/v1/client-auth/logs/packages/:id', desc: '获取日志包详情' },
  { method: 'GET', path: '/api/v1/client-auth/logs/packages/:id/download', desc: '下载日志包' },
]

const applyData = (d) => {
  if (!d) return
  form.name = d.name || ''
  form.description = d.description || ''
  form.html_content = d.html_content || ''
  form.css_content = d.css_content || ''
  form.js_content = d.js_content || ''
  registrationEnabled.value = d.registration_enabled || false
  portalPort.value = d.portal_port || ''
  if (d.config) {
    try {
      const cfg = JSON.parse(d.config)
      Object.assign(config, cfg)
    } catch {}
  }
}

const fetchConfig = async () => {
  loading.value = true
  try {
    const res = await getActivePortal()
    applyData(res.data?.data)
  } catch (e) {
    ElMessage.error('加载门户配置失败')
  } finally {
    loading.value = false
  }
}

const handleSave = async () => {
  saving.value = true
  try {
    const data = {
      name: form.name,
      description: form.description,
      html_content: form.html_content,
      css_content: form.css_content,
      js_content: form.js_content,
      config: JSON.stringify(config),
      registration_enabled: registrationEnabled.value,
      portal_port: portalPort.value,
    }
    await updateActivePortal(data)
    ElMessage.success('门户配置已保存')
  } catch (e) {
    ElMessage.error(e.response?.data?.message || '保存失败')
  } finally {
    saving.value = false
  }
}

const handlePreview = () => {
  const url = window.location.origin + '/p'
  window.open(url, '_blank')
}

const handleReset = async () => {
  try {
    await ElMessageBox.confirm(
      '此操作将覆盖当前所有自定义配置，恢复为系统内置默认模板。此操作不可撤销！',
      '确认重置',
      { confirmButtonText: '确认重置', cancelButtonText: '取消', type: 'warning' }
    )
    const res = await resetPortal()
    applyData(res.data?.data)
    ElMessage.success('已重置为默认模板')
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('重置失败')
    }
  }
}

onMounted(() => {
  fetchConfig()
})
</script>

<style scoped>
.portal-management {
  padding: 20px;
}
.header-card {
  margin-bottom: 16px;
}
.site-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.site-header h2 {
  margin: 0 0 4px 0;
  font-size: 18px;
}
.site-header p {
  margin: 0;
  font-size: 13px;
  color: #909399;
}
.site-actions {
  display: flex;
  gap: 8px;
}
.portal-body {
  min-height: 400px;
}
.color-row {
  display: flex;
  align-items: center;
  gap: 8px;
}
.form-hint {
  font-size: 12px;
  color: #909399;
  margin-top: 4px;
}
.editor-hint {
  margin-bottom: 12px;
}
.editor-hint code {
  background: #f5f7fa;
  padding: 2px 6px;
  border-radius: 4px;
  font-size: 12px;
  color: #667eea;
}
.api-section-title {
  font-size: 14px;
  font-weight: 600;
  color: #303133;
  margin-bottom: 12px;
}
</style>

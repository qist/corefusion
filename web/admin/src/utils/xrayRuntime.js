import { ElMessage } from 'element-plus'
import i18n from '../locales'

let lastWarningAt = 0
let lastWarningText = ''

function warnWithCooldown(message) {
  const now = Date.now()
  if (message === lastWarningText && now-lastWarningAt < 1500) {
    return
  }
  lastWarningText = message
  lastWarningAt = now
  ElMessage.warning(message)
}

export function isXrayNotRunningError(error) {
  const code = error?.response?.data?.code || ''
  const message = error?.response?.data?.message || error?.message || ''
  if (code === 'CONNECT_FAILED') {
    return true
  }
  return /failed to connect to xray|context deadline exceeded|connection refused/i.test(message)
}

export function notifyXrayRuntimeApplyFailure(error, persistedMessage) {
  const t = i18n.global.t
  if (isXrayNotRunningError(error)) {
    warnWithCooldown(t('xrayRuntime.persistedButNotRunning', { persistedMessage }))
    return
  }

  const message = error?.response?.data?.message || error?.message || t('xrayRuntime.applyFailed')
  warnWithCooldown(t('xrayRuntime.persistedButApplyFailed', { persistedMessage, message }))
}

export async function ensureXrayRunning(client, serverId, persistedMessage) {
  const t = i18n.global.t
  try {
    const services = await client.get(`/node-servers/${serverId}/services/process-status`, { suppressErrorMessage: true })
    const xray = (services || []).find(s => s.core_type === 'xray')
    if (xray?.state === 'running') {
      return true
    }
  } catch {}

  warnWithCooldown(t('xrayRuntime.persistedButNotRunning', { persistedMessage }))
  return false
}

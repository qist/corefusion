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

// xray 的业务错误说明控制面是通的（消息已送达、被 xray 拒绝），既不是「进程未启动」
// 也不是「控制接口不可用」。例如 existing tag found（tag 冲突）、配置非法等。
const XRAY_BIZ_ERROR_RE = /existing tag|existing handler|not enough information for making a decision|failed to build|failed to parse|invalid (?:config|params)|unsupported|already exists/i

export function isXrayNotRunningError(error) {
  const code = error?.response?.data?.code || ''
  const message = error?.response?.data?.message || error?.message || ''
  if (XRAY_BIZ_ERROR_RE.test(message)) {
    return false
  }
  if (code === 'CONNECT_FAILED' || code === 'REMOTE_API_FAILED') {
    return true
  }
  return /failed to connect to xray|connect to xray|context deadline exceeded|connection refused|dial xray/i.test(message)
}

// cleanXrayErrorMessage 去掉 gRPC 包装噪音，只留下 xray 给出的原因。
function cleanXrayErrorMessage(message) {
  return String(message || '')
    .replace(/^remote error:\s*/i, '')
    .replace(/^rpc error:\s*code = \w+\s*desc =\s*/i, '')
    .replace(/^rpc error:\s*/i, '')
    .trim()
}

// probeXrayProcessState 只用于「热更失败之后」的归因，不再作为保存前的门禁。
//
// 以前保存前先查 process-status，非 running 就提示「Xray 未启动」并放弃热更。
// 该接口对远程节点取自 Agent 心跳缓存：核心刚重启、Agent 刚重连或缓存尚未上报时
// 都会返回 stopped，于是出现「进程明明在跑，却提示未启动」的误报。现在改成：
// 先尝试热更，失败后再看进程状态，只有在**确实没在跑**时才提示未启动。
//
// 返回 'running' / 'stopped' / ''（查不到，视为未知）。
export async function probeXrayProcessState(client, serverId) {
  try {
    const services = await client.get(`/node-servers/${serverId}/services/process-status`, { suppressErrorMessage: true })
    const xray = (services || []).find(s => s.core_type === 'xray')
    return xray?.state || ''
  } catch {
    return ''
  }
}

// notifyXrayRuntimeApplyFailure 把「已落库但热更失败」的原因说清楚。
//
// 关键区分：进程在跑 + 控制接口连不上 ≠ 进程没启动。前者是 xray 的 api 端口
// （默认 127.0.0.1:10085）不可达 / api 段未生效，重启核心或修 api 配置即可；
// 后者才是真的没运行。旧文案一律写成「Xray 未启动」，把用户引向错误的方向。
export async function notifyXrayRuntimeApplyFailure(error, persistedMessage, client, serverId) {
  const t = i18n.global.t
  const rawMessage = error?.response?.data?.message || error?.message || ''
  const message = cleanXrayErrorMessage(rawMessage)

  if (isXrayNotRunningError(error)) {
    const state = client && serverId != null ? await probeXrayProcessState(client, serverId) : ''
    if (state === 'running') {
      warnWithCooldown(t('xrayRuntime.persistedButApiUnavailable', { persistedMessage, message }))
      return
    }
    warnWithCooldown(t('xrayRuntime.persistedButNotRunning', { persistedMessage }))
    return
  }

  warnWithCooldown(t('xrayRuntime.persistedButApplyFailed', {
    persistedMessage,
    message: message || t('xrayRuntime.applyFailed'),
  }))
}

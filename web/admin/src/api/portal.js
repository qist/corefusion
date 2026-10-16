import client from './client'

// Portal site management (active config — direct edit)
export function getActivePortal() {
  return client.get('/portal/active')
}

export function updateActivePortal(data) {
  return client.put('/portal/active', data)
}

export function resetPortal() {
  return client.post('/portal/reset')
}

// 内置语言包（供「文案」tab 缺失 key 检测 / 一键补齐）
export function getBuiltinPortalLang() {
  return client.get('/portal/lang/builtin')
}

// Render cache management
export function getPortalCacheStatus() {
  return client.get('/portal/cache')
}

export function clearPortalCache() {
  return client.delete('/portal/cache')
}

export function rebuildPortalCache() {
  return client.post('/portal/cache/rebuild')
}

export function setPortalDevMode(devMode) {
  return client.put('/portal/cache/dev-mode', { dev_mode: devMode })
}

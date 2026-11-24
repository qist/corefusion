import { onMounted, onUnmounted } from 'vue'

// 通用「静默自动刷新」：定时拉取 + 页面重新可见时立即拉取。
//
// 为什么需要：很多数据是后台任务/定时探测写入的（链路探测结果、池条目健康、
// 异步测速），只靠组件挂载时拉一次的话，页面开着就一直是旧数据，必须 F5。
//
// 安全边界（避免覆盖用户正在编辑的内容）：
//   - 页面在后台（document.hidden）时跳过；
//   - 焦点在输入框/下拉框时不刷新；
//   - 调用方可传 isBusy() 额外跳过（弹窗打开、正在执行检测等）。
// 事件触发（events）与可见性恢复共用同一套守卫。
export function useAutoRefresh(refresh, opts = {}) {
  const { interval = 30000, isBusy, events = [] } = opts
  let timer = 0
  let running = false

  function canRefresh() {
    if (typeof document === 'undefined') return false
    if (document.hidden) return false
    const el = document.activeElement
    if (el && ['INPUT', 'TEXTAREA', 'SELECT'].includes(el.tagName)) return false
    if (typeof isBusy === 'function' && isBusy()) return false
    return true
  }

  async function tick() {
    if (running || !canRefresh()) return
    running = true
    try {
      await refresh()
    } finally {
      running = false
    }
  }

  function onVisible() {
    if (!document.hidden) tick()
  }

  function onExternalRefresh() {
    tick()
  }

  onMounted(() => {
    document.addEventListener('visibilitychange', onVisible)
    for (const name of events) window.addEventListener(name, onExternalRefresh)
    timer = window.setInterval(tick, interval)
  })

  onUnmounted(() => {
    if (timer) window.clearInterval(timer)
    document.removeEventListener('visibilitychange', onVisible)
    for (const name of events) window.removeEventListener(name, onExternalRefresh)
  })

  return { refreshNow: tick }
}

// 链路探测/优选池的数据变更事件：任意一处保存配置或跑完检测后广播，
// 其他已挂载的探测组件与池列表立即刷新（同一页面里存在多个实例）。
export const CFIP_REFRESH_EVENT = 'cfip:refresh'

export function notifyCfipRefresh() {
  if (typeof window !== 'undefined') {
    window.dispatchEvent(new CustomEvent(CFIP_REFRESH_EVENT))
  }
}

// 端口分配：优先使用后端确认「该节点未占用」的端口，避免新建入站时与已有入站
// （含不同传输层/不同核心）撞同一端口 —— 同一端口号只要被任一入站或端点用过，
// 无论 TCP/UDP 都算已占用。
//
// 后端接口：GET /node-servers/:id/free-port
//   → 汇总 inbound_configs + core_configs(inbounds/endpoints) + 保留端口后随机取空闲值，
//     本地节点还会真实探测 TCP/UDP 可绑定。
//
// 使用方式：组件里同步调用 takeFreePort(serverId)（内部会预取并自动补货），
// 后端不可用时回退随机端口，保证功能不阻塞。
import client from '../../../../api/client'

let cached = 0
let inflight = null

// prefetchFreePort 预取一个空闲端口放入缓存（幂等，重复调用复用同一请求）。
export async function prefetchFreePort(serverId) {
  if (!serverId) return 0
  if (inflight) return inflight
  inflight = (async () => {
    try {
      const r = await client.get(`/node-servers/${serverId}/free-port`)
      const p = Number(r?.port || 0)
      if (p > 0) cached = p
      return cached
    } catch {
      return 0
    } finally {
      inflight = null
    }
  })()
  return inflight
}

// takeFreePort 同步取一个端口：命中后端缓存则用（无重复），否则回退随机端口。
// 取用后会立即在后台补货，供下次调用。
export function takeFreePort(serverId) {
  const p = cached
  cached = 0
  prefetchFreePort(serverId)
  if (p > 0) return p
  return Math.floor(Math.random() * 50000 + 10000)
}

// genFallbackPort 纯随机端口（后端不可用时的兜底，保持与原行为一致）。
export function genFallbackPort() {
  return Math.floor(Math.random() * 50000 + 10000)
}

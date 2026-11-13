// 证书中心 ↔ 入站 TLS 的路径关联（xray / sing-box 共用）。
//
// 入站配置里存的是**文件路径**（如 certs/<dir>/cert.pem），证书中心里的规范路径由
// 域名决定。历史/改名证书的配置仍写名称目录（name=gg、domain=tycng.com 时是
// certs/gg/），所以「按路径反查是哪张证书」必须按**目录别名**比较：
//
//   certs/<domain>[/<keytype>]   ← 规范路径
//   certs/<name>[/<keytype>]     ← 历史名称目录（CertDirAliases 会一起落地）
//   dir(cert_path) / dir(fullchain_path) ← 库里的实际路径
//
// 旧实现用 `includes` 做子串匹配，有两个坑：
//   ① 别名目录匹配不上 → 编辑时「选择证书」永远空着（用户看到的就是这个）；
//   ② 子串误命中 → certs/test.tycng.com/... 会匹配到
//      certs/google-test.tycng.com/...，选中即写错证书路径。
// 这里统一做**目录级精确比较**。

// normalizeCertRefPath 把配置里的引用路径归一化成 certs/... 形式：
// 去掉前导的 ../../、开头的 /，以及绝对路径里的 <dataDir>/ 前缀。
export function normalizeCertRefPath(p) {
  let v = String(p || '').trim().replace(/\\/g, '/')
  while (v.startsWith('../')) v = v.slice(3)
  v = v.replace(/^\.\//, '')
  v = v.replace(/^\/+/, '')
  const i = v.indexOf('certs/')
  if (i > 0) v = v.slice(i)
  return v
}

// certDirOf 取归一化路径的目录部分（certs/tycng.com/rsa/cert.pem → certs/tycng.com/rsa）。
export function certDirOf(p) {
  const v = normalizeCertRefPath(p)
  const i = v.lastIndexOf('/')
  return i > 0 ? v.slice(0, i) : ''
}

// certKeyTypeSuffix 返回目录后缀：ECC 无后缀，RSA 为 /rsa（见 runtime/cert.CertRelPaths）。
function certKeyTypeSuffix(keyType) {
  const kt = String(keyType || '').trim().toLowerCase()
  return kt && kt !== 'ecc' ? '/' + kt : ''
}

// certDirAliases 返回一张证书可能被引用的全部相对目录。
export function certDirAliases(cert) {
  if (!cert) return []
  const dirs = []
  const seen = new Set()
  const add = (dir) => {
    if (dir && !seen.has(dir)) {
      seen.add(dir)
      dirs.push(dir)
    }
  }
  add(certDirOf(cert.cert_path))
  add(certDirOf(cert.fullchain_path))
  const suffix = certKeyTypeSuffix(cert.key_type)
  for (const name of [cert.domain, cert.name]) {
    const clean = String(name || '').trim().replace(/^\/+|\/+$/g, '')
    if (clean) add('certs/' + clean + suffix)
  }
  return dirs
}

// findCertByRef 按入站里引用的证书/私钥路径反查证书中心里的那一张。
// refs 可传多个路径（certificateFile / keyFile 等），任一命中即算关联。
export function findCertByRef(certs, refs) {
  if (!Array.isArray(certs) || !certs.length) return null
  const targets = new Set()
  for (const ref of refs || []) {
    const dir = certDirOf(ref)
    if (dir) targets.add(dir)
  }
  if (!targets.size) return null
  for (const cert of certs) {
    if (certDirAliases(cert).some((d) => targets.has(d))) return cert
  }
  return null
}

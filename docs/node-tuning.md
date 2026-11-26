# 节点内核调优与安全加固

本文档提供**整段可复制**的配置，不需要逐条执行。适用于代理 / 转发节点（xray、sing-box、nginx/caddy 前置）。

> 执行前请保留一个已登录的 SSH 会话，避免规则写错后断连。生产环境建议在维护窗口操作。
> 参数是通用起点，请按机器规格（内存、带宽、并发连接数）微调。

## 1. 内核参数（持久化）

把下面整块写入 `/etc/sysctl.d/99-corefusion.conf`：

```ini
# ---- core ----
# 高并发下的 listen backlog
net.core.somaxconn = 32768
# socket 收发缓冲上限（16 MiB）
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.core.rmem_default = 262144
net.core.wmem_default = 262144
# 网卡收包队列（网卡快于协议栈时）
net.core.netdev_max_backlog = 16384
net.core.netdev_budget = 600
net.core.netdev_budget_usecs = 8000
# BBR 依赖 fq
net.core.default_qdisc = fq

# ---- tcp ----
net.ipv4.tcp_congestion_control = bbr
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216
# SYN 队列 / TIME_WAIT
net.ipv4.tcp_max_syn_backlog = 8192
net.ipv4.tcp_max_tw_buckets = 5000
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 15
# 出站可用端口范围
net.ipv4.ip_local_port_range = 1024 65535
# ICMP 被墙时启用 PLPMTUD
net.ipv4.tcp_mtu_probing = 1
# TCP Fast Open（客户端 + 服务端）
net.ipv4.tcp_fastopen = 3
# keepalive
net.ipv4.tcp_keepalive_time = 600
net.ipv4.tcp_keepalive_intvl = 30
net.ipv4.tcp_keepalive_probes = 10
# 上传缓冲，降低 bufferbloat
net.ipv4.tcp_notsent_lowat = 16384
# 空闲后不重置拥塞窗口
net.ipv4.tcp_slow_start_after_idle = 0
# SYN flood 防护
net.ipv4.tcp_syncookies = 1
# 转发节点必须开启
net.ipv4.ip_forward = 1

# ---- conntrack ----
net.netfilter.nf_conntrack_max = 262144
net.netfilter.nf_conntrack_buckets = 16384
net.netfilter.nf_conntrack_tcp_timeout_established = 180
net.netfilter.nf_conntrack_udp_timeout = 60

# ---- ipv6 ----
# 节点没有 IPv6 时整段忽略（检查：ip -6 addr show scope global）
net.ipv6.conf.all.forwarding = 1
net.ipv6.conf.all.accept_redirects = 0
net.ipv6.conf.all.accept_source_route = 0
# 服务器使用稳定地址，关闭隐私扩展
net.ipv6.conf.all.use_tempaddr = 0
# 邻居表（高并发下避免 neighbor table overflow）
net.ipv6.neigh.default.gc_thresh1 = 1024
net.ipv6.neigh.default.gc_thresh2 = 4096
net.ipv6.neigh.default.gc_thresh3 = 8192
# 路由缓存上限（4.2+；3.10 没有该参数，写了会报 unknown key）
net.ipv6.route.max_size = 262144

# ---- security ----
# 关闭 ICMP 重定向与源路由（防 MITM）
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.all.accept_source_route = 0
# rp_filter：0 = 宽松（多网卡 / 非对称路由 / 代理场景），1 = 严格
net.ipv4.conf.all.rp_filter = 0

# ---- system ----
# 以下在容器内通常由宿主决定，只在宿主机设置
fs.file-max = 1024000
fs.nr_open = 1048576
vm.swappiness = 10
kernel.pid_max = 4194304
```

应用并验证：

```sh
# BBR 在多数发行版是模块，先加载并持久化
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

如果 `tcp_congestion_control` 写入后报 `Protocol not available`，说明内核没有 bbr 模块，需先升级内核。

## 与已有优化共存（谁覆盖谁）

节点上很可能已经有人写过优化文件，直接追加会出现「改了不生效」。两边的规则是相反的：

**内核参数（sysctl）：后加载的覆盖先加载的**

加载顺序固定为：

```
/etc/sysctl.d/*.conf（按文件名字典序）
→ /run/sysctl.d/*.conf
→ /usr/local/lib/sysctl.d/*.conf
→ /usr/lib/sysctl.d/*.conf
→ /lib/sysctl.d/*.conf
→ /etc/sysctl.conf（永远最后）
```

所以：

- `/etc/sysctl.conf` 里的同 key **一定覆盖** `/etc/sysctl.d/99-corefusion.conf`；
- `/etc/sysctl.d/99-sysctl.conf`（常见软链，指向 `../sysctl.conf`）字典序在 `99-corefusion.conf` 之后，也会覆盖；
- `10-*`、`50-*` 等更早的文件会被我们覆盖。

做法：先查一遍再决定写哪里，**不要同一 key 在两个文件里各写一份**。

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control   # 查当前生效值
```

如果 `/etc/sysctl.conf` 里已有同 key，二选一：

1. 把那些行注释掉，统一由 `/etc/sysctl.d/99-corefusion.conf` 管理；
2. 或直接改 `/etc/sysctl.conf`，不要再用本文件的同名项。

**SSH：先匹配的生效（和 sysctl 相反）**

`sshd` 读到第一条指令就定值，drop-in 在主配置 `Include` 的位置展开、按文件名字典序处理。所以：

- 本文件用 `00-corefusion-*.conf` 前缀，保证在 drop-in 里最优先；
- `Include /etc/ssh/sshd_config.d/*.conf` 必须在主配置靠前的位置（在 `PasswordAuthentication` 等指令之前），否则主配置里的行会先命中，drop-in 直接失效；
- 若已有 `00-`、`01-` 等更靠前的文件写了同一指令，以那个为准。

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

体检里的 `CFG-01` / `CFG-02` 会自动列出检测到的文件和「谁生效」，并在「完整调优文档」第 0 节给出同样的提示。

## 内核版本差异（重要）

不是所有参数都适用于 3.x。CentOS 7 自带 3.10，**没有 BBR 和 fq**，照抄 4.x 的配置会得到 `Invalid argument`：

| 参数 | 3.x（CentOS 7 / 3.10） | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ 不支持，保持 `cubic` | ✓ | ✓ |
| `net.core.default_qdisc = fq` | ✗ 用 `fq_codel` | ✓ | ✓ |
| `net.ipv4.tcp_migrate_req` | ✗（4.14+） | ✗ | ✓ |
| `net.mptcp.enabled` | ✗ | ✗ | ✓（5.6+） |
| `net.ipv4.tcp_notsent_lowat` | ✓（3.6+） | ✓ | ✓ |
| `net.ipv4.tcp_fastopen` | ✓（3.7+） | ✓ | ✓ |

体检会读取节点实际内核版本：3.10 上只提示「内核不支持 BBR」，不会生成 BBR 命令，
生成的 Markdown 也会自动省略 BBR/fq 并改用 `fq_codel`。

说明：

- **IPv6**：只有节点确实有全局 IPv6 时才需要这一组。`forwarding=1` 用于容器网络 / 透明转发；
  纯用户态代理（xray/sing-box/nginx）不依赖它。静态地址不要配 `accept_ra=1`，SLAAC 环境保持默认 `2`。
  体检在检测到全局 IPv6 时会检查 `forwarding`、`accept_redirects`、`use_tempaddr`、邻居表和路由缓存上限
  （`IPV6-01` ~ `IPV6-06`）；没有 IPv6 的节点不会产生这些项。
- `fs.file-max`、`fs.nr_open`、`vm.swappiness`、`kernel.pid_max` 在容器内可能只读，需在宿主机设置。
- `net.ipv4.conf.all.rp_filter` 线上代理节点一般用 `0`；纯单网卡且不需要非对称路由时可改 `1`。
- 如果节点只跑 TCP，可忽略 `nf_conntrack_udp_timeout`。
- 数据库节点再考虑 `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`，代理节点不需要。

## 2. SSH 加固

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

说明：

- **不要为了"安全"关掉 root 登录**。`PermitRootLogin yes` 保持不动即可，真正要关的是密码入口；
  `PasswordAuthentication no` + `KbdInteractiveAuthentication no` 之后，root 只能用密钥登录。
- 如果原本就是 `PermitRootLogin prohibit-password` 也可以保留（效果同样是只允许密钥）。
- 确认至少一把私钥可以登录后，再关闭当前会话。

## 3. 限制 SSH 来源 IP

把 `<YOUR_IP>` 换成办公网 / VPN 地址（支持 CIDR）。按节点实际使用的防火墙选择一段：

**firewalld**

```sh
firewall-cmd --permanent --remove-service=ssh
firewall-cmd --permanent --add-rich-rule='rule family="ipv4" source address="<YOUR_IP>" port port="22" protocol="tcp" accept'
firewall-cmd --reload
firewall-cmd --list-rich-rules
```

**ufw**

```sh
ufw delete allow 22/tcp
ufw allow from <YOUR_IP> to any port 22 proto tcp
ufw reload
ufw status numbered
```

**iptables**

```sh
iptables -I INPUT -p tcp --dport 22 -s <YOUR_IP> -j ACCEPT
iptables -A INPUT -p tcp --dport 22 -j DROP
# 持久化：apt install iptables-persistent && netfilter-persistent save
```

**nftables**

```sh
# 表名/链名按实际规则集调整（nft list ruleset）
nft insert rule inet filter input tcp dport 22 ip saddr <YOUR_IP> accept
nft add rule inet filter input tcp dport 22 drop
# 持久化：nft list ruleset > /etc/nftables.conf && systemctl enable nftables
```

注意：

- 先加允许规则，再加默认拒绝，顺序不能反。
- 云主机还有安全组时，本机防火墙和安全组要同时放行你的来源 IP。
- 如果 SSH 端口不是 22，把命令里的 `22` 换成实际端口。

## 4. 回滚

```sh
rm -f /etc/sysctl.d/99-corefusion.conf
sysctl --system

rm -f /etc/ssh/sshd_config.d/00-corefusion-hardening.conf
sshd -t && systemctl reload sshd
```

防火墙规则用对应的删除命令移除：

```sh
# firewalld
firewall-cmd --permanent --remove-rich-rule='rule family="ipv4" source address="<YOUR_IP>" port port="22" protocol="tcp" accept'
firewall-cmd --reload

# ufw
ufw delete allow from <YOUR_IP> to any port 22 proto tcp

# iptables
iptables -D INPUT -p tcp --dport 22 -s <YOUR_IP> -j ACCEPT
iptables -D INPUT -p tcp --dport 22 -j DROP

# nftables
nft -a list ruleset   # 找到 handle 后删除
nft delete rule inet filter input handle <HANDLE>
```

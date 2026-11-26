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

- `fs.file-max`、`fs.nr_open`、`vm.swappiness`、`kernel.pid_max` 在容器内可能只读，需在宿主机设置。
- `net.ipv4.conf.all.rp_filter` 线上代理节点一般用 `0`；纯单网卡且不需要非对称路由时可改 `1`。
- 如果节点只跑 TCP，可忽略 `nf_conntrack_udp_timeout`。
- 数据库节点再考虑 `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`，代理节点不需要。

## 2. SSH 加固

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/99-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin prohibit-password
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

确认至少一把私钥可以登录后，再关闭当前会话。

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

rm -f /etc/ssh/sshd_config.d/99-corefusion-hardening.conf
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

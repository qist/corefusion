# 節點核心調優與安全加固

> 語言: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

本文件提供**整段可複製**的設定，不需要逐條執行。適用於代理 / 轉送節點（xray、sing-box、nginx/caddy 前端）。

> 執行前請保留一個已登入的 SSH 工作階段，避免規則寫錯後斷線。生產環境建議在維護時段操作。
> 參數是通用起點，請依機器規格（記憶體、頻寬、並行連線數）微調。

## 1. 核心參數（持久化）

把下面整段寫入 `/etc/sysctl.d/99-corefusion.conf`:

```ini
# ---- core ----
net.core.somaxconn = 32768
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.core.rmem_default = 262144
net.core.wmem_default = 262144
net.core.netdev_max_backlog = 16384
net.core.netdev_budget = 600
net.core.netdev_budget_usecs = 8000
# BBR requires fq
net.core.default_qdisc = fq

# ---- tcp ----
net.ipv4.tcp_congestion_control = bbr
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216
net.ipv4.tcp_max_syn_backlog = 8192
net.ipv4.tcp_max_tw_buckets = 5000
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 15
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_mtu_probing = 1
net.ipv4.tcp_fastopen = 3
net.ipv4.tcp_keepalive_time = 600
net.ipv4.tcp_keepalive_intvl = 30
net.ipv4.tcp_keepalive_probes = 10
net.ipv4.tcp_notsent_lowat = 16384
net.ipv4.tcp_slow_start_after_idle = 0
net.ipv4.tcp_syncookies = 1
net.ipv4.ip_forward = 1

# ---- conntrack ----
net.netfilter.nf_conntrack_max = 262144
net.netfilter.nf_conntrack_buckets = 16384
net.netfilter.nf_conntrack_tcp_timeout_established = 180
net.netfilter.nf_conntrack_udp_timeout = 60

# ---- ipv6 ----
# skip this block when the node has no IPv6 (check: ip -6 addr show scope global)
net.ipv6.conf.all.forwarding = 1
net.ipv6.conf.all.accept_redirects = 0
net.ipv6.conf.all.accept_source_route = 0
net.ipv6.conf.all.use_tempaddr = 0
net.ipv6.neigh.default.gc_thresh1 = 1024
net.ipv6.neigh.default.gc_thresh2 = 4096
net.ipv6.neigh.default.gc_thresh3 = 8192
# 4.2+; not present on 3.10
net.ipv6.route.max_size = 262144

# ---- security ----
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.all.accept_source_route = 0
net.ipv4.conf.all.rp_filter = 0

# ---- system (host only in containers) ----
fs.file-max = 1024000
fs.nr_open = 1048576
vm.swappiness = 10
kernel.pid_max = 4194304
```

套用並驗證:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

如果寫入 `tcp_congestion_control` 後出現 `Protocol not available`，代表核心沒有 bbr 模組，需先升級核心。

## 與既有調優共存（誰覆蓋誰）

節點上很可能已經有人寫過調優檔案，直接追加會出現「改了不生效」。兩邊的規則相反：

**核心參數（sysctl）：後載入的覆蓋先載入的**

載入順序固定為

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

所以：

- `/etc/sysctl.conf` 裡的同一個 key **一定覆蓋** `/etc/sysctl.d/99-corefusion.conf`；
- `/etc/sysctl.d/99-sysctl.conf`（常見軟連結，指向 `../sysctl.conf`）字典序在 `99-corefusion.conf` 之後，也會覆蓋；
- `10-*`、`50-*` 等更早的檔案會被我們覆蓋。

做法：先查一遍再決定寫哪裡，**不要同一個 key 在兩個檔案各寫一份**。

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

如果 `/etc/sysctl.conf` 已有同一個 key，二選一：

1. 把那些行註解掉，統一由 `/etc/sysctl.d/99-corefusion.conf` 管理；
2. 或直接改 `/etc/sysctl.conf`，不要再用本文件的同名項目。

**SSH：先匹配的生效（和 sysctl 相反）**

`sshd` 讀到第一條指令就定值，drop-in 在主設定 `Include` 的位置展開、依檔名字典序處理。所以：

- 本文件用 `00-corefusion-*.conf` 前綴，保證在 drop-in 裡最優先；
- `Include /etc/ssh/sshd_config.d/*.conf` 必須在主設定靠前的位置（在 `PasswordAuthentication` 等指令之前），否則主設定裡的行會先命中，drop-in 直接失效；
- 若已有 `00-`、`01-` 等更靠前的檔案寫了同一指令，以那個為準。

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

體檢裡的 `CFG-01` / `CFG-02` 會自動列出偵測到的檔案和「誰生效」，並在「完整調優文件」第 0 節給出同樣提示。

## 核心版本差異（重要）

不是所有參數都適用於 3.x。CentOS 7 自帶 3.10，**沒有 BBR 和 fq**，照抄 4.x 設定會得到 `Invalid argument`：

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ 不支援，保持 `cubic` | OK | OK |
| `net.core.default_qdisc = fq` | ✗ 改用 `fq_codel` | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗（4.14+） | ✗（4.14+） | OK |
| `net.mptcp.enabled` | ✗（5.6+） | ✗（5.6+） | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

體檢會讀取節點實際核心版本：3.10 只提示「核心不支援 BBR」，不會產生 BBR 指令，產生的 Markdown 也會自動省略 BBR/fq 並改用 `fq_codel`。

說明:

- **通道位址不要手動處理**：sing-box/WARP 的 `singbr0`、`wg0`、`tun0` 上的 IPv6 由代理自動分配，重啟/重連會變。不要手動改位址、也不要寫進 sysctl；體檢會標成 `tun`，不計入節點公網 IPv6。
- **IPv6**：只有節點確實有全域 IPv6 時才需要這一組。`forwarding=1` 用於容器網路 / 透明轉送；純使用者態代理（xray/sing-box/nginx）不依賴它。靜態位址不要設 `accept_ra=1`，SLAAC 環境保持預設 `2`。體檢在偵測到全域 IPv6 時會檢查 `forwarding`、`accept_redirects`、`use_tempaddr`、鄰居表和路由快取上限（`IPV6-01` ~ `IPV6-06`）；沒有 IPv6 的節點不會產生這些項目。
- `fs.file-max`、`fs.nr_open`、`vm.swappiness`、`kernel.pid_max` 在容器內可能唯讀，需在宿主機設定。
- `net.ipv4.conf.all.rp_filter` 線上代理節點一般用 `0`；純單網卡且不需要非對稱路由時可改 `1`。
- 如果節點只跑 TCP，可忽略 `nf_conntrack_udp_timeout`。
- 資料庫節點再考慮 `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`，代理節點不需要。

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

說明:

- **不要為了「安全」關掉 root 登入**。`PermitRootLogin yes` 保持不動即可，真正要關的是密碼入口；`PasswordAuthentication no` + `KbdInteractiveAuthentication no` 之後，root 只能用金鑰登入。
- 如果原本就是 `PermitRootLogin prohibit-password` 也可以保留（效果同樣只允許金鑰）。
- 確認至少一把私鑰可以登入後，再關閉目前工作階段。

## 3. 限制 SSH 來源 IP

把 `<YOUR_IP>` 換成辦公網 / VPN 位址（支援 CIDR）。依節點實際使用的防火牆選一段：

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
# persist: apt install iptables-persistent && netfilter-persistent save
```

**nftables**

```sh
# adjust table/chain names to your ruleset (nft list ruleset)
nft insert rule inet filter input tcp dport 22 ip saddr <YOUR_IP> accept
nft add rule inet filter input tcp dport 22 drop
# persist: nft list ruleset > /etc/nftables.conf && systemctl enable nftables
```

注意:

- 先加允許規則，再加預設拒絕，順序不能反。
- 雲端主機還有安全群組時，本機防火牆和安全群組要同時放行你的來源 IP。
- 如果 SSH 連接埠不是 22，把指令裡的 `22` 換成實際連接埠。

## 4. 回滾

```sh
rm -f /etc/sysctl.d/99-corefusion.conf
sysctl --system

rm -f /etc/ssh/sshd_config.d/00-corefusion-hardening.conf
sshd -t && systemctl reload sshd
```

Firewall rules must be removed with the matching delete command:

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
nft -a list ruleset   # find the handle
nft delete rule inet filter input handle <HANDLE>
```

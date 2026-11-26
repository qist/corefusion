# Tinh chỉnh kernel và tăng cường bảo mật cho node

> Ngôn ngữ: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

Tài liệu cung cấp các khối **sao chép nguyên văn**, không cần chạy từng dòng. Dành cho node proxy / chuyển tiếp (xray, sing-box, nginx/caddy phía trước).

> Hãy giữ một phiên SSH đang đăng nhập trước khi áp dụng, tránh việc viết sai quy tắc làm mất kết nối. Môi trường production nên làm trong cửa sổ bảo trì.
> Giá trị là điểm khởi đầu chung; hãy điều chỉnh theo RAM, băng thông và số kết nối đồng thời.

## 1. Tham số kernel (lưu bền)

Ghi toàn bộ khối sau vào `/etc/sysctl.d/99-corefusion.conf`:

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

Áp dụng và kiểm tra:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

Nếu ghi `tcp_congestion_control` báo `Protocol not available`, kernel không có module bbr — cần nâng cấp kernel trước.

## Cùng tồn tại với cấu hình sẵn có (ai thắng)

Node thường đã có tệp tinh chỉnh riêng; thêm vào mà không kiểm tra sẽ dẫn đến "đã sửa nhưng không có tác dụng". Hai bên có quy tắc ngược nhau:

**sysctl: tệp nạp sau cùng thắng**

Thứ tự nạp

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

Vì vậy:

- cùng key trong `/etc/sysctl.conf` **luôn ghi đè** `/etc/sysctl.d/99-corefusion.conf`;
- `/etc/sysctl.d/99-sysctl.conf` (symlink phổ biến trỏ tới `../sysctl.conf`) đứng sau `99-corefusion.conf` nên cũng ghi đè;
- các tệp `10-*`, `50-*` và sớm hơn sẽ bị chúng ta ghi đè.

Kiểm tra trước rồi mới quyết định ghi ở đâu; **mỗi key chỉ ghi ở một tệp**.

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

Nếu `/etc/sysctl.conf` đã có key đó, chọn một trong hai:

1. comment các dòng đó và quản lý tập trung bằng `/etc/sysctl.d/99-corefusion.conf`;
2. hoặc sửa trực tiếp `/etc/sysctl.conf` và bỏ mục trùng ở đây.

**sshd: khớp đầu tiên thắng (ngược với sysctl)**

`sshd` lấy giá trị đầu tiên đọc được. Drop-in được mở tại dòng `Include`, theo thứ tự từ điển. Do đó:

- tài liệu dùng tiền tố `00-corefusion-*.conf` để đứng đầu trong các drop-in;
- `Include /etc/ssh/sshd_config.d/*.conf` phải nằm gần đầu tệp chính (trước `PasswordAuthentication`...), nếu không tệp chính sẽ thắng và drop-in bị bỏ qua;
- nếu đã có `00-`/`01-` đặt cùng chỉ thị thì tệp đó thắng.

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

Kiểm tra sẽ báo là `CFG-01` / `CFG-02` và nhắc lại ở mục 0 của hướng dẫn được tạo.

## Khác biệt phiên bản kernel (quan trọng)

Không phải tham số nào cũng có trên 3.x. CentOS 7 dùng 3.10: **không có BBR và fq**; sao chép giá trị 4.x sẽ báo `Invalid argument`:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ không hỗ trợ, giữ `cubic` | OK | OK |
| `net.core.default_qdisc = fq` | ✗ dùng `fq_codel` | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ (4.14+) | OK |
| `net.mptcp.enabled` | ✗ (5.6+) | ✗ (5.6+) | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

Kiểm tra đọc phiên bản kernel thực tế: trên 3.10 chỉ báo "kernel không cung cấp BBR", không tạo lệnh BBR, và Markdown tạo ra cũng bỏ BBR/fq và dùng `fq_codel`.

Ghi chú:

- **Không tự xử lý địa chỉ tunnel**: IPv6 trên `singbr0` / `wg0` / `tun0` (sing-box, WARP) do proxy cấp và thay đổi khi kết nối lại. Không sửa và không ghi vào sysctl; kiểm tra sẽ đánh dấu `tun` và không tính vào IPv6 công khai của node.
- **IPv6**: chỉ cần khi node thực sự có IPv6 toàn cầu. `forwarding=1` dùng cho mạng container / chuyển tiếp trong suốt; proxy không gian người dùng (xray/sing-box/nginx) không phụ thuộc. Với địa chỉ tĩnh đừng đặt `accept_ra=1`; với SLAAC giữ mặc định `2`. Kiểm tra sẽ xem `forwarding`, `accept_redirects`, `use_tempaddr`, bảng neighbor và giới hạn cache route (`IPV6-01` ~ `IPV6-06`); node không có IPv6 sẽ không sinh mục này.
- `fs.file-max`, `fs.nr_open`, `vm.swappiness`, `kernel.pid_max` thường chỉ đọc trong container: đặt trên máy chủ.
- `net.ipv4.conf.all.rp_filter` thường là `0` trên node proxy; với một NIC không có định tuyến bất đối xứng có thể dùng `1`.
- Nếu chỉ chạy TCP, có thể bỏ qua `nf_conntrack_udp_timeout`.
- Node cơ sở dữ liệu có thể thêm `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`; proxy không cần.

## 2. Tăng cường SSH

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

Ghi chú:

- **Đừng tắt đăng nhập root vì "bảo mật"**. `PermitRootLogin yes` có thể giữ nguyên; thứ cần đóng là mật khẩu. Với `PasswordAuthentication no` + `KbdInteractiveAuthentication no`, root chỉ vào bằng khóa.
- Nếu đang là `PermitRootLogin prohibit-password` cũng được (vẫn chỉ khóa).
- Xác nhận ít nhất một khóa đăng nhập được trước khi đóng phiên hiện tại.

## 3. Giới hạn nguồn SSH

Thay `<YOUR_IP>` bằng địa chỉ văn phòng/VPN (hỗ trợ CIDR). Chọn khối theo firewall đang dùng:

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

Ghi chú:

- Thêm quy tắc cho phép trước, rồi mới drop mặc định; không đảo thứ tự.
- Nếu có security group trên cloud, cũng phải mở IP nguồn ở đó.
- Nếu SSH không phải cổng 22, thay `22` trong lệnh.

## 4. Hoàn tác

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

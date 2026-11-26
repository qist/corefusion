# ปรับแต่งเคอร์เนลและเสริมความปลอดภัยของโหนด

> ภาษา: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

เอกสารนี้มีบล็อกที่**คัดลอกได้ทั้งก้อน** ไม่ต้องรันทีละบรรทัด เหมาะกับโหนดพร็อกซี / ส่งต่อ (xray, sing-box, nginx/caddy ด้านหน้า)

> ก่อนใช้งานให้เปิด SSH session ที่ล็อกอินอยู่ไว้หนึ่งอัน เพื่อไม่ให้พลาดการเชื่อมต่อหากกฎผิด พรอดักชันแนะนำให้ทำในช่วงบำรุงรักษา
> ค่าเหล่านี้เป็นจุดเริ่มต้นทั่วไป ควรปรับตาม RAM แบนด์วิดท์ และจำนวนการเชื่อมต่อ

## 1. พารามิเตอร์เคอร์เนล (ถาวร)

เขียนบล็อกทั้งหมดลงใน `/etc/sysctl.d/99-corefusion.conf`:

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

ใช้งานและตรวจสอบ:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

หากเขียน `tcp_congestion_control` แล้วขึ้น `Protocol not available` แสดงว่าเคอร์เนลไม่มีโมดูล bbr ต้องอัปเกรดเคอร์เนลก่อน

## อยู่ร่วมกับการปรับแต่งเดิม (ใครชนะ)

โหนดมักมีไฟล์ปรับแต่งของตัวเองอยู่แล้ว หากเพิ่มเข้าไปโดยไม่ตรวจจะเจอ "แก้แล้วแต่ไม่เกิดผล" กฎของสองฝั่งตรงกันข้าม:

**sysctl: ไฟล์ที่โหลดทีหลังชนะ**

ลำดับการโหลด

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

ดังนั้น:

- key เดียวกันใน `/etc/sysctl.conf` **จะทับ** `/etc/sysctl.d/99-corefusion.conf` เสมอ;
- `/etc/sysctl.d/99-sysctl.conf` (symlink ที่พบบ่อยไปยัง `../sysctl.conf`) อยู่หลัง `99-corefusion.conf` จึงทับด้วย;
- ไฟล์ `10-*`, `50-*` และที่มาก่อนจะถูกเราทับ

ตรวจก่อนแล้วค่อยตัดสินใจว่าจะเขียนที่ไหน **หนึ่ง key ควรมีที่เดียว**

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

หาก `/etc/sysctl.conf` มี key นี้อยู่แล้ว เลือกอย่างใดอย่างหนึ่ง:

1. คอมเมนต์บรรทัดเหล่านั้น แล้วจัดการผ่าน `/etc/sysctl.d/99-corefusion.conf`;
2. หรือแก้ `/etc/sysctl.conf` โดยตรง แล้วลบรายการซ้ำที่นี่

**sshd: ค่าที่ตรงก่อนชนะ (ตรงข้ามกับ sysctl)**

`sshd` ใช้ค่าที่อ่านเจอตัวแรก drop-in ถูกขยายตรงบรรทัด `Include` ตามลำดับชื่อไฟล์ ดังนั้น:

- เอกสารนี้ใช้คำนำหน้า `00-corefusion-*.conf` เพื่อให้มาก่อนในกลุ่ม drop-in;
- `Include /etc/ssh/sshd_config.d/*.conf` ต้องอยู่ตอนต้นของไฟล์หลัก (ก่อน `PasswordAuthentication` ฯลฯ) มิฉะนั้นไฟล์หลักจะชนะและ drop-in ถูกเพิกเฉย;
- ถ้ามีไฟล์ `00-`/`01-` ที่เขียนคำสั่งเดียวกันอยู่แล้ว ให้ใช้ไฟล์นั้น

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

การตรวจจะรายงานเป็น `CFG-01` / `CFG-02` และแสดงซ้ำในหัวข้อ 0 ของคู่มือที่สร้างขึ้น

## ความต่างของเวอร์ชันเคอร์เนล (สำคัญ)

ไม่ใช่ทุกพารามิเตอร์จะมีใน 3.x CentOS 7 ใช้ 3.10 ซึ่ง**ไม่มี BBR และ fq** หากคัดลอกค่าของ 4.x จะได้ `Invalid argument`:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ ไม่รองรับ คง `cubic` | OK | OK |
| `net.core.default_qdisc = fq` | ✗ ใช้ `fq_codel` | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ (4.14+) | OK |
| `net.mptcp.enabled` | ✗ (5.6+) | ✗ (5.6+) | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

การตรวจจะอ่านเวอร์ชันเคอร์เนลจริง: บน 3.10 จะแจ้งเพียง "เคอร์เนลไม่รองรับ BBR" ไม่สร้างคำสั่ง BBR และ Markdown ที่สร้างจะตัด BBR/fq ออกและใช้ `fq_codel`

หมายเหตุ:

- **อย่าจัดการที่อยู่ tunnel เอง**: IPv6 บน `singbr0` / `wg0` / `tun0` (sing-box, WARP) ถูกกำหนดโดยพร็อกซีและเปลี่ยนเมื่อเชื่อมต่อใหม่ ไม่ต้องแก้และไม่ต้องเขียนลง sysctl การตรวจจะทำเครื่องหมายเป็น `tun` และไม่นับเป็น IPv6 สาธารณะของโหนด
- **IPv6**: ต้องใช้เฉพาะเมื่อโหนดมี IPv6 สาธารณะจริง `forwarding=1` ใช้สำหรับเครือข่ายคอนเทนเนอร์ / ส่งต่อโปร่งใส พร็อกซีแบบ userland (xray/sing-box/nginx) ไม่จำเป็นต้องใช้ ที่อยู่แบบ static ไม่ควรตั้ง `accept_ra=1` ส่วน SLAAC ให้คงค่าเริ่มต้น `2` การตรวจจะดู `forwarding`, `accept_redirects`, `use_tempaddr`, ตาราง neighbor และขีดจำกัด route cache (`IPV6-01` ~ `IPV6-06`); โหนดที่ไม่มี IPv6 จะไม่เกิดรายการเหล่านี้
- `fs.file-max`, `fs.nr_open`, `vm.swappiness`, `kernel.pid_max` มักเป็นอ่านอย่างเดียวในคอนเทนเนอร์ ต้องตั้งบนโฮสต์
- `net.ipv4.conf.all.rp_filter` บนโหนดพร็อกซีมักใช้ `0`; ถ้าเป็นการ์ดเดียวและไม่มี asymmetric routing อาจใช้ `1`
- ถ้าโหนดรันเฉพาะ TCP สามารถข้าม `nf_conntrack_udp_timeout` ได้
- โหนดฐานข้อมูลอาจเพิ่ม `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax` แต่พร็อกซีไม่จำเป็น

## 2. เสริมความปลอดภัย SSH

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

หมายเหตุ:

- **อย่าปิดการล็อกอิน root เพื่อ "ความปลอดภัย"** `PermitRootLogin yes` เปิดไว้ได้ สิ่งที่ต้องปิดคือช่องทางรหัสผ่าน เมื่อตั้ง `PasswordAuthentication no` + `KbdInteractiveAuthentication no` แล้ว root จะเข้าด้วยคีย์เท่านั้น
- หากเดิมเป็น `PermitRootLogin prohibit-password` ก็ใช้ได้ (อนุญาตเฉพาะคีย์เช่นกัน)
- ยืนยันว่ามีกุญแจอย่างน้อยหนึ่งอันล็อกอินได้ก่อนปิด session ปัจจุบัน

## 3. จำกัดแหล่งที่มาของ SSH

แทน `<YOUR_IP>` ด้วยที่อยู่สำนักงาน/VPN (รองรับ CIDR) เลือกบล็อกตามไฟร์วอลล์ที่ใช้จริง:

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

หมายเหตุ:

- เพิ่มกฎอนุญาตก่อน แล้วจึงตั้ง default drop ห้ามสลับลำดับ
- หากมี security group บนคลาวด์ ต้องเปิด IP ต้นทางที่นั่นด้วย
- หาก SSH ไม่ใช่พอร์ต 22 ให้เปลี่ยน `22` ในคำสั่ง

## 4. ย้อนกลับ

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

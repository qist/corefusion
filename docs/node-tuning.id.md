<!-- source: node-tuning.md | locale: id | updated: 2026-11-26 -->
# Penyetelan kernel dan pengerasan node

> Bahasa: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

Dokumen ini menyediakan blok yang **bisa disalin utuh**, tanpa menjalankan baris per baris. Untuk node proxy / penerusan (xray, sing-box, nginx/caddy di depan).

> Pertahankan sesi SSH yang sudah login sebelum menerapkan, agar kesalahan aturan tidak memutus akses. Di produksi, lakukan di jendela pemeliharaan.
> Nilainya titik awal umum; sesuaikan dengan RAM, bandwidth, dan jumlah koneksi.

## 1. Parameter kernel (persisten)

Tulis seluruh blok ke `/etc/sysctl.d/99-corefusion.conf`:

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

Terapkan dan verifikasi:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

Jika penulisan `tcp_congestion_control` menghasilkan `Protocol not available`, kernel tidak punya modul bbr — tingkatkan kernel dulu.

## Berjalan bersama penyetelan yang sudah ada (siapa menang)

Node sering sudah punya file penyetelan sendiri; menambahkan begitu saja membuat "sudah diubah tapi tidak berefek". Aturannya justru berlawanan:

**sysctl: file yang dimuat terakhir menang**

Urutan pemuatan

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

Karena itu:

- key yang sama di `/etc/sysctl.conf` **selalu menimpa** `/etc/sysctl.d/99-corefusion.conf`;
- `/etc/sysctl.d/99-sysctl.conf` (symlink umum ke `../sysctl.conf`) dimuat setelah `99-corefusion.conf` dan juga menimpa;
- file `10-*`, `50-*` dan yang lebih awal ditimpa oleh kita.

Periksa dulu, baru tentukan tempat menulis; **satu key hanya di satu file**.

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

Jika `/etc/sysctl.conf` sudah punya key tersebut, pilih salah satu:

1. komentari baris itu dan kelola lewat `/etc/sysctl.d/99-corefusion.conf`;
2. atau ubah `/etc/sysctl.conf` dan hapus entri di sini.

**sshd: kecocokan pertama menang (kebalikan sysctl)**

`sshd` memakai nilai pertama yang dibaca. Drop-in diperluas di baris `Include` dengan urutan leksikografis. Jadi:

- kami memakai awalan `00-corefusion-*.conf` agar paling awal di antara drop-in;
- `Include /etc/ssh/sshd_config.d/*.conf` harus di bagian atas konfigurasi utama (sebelum `PasswordAuthentication` dll), jika tidak konfigurasi utama menang dan drop-in diabaikan;
- jika sudah ada `00-`/`01-` dengan direktif sama, itu yang menang.

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

Pemeriksaan melaporkannya sebagai `CFG-01` / `CFG-02` dan mengulanginya di bagian 0 panduan yang dihasilkan.

## Perbedaan versi kernel (penting)

Tidak semua parameter ada di 3.x. CentOS 7 memakai 3.10: **tanpa BBR dan fq**; menyalin nilai 4.x menghasilkan `Invalid argument`:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ tidak tersedia, pertahankan `cubic` | OK | OK |
| `net.core.default_qdisc = fq` | ✗ gunakan `fq_codel` | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ (4.14+) | OK |
| `net.mptcp.enabled` | ✗ (5.6+) | ✗ (5.6+) | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

Pemeriksaan membaca versi kernel sebenarnya: di 3.10 hanya melaporkan "kernel tidak menyediakan BBR", tidak membuat perintah BBR, dan Markdown yang dihasilkan menghapus BBR/fq serta memakai `fq_codel`.

Catatan:

- **Jangan sentuh alamat tunnel**: IPv6 pada `singbr0` / `wg0` / `tun0` (sing-box, WARP) dialokasikan proxy dan berubah saat tersambung ulang. Jangan diedit atau dimasukkan ke sysctl; pemeriksaan menandainya `tun` dan tidak menghitungnya sebagai IPv6 publik node.
- **IPv6**: hanya perlu jika node benar-benar punya IPv6 global. `forwarding=1` untuk jaringan kontainer / penerusan transparan; proxy userland (xray/sing-box/nginx) tidak bergantung padanya. Untuk alamat statis jangan pakai `accept_ra=1`; pada SLAAC biarkan `2`. Pemeriksaan lalu memeriksa `forwarding`, `accept_redirects`, `use_tempaddr`, tabel tetangga, dan batas cache rute (`IPV6-01` ~ `IPV6-06`); tanpa IPv6 item ini tidak muncul.
- `fs.file-max`, `fs.nr_open`, `vm.swappiness`, `kernel.pid_max` sering read-only di kontainer: setel di host.
- `net.ipv4.conf.all.rp_filter` umumnya `0` di node proxy; dengan satu NIC tanpa routing asimetris bisa `1`.
- Jika hanya TCP, `nf_conntrack_udp_timeout` boleh diabaikan.
- Node basis data dapat menambah `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`; proxy tidak perlu.

## 2. Pengerasan SSH

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

Catatan:

- **Jangan matikan login root "demi keamanan"**. `PermitRootLogin yes` boleh dibiarkan; yang harus ditutup adalah jalur kata sandi. Dengan `PasswordAuthentication no` + `KbdInteractiveAuthentication no`, root hanya bisa masuk dengan kunci.
- Jika sudah `PermitRootLogin prohibit-password`, itu juga boleh (tetap hanya kunci).
- Pastikan setidaknya satu kunci bisa login sebelum menutup sesi saat ini.

## 3. Batasi SSH ke sumber tepercaya

Ganti `<YOUR_IP>` dengan alamat kantor/VPN (CIDR didukung). Pilih blok sesuai firewall yang dipakai:

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

Catatan:

- Tambahkan aturan izin dulu, baru default drop; jangan dibalik.
- Jika ada security group cloud, izinkan IP sumber di sana juga.
- Jika SSH bukan di port 22, ganti `22` pada perintah.

## 4. Rollback

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

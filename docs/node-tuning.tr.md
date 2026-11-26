<!-- source: node-tuning.md | locale: tr | updated: 2026-11-26 -->
# Düğüm çekirdek ayarı ve güvenlik sertleştirmesi

> Dil: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

Bu belge **tamamen kopyalanabilir** bloklar sunar; satır satır çalıştırmanız gerekmez. Proxy / yönlendirme düğümleri (xray, sing-box, önünde nginx/caddy) içindir.

> Uygulamadan önce açık bir SSH oturumu bırakın; kural hatası erişiminizi kesmesin. Üretimde bakım penceresinde yapın.
> Değerler genel bir başlangıçtır; RAM, bant genişliği ve eşzamanlı bağlantıya göre uyarlayın.

## 1. Çekirdek parametreleri (kalıcı)

Bloğun tamamını şuraya yazın: `/etc/sysctl.d/99-corefusion.conf`:

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

Uygula ve doğrula:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

`tcp_congestion_control` yazılırken `Protocol not available` alırsanız çekirdekte bbr modülü yoktur; önce çekirdeği yükseltin.

## Mevcut ayarlarla birlikte çalışma (hangisi kazanır)

Düğümde çoğu zaman kendi ayar dosyaları vardır; körlemesine eklemek "değiştirdim ama etkisi yok" sonucu verir. Kurallar tam tersidir:

**sysctl: en son yüklenen dosya kazanır**

Yükleme sırası

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

Bu nedenle:

- `/etc/sysctl.conf` içindeki aynı anahtarlar `/etc/sysctl.d/99-corefusion.conf` dosyasını **her zaman ezer**;
- `/etc/sysctl.d/99-sysctl.conf` (`../sysctl.conf`e sık bağlı symlink) `99-corefusion.conf` sonrasında gelir ve o da ezer;
- `10-*`, `50-*` ve daha erken dosyaları biz ezeriz.

Önce kontrol edin, sonra nereye yazacağınıza karar verin; **aynı anahtar yalnızca tek dosyada** olsun.

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

`/etc/sysctl.conf` aynı anahtarı zaten içeriyorsa iki seçenek:

1. o satırları yorum satırı yapıp `/etc/sysctl.d/99-corefusion.conf` ile yönetmek;
2. ya da `/etc/sysctl.conf` dosyasını düzenleyip buradaki kaydı kaldırmak.

**sshd: ilk eşleşme kazanır (sysctl'in tersi)**

`sshd` okuduğu ilk değeri kullanır. Drop-in dosyaları `Include` satırında, dosya adı sözlük sırasına göre işlenir. Bu yüzden:

- `00-corefusion-*.conf` öneki kullanılır, böylece drop-in'ler arasında ilk olur;
- `Include /etc/ssh/sshd_config.d/*.conf` ana yapılandırmanın üst kısmında (`PasswordAuthentication` vb. öncesinde) olmalıdır; aksi halde ana dosya kazanır ve drop-in yok sayılır;
- aynı direktifi yazan mevcut bir `00-`/`01-` dosyası varsa o kazanır.

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

Denetim bunu `CFG-01` / `CFG-02` olarak gösterir ve oluşturulan kılavuzun 0. bölümünde tekrarlar.

## Çekirdek sürüm farkları (önemli)

Her parametre 3.x'te yoktur. CentOS 7 3.10 kullanır: **BBR ve fq yok**; 4.x değerlerini kopyalamak `Invalid argument` verir:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ yok, `cubic` kalsın | OK | OK |
| `net.core.default_qdisc = fq` | ✗ `fq_codel` kullanın | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ (4.14+) | OK |
| `net.mptcp.enabled` | ✗ (5.6+) | ✗ (5.6+) | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

Denetim gerçek çekirdek sürümünü okur: 3.10'da yalnızca "çekirdek BBR sağlamıyor" der, BBR komutu üretmez; oluşturulan Markdown da BBR/fq çıkarıp `fq_codel` kullanır.

Notlar:

- **Tünel adreslerine dokunmayın**: `singbr0` / `wg0` / `tun0` (sing-box, WARP) üzerindeki IPv6 proxy tarafından atanır ve yeniden bağlanınca değişir. Düzenlemeyin, sysctl'e yazmayın; denetim bunları `tun` olarak işaretler ve düğümün genel IPv6'sına saymaz.
- **IPv6**: yalnızca düğümde gerçekten genel IPv6 varsa gerekir. `forwarding=1` konteyner ağları / şeffaf yönlendirme içindir; kullanıcı alanı proxyleri (xray/sing-box/nginx) buna bağlı değildir. Sabit adreslerde `accept_ra=1` kullanmayın; SLAAC'ta varsayılan `2` kalsın. Denetim `forwarding`, `accept_redirects`, `use_tempaddr`, komşu tablosu ve rota önbelleği sınırını (`IPV6-01` ~ `IPV6-06`) kontrol eder; IPv6 yoksa bu maddeler oluşmaz.
- `fs.file-max`, `fs.nr_open`, `vm.swappiness`, `kernel.pid_max` konteynerde genelde salt okunurdur; ana makinede ayarlayın.
- `net.ipv4.conf.all.rp_filter` proxy düğümlerinde genelde `0`; tek NIC ve asimetrik yönlendirme yoksa `1` olabilir.
- Yalnızca TCP varsa `nf_conntrack_udp_timeout` gerekmez.
- Veritabanı düğümleri `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax` ekleyebilir; proxy için gerekmez.

## 2. SSH sertleştirme

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

Notlar:

- **"Güvenlik" için root girişini kapatmayın**. `PermitRootLogin yes` kalabilir; kapatılması gereken parola girişidir. `PasswordAuthentication no` + `KbdInteractiveAuthentication no` ile root yalnızca anahtarla girer.
- Zaten `PermitRootLogin prohibit-password` varsa o da olur (yine yalnızca anahtar).
- Mevcut oturumu kapatmadan önce en az bir anahtarın çalıştığını doğrulayın.

## 3. SSH'yi güvenilir kaynaklarla sınırla

`<YOUR_IP>` yerine ofis/VPN adresinizi yazın (CIDR desteklenir). Kullandığınız güvenlik duvarının bloğunu seçin:

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

Notlar:

- Önce izin kuralı, sonra varsayılan drop; sırayı ters çevirmeyin.
- Bulut security group varsa kaynak IP'yi orada da açın.
- SSH 22 değilse komutlardaki `22` değerini değiştirin.

## 4. Geri alma

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

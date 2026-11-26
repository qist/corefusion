# تنظیم هسته و سخت‌سازی امنیتی گره

> زبان: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

این سند بلوک‌های **قابل کپی کامل** ارائه می‌دهد؛ لازم نیست خط‌به‌خط اجرا کنید. مناسب گره‌های پروکسی / فوروارد (xray، sing-box، nginx/caddy در جلو).

> پیش از اعمال، یک نشست SSH باز نگه دارید تا خطای قاعده دسترسی را قطع نکند. در محیط تولید در پنجره نگهداری انجام دهید.
> مقادیر نقطه شروع عمومی‌اند؛ بر اساس RAM، پهنای باند و تعداد اتصال تنظیم کنید.

## 1. پارامترهای هسته (پایدار)

کل بلوک را در این فایل بنویسید: `/etc/sysctl.d/99-corefusion.conf`:

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

اعمال و بررسی:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

اگر هنگام نوشتن `tcp_congestion_control` خطای `Protocol not available` دیدید، هسته ماژول bbr ندارد؛ ابتدا هسته را ارتقا دهید.

## هم‌زیستی با تنظیمات موجود (کدام غالب است)

معمولاً گره از قبل فایل‌های تنظیم خود را دارد؛ افزودن بدون بررسی به «تغییر دادم ولی اثر نکرد» می‌رسد. قاعده دو طرف برعکس است:

**sysctl: فایلی که دیرتر بارگذاری شود غالب است**

ترتیب بارگذاری

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

بنابراین:

- همان کلید در `/etc/sysctl.conf` **همیشه** `/etc/sysctl.d/99-corefusion.conf` را بازنویسی می‌کند؛
- `/etc/sysctl.d/99-sysctl.conf` (لینک رایج به `../sysctl.conf`) بعد از `99-corefusion.conf` می‌آید و آن را هم بازنویسی می‌کند؛
- فایل‌های `10-*`، `50-*` و پیش‌تر توسط ما بازنویسی می‌شوند.

اول بررسی کنید، بعد تصمیم بگیرید کجا بنویسید؛ **هر کلید فقط در یک فایل**.

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

اگر کلید در `/etc/sysctl.conf` هست، یکی را انتخاب کنید:

1. آن خطوط را کامنت کنید و از `/etc/sysctl.d/99-corefusion.conf` مدیریت کنید؛
2. یا مستقیماً `/etc/sysctl.conf` را ویرایش کنید و تکرار را اینجا بردارید.

**sshd: اولین تطابق غالب است (برعکس sysctl)**

`sshd` اولین مقدار خوانده‌شده را می‌گیرد. drop-inها در محل `Include` و به ترتیب الفبایی فایل‌ها باز می‌شوند. پس:

- از پیشوند `00-corefusion-*.conf` استفاده می‌کنیم تا میان drop-inها اول باشد؛
- `Include /etc/ssh/sshd_config.d/*.conf` باید در ابتدای فایل اصلی (پیش از `PasswordAuthentication` و مانند آن) باشد، وگرنه فایل اصلی غالب می‌شود و drop-in بی‌اثر است؛
- اگر فایل `00-`/`01-` موجود همان دستور را داشته باشد، آن غالب است.

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

بررسی این را به‌صورت `CFG-01` / `CFG-02` نشان می‌دهد و در بخش ۰ راهنمای تولیدشده تکرار می‌کند.

## تفاوت نسخه هسته (مهم)

همه پارامترها در 3.x وجود ندارند. CentOS 7 هسته 3.10 دارد: **بدون BBR و fq**؛ کپی مقادیر 4.x خطای `Invalid argument` می‌دهد:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ پشتیبانی نمی‌شود، `cubic` بماند | OK | OK |
| `net.core.default_qdisc = fq` | ✗ از `fq_codel` استفاده کنید | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ (4.14+) | OK |
| `net.mptcp.enabled` | ✗ (5.6+) | ✗ (5.6+) | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

بررسی نسخه واقعی هسته را می‌خواند: روی 3.10 فقط می‌گوید «هسته BBR ندارد»، فرمان BBR تولید نمی‌کند و Markdown هم BBR/fq را حذف و از `fq_codel` استفاده می‌کند.

یادداشت‌ها:

- **آدرس‌های تونل را دستی تغییر ندهید**: IPv6 روی `singbr0` / `wg0` / `tun0` (sing-box، WARP) توسط پروکسی تخصیص می‌یابد و با اتصال مجدد عوض می‌شود. ویرایش نکنید و در sysctl ننویسید؛ بررسی آن‌ها را `tun` علامت می‌زند و جزو IPv6 عمومی گره حساب نمی‌کند.
- **IPv6**: فقط وقتی لازم است که گره واقعاً IPv6 عمومی داشته باشد. `forwarding=1` برای شبکه کانتینر / فوروارد شفاف است؛ پروکسی‌های userland (xray/sing-box/nginx) به آن وابسته نیستند. با آدرس ثابت `accept_ra=1` نگذارید؛ در SLAAC مقدار پیش‌فرض `2` بماند. بررسی موارد `forwarding`، `accept_redirects`، `use_tempaddr`، جدول همسایه و سقف کش مسیر (`IPV6-01` ~ `IPV6-06`) را می‌بیند؛ گره بدون IPv6 این موارد را تولید نمی‌کند.
- `fs.file-max`، `fs.nr_open`، `vm.swappiness`، `kernel.pid_max` در کانتینر اغلب فقط‌خواندنی‌اند؛ روی میزبان تنظیم کنید.
- `net.ipv4.conf.all.rp_filter` در گره‌های پروکسی معمولاً `0` است؛ با یک کارت شبکه و بدون مسیریابی نامتقارن می‌توان `1` گذاشت.
- اگر فقط TCP دارید، `nf_conntrack_udp_timeout` لازم نیست.
- گره پایگاه‌داده ممکن است `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax` هم بخواهد؛ پروکسی نیازی ندارد.

## 2. سخت‌سازی SSH

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

یادداشت‌ها:

- **ورود root را به بهانه «امنیت» نبندید**. `PermitRootLogin yes` می‌تواند بماند؛ چیزی که باید بسته شود ورود با رمز است. با `PasswordAuthentication no` + `KbdInteractiveAuthentication no`، root فقط با کلید وارد می‌شود.
- اگر از قبل `PermitRootLogin prohibit-password` دارید، آن هم خوب است (فقط کلید).
- پیش از بستن نشست فعلی مطمئن شوید حداقل یک کلید کار می‌کند.

## 3. محدودکردن SSH به منابع مطمئن

`<YOUR_IP>` را با آدرس دفتر/VPN (CIDR مجاز) جایگزین کنید. بلوک مناسب فایروال فعلی را انتخاب کنید:

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

یادداشت‌ها:

- اول قاعده اجازه، بعد drop پیش‌فرض؛ ترتیب را برعکس نکنید.
- اگر security group ابری دارید، IP مبدأ را آنجا هم باز کنید.
- اگر SSH روی 22 نیست، `22` را در دستورها عوض کنید.

## 4. بازگردانی

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

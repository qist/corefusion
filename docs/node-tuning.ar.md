# ضبط النواة وتقوية أمان العقدة

> اللغة: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

يقدّم هذا المستند كتلًا **قابلة للنسخ كاملة**، دون تنفيذ سطر بسطر. مخصّص لعقد البروكسي / إعادة التوجيه (xray وsing-box وnginx/caddy أمامها).

> أبقِ جلسة SSH مفتوحة قبل التطبيق حتى لا تفقد الوصول بسبب قاعدة خاطئة. في الإنتاج نفّذ ذلك في نافذة صيانة.
> القيم نقطة بداية عامة؛ عدّلها حسب الذاكرة وعرض النطاق وعدد الاتصالات.

## 1. معلمات النواة (دائمة)

اكتب الكتلة كاملة في `/etc/sysctl.d/99-corefusion.conf`:

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

التطبيق والتحقق:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

إذا ظهر `Protocol not available` عند كتابة `tcp_congestion_control` فالنواة لا تحتوي وحدة bbr؛ حدّث النواة أولًا.

## التعايش مع الضبط الموجود (من يفوز)

كثيرًا ما تكون للعقدة ملفات ضبط خاصة بها؛ الإضافة دون فحص تؤدي إلى «غيّرت ولم يحدث أثر». القاعدتان متعاكستان:

**sysctl: الملف الذي يُحمَّل أخيرًا هو الغالب**

ترتيب التحميل

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

لذلك:

- نفس المفتاح في `/etc/sysctl.conf` **يتجاوز دائمًا** `/etc/sysctl.d/99-corefusion.conf`؛
- الملف `/etc/sysctl.d/99-sysctl.conf` (رابط شائع إلى `../sysctl.conf`) يأتي بعد `99-corefusion.conf` ويتجاوزه أيضًا؛
- الملفات `10-*` و`50-*` والأسبق تتجاوزها إعداداتنا.

افحص أولًا ثم قرّر أين تكتب؛ **مفتاح واحد في ملف واحد فقط**.

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

إذا كان المفتاح موجودًا في `/etc/sysctl.conf` فاختر أحد الأمرين:

1. تعليق تلك الأسطر وإدارته من `/etc/sysctl.d/99-corefusion.conf`؛
2. أو تعديل `/etc/sysctl.conf` مباشرة وحذف التكرار هنا.

**sshd: أول تطابق هو الغالب (عكس sysctl)**

يأخذ `sshd` أول قيمة يقرأها. تُوسَّع ملفات drop-in عند سطر `Include` وبالترتيب الأبجدي. لذلك:

- نستخدم البادئة `00-corefusion-*.conf` ليكون الأول بين drop-in؛
- يجب أن يكون `Include /etc/ssh/sshd_config.d/*.conf` قرب أعلى الملف الرئيسي (قبل `PasswordAuthentication` وما شابه)، وإلا يفوز الملف الرئيسي ويُهمل drop-in؛
- إن وُجد ملف `00-`/`01-` يعرّف نفس التوجيه فهو الغالب.

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

يُظهر الفحص ذلك كـ `CFG-01` / `CFG-02` ويكرّره في القسم 0 من الدليل المُنشأ.

## فروق إصدارات النواة (مهم)

ليست كل المعلمات موجودة في 3.x. CentOS 7 يستخدم 3.10: **بلا BBR وfq**؛ نسخ قيم 4.x يعطي `Invalid argument`:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ غير مدعوم، أبقِ `cubic` | OK | OK |
| `net.core.default_qdisc = fq` | ✗ استخدم `fq_codel` | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ (4.14+) | OK |
| `net.mptcp.enabled` | ✗ (5.6+) | ✗ (5.6+) | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

يقرأ الفحص إصدار النواة الفعلي: على 3.10 يذكر فقط «النواة لا توفّر BBR»، ولا يولّد أوامر BBR، ويحذف Markdown الناتج BBR/fq ويستخدم `fq_codel`.

ملاحظات:

- **لا تعالج عناوين النفق يدويًا**: عنوان IPv6 على `singbr0` / `wg0` / `tun0` (sing-box وWARP) يخصّصه البروكسي ويتغيّر عند إعادة الاتصال. لا تعدّله ولا تكتبه في sysctl؛ يوسمه الفحص بـ `tun` ولا يحسبه ضمن IPv6 العامة للعقدة.
- **IPv6**: يلزم فقط إذا كانت للعقدة IPv6 عامة فعلية. `forwarding=1` لشبكات الحاويات / إعادة التوجيه الشفافة؛ وكلاء userland (xray/sing-box/nginx) لا يعتمدون عليه. مع العناوين الثابتة لا تضبط `accept_ra=1`، وفي SLAAC اترك الافتراضي `2`. يفحص الفحص `forwarding` و`accept_redirects` و`use_tempaddr` وجدول الجيران وحد ذاكرة المسارات (`IPV6-01` ~ `IPV6-06`)؛ والعقد بلا IPv6 لا تُنشئ هذه العناصر.
- `fs.file-max` و`fs.nr_open` و`vm.swappiness` و`kernel.pid_max` غالبًا للقراءة فقط داخل الحاوية؛ اضبطها على المضيف.
- `net.ipv4.conf.all.rp_filter` عادة `0` على عقد البروكسي؛ ومع بطاقة واحدة بلا توجيه غير متماثل يمكن `1`.
- إن كان TCP فقط فيمكن تجاهل `nf_conntrack_udp_timeout`.
- قد تحتاج عقد قواعد البيانات `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`؛ والبروكسي لا يحتاجها.

## 2. تقوية SSH

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

ملاحظات:

- **لا تُعطّل دخول root بدعوى «الأمان»**. يمكن إبقاء `PermitRootLogin yes`؛ الذي يجب إغلاقه هو دخول كلمة المرور. مع `PasswordAuthentication no` + `KbdInteractiveAuthentication no` يدخل root بالمفتاح فقط.
- إن كان `PermitRootLogin prohibit-password` موجودًا فهو مناسب أيضًا (مفتاح فقط).
- تأكد من عمل مفتاح واحد على الأقل قبل إغلاق الجلسة الحالية.

## 3. تقييد SSH بمصادر موثوقة

استبدل `<YOUR_IP>` بعنوان المكتب/VPN (CIDR مسموح). اختر الكتلة حسب جدار الحماية المستخدم:

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

ملاحظات:

- أضف قاعدة السماح أولًا ثم الرفض الافتراضي؛ لا تعكس الترتيب.
- إن وُجد security group سحابي فاسمح بعنوان المصدر هناك أيضًا.
- إن لم يكن SSH على المنفذ 22 فاستبدل `22` في الأوامر.

## 4. التراجع

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

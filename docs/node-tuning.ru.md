# Тюнинг ядра и усиление безопасности узла

> Язык: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

Документ содержит **готовые для копирования** блоки — выполнять построчно не нужно. Для прокси-/форвардинг-узлов (xray, sing-box, nginx/caddy перед ними).

> Перед применением держите открытую SSH-сессию, чтобы ошибка в правилах не отрезала доступ. В продакшене делайте это в окно обслуживания.
> Значения — общая отправная точка; подстройте под RAM, канал и число соединений.

## 1. Параметры ядра (постоянно)

Запишите весь блок в `/etc/sysctl.d/99-corefusion.conf`:

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

Применить и проверить:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

Если при записи `tcp_congestion_control` появляется `Protocol not available`, в ядре нет модуля bbr — сначала обновите ядро.

## Сосуществование с уже имеющимся тюнингом (кто побеждает)

На узле часто уже есть свои файлы тюнинга; если просто дописать, получится «изменил, а эффекта нет». Правила противоположны:

**sysctl: побеждает последний загруженный файл**

Порядок загрузки

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

Поэтому:

- те же ключи в `/etc/sysctl.conf` **всегда перекрывают** `/etc/sysctl.d/99-corefusion.conf`;
- `/etc/sysctl.d/99-sysctl.conf` (частая ссылка на `../sysctl.conf`) идёт после `99-corefusion.conf` и тоже перекрывает;
- файлы `10-*`, `50-*` и более ранние перекрываем мы.

Сначала проверьте, потом решайте, куда писать; **каждый ключ — только в одном файле**.

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

Если ключ уже есть в `/etc/sysctl.conf`, выберите одно:

1. закомментировать те строки и управлять через `/etc/sysctl.d/99-corefusion.conf`;
2. или править `/etc/sysctl.conf`, убрав дубль здесь.

**sshd: побеждает первое совпадение (наоборот, чем sysctl)**

`sshd` берёт первое прочитанное значение. Drop-in подключаются на строке `Include` в лексикографическом порядке. Поэтому:

- используем префикс `00-corefusion-*.conf`, чтобы быть первыми среди drop-in;
- `Include /etc/ssh/sshd_config.d/*.conf` должен стоять в начале основного конфига (до `PasswordAuthentication` и т.п.), иначе победит основной файл и drop-in не подействует;
- если уже есть `00-`/`01-` с той же директивой, побеждает он.

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

Проверка показывает это как `CFG-01` / `CFG-02` и повторяет в разделе 0 сгенерированного руководства.

## Различия версий ядра (важно)

Не все параметры есть в 3.x. CentOS 7 — это 3.10: **нет BBR и fq**; копирование значений 4.x даёт `Invalid argument`:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ недоступно, оставить `cubic` | OK | OK |
| `net.core.default_qdisc = fq` | ✗ использовать `fq_codel` | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ (4.14+) | OK |
| `net.mptcp.enabled` | ✗ (5.6+) | ✗ (5.6+) | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

Проверка читает реальную версию ядра: на 3.10 только сообщает «ядро не поддерживает BBR», не генерирует команды BBR, а Markdown опускает BBR/fq и использует `fq_codel`.

Примечания:

- **Не трогайте адреса туннелей**: IPv6 на `singbr0` / `wg0` / `tun0` (sing-box, WARP) выдаёт прокси, при переподключении он меняется. Не редактируйте и не вписывайте в sysctl; проверка помечает их как `tun` и не считает публичным IPv6 узла.
- **IPv6**: нужен только если у узла есть глобальный IPv6. `forwarding=1` — для сетей контейнеров / прозрачного форвардинга; пользовательским прокси (xray/sing-box/nginx) он не нужен. При статических адресах не ставьте `accept_ra=1`, при SLAAC оставьте `2`. Проверка тогда смотрит `forwarding`, `accept_redirects`, `use_tempaddr`, таблицу соседей и лимит кеша маршрутов (`IPV6-01` ~ `IPV6-06`); без IPv6 этих пунктов нет.
- `fs.file-max`, `fs.nr_open`, `vm.swappiness`, `kernel.pid_max` в контейнере часто только для чтения — задавайте на хосте.
- `net.ipv4.conf.all.rp_filter` на прокси-узлах обычно `0`; при одной сетевой карте без асимметричной маршрутизации можно `1`.
- Если только TCP, `nf_conntrack_udp_timeout` можно не задавать.
- Узлам БД могут понадобиться `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`; прокси — нет.

## 2. Усиление SSH

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

Примечания:

- **Не отключайте вход root «ради безопасности»**. `PermitRootLogin yes` можно оставить; закрывать нужно пароль. После `PasswordAuthentication no` + `KbdInteractiveAuthentication no` root входит только по ключу.
- Если уже стоит `PermitRootLogin prohibit-password` — тоже подходит (только ключ).
- Убедитесь, что хотя бы один ключ работает, прежде чем закрывать текущую сессию.

## 3. Ограничить SSH доверенными источниками

Замените `<YOUR_IP>` на адрес офиса/VPN (можно CIDR). Выберите блок для используемого фаервола:

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

Примечания:

- Сначала разрешающее правило, затем default drop — порядок не менять.
- Если есть облачный security group, разрешите IP и там.
- Если SSH не на 22, замените `22` в командах.

## 4. Откат

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

# 노드 커널 튜닝과 보안 강화

> 언어: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

이 문서는 **통째로 복사**할 수 있는 블록을 제공합니다. 한 줄씩 실행할 필요가 없습니다. 프록시/포워딩 노드(xray, sing-box, nginx/caddy 전단)용입니다.

> 적용 전에 SSH 세션을 하나 열어 두세요. 규칙 오류로 접속이 끊기는 것을 방지합니다. 운영 환경은 점검 시간에 하세요.
> 값은 일반적인 출발점입니다. 메모리, 대역폭, 동시 연결 수에 맞게 조정하세요.

## 1. 커널 파라미터(영구 적용)

다음 블록 전체를 기록: `/etc/sysctl.d/99-corefusion.conf`:

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

적용 및 확인:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

`tcp_congestion_control` 기록 시 `Protocol not available`이 나오면 커널에 bbr 모듈이 없습니다. 먼저 커널을 업그레이드하세요.

## 기존 튜닝과의 공존(누가 우선인가)

노드에 이미 자체 튜닝 파일이 있는 경우가 많습니다. 그대로 덧붙이면 "바꿨는데 효과 없음"이 됩니다. 두 규칙은 반대입니다:

**sysctl: 나중에 로드된 파일이 우선**

로드 순서

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

따라서:

- `/etc/sysctl.conf`의 같은 key는 **항상** `/etc/sysctl.d/99-corefusion.conf`를 덮어씁니다;
- `/etc/sysctl.d/99-sysctl.conf`(`../sysctl.conf`로 향하는 흔한 심볼릭 링크)는 `99-corefusion.conf`보다 뒤라 역시 덮어씁니다;
- `10-*`, `50-*` 등 더 앞선 파일은 우리가 덮어씁니다.

먼저 확인하고 어디에 쓸지 결정하세요. **같은 key를 두 파일에 쓰지 마세요**.

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

`/etc/sysctl.conf`에 같은 key가 이미 있으면 둘 중 하나:

1. 해당 줄을 주석 처리하고 `/etc/sysctl.d/99-corefusion.conf`로 일원화;
2. 또는 `/etc/sysctl.conf`를 직접 수정하고 여기서는 제거.

**sshd: 첫 번째 일치가 우선(sysctl과 반대)**

`sshd`는 처음 읽은 값으로 확정합니다. drop-in은 `Include` 위치에서 파일명 사전순으로 펼쳐집니다. 그래서:

- `00-corefusion-*.conf` 접두사를 써서 drop-in 중 가장 먼저 오게 합니다;
- `Include /etc/ssh/sshd_config.d/*.conf`는 메인 설정 상단(`PasswordAuthentication` 등보다 앞)에 있어야 합니다. 뒤에 있으면 메인 설정이 먼저 일치해 drop-in이 무효가 됩니다;
- 이미 `00-`/`01-` 파일이 같은 지시자를 쓰고 있으면 그쪽이 우선입니다.

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

검사에서는 `CFG-01` / `CFG-02`로 표시되고, 생성 가이드 0절에도 같은 내용이 나옵니다.

## 커널 버전 차이(중요)

3.x에는 없는 파라미터가 있습니다. CentOS 7은 3.10이라 **BBR과 fq가 없습니다**. 4.x 값을 그대로 쓰면 `Invalid argument`가 됩니다:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ 미지원, `cubic` 유지 | OK | OK |
| `net.core.default_qdisc = fq` | ✗ `fq_codel` 사용 | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ (4.14+) | OK |
| `net.mptcp.enabled` | ✗ (5.6+) | ✗ (5.6+) | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

검사는 실제 커널 버전을 읽습니다. 3.10에서는 "커널이 BBR을 지원하지 않음"만 표시하고 BBR 명령을 만들지 않으며, 생성 Markdown도 BBR/fq를 빼고 `fq_codel`을 씁니다.

참고:

- **터널 주소는 직접 건드리지 마세요**: `singbr0` / `wg0` / `tun0`(sing-box, WARP)의 IPv6는 프록시가 자동 할당하며 재연결 시 바뀝니다. 편집하거나 sysctl에 넣지 마세요. 검사에서는 `tun`으로 표시하고 노드 공인 IPv6로 계산하지 않습니다.
- **IPv6**: 노드에 실제 글로벌 IPv6가 있을 때만 필요합니다. `forwarding=1`은 컨테이너 네트워크/투명 포워딩용이며, 사용자 공간 프록시(xray/sing-box/nginx)는 의존하지 않습니다. 고정 주소에서는 `accept_ra=1`을 쓰지 말고, SLAAC에서는 기본 `2`를 유지하세요. 검사는 `forwarding`, `accept_redirects`, `use_tempaddr`, 이웃 테이블, 라우트 캐시 상한(`IPV6-01` ~ `IPV6-06`)을 확인하며, IPv6가 없으면 항목이 생기지 않습니다.
- `fs.file-max`, `fs.nr_open`, `vm.swappiness`, `kernel.pid_max`는 컨테이너에서 읽기 전용일 수 있으니 호스트에서 설정하세요.
- `net.ipv4.conf.all.rp_filter`는 프록시 노드에서 보통 `0`; 단일 NIC에 비대칭 라우팅이 없으면 `1`도 가능합니다.
- TCP만 쓴다면 `nf_conntrack_udp_timeout`은 무시해도 됩니다.
- DB 노드는 `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`도 고려하지만 프록시에는 불필요합니다.

## 2. SSH 강화

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

참고:

- **"보안"을 위해 root 로그인을 끄지 마세요**. `PermitRootLogin yes`는 그대로 두어도 되고, 닫아야 할 것은 비밀번호 입구입니다. `PasswordAuthentication no` + `KbdInteractiveAuthentication no`면 root는 키로만 접속합니다.
- 이미 `PermitRootLogin prohibit-password`라면 그대로도 됩니다(역시 키 전용).
- 현재 세션을 닫기 전에 키로 로그인되는지 확인하세요.

## 3. SSH 접속 출처 제한

`<YOUR_IP>`를 사무실/VPN 주소(CIDR 가능)로 바꾸세요. 실제 사용하는 방화벽 블록을 선택:

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

주의:

- 허용 규칙을 먼저, 기본 거부를 나중에. 순서를 바꾸지 마세요.
- 클라우드 security group이 있으면 거기서도 같은 출처를 허용해야 합니다.
- SSH 포트가 22가 아니면 명령의 `22`를 바꾸세요.

## 4. 롤백

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

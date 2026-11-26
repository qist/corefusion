<!-- source: node-tuning.md | locale: en | updated: 2026-11-26 -->
# Node kernel tuning and hardening

> Language: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

Copy the whole block; no need to run commands one by one. Targets proxy / forwarding nodes
(xray, sing-box, nginx/caddy frontend).

> Keep an existing SSH session open while applying, in case a rule cuts your access.
> Prefer a maintenance window. Values are a generic starting point; adjust for RAM, bandwidth and concurrency.

## 1. Kernel parameters (persistent)

Write the whole block to `/etc/sysctl.d/99-corefusion.conf`:

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
# skip this block if the node has no IPv6 (check: ip -6 addr show scope global)
net.ipv6.conf.all.forwarding = 1
net.ipv6.conf.all.accept_redirects = 0
net.ipv6.conf.all.accept_source_route = 0
# servers should keep stable addresses
net.ipv6.conf.all.use_tempaddr = 0
# neighbour table (avoid overflow under high concurrency)
net.ipv6.neigh.default.gc_thresh1 = 1024
net.ipv6.neigh.default.gc_thresh2 = 4096
net.ipv6.neigh.default.gc_thresh3 = 8192
# route cache size (4.2+; not present on 3.10 - would fail with unknown key)
net.ipv6.route.max_size = 262144

# ---- security ----
net.ipv4.conf.all.accept_redirects = 0
net.ipv4.conf.all.send_redirects = 0
net.ipv4.conf.all.accept_source_route = 0
# 0 = loose (proxy / asymmetric routing), 1 = strict
net.ipv4.conf.all.rp_filter = 0

# ---- system (host only in containers) ----
fs.file-max = 1024000
fs.nr_open = 1048576
vm.swappiness = 10
kernel.pid_max = 4194304
```

Apply and verify:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

If writing `tcp_congestion_control` returns `Protocol not available`, the kernel has no bbr module — upgrade the kernel.

## Coexisting with existing tuning (who wins)

Nodes often already have their own tuning files. Appending blindly leads to "I changed it but nothing happened".
The two systems have **opposite** rules:

**sysctl: the last file loaded wins**

Load order is fixed:

```
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

So `/etc/sysctl.conf` always overrides `/etc/sysctl.d/99-corefusion.conf`, and so does
`/etc/sysctl.d/99-sysctl.conf` (a common symlink to `../sysctl.conf`). Files such as `10-*` / `50-*`
are overridden by ours.

Check first and keep each key in **one** file:

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control   # effective value
```

If `/etc/sysctl.conf` already sets a key, either comment those lines out and manage it from
`/etc/sysctl.d/99-corefusion.conf`, or edit `/etc/sysctl.conf` and drop the duplicate here.

**sshd: the first value wins (opposite of sysctl)**

`sshd` takes the first value it reads. Drop-ins are expanded at the `Include` line and processed in
lexical order. Therefore:

- this guide writes `00-corefusion-*.conf` so it comes first among drop-ins;
- `Include /etc/ssh/sshd_config.d/*.conf` must appear near the top of the main config (before
  `PasswordAuthentication` etc.), otherwise the main file wins and the drop-in is ignored;
- an existing `00-` / `01-` file with the same directive still takes precedence.

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

The checkup reports this as `CFG-01` / `CFG-02` and the generated guide repeats it in section 0.

## Kernel version differences (important)

Not every parameter exists on 3.x. CentOS 7 ships 3.10, which has **no BBR and no fq**; copying the 4.x block
fails with `Invalid argument`:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ not available, keep `cubic` | ✓ | ✓ |
| `net.core.default_qdisc = fq` | ✗ use `fq_codel` | ✓ | ✓ |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ | ✓ |
| `net.mptcp.enabled` | ✗ | ✗ | ✓ (5.6+) |
| `net.ipv4.tcp_notsent_lowat` | ✓ (3.6+) | ✓ | ✓ |
| `net.ipv4.tcp_fastopen` | ✓ (3.7+) | ✓ | ✓ |

The checkup reads the real kernel version: on 3.10 it only reports "kernel does not provide BBR" and never emits
BBR commands; the generated Markdown drops BBR/fq and uses `fq_codel` instead.

Notes:

- **Do not touch tunnel addresses**: IPv6 on `singbr0` / `wg0` / `tun0` (sing-box, WARP) is assigned by the proxy
  and changes on reconnect. Do not edit it or put it into sysctl; the checkup labels it `tun` and excludes it from
  the node's public IPv6.
- **IPv6**: only needed when the node actually has a global IPv6 address. `forwarding=1` is for container
  networks / transparent forwarding; userland proxies (xray/sing-box/nginx) do not depend on it. Do not set
  `accept_ra=1` with static addresses; keep the default `2` under SLAAC. When a global IPv6 address is found,
  the checkup inspects `forwarding`, `accept_redirects`, `use_tempaddr`, the neighbour table and the route cache
  limit (`IPV6-01` ~ `IPV6-06`). Nodes without IPv6 produce none of these items.
- `fs.file-max`, `fs.nr_open`, `vm.swappiness`, `kernel.pid_max` are host-wide; set them on the host, not inside a container.
- `rp_filter=0` is the safe default for proxy nodes; use `1` on single-NIC hosts without asymmetric routing.
- Skip `nf_conntrack_udp_timeout` on TCP-only nodes.
- Database hosts may also tune `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`; proxy nodes do not need them.

## 2. SSH hardening

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

Notes:

- **Do not disable root login just for "security"**. Keeping `PermitRootLogin yes` is fine; the real gate is
  password authentication. With `PasswordAuthentication no` + `KbdInteractiveAuthentication no`, root can only
  log in with a key.
- `PermitRootLogin prohibit-password` is also fine if that is what you already use (key-only either way).
- Verify a key-based login still works before closing the current session.

## 3. Restrict SSH to trusted sources

Replace `<YOUR_IP>` with your office/VPN address (CIDR allowed). Pick the block for the firewall in use.

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
# Persist: apt install iptables-persistent && netfilter-persistent save
```

**nftables**

```sh
# Adjust table/chain names to your ruleset (nft list ruleset)
nft insert rule inet filter input tcp dport 22 ip saddr <YOUR_IP> accept
nft add rule inet filter input tcp dport 22 drop
# Persist: nft list ruleset > /etc/nftables.conf && systemctl enable nftables
```

Notes:

- Add the allow rule before the default drop.
- If a cloud security group is in front, allow your source IP there as well.
- Change `22` if sshd listens on a different port.

## 4. Rollback

```sh
rm -f /etc/sysctl.d/99-corefusion.conf
sysctl --system

rm -f /etc/ssh/sshd_config.d/00-corefusion-hardening.conf
sshd -t && systemctl reload sshd
```

Remove firewall rules with the matching delete command:

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

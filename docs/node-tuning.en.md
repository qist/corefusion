# Node kernel tuning and hardening

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
sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

Notes:

- `fs.file-max`, `fs.nr_open`, `vm.swappiness`, `kernel.pid_max` are host-wide; set them on the host, not inside a container.
- `rp_filter=0` is the safe default for proxy nodes; use `1` on single-NIC hosts without asymmetric routing.
- Skip `nf_conntrack_udp_timeout` on TCP-only nodes.
- Database hosts may also tune `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`; proxy nodes do not need them.

## 2. SSH hardening

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/99-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
PermitRootLogin prohibit-password
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

Verify a key-based login still works before closing the current session.

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

rm -f /etc/ssh/sshd_config.d/99-corefusion-hardening.conf
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

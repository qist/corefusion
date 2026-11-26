<!-- source: node-tuning.md | locale: de | updated: 2026-11-26 -->
# Kernel-Tuning und Härtung für Knoten

> Sprache: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

Dieses Dokument enthält **komplett kopierbare** Blöcke; du musst nichts Zeile für Zeile ausführen. Geeignet für Proxy-/Weiterleitungsknoten (xray, sing-box, nginx/caddy davor).

> Halte vor dem Anwenden eine bestehende SSH-Sitzung offen, damit ein Regel-Fehler dich nicht aussperrt. In Produktion besser im Wartungsfenster arbeiten.
> Die Werte sind ein allgemeiner Startpunkt; passe sie an RAM, Bandbreite und Verbindungszahl an.

## 1. Kernel-Parameter (persistent)

Schreibe den ganzen Block nach `/etc/sysctl.d/99-corefusion.conf`:

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

Anwenden und prüfen:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

Meldet `tcp_congestion_control` beim Schreiben `Protocol not available`, fehlt das bbr-Modul — dann zuerst den Kernel aktualisieren.

## Zusammenarbeit mit vorhandenem Tuning (wer gewinnt)

Auf vielen Knoten existieren bereits eigene Tuning-Dateien. Blindes Anhängen führt zu "geändert, aber wirkungslos". Die Regeln sind genau umgekehrt:

**sysctl: die zuletzt geladene Datei gewinnt**

Ladereihenfolge

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

Daher:

- gleiche Keys in `/etc/sysctl.conf` **überschreiben immer** `/etc/sysctl.d/99-corefusion.conf`;
- `/etc/sysctl.d/99-sysctl.conf` (häufig ein Symlink auf `../sysctl.conf`) sortiert nach `99-corefusion.conf` und überschreibt ebenfalls;
- `10-*`, `50-*` und frühere Dateien werden von uns überschrieben.

Erst prüfen, dann entscheiden, wo geschrieben wird — **jeder Key nur in einer Datei**.

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

Steht ein Key bereits in `/etc/sysctl.conf`, gibt es zwei Wege:

1. diese Zeilen auskommentieren und zentral über `/etc/sysctl.d/99-corefusion.conf` verwalten;
2. oder direkt `/etc/sysctl.conf` bearbeiten und den Eintrag hier weglassen.

**sshd: der erste Treffer gewinnt (umgekehrt zu sysctl)**

`sshd` nimmt den ersten Wert, den es liest. Drop-ins werden an der `Include`-Zeile in lexikografischer Reihenfolge eingebunden:

- dieses Dokument nutzt `00-corefusion-*.conf`, damit es unter den Drop-ins zuerst kommt;
- `Include /etc/ssh/sshd_config.d/*.conf` muss weit oben in der Hauptkonfiguration stehen (vor `PasswordAuthentication` usw.), sonst gewinnt die Hauptdatei und das Drop-in ist wirkungslos;
- existiert bereits eine `00-`/`01-`-Datei mit derselben Direktive, gewinnt diese.

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

Die Prüfung meldet das als `CFG-01` / `CFG-02` und wiederholt es in Abschnitt 0 des generierten Leitfadens.

## Kernel-Versionsunterschiede (wichtig)

Nicht alle Parameter existieren in 3.x. CentOS 7 nutzt 3.10 — **kein BBR, kein fq**; die 4.x-Werte führen zu `Invalid argument`:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ nicht verfügbar, `cubic` behalten | OK | OK |
| `net.core.default_qdisc = fq` | ✗ `fq_codel` verwenden | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ (4.14+) | OK |
| `net.mptcp.enabled` | ✗ (5.6+) | ✗ (5.6+) | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

Die Prüfung liest die echte Kernelversion: auf 3.10 wird nur "Kernel unterstützt kein BBR" gemeldet, keine BBR-Befehle erzeugt, und das generierte Markdown lässt BBR/fq weg und nutzt `fq_codel`.

Hinweise:

- **Tunnel-Adressen nicht manuell ändern**: IPv6 auf `singbr0` / `wg0` / `tun0` (sing-box, WARP) wird vom Proxy vergeben und ändert sich beim Reconnect. Nicht editieren und nicht in sysctl schreiben; die Prüfung markiert sie als `tun` und zählt sie nicht zum öffentlichen IPv6 des Knotens.
- **IPv6**: nur nötig, wenn der Knoten wirklich globales IPv6 hat. `forwarding=1` dient Containernetzen / transparentem Forwarding; reine Userland-Proxys (xray/sing-box/nginx) brauchen es nicht. Bei statischen Adressen kein `accept_ra=1`, bei SLAAC den Standard `2` lassen. Die Prüfung kontrolliert dann `forwarding`, `accept_redirects`, `use_tempaddr`, Nachbartabelle und Route-Cache-Limit (`IPV6-01` ~ `IPV6-06`); ohne IPv6 entstehen diese Punkte nicht.
- `fs.file-max`, `fs.nr_open`, `vm.swappiness`, `kernel.pid_max` sind im Container oft read-only — auf dem Host setzen.
- `net.ipv4.conf.all.rp_filter` ist auf Proxy-Knoten meist `0`; bei einer NIC ohne asymmetrisches Routing kann `1` sinnvoll sein.
- Läuft nur TCP, kann `nf_conntrack_udp_timeout` entfallen.
- Datenbankknoten zusätzlich `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`; Proxy-Knoten nicht nötig.

## 2. SSH härten

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

Hinweise:

- **Root-Login nicht aus "Sicherheit" abschalten**. `PermitRootLogin yes` kann bleiben; entscheidend ist die Passwort-Anmeldung. Mit `PasswordAuthentication no` + `KbdInteractiveAuthentication no` kommt root nur noch per Schlüssel rein.
- Ein vorhandenes `PermitRootLogin prohibit-password` kann ebenfalls bleiben (ebenfalls nur Schlüssel).
- Erst prüfen, dass mindestens ein Schlüssel funktioniert, dann die aktuelle Sitzung schließen.

## 3. SSH auf vertrauenswürdige Quellen beschränken

`<YOUR_IP>` durch Büro-/VPN-Adresse ersetzen (CIDR erlaubt). Den Block zur tatsächlich verwendeten Firewall wählen:

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

Hinweise:

- Erst die Allow-Regel, dann das Default-Drop — Reihenfolge nicht umdrehen.
- Bei Cloud-Security-Groups muss die Quell-IP auch dort freigegeben werden.
- Läuft SSH nicht auf 22, im Befehl `22` ersetzen.

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

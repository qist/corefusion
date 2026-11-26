# Ajuste del kernel y endurecimiento del nodo

> Idioma: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

Este documento ofrece bloques **copiables completos**, sin ejecutar línea por línea. Pensado para nodos proxy / de reenvío (xray, sing-box, nginx/caddy delante).

> Mantén una sesión SSH abierta antes de aplicar, para no quedarte fuera si una regla falla. En producción, usa una ventana de mantenimiento.
> Los valores son un punto de partida; ajústalos a RAM, ancho de banda y concurrencia.

## 1. Parámetros del kernel (persistentes)

Escribe el bloque completo en `/etc/sysctl.d/99-corefusion.conf`:

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

Aplicar y verificar:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

Si al escribir `tcp_congestion_control` aparece `Protocol not available`, el kernel no tiene el módulo bbr: actualiza el kernel primero.

## Convivencia con ajustes existentes (quién gana)

Es habitual que el nodo ya tenga archivos de ajuste propios; añadir sin mirar produce "lo cambié y no pasa nada". Las reglas son opuestas:

**sysctl: gana el último archivo cargado**

Orden de carga

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

Por tanto:

- las mismas claves en `/etc/sysctl.conf` **siempre sobrescriben** `/etc/sysctl.d/99-corefusion.conf`;
- `/etc/sysctl.d/99-sysctl.conf` (enlace típico a `../sysctl.conf`) va después de `99-corefusion.conf` y también lo sobrescribe;
- los archivos `10-*`, `50-*` y anteriores son sobrescritos por nosotros.

Revisa primero y decide dónde escribir; **cada clave en un solo archivo**.

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

Si `/etc/sysctl.conf` ya define la clave, elige una opción:

1. comentar esas líneas y gestionarlo desde `/etc/sysctl.d/99-corefusion.conf`;
2. o editar `/etc/sysctl.conf` y no repetir la clave aquí.

**sshd: gana la primera coincidencia (al revés que sysctl)**

`sshd` toma el primer valor que lee. Los drop-ins se expanden en la línea `Include` y en orden lexicográfico. Por eso:

- usamos el prefijo `00-corefusion-*.conf` para ir primero entre los drop-ins;
- `Include /etc/ssh/sshd_config.d/*.conf` debe estar cerca del inicio del archivo principal (antes de `PasswordAuthentication`, etc.); si no, gana el principal y el drop-in no sirve;
- si ya existe un `00-`/`01-` con la misma directiva, gana ese.

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

La comprobación lo reporta como `CFG-01` / `CFG-02` y lo repite en la sección 0 de la guía generada.

## Diferencias por versión del kernel (importante)

No todos los parámetros existen en 3.x. CentOS 7 usa 3.10: **sin BBR ni fq**; copiar los valores de 4.x da `Invalid argument`:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ no disponible, mantener `cubic` | OK | OK |
| `net.core.default_qdisc = fq` | ✗ usar `fq_codel` | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ (4.14+) | OK |
| `net.mptcp.enabled` | ✗ (5.6+) | ✗ (5.6+) | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

La comprobación lee la versión real: en 3.10 solo informa "el kernel no admite BBR", no genera comandos BBR, y el Markdown omite BBR/fq y usa `fq_codel`.

Notas:

- **No tocar las direcciones de túnel**: la IPv6 de `singbr0` / `wg0` / `tun0` (sing-box, WARP) la asigna el proxy y cambia al reconectar. No la edites ni la pongas en sysctl; la comprobación la marca como `tun` y no la cuenta como IPv6 pública del nodo.
- **IPv6**: solo necesaria si el nodo tiene IPv6 global. `forwarding=1` sirve para redes de contenedores / reenvío transparente; los proxies en espacio de usuario (xray/sing-box/nginx) no dependen de ello. Con direcciones estáticas no pongas `accept_ra=1`; con SLAAC deja el valor `2`. La comprobación revisa `forwarding`, `accept_redirects`, `use_tempaddr`, tabla de vecinos y límite de caché de rutas (`IPV6-01` ~ `IPV6-06`); sin IPv6 no aparecen.
- `fs.file-max`, `fs.nr_open`, `vm.swappiness`, `kernel.pid_max` suelen ser de solo lectura en contenedores: configúralos en el host.
- `net.ipv4.conf.all.rp_filter` suele ser `0` en nodos proxy; con una sola NIC sin rutas asimétricas puede usarse `1`.
- Si solo hay TCP, `nf_conntrack_udp_timeout` puede omitirse.
- Los nodos de base de datos pueden añadir `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`; los proxy no lo necesitan.

## 2. Endurecer SSH

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

Notas:

- **No desactives el login de root "por seguridad"**. `PermitRootLogin yes` puede quedarse; lo que hay que cerrar es la contraseña. Con `PasswordAuthentication no` + `KbdInteractiveAuthentication no`, root solo entra con clave.
- Si ya tienes `PermitRootLogin prohibit-password`, también vale (solo clave).
- Verifica que al menos una clave funciona antes de cerrar la sesión actual.

## 3. Limitar SSH a orígenes de confianza

Sustituye `<YOUR_IP>` por tu oficina/VPN (admite CIDR). Elige el bloque del firewall en uso:

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

Notas:

- Primero la regla de permiso, después el drop por defecto; no invertir el orden.
- Si hay security group en el cloud, permite también tu IP allí.
- Si SSH no usa el 22, cambia el `22` del comando.

## 4. Revertir

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

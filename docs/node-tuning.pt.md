# Ajuste de kernel e endurecimento do nó

> Idioma: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

Este documento traz blocos **copiáveis por inteiro**, sem executar linha a linha. Para nós proxy / de encaminhamento (xray, sing-box, nginx/caddy na frente).

> Mantenha uma sessão SSH aberta antes de aplicar, para não perder o acesso se uma regra falhar. Em produção, use janela de manutenção.
> Os valores são um ponto de partida; ajuste conforme RAM, banda e concorrência.

## 1. Parâmetros do kernel (persistentes)

Grave o bloco completo em `/etc/sysctl.d/99-corefusion.conf`:

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

Aplicar e verificar:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

Se `tcp_congestion_control` retornar `Protocol not available`, o kernel não tem o módulo bbr: atualize o kernel primeiro.

## Convivência com ajustes existentes (quem vence)

É comum o nó já ter arquivos de ajuste próprios; acrescentar sem olhar gera "mudei e não surtiu efeito". As regras são opostas:

**sysctl: o último arquivo carregado vence**

Ordem de carga

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

Portanto:

- as mesmas chaves em `/etc/sysctl.conf` **sempre sobrescrevem** `/etc/sysctl.d/99-corefusion.conf`;
- `/etc/sysctl.d/99-sysctl.conf` (link comum para `../sysctl.conf`) vem depois de `99-corefusion.conf` e também sobrescreve;
- arquivos `10-*`, `50-*` e anteriores são sobrescritos por nós.

Verifique primeiro e decida onde gravar; **cada chave em apenas um arquivo**.

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

Se `/etc/sysctl.conf` já define a chave, escolha uma opção:

1. comentar essas linhas e gerenciar em `/etc/sysctl.d/99-corefusion.conf`;
2. ou editar `/etc/sysctl.conf` e remover a chave daqui.

**sshd: a primeira ocorrência vence (ao contrário do sysctl)**

`sshd` usa o primeiro valor lido. Os drop-ins são expandidos na linha `Include`, em ordem lexicográfica. Por isso:

- usamos o prefixo `00-corefusion-*.conf` para vir primeiro entre os drop-ins;
- `Include /etc/ssh/sshd_config.d/*.conf` precisa ficar no topo do arquivo principal (antes de `PasswordAuthentication` etc.), senão o principal vence e o drop-in é ignorado;
- se já existe um `00-`/`01-` com a mesma diretiva, ele vence.

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

A verificação reporta isso como `CFG-01` / `CFG-02` e repete na seção 0 do guia gerado.

## Diferenças de versão do kernel (importante)

Nem todo parâmetro existe no 3.x. O CentOS 7 usa 3.10: **sem BBR e sem fq**; copiar os valores 4.x gera `Invalid argument`:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ indisponível, manter `cubic` | OK | OK |
| `net.core.default_qdisc = fq` | ✗ usar `fq_codel` | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ (4.14+) | OK |
| `net.mptcp.enabled` | ✗ (5.6+) | ✗ (5.6+) | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

A verificação lê a versão real: no 3.10 apenas informa "o kernel não oferece BBR", não gera comandos BBR, e o Markdown omite BBR/fq usando `fq_codel`.

Notas:

- **Não mexa nos endereços de túnel**: o IPv6 de `singbr0` / `wg0` / `tun0` (sing-box, WARP) é atribuído pelo proxy e muda ao reconectar. Não edite nem coloque no sysctl; a verificação marca como `tun` e não conta como IPv6 público do nó.
- **IPv6**: só é necessário se o nó tiver IPv6 global. `forwarding=1` serve para redes de contêineres / encaminhamento transparente; proxies em espaço de usuário (xray/sing-box/nginx) não dependem disso. Com endereços estáticos não use `accept_ra=1`; em SLAAC mantenha `2`. A verificação então checa `forwarding`, `accept_redirects`, `use_tempaddr`, tabela de vizinhos e limite de cache de rotas (`IPV6-01` ~ `IPV6-06`); sem IPv6 nada aparece.
- `fs.file-max`, `fs.nr_open`, `vm.swappiness`, `kernel.pid_max` costumam ser somente leitura em contêineres: ajuste no host.
- `net.ipv4.conf.all.rp_filter` normalmente é `0` em nós proxy; com uma única NIC sem rotas assimétricas pode usar `1`.
- Se só houver TCP, `nf_conntrack_udp_timeout` pode ser ignorado.
- Nós de banco de dados podem acrescentar `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax`; proxy não precisa.

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

- **Não desative o login root "por segurança"**. `PermitRootLogin yes` pode ficar; o que deve ser fechado é a senha. Com `PasswordAuthentication no` + `KbdInteractiveAuthentication no`, root entra apenas por chave.
- Se já usa `PermitRootLogin prohibit-password`, também serve (somente chave).
- Confirme que ao menos uma chave funciona antes de fechar a sessão atual.

## 3. Restringir SSH a origens confiáveis

Troque `<YOUR_IP>` pelo endereço do escritório/VPN (CIDR permitido). Escolha o bloco do firewall em uso:

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

- Primeiro a regra de permissão, depois o drop padrão; não inverta.
- Com security group na nuvem, libere também seu IP lá.
- Se o SSH não usa a 22, troque `22` nos comandos.

## 4. Reverter

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

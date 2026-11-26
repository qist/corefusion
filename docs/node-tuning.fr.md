# Réglage du noyau et durcissement du nœud

> Langue: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

Ce document fournit des blocs **copiables en entier**, sans exécution ligne par ligne. Pour les nœuds proxy / de relais (xray, sing-box, nginx/caddy devant).

> Gardez une session SSH ouverte avant d'appliquer, pour ne pas perdre l'accès si une règle est erronée. En production, faites-le en fenêtre de maintenance.
> Les valeurs sont un point de départ ; adaptez-les à la RAM, la bande passante et la concurrence.

## 1. Paramètres du noyau (persistants)

Écrivez le bloc complet dans `/etc/sysctl.d/99-corefusion.conf`:

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

Appliquer et vérifier:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

Si `tcp_congestion_control` renvoie `Protocol not available`, le module bbr est absent : mettez d'abord le noyau à jour.

## Coexistence avec un réglage existant (qui gagne)

Le nœud possède souvent déjà ses propres fichiers de réglage ; ajouter sans regarder donne « modifié mais sans effet ». Les règles sont inverses :

**sysctl : le dernier fichier chargé gagne**

Ordre de chargement

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

Donc :

- les mêmes clés dans `/etc/sysctl.conf` **écrasent toujours** `/etc/sysctl.d/99-corefusion.conf` ;
- `/etc/sysctl.d/99-sysctl.conf` (lien fréquent vers `../sysctl.conf`) se charge après `99-corefusion.conf` et l'écrase aussi ;
- les fichiers `10-*`, `50-*` et antérieurs sont écrasés par les nôtres.

Vérifiez d'abord, puis choisissez où écrire ; **une seule occurrence par clé**.

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

Si `/etc/sysctl.conf` contient déjà la clé, deux choix :

1. commenter ces lignes et tout gérer via `/etc/sysctl.d/99-corefusion.conf` ;
2. ou modifier `/etc/sysctl.conf` et supprimer la clé ici.

**sshd : la première valeur gagne (inverse de sysctl)**

`sshd` retient la première valeur lue. Les drop-ins sont inclus à la ligne `Include`, par ordre lexicographique. Donc :

- nous utilisons le préfixe `00-corefusion-*.conf` pour passer en premier ;
- `Include /etc/ssh/sshd_config.d/*.conf` doit être en haut du fichier principal (avant `PasswordAuthentication`, etc.), sinon le principal gagne et le drop-in est ignoré ;
- si un `00-`/`01-` existant définit la même directive, c'est lui qui gagne.

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

Le contrôle le signale via `CFG-01` / `CFG-02` et le rappelle en section 0 du guide généré.

## Différences de version du noyau (important)

Tous les paramètres n'existent pas en 3.x. CentOS 7 utilise 3.10 : **pas de BBR ni de fq** ; copier les valeurs 4.x donne `Invalid argument` :

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ indisponible, garder `cubic` | OK | OK |
| `net.core.default_qdisc = fq` | ✗ utiliser `fq_codel` | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗ (4.14+) | ✗ (4.14+) | OK |
| `net.mptcp.enabled` | ✗ (5.6+) | ✗ (5.6+) | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

Le contrôle lit la version réelle : en 3.10 il indique seulement « le noyau ne fournit pas BBR », ne génère pas de commandes BBR, et le Markdown omet BBR/fq au profit de `fq_codel`.

Remarques:

- **Ne touchez pas aux adresses de tunnel** : l'IPv6 de `singbr0` / `wg0` / `tun0` (sing-box, WARP) est attribuée par le proxy et change à la reconnexion. Ne l'éditez pas et ne la mettez pas dans sysctl ; le contrôle la marque `tun` et l'exclut de l'IPv6 publique du nœud.
- **IPv6** : utile uniquement si le nœud a une IPv6 globale. `forwarding=1` sert aux réseaux de conteneurs / au transfert transparent ; les proxys en espace utilisateur (xray/sing-box/nginx) n'en dépendent pas. Avec des adresses statiques, pas de `accept_ra=1` ; en SLAAC, laissez `2`. Le contrôle examine alors `forwarding`, `accept_redirects`, `use_tempaddr`, la table des voisins et la limite du cache de routes (`IPV6-01` ~ `IPV6-06`) ; sans IPv6, rien n'apparaît.
- `fs.file-max`, `fs.nr_open`, `vm.swappiness`, `kernel.pid_max` sont souvent en lecture seule dans un conteneur : réglez-les sur l'hôte.
- `net.ipv4.conf.all.rp_filter` vaut généralement `0` sur un nœud proxy ; avec une seule NIC sans routage asymétrique, `1` est possible.
- Si seul TCP est utilisé, `nf_conntrack_udp_timeout` peut être ignoré.
- Les nœuds de base de données peuvent ajouter `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax` ; inutile pour un proxy.

## 2. Durcir SSH

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

Remarques:

- **Ne désactivez pas la connexion root « pour la sécurité »**. `PermitRootLogin yes` peut rester ; ce qu'il faut fermer, c'est le mot de passe. Avec `PasswordAuthentication no` + `KbdInteractiveAuthentication no`, root ne passe que par clé.
- Un `PermitRootLogin prohibit-password` existant convient aussi (clé uniquement).
- Vérifiez qu'au moins une clé fonctionne avant de fermer la session en cours.

## 3. Limiter SSH aux sources de confiance

Remplacez `<YOUR_IP>` par votre adresse bureau/VPN (CIDR accepté). Choisissez le bloc du pare-feu utilisé :

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

Remarques:

- D'abord la règle d'autorisation, ensuite le refus par défaut ; ne pas inverser.
- Avec un security group cloud, autorisez aussi votre IP de ce côté.
- Si SSH n'écoute pas sur 22, remplacez `22` dans les commandes.

## 4. Retour arrière

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

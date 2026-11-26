# ノードのカーネルチューニングとセキュリティ強化

> 言語: [简体中文](node-tuning.md) · [English](node-tuning.en.md) · [繁體中文](node-tuning.zh-TW.md) · [Русский](node-tuning.ru.md) · [日本語](node-tuning.ja.md) · [한국어](node-tuning.ko.md) · [Deutsch](node-tuning.de.md) · [Français](node-tuning.fr.md) · [Español](node-tuning.es.md) · [Português](node-tuning.pt.md) · [Türkçe](node-tuning.tr.md) · [Bahasa Indonesia](node-tuning.id.md) · [ไทย](node-tuning.th.md) · [Tiếng Việt](node-tuning.vi.md) · [العربية](node-tuning.ar.md) · [فارسی](node-tuning.fa.md)

本ドキュメントは**そのままコピーできる**ブロックを提供します。1 行ずつ実行する必要はありません。プロキシ / 転送ノード（xray、sing-box、nginx/caddy 前置）向けです。

> 適用前に SSH セッションを 1 つ残してください。ルールの書き間違いで切断されるのを防ぎます。本番はメンテナンス時間帯に。
> 値は一般的な出発点です。メモリ・帯域・同時接続数に合わせて調整してください。

## 1. カーネルパラメータ（永続化）

次のブロック全体を書き込みます: `/etc/sysctl.d/99-corefusion.conf`:

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

適用と確認:

```sh
# BBR is a module on most distros; load and persist it first
modprobe tcp_bbr 2>/dev/null || true
mkdir -p /etc/modules-load.d && echo tcp_bbr > /etc/modules-load.d/99-corefusion-bbr.conf

sysctl --system
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control net.netfilter.nf_conntrack_max
```

`tcp_congestion_control` の書き込みで `Protocol not available` が出る場合、カーネルに bbr モジュールがありません。先にカーネルを更新してください。

## 既存のチューニングとの共存（どちらが優先か）

ノードには既に独自のチューニングファイルがあることが多く、そのまま追記すると「変更したのに効かない」になります。両者の規則は逆です:

**sysctl: 後から読み込まれたファイルが優先**

読み込み順

```text
/etc/sysctl.d/*.conf (lexical order)
-> /run/sysctl.d/*.conf
-> /usr/local/lib/sysctl.d/*.conf
-> /usr/lib/sysctl.d/*.conf
-> /lib/sysctl.d/*.conf
-> /etc/sysctl.conf (always last)
```

したがって:

- `/etc/sysctl.conf` の同じ key は **必ず** `/etc/sysctl.d/99-corefusion.conf` を上書きします;
- `/etc/sysctl.d/99-sysctl.conf`（`../sysctl.conf` へのよくある symlink）は `99-corefusion.conf` より後なので、これも上書きします;
- `10-*`、`50-*` などより前のファイルはこちらが上書きします。

まず確認してから書き込み先を決め、**同じ key を 2 つのファイルに書かない**でください。

```sh
grep -rn '^net\.\|^fs\.\|^vm\.\|^kernel\.' /etc/sysctl.d /run/sysctl.d /usr/lib/sysctl.d /etc/sysctl.conf 2>/dev/null
sysctl net.core.somaxconn net.ipv4.tcp_congestion_control
```

`/etc/sysctl.conf` に同じ key がある場合、どちらか:

1. その行をコメントアウトし、`/etc/sysctl.d/99-corefusion.conf` に一本化する;
2. `/etc/sysctl.conf` を直接編集し、こちら側の同名項目を削除する。

**sshd: 最初に一致した値が有効（sysctl と逆）**

`sshd` は最初に読んだ値で確定します。drop-in は `Include` の位置でファイル名の辞書順に展開されます。よって:

- 本ドキュメントは `00-corefusion-*.conf` を使い、drop-in の中で最優先にします;
- `Include /etc/ssh/sshd_config.d/*.conf` はメイン設定の前方（`PasswordAuthentication` などより前）に必要です。後ろだとメイン設定が先に一致し、drop-in は無効になります;
- 既に `00-`/`01-` が同じディレクティブを書いている場合はそちらが優先です。

```sh
grep -n '^Include' /etc/ssh/sshd_config
grep -rn -E '^(PasswordAuthentication|KbdInteractiveAuthentication|PermitRootLogin|MaxAuthTries)' /etc/ssh/sshd_config /etc/ssh/sshd_config.d 2>/dev/null
sshd -T | grep -Ei '^(passwordauthentication|kbdinteractiveauthentication|permitrootlogin|maxauthtries) '
```

チェックでは `CFG-01` / `CFG-02` として表示され、生成ガイドの第 0 節にも同じ内容が出ます。

## カーネルバージョン差異（重要）

3.x には存在しないパラメータがあります。CentOS 7 は 3.10 で **BBR も fq もありません**。4.x の値をそのまま書くと `Invalid argument` になります:

| Parameter | 3.x (CentOS 7 / 3.10) | 4.9+ | 4.13+ / 5.6+ |
|---|---|---|---|
| `net.ipv4.tcp_congestion_control = bbr` | ✗ 非対応、`cubic` のまま | OK | OK |
| `net.core.default_qdisc = fq` | ✗ `fq_codel` を使用 | OK | OK |
| `net.ipv4.tcp_migrate_req` | ✗（4.14+） | ✗（4.14+） | OK |
| `net.mptcp.enabled` | ✗（5.6+） | ✗（5.6+） | OK |
| `net.ipv4.tcp_notsent_lowat` | OK (3.6+) | OK | OK |
| `net.ipv4.tcp_fastopen` | OK (3.7+) | OK | OK |

チェックは実際のカーネル版本を読みます。3.10 では「カーネルが BBR 非対応」とだけ表示し、BBR コマンドは生成しません。生成 Markdown も BBR/fq を省き `fq_codel` を使います。

補足:

- **トンネルのアドレスは手動で触らない**: `singbr0` / `wg0` / `tun0`（sing-box、WARP）の IPv6 はプロキシが自動割当し、再接続で変わります。編集も sysctl への記述も不要です。チェックでは `tun` と表示し、ノードの公開 IPv6 には数えません。
- **IPv6**: ノードにグローバル IPv6 がある場合のみ必要です。`forwarding=1` はコンテナネットワーク / 透過転送向けで、ユーザーランドのプロキシ（xray/sing-box/nginx）は依存しません。固定アドレスでは `accept_ra=1` にせず、SLAAC では既定の `2` のままにします。チェックは `forwarding`、`accept_redirects`、`use_tempaddr`、近隣テーブル、ルートキャッシュ上限（`IPV6-01` ~ `IPV6-06`）を確認し、IPv6 が無ければ項目は出ません。
- `fs.file-max`、`fs.nr_open`、`vm.swappiness`、`kernel.pid_max` はコンテナ内で読み取り専用のことが多いため、ホスト側で設定します。
- `net.ipv4.conf.all.rp_filter` はプロキシノードでは通常 `0`。単一 NIC で非対称ルーティングが無ければ `1` でも可。
- TCP のみなら `nf_conntrack_udp_timeout` は不要です。
- DB ノードは `kernel.shmmax` / `kernel.shmall` / `kernel.msgmax` も検討しますが、プロキシには不要です。

## 2. SSH の強化

```sh
mkdir -p /etc/ssh/sshd_config.d
cat > /etc/ssh/sshd_config.d/00-corefusion-hardening.conf <<'EOF'
PasswordAuthentication no
KbdInteractiveAuthentication no
MaxAuthTries 4
EOF
sshd -t && systemctl reload sshd
```

補足:

- **「安全のため」に root ログインを無効化しないでください**。`PermitRootLogin yes` のままで問題なく、閉じるべきはパスワード入口です。`PasswordAuthentication no` + `KbdInteractiveAuthentication no` にすれば root は鍵のみになります。
- 既に `PermitRootLogin prohibit-password` ならそれも可（同じく鍵のみ）。
- 現在のセッションを閉じる前に、鍵でログインできることを確認してください。

## 3. SSH の接続元を制限

`<YOUR_IP>` を社内/VPN のアドレス（CIDR 可）に置き換えます。実際に使うファイアウォールのブロックを選んでください:

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

注意:

- 許可ルールを先に、既定拒否を後に。順序を逆にしないでください。
- クラウドの security group がある場合、そちらでも同じ送信元を許可します。
- SSH ポートが 22 でなければ、コマンド内の `22` を置き換えます。

## 4. ロールバック

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

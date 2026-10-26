<!-- source: USER-GUIDE.md | locale: pt | updated: 2026-10-26 -->
# Guia de operações CoreFusion

> Idiomas: [简体中文](USER-GUIDE.md) · [English](USER-GUIDE.en.md) · [繁體中文](USER-GUIDE.zh-TW.md) · [Русский](USER-GUIDE.ru.md) · [日本語](USER-GUIDE.ja.md) · [한국어](USER-GUIDE.ko.md) · [Deutsch](USER-GUIDE.de.md) · [Français](USER-GUIDE.fr.md) · [Español](USER-GUIDE.es.md) · Português · [Türkçe](USER-GUIDE.tr.md) · [Bahasa Indonesia](USER-GUIDE.id.md) · [ไทย](USER-GUIDE.th.md) · [Tiếng Việt](USER-GUIDE.vi.md) · [العربية](USER-GUIDE.ar.md) · [فارسی](USER-GUIDE.fa.md)

> Este guia é para usuários do CoreFusion e cobre todo o fluxo de trabalho, desde a instalação até as operações diárias.

---

## Conteúdo

1. [Visão geral do sistema](#1-visão-geral-do-sistema)
2. [Instalação](#2-instalação)
3. [Configuração inicial](#3-configuração-inicial)
4. [Painel](#4-painel)
5. [Gerenciamento de nós](#5-gerenciamento-de-nós)
6. [Centro de protocolo](#6-centro-de-protocolo)
7. [Gerenciamento de proxy](#7-gerenciamento-de-proxy)
8. [Recursos de roteamento](#8-recursos-de-roteamento)
9. [Central de assinaturas](#9-central-de-assinaturas)
10. [Gestão de clientes](#10-gestão-de-clientes)
11. [Gestão do plano](#11-gestão-do-plano)
12. [Gerenciamento de recarga](#12-gerenciamento-de-recarga)
13. [Gerenciamento de ingressos](#13-gerenciamento-de-ingressos)
14. [Gerenciamento de certificados](#14-gerenciamento-de-certificados)
15. [Cotas de tráfego](#15-cotas-de-tráfego)
16. [Proxies encadeados](#16-proxies-encadeados)
17. [Usuários on-line](#17-usuários-on-line)
18. [Gerenciamento de usuários](#18-gerenciamento-de-usuários)
19. [Site do portal](#19-site-do-portal)
20. [Configurações do sistema](#20-configurações-do-sistema)
21. [Consola de operações](#21-consola-de-operações)
22. [Monitoramento de banco de dados](#22-monitoramento-de-banco-de-dados)
23. [Registro de auditoria](#23-registro-de-auditoria)
24. [Gerenciamento de registros](#24-gerenciamento-de-registros)
25. [Extensões de operações](#25-extensões-de-operações)
26. [Perguntas frequentes](#26-perguntas-frequentes)
- [Apêndice](#apêndice)

---

## 1. Visão geral do sistema

CoreFusion é um painel de gerenciamento de proxy multiprotocolo que suporta VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC e muito mais, com gerenciamento de nós, entrega de assinatura, gerenciamento de clientes, faturamento, certificados e proxies encadeados.

### 1.1 Recursos principais

| Módulo | Descrição |
|--------|-------------|
| Multiprotocolo | VLESS, VMess, Trojan, Shadowsocks, Hysteria2, TUIC, WireGuard, etc. |
| Multi-núcleo | Xray-core e sing-box (Hysteria2/TUIC e outros protocolos QUIC executados em sing-box) |
| Vários nós | Cinco tipos de nós: autônomo, espelho, ativo/em espera, entrada encadeada, saída encadeada |
| Entrega de assinatura | Vários formatos de assinatura, endpoints e gerenciamento de formatos |
| Faturamento | Modelos de planos, cartões de recarga, saldo, pedidos, comissão de revendedor |
| Certificados | Emissão Let's Encrypt, validação DNS-01/HTTP-01, distribuição de certificados |
| Proxy encadeado | Balanceamento de carga de múltiplas saídas, registro de nó externo, roteamento encadeado |
| Cotas de tráfego | Monitoramento de tráfego mensal por nó, suspensão automática e redefinição de logs |
| Portal | Editor visual de portal, modelos multilíngues e personalizados |
| Notificações | Email e Telegram, modelos e transmissões |

### 1.2 Arquitetura

- **Back-end**: Vá, gRPC
- **Frontend**: Vue 3 + Element Plus
- **Banco de dados**: SQLite (padrão) / MySQL / PostgreSQL
- **Proxy**: proxy reverso Caddy/Nginx
- **Porta do painel**: atribuída pelo assistente de configuração (porta livre aleatória por padrão; 8080 é uma escolha comum)
- **Porta gRPC do agente**: 18443 por padrão

---

## 2. Instalação

### 2.1 Ambientes suportados

| Artigo | Suporte |
|------|---------|
| Distribuições | Debian/Ubuntu, RHEL/CentOS/Rocky/AlmaLinux/Fedora, Alpine, Arch, openSUSE |
| Arquiteturas de CPU | Binários oficiais: `amd64`, `arm64`; compilações de origem também suportam `armv7`, `armv6`, `386`, `ppc64le`, `s390x`, `riscv64`, `loong64` |
| Gerente de serviços | systemd (recomendado); volta para `nohup` em contêineres ou outros ambientes sem systemd |
| Memória / disco | ≥ 1 GB / ≥ 10 GB (banco de dados, certificados, logs e binários principais residem no diretório de dados) |
| Porta do painel | escolhido no assistente de configuração (porta livre aleatória por padrão; `8080` e similares são adequados) |
| Agente gRPC | `18443` por padrão |

Os comandos externos de tempo de execução são instalados automaticamente pelo instalador (`scripts/install.sh` e o script de instalação do Agente gerado pelo painel; pule com `--no-deps` / `CF_SKIP_DEPS=1`):

| Objetivo | Pacote |
|---------|---------|
| Firewall/limite de taxa/marcação de simultaneidade | `nftables` (`nft`) |
| Modelagem de tráfego, links e soquetes | `iproute2` (`tc` / `ip` / `ss`) |
| Fallback quando o NFT não está disponível | `iptables` (`iptables-nft` em EL8+, `iptables` em EL7) |
| Rastreamento de conexão | `conntrack-tools` (`conntrack`) |
| Carregar `nf_conntrack`/`ifb` | `kmod` (`modprobe` / `lsmod`) |
| Detecção e limpeza de processos | `procps` (`pgrep` / `pkill`) |
| Verificações de porta/parâmetros DH | `lsof`/`openssl` |
| Criação de usuário proxy | `shadow-utils` (Debian `passwd`, Alpine/Arch `shadow`) |
| Downloads e arquivos | `curl`/`ca-certificates`/`tar`/`gzip`/`xz` |

> A falta de `nft`/`tc` afeta apenas a limitação de taxa e recursos de firewall; o painel ainda inicia.
> Execute `--cpu-info` primeiro para ver como o ambiente é detectado e quais pacotes estão faltando.

### 2.2 Instalação unifilar (recomendado)

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh -o install.sh
sudo bash install.sh
```

Ou canalize diretamente (sem interação, sem sinalizadores extras):

```bash
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash
```

O roteiro irá:

1. Detecte a distribuição, a arquitetura da CPU e a libc e, em seguida, instale pacotes básicos e dependências de tempo de execução;
2. Baixe `corefusion-linux-<arch>` das versões GitHub e verifique-o com o `.sha256` correspondente;
3. Instale em `/opt/corefusion` e crie `data/{sqlite,certs,logs,runtime}`;
4. Escreva apenas um espaço reservado `.env` com `SAFE_PATHS=data` — sem configuração de banco de dados — para que a primeira inicialização entre no assistente de configuração (o `.env` completo é gerado pelo assistente);
5. Instale e inicie um serviço systemd (ou execute em `nohup` quando systemd não estiver disponível);
6. Aguarde até estar pronto e imprima a configuração URL, token e caminhos de diretório.

Exemplo de saída:

```text
==================================================================
 CoreFusion installation complete
------------------------------------------------------------------
 Install dir : /opt/corefusion
 Data dir    : /opt/corefusion/data
 Binary      : /opt/corefusion/corefusion
 Logs        : journalctl -u corefusion -f
------------------------------------------------------------------
 Setup wizard : http://<server-ip>:<random-port>/init?init_token=<token>
 Setup token  : <token>
 Valid for    : 2 hours (see startup log)
 Note         : the token is shown once; after setup this port closes and the panel switches to its normal port.
==================================================================
```

Se o painel escutar apenas em `127.0.0.1` (por exemplo, atrás de um proxy reverso), o script imprimirá um comando de túnel SSH em vez de um URL inacessível.

> Perdeu o token? Leia o log de inicialização novamente: `journalctl -u corefusion -n 50 --no-pager`
> (sem systemd: `tail -n 50 /opt/corefusion/data/logs/panel.log`).

### 2.2.1 Saída multilíngue

O instalador inclui quatro idiomas de interface: chinês simplificado, inglês, persa (فارسی) e russo (Русский). Ele detecta automaticamente o `LANG` do sistema ou você pode escolher explicitamente:

```bash
# Inglês
sudo bash install.sh --lang en

# Persa via variável de ambiente
sudo CF_LANG=fa bash install.sh

# Passar opções em uma instalação por pipe
curl -fsSL https://raw.githubusercontent.com/qist/corefusion/main/scripts/install.sh | sudo bash -s -- --lang ru
```

Valores suportados: `zh` / `en` / `fa` / `ru`; `--lang=ru` também é aceito. Um valor explícito não suportado falha com a lista de idiomas disponíveis.

### 2.3 Fixação de versão/espelhos/instalação offline

```bash
# Pin a version
sudo bash install.sh --version v0.13.0-alpha.2

# GitHub download mirror (used as a URL prefix)
sudo bash install.sh --mirror https://ghproxy.net --version latest

# Use a local binary (offline / intranet / pre-built)
sudo bash install.sh --local ./dist/corefusion-linux-arm64

# Explicit target architecture
sudo bash install.sh --arch arm64 --local ./corefusion-linux-arm64
```

> Sem `--version`, o script usa a entrada mais recente na lista de versões GitHub.
> A maioria dos lançamentos são pré-lançamentos, portanto o endpoint `/releases/latest` não é usado.
>
> Os lançamentos oficiais fornecem apenas `amd64`/`arm64`; use `--source` ou um binário compilado cruzadamente com `--local` para outras arquiteturas.

### 2.4 Construindo a partir da fonte

```bash
sudo bash install.sh --source --version v0.13.0-alpha.2
```

`--source` clona o repositório (ou usa o checkout existente quando o script está dentro dele) e executa `make panel`. As versões do conjunto de ferramentas são escolhidas por ambiente:

| Dependência | Seleção |
|------------|-----------|
| Vá | Leia de `toolchain`/`go` em `go.mod` (atualmente **Go 1.27+**); se a versão do sistema for muito antiga, o conjunto de ferramentas correspondente será baixado em `<install-dir>/.toolchain` |
| Nó | **Nó 22 LTS** por padrão (Vite requer ≥ 18); uma versão oficial correspondente é baixada quando necessário |
| Nó (glibc <2,28) | Usa a compilação `unofficial-builds` glibc-217 (por exemplo, CentOS 7) |
| Nó (Alpino / Musl) | Usa a distribuição `nodejs` / `npm` |

Substituições:

```bash
# Force Go / Node versions
sudo CF_GO_VERSION=1.27.5 CF_NODE_MAJOR=22 bash install.sh --source

# Go toolchain mirror (default https://go.dev/dl)
sudo CF_GO_MIRROR=https://golang.google.cn/dl bash install.sh --source
```

Construção manual (desenvolvimento/depuração):

```bash
git clone https://github.com/qist/corefusion.git
cd corefusion

# Frontend + panel for the current architecture; the frontend is embedded via go:embed
make panel

# Cross-compile
make panel-linux-amd64
make panel-linux-arm64
```

> `go build ./...` não produz um binário. Construa o painel com
> `go build -o corefusion ./cmd/corefusion/` ou `make panel`.

### 2.5 Sinalizadores do instalador

| Bandeira | Descrição |
|------|-------------|
| `-d, --dir <PATH>` | Diretório de instalação, padrão `/opt/corefusion` |
| `-v, --version <TAG>` | Versão do painel, por ex. `v0.13.0-alpha.2`; padrão `latest` |
| `--binary` | Use o binário da versão GitHub (padrão) |
| `--source` | Construa a partir do código-fonte e prepare Go/Node correspondente |
| `--local <FILE>` | Use um binário local (instalação offline) |
| `--arch <ARCH>` | Arquitetura de destino: `amd64` / `arm64` / `armv7` / `armv6` / `386` / `ppc64le` / `s390x` / `riscv64` / `loong64` |
| `--mirror <URL>` | Prefixo de download GitHub, por ex. `https://ghproxy.net` |
| `--repo <URL>` | Repositório de origem, padrão `https://github.com/qist/corefusion.git` |
| `--ref <REF>` | Ramo/tag de origem; o padrão é `--version`, caso contrário, `main` |
| `--no-deps` | Ignorar a instalação da dependência do sistema |
| `--no-start` | Instale e grave o arquivo de serviço sem iniciar |
| `--cpu-info` | Imprima a detecção do ambiente e as dependências ausentes e saia |
| `--lang <LANG>` | Idioma de saída: `zh` / `en` / `fa` / `ru` (padrão: `LANG` do sistema) |
| `--uninstall` | Pare o serviço e remova o programa (mantém os dados e `.env`) |
| `--purge` | Com `--uninstall`, remova também o diretório de dados e `.env` |

Variáveis de ambiente relacionadas:

| Variável | Objetivo |
|----------|---------|
| `CF_GO_VERSION` | Substitua a versão Go necessária para compilações de origem |
| `CF_NODE_MAJOR` | Substituir a versão principal do Node (padrão 22) |
| `CF_GO_MIRROR` | Espelho de download do conjunto de ferramentas Go |
| `CF_SKIP_DEPS` | Defina como `1` no script de instalação do agente para ignorar a instalação de dependência |
| `CF_LANG` | Substituir o idioma de saída do instalador (equivalente a `--lang`) |

### 2.6 Layout do diretório

```text
/opt/corefusion/
├── corefusion                 # panel binary
├── corefusion.bak             # previous version (when upgrading)
├── .env                       # environment file (0600, generated by the setup wizard)
├── .toolchain/                # Go / Node downloaded for source builds
└── data/                      # data directory (SAFE_PATHS)
    ├── sqlite/app.db          # default SQLite database
    ├── certs/                 # certificates and keys
    ├── logs/                  # runtime logs (without systemd)
    ├── runtime/               # core runtime config and state
    ├── binaries/              # downloaded xray / sing-box cores
    └── proxy/                 # proxy (Caddy/Nginx) sites and config
```

> O instalador cria apenas `sqlite/`, `certs/`, `logs/` e `runtime/`; `binaries/`, `proxy/`, `rules/` e outros são criados pelo painel ou quando os núcleos são instalados.
> O diretório de dados vem de `SAFE_PATHS` em `.env` e pode ser um caminho absoluto. A alteração do diretório de instalação não move os dados existentes.

### 2.7 Gestão de serviços

systemd (maioria das distribuições):

```bash
systemctl status corefusion --no-pager
systemctl restart corefusion
systemctl stop corefusion
journalctl -u corefusion -f          # follow logs
journalctl -u corefusion -n 100      # last 100 lines
```

Sem systemd (contêineres, etc.):

```bash
cd /opt/corefusion && ./corefusion            # foreground (debugging)
cd /opt/corefusion && setsid nohup ./corefusion >> data/logs/panel.log 2>&1 &
tail -f data/logs/panel.log
```

> Sem systemd o processo não inicia na inicialização; reinicie-o manualmente após uma reinicialização (ou execute novamente o instalador).

### 2.8 Atualizar e desinstalar

Atualização (mantém dados e configuração):

```bash
sudo bash install.sh --version <new-version>
```

O script interrompe o serviço, faz backup do binário antigo como `corefusion.bak`, substitui-o e reinicia. Os `.env` e `data/` existentes permanecem intactos.

Desinstalar:

```bash
# Remove the program only; keep data and .env
sudo bash install.sh --uninstall

# Also remove the data directory and .env (irreversible)
sudo bash install.sh --uninstall --purge
```

### 2.9 Arquivo de ambiente (.env)

O `.env` completo é **gerado pelo assistente de configuração** quando a configuração é salva (DSN do banco de dados, diretório de dados, segredos, portas, domínios).

Em uma nova instalação, o instalador grava apenas este espaço reservado, para que o painel não confunda o `data/` existente com uma instalação inicializada e pule o assistente:

```env
SAFE_PATHS=data
```

> Quando já existe uma configuração completa (`.env` com `CF_DB_DSN`), o instalador nunca a substitui e o assistente não aparece novamente.
> Edições manuais posteriores em `.env` entram em vigor após uma reinicialização.

#### Diretório de dados

```env
SAFE_PATHS=data                       # relative to the install directory
# SAFE_PATHS=/srv/corefusion-data     # or an absolute path
```

#### Banco de dados

`CF_DB_DSN` é detectado pelo prefixo:

| Prefixo | Banco de dados |
|--------|----------|
| não definido ou um caminho vazio | SQLite (padrão) |
| `sqlite://` | SQLite (explícito) |
| `mysql://` | MySQL |
| `postgres://`/`postgresql://` | PostgreSQL |

`CF_DB_TYPE` é opcional e substitui a detecção (`sqlite` / `mysql` / `postgres`).

> **Senhas criptografadas**: DSNs MySQL/PostgreSQL gravados pelo painel (assistente de configuração/switch de banco de dados)
> são armazenados em `.env` como `enc:...` (AES-256-GCM, chave `CF_SECRET_ENCRYPTION_KEY`).
> Eles são descriptografados na leitura; DSNs de texto simples escritos à mão ainda funcionam.

**SQLite (padrão)**

```env
# Bare path (detected as SQLite)
CF_DB_DSN=data/sqlite/app.db

# Or explicit sqlite:// prefix
CF_DB_DSN=sqlite:///opt/corefusion/data/sqlite/app.db
```

**MySQL**

```env
# Format: mysql://user:password@tcp(host:port)/dbname?params
CF_DB_DSN=mysql://corefusion:password@tcp(127.0.0.1:3306)/corefusion?charset=utf8mb4&parseTime=True&loc=Local
```

**PostgreSQL**

```env
# Format: postgres://user:password@host:port/dbname?params
CF_DB_DSN=postgres://corefusion:password@127.0.0.1:5432/corefusion?sslmode=disable
```

#### Conjunto de conexões (opcional, MySQL/PostgreSQL)

```env
CF_DB_MAX_OPEN_CONNS=100
CF_DB_MAX_IDLE_CONNS=10
```

#### Outras variáveis comuns

| Variável | Descrição |
|----------|-------------|
| `JWT_SECRET` | Chave de assinatura para tokens de login; gerado na primeira partida quando falta |
| `CF_SECRET_ENCRYPTION_KEY` | Chave para criptografar configurações confidenciais (AES-256-GCM); gerado quando falta |
| `CF_INITIAL_ADMIN_PASSWORD` | Senha de administrador inicial (opcional; aleatória e impressa uma vez por padrão) |
| `CF_NETWORK_TRAFFIC_ACCOUNTING` | Defina como `0` para desabilitar a contabilidade de tráfego baseada em iptables (habilitada por padrão) |
| `CF_NETWORK_TRAFFIC_TABLE` | tabela iptables usada para contabilidade de tráfego, padrão `corefusion` |

> Proxies reversos confiáveis para resolver IPs de clientes reais não são mais configurados por meio de variáveis de ambiente;
> eles são armazenados como `panel.trusted_proxies` em **Configurações → Rede**.

> `.env` contém segredos; o painel restringe suas permissões para `0600` na inicialização. Nunca cometa isso.

### 2.10 Docker (opcional, crie sua própria imagem)

O repositório **não** envia um Dockerfile. Se você o colocar em contêiner, escreva sua própria imagem. Invocação de referência:

```bash
docker run -d \
  --name corefusion \
  -p 8080:8080 \
  -p 18443:18443 \
  -v /opt/corefusion/data:/app/data \
  -v /opt/corefusion/.env:/app/.env \
  <your-image>
```

> Os contêineres geralmente não têm privilégios systemd e `nft`, portanto, os recursos de limitação de taxa/firewall não funcionarão.
> O diretório de trabalho deve corresponder à base do `SAFE_PATHS` relativo.

### 2.11 Verificações pós-instalação

```bash
# Environment and dependencies
sudo bash install.sh --cpu-info

# Service state
systemctl is-active corefusion

# Startup log (contains the setup URL and token)
journalctl -u corefusion -n 50 --no-pager

# Listening ports
ss -lntp | grep corefusion
```

---

## 3. Configuração inicial

Na primeira inicialização, se `.env` não tiver um `CF_DB_DSN` válido, o painel ainda **não** atende a UI do administrador. Começa o
**assistente de configuração** em uma porta aleatória em 18000–18999 com um token único e, em seguida, reinicia como o painel normal depois de salvo.

### 3.1 Abra o assistente de configuração

1. Copie o “assistente de configuração” URL da saída do instalador; ou
2. Leia o log no servidor:

```bash
journalctl -u corefusion -n 50 --no-pager | grep -A2 'setup wizard'
```

```text
CoreFusion first start - setup wizard
  URL   : http://<ip>:<port>/init?init_token=<token>
  Token : <token>
  Valid : 2h0m0s
```

3. Abra o URL em um navegador (ele já contém o `init_token`, portanto, nenhuma entrada manual é necessária).

> A sessão do assistente expira após **2 horas** por padrão. Se expirar ou o painel reiniciar, obtenha um novo.
> Não compartilhe o token. Após a conclusão da configuração, esta porta fecha e apenas a porta normal do painel permanece.

### 3.2 Etapas do assistente

O assistente possui 8 etapas. Você pode voltar a qualquer momento; nada é confirmado até o salvamento final.

| Passo | Conteúdo |
|------|---------|
| 1. Banco de dados | SQLite/MySQL/PostgreSQL |
| 2. Diretório de dados | Escrito para `SAFE_PATHS` em `.env` |
| 3. Chaves de criptografia | `JWT_SECRET` e `CF_SECRET_ENCRYPTION_KEY` |
| 4. Núcleos | Quais núcleos de proxy instalar |
| 5. Procuração | Nenhum / Nginx / Caddy |
| 6. Sites e domínios | Portas, caminho da web, conta de administrador, moeda, fuso horário, domínios |
| 7. Certificado HTTPS | Desativado/carregar/ACME |
| 8. Resumo | Revise, baixe `.env`, salve e reinicie |

#### Etapa 1: banco de dados

- **SQLite (arquivo local, recomendado)**: deixe o caminho vazio para usar `<data-dir>/sqlite/app.db`.
- **MySQL / PostgreSQL**: host, porta, usuário, senha, nome do banco de dados; PostgreSQL também suporta `disable`/`require`/`verify-full`.
- Clique em “Testar conexão” para validar. Salvar também testa isso. Um banco de dados ausente é criado automaticamente (crie-o manualmente se a conta não tiver `CREATE`).
- O DSN (incluindo a senha) é criptografado antes de ser gravado em `.env`.

#### Etapa 2: diretório de dados

Armazena banco de dados, certificados, logs, binários principais e muito mais; este é `SAFE_PATHS` em `.env`. O padrão é `data`
(relativo ao diretório de instalação); um caminho absoluto é permitido. **Não altere casualmente mais tarde**, ou o painel
não encontrará o banco de dados e os certificados existentes.

#### Etapa 3: chaves de criptografia

- `JWT_SECRET`: assina tokens de login.
- `CF_SECRET_ENCRYPTION_KEY`: criptografa configurações confidenciais (senha do banco de dados, credenciais DNS, cartões de recarga, etc.).
- Ambos são gerados aleatoriamente por padrão; use “Regenerar” para obter novos valores. **Alterar `CF_SECRET_ENCRYPTION_KEY` torna os dados criptografados anteriormente ilegíveis** — faça backup deles.

#### Etapa 4: núcleos

Selecione os núcleos de proxy a serem instalados:

- **Xray**: VLESS/VMess/Trojan/Shadowsocks, etc.;
- **sing-box**: carrega Hysteria2/TUIC e outros protocolos QUIC.

Depois de salvar e reiniciar o painel baixa e instala-os automaticamente. Verifique as versões e o status em “Protocolos”. Você também pode instalá-los mais tarde.

#### Etapa 5: proxy

- **Nenhum**: o painel atende os clientes diretamente (manuseie TLS você mesmo).
- **Nginx / Caddy**: o painel gerencia o proxy reverso e a hospedagem de certificados; escolha um.

Com um proxy selecionado, o painel normalmente escuta apenas em `127.0.0.1` e o proxy atende o mundo externo.

#### Etapa 6: Sites e domínios

| Campo | Descrição |
|-------|-------------|
| Endereço de escuta | Derivado automaticamente: proxy → `127.0.0.1`; nenhum proxy com certificado → `0.0.0.0`; caso contrário → `127.0.0.1` |
| Porta do painel | Porta livre aleatória por padrão; altere se quiser (por exemplo, 8080) |
| Porta de assinatura | Opcional; `0` compartilha a porta do painel |
| Porta do portal | Opcional; `0` compartilha a porta do painel |
| Caminho da web | Caminho da UI Admin, padrão `/`; o API é sempre `/api/v1` |
| Nome de usuário/senha de administrador | Aleatório por padrão; **a senha é mostrada apenas uma vez no resumo** — salve-a |
| Modo de acesso | “Domínio” ou “IP” |
| Moeda/fuso horário padrão | Moeda padrão para cartões de recarga e saldos; painel e fuso horário do log do proxy (por exemplo, `Asia/Shanghai`) |
| Adicionar ao proxy reverso | Quando um proxy e domínios são definidos, adicione domínios de painel/assinatura/portal ao proxy de uma só vez |

Inserir domínios grava as configurações correspondentes (domínio de retorno de chamada, ponto final de assinatura, domínio de compartilhamento).

#### Etapa 7: certificado HTTPS

| Fonte | Descrição |
|--------|-------------|
| Desativar HTTPS | Painel/portal/assinaturas são atendidas por HTTP (inseguro); com um proxy reverso você pode adicionar TLS no lado do proxy posteriormente |
| Carregar certificado | Cole o certificado PEM e a chave privada PEM (somente modo de acesso de domínio) |
| ACME | Emite o certificado quando você envia esta etapa; suporta certificados de domínio e IP |

Opções ACME:

- **CA**: Vamos criptografar, vamos criptografar (preparação), ZeroSSL, Google CA pública;
- **Destino**: o domínio configurado ou o IP público (certificados IP são suportados apenas pelo Let's Encrypt);
- **Desafio**: `HTTP-01` (requer porta 80) ou `DNS-01` (curingas, sem porta 80; escolha um provedor DNS e insira as credenciais);
- **Tipo/tamanho de chave**: ECC 256/384 ou RSA 2048/3072/4096;
- **CA pública Google**: requer EAB; insira o Key ID/HMAC manualmente ou use uma conta de serviço do GCP.

> Os certificados IP detectam automaticamente o IPv4/IPv6 público do host. Atrás de um proxy, o endereço detectado é o do host, não a saída do proxy.
> Os certificados de domínio também podem ser emitidos posteriormente em “Certificados”.

#### Etapa 8: Resumo

Mostra o banco de dados, diretório de dados, núcleos, proxy, portas, conta de administrador, domínios e chaves `.env` (segredos mascarados) e fornece:

- **Baixar .env**: guarde uma cópia da configuração;
- **URLs de acesso**: URLs finais do painel/assinatura/portal; HTTP está marcado como inseguro.

Clique em “Salvar e reiniciar” para gravar o banco de dados e `.env`; o painel reinicia em serviço normal.

### 3.3 Após a configuração

- O serviço é reiniciado e o log de inicialização imprime o endereço de escuta final;
- Faça login com a conta de administrador, acesse URL e caminho da web a partir do resumo;
- Se a senha for perdida, redefina-a offline com `./corefusion admin reset-password`.

### 3.4 Configuração não interativa do CLI

Para automação ou servidores headless, escreva a configuração e marque a configuração como concluída diretamente:

```bash
cd /opt/corefusion
./corefusion init \
  --db-type sqlite \
  --data-dir /opt/corefusion/data \
  --admin-username admin \
  --admin-password 'change-me-strong-password' \
  --listen-port 8080 \
  --proxy-type caddy \
  --cores xray,singbox \
  --panel-domain panel.example.com \
  --tls-mode acme --tls-domain panel.example.com --tls-email you@example.com
```

Sinalizadores comuns:

| Bandeira | Descrição |
|------|-------------|
| `--db-type`/`--db-host`/`--db-port`/`--db-user`/`--db-password`/`--db-name` | Configurações do banco de dados |
| `--data-dir` | Diretório de dados; o padrão é `SAFE_PATHS` ou `./data` |
| `--listen-port`/`--subscription-port`/`--portal-port`/`--web-path` | Portas e caminho da web (porta livre aleatória por padrão) |
| `--admin-username`/`--admin-password` | Conta de administrador (senha aleatória impressa uma vez quando omitida) |
| `--proxy-type` | `none`/`nginx`/`caddy` |
| `--cores` | Núcleos a serem instalados, separados por vírgula: `xray,singbox` |
| `--panel-domain`/`--subscription-domain`/`--portal-domain` | Domínios (gravados nas configurações e adicionados ao proxy reverso) |
| `--tls-mode` | `none`/`self_signed`/`acme` |
| `--tls-domain`/`--tls-email`/`--tls-challenge`/`--tls-key-type`/`--tls-key-bits` | Opções de certificado (o padrão `--tls-challenge` é `http-01`; `dns-01` precisa de `--tls-dns-channel` ou `--tls-dns-provider`) |
| `--tls-ip-cert` | Emitir um certificado para o IP público detectado (apenas Let's Encrypt) |
| `--acme-provider` | `lets_encrypt`/`lets_encrypt_staging`/`zerossl`/`google` |
| `--currency`/`--timezone` | Moeda/fuso horário padrão |

Lista completa: `./corefusion init --help`. Então inicie o serviço:

```bash
systemctl restart corefusion
```

### 3.5 Executando novamente o assistente de configuração

Para recomeçar (remove `.env` e as marcas de configuração, **mantém os dados**):

```bash
cd /opt/corefusion
./corefusion reset-init      # type RESET to confirm
systemctl restart corefusion
```

Em seguida, use o novo URL e o token do log de inicialização.

> A reinicialização redefine a conta de administrador e o caminho da web. Os dados comerciais no banco de dados antigo permanecem, mas pode ser necessário recriar a conta de login.
>
> `reset-init` exclui todo o `.env` (incluindo `SAFE_PATHS`). **Se o seu diretório de dados for um caminho absoluto personalizado**, restaure `SAFE_PATHS` antes de reiniciar, caso contrário, o painel aparecerá em `./data`:
>
> ```bash
> printf 'SAFE_PATHS=/srv/corefusion-data\n' > /opt/corefusion/.env
> ```

### 3.6 Segurança após o primeiro login

#### Alterar a senha inicial

1. Clique no nome de usuário no canto superior direito;
2. Escolha “Alterar senha”;
3. Digite as senhas antigas e novas;
4. Clique em “Salvar”.

#### Habilitar login com código dinâmico (2FA / TOTP)

> Depois de habilitada, a conta muda para **login com código dinâmico**: a senha estática não é mais aceita; apenas o
> O código de 6 dígitos do seu aplicativo autenticador (ou um código de recuperação único) funciona. Isto **substitui** a senha de login — é
> não verificação em duas etapas “senha + código”.

1. Clique no nome de usuário no canto superior direito;
2. Escolha “Ativar 2FA”;
3. Digite a senha atual (apenas para confirmar esta alteração);
4. Digitalize o código QR com um aplicativo autenticador (Google Authenticator/1Password, etc.) ou insira a chave manualmente;
5. Digite o código de 6 dígitos para finalizar a habilitação;
6. **Salve os códigos de recuperação imediatamente** (para download); cada código funciona uma vez.

Fazendo login depois:

- Após inserir o nome de usuário, o campo da senha passa a ser “código de 6 dígitos/código de recuperação” e o botão passa a ser “Entrar com Código”;
- Digite o código atual; **a senha estática não pode mais ser usada**;
- Os códigos de recuperação são de uso único; regenere-os em “Login de código dinâmico (2FA)” quando eles acabarem (isso invalida todos os códigos antigos).

Se o autenticador for perdido e os códigos de recuperação se esgotarem, desative o 2FA offline no servidor:

```bash
cd /opt/corefusion
./corefusion admin 2fa reset -u admin    # disables 2FA, clears the secret and recovery codes, revokes all sessions
```

> Um único superadministrador sem autenticador nem códigos de recuperação não pode se recuperar por meio da IU da web e deve usar o comando acima.

#### Registre um Passkey (opcional)

1. Clique no nome de usuário no canto superior direito;
2. Escolha “Gerenciamento Passkey”;
3. Clique em “Registrar novo Passkey”;
4. Conclua o prompt do dispositivo.

> **Observação**: As chaves de acesso exigem HTTPS ou localhost, e o domínio deve estar na lista de permissões.

---

## 4. Painel

O painel mostra a visão geral do cluster de vários nós e a integridade do nó.

### 4.1 O que mostra

- **Informações do sistema**: nome do host, versão do sistema operacional, tempo de atividade, CPU, memória
- **Tráfego**: largura de banda em tempo real, tráfego total, tendência mensal
- **Saúde do nó**: contagens on-line/off-line/degradadas, nós de alta carga
- **Alertas**: nós off-line, nós degradados, certificados expirando em breve, avisos de cota
- **Recursos do cluster**: média de CPU, memória, disco e taxa de transferência

### 4.2 Ações

- Clique em “Ver detalhes” para abrir a página de detalhes de um nó
- Clique em um cartão de nó para ver seu status ativo
- Use os filtros para restringir por status, tipo ou região

---

## 5. Gerenciamento de nós

O gerenciamento de nós é o núcleo do CoreFusion e oferece suporte a vários tipos de nós.

### 5.1 Tipos de nós

| Tipo | Descrição |
|------|-------------|
| Autônomo (`standalone`) | Um nó central independente |
| Espelho (`mirror`) | Sincroniza a configuração de um nó de destino |
| Primário/backup (`primary_backup`) | Nós parceiros que fazem failover entre si |
| Entrada de cadeia (`chain_entry`) | Nó de entrada de um proxy encadeado |
| Saída de corrente (`chain_exit`) | Nó de saída de um proxy encadeado |

### 5.2 Adicionando um nó

1. Abra “Gerenciamento de nós”;
2. Clique em “Adicionar servidor”;
3. Preencha o básico:
   - Nome, por ex. `HongKong-HK01`
   - Nome do host: IP ou domínio
   - Região, por ex. `HK`, `JP`, `US`
   - Tipo de nó;
4. Defina a porta do Agente (padrão `18443`);
5. Selecione os núcleos suportados;
6. Clique em “Salvar”.

### 5.3 Implantação com um clique

1. Após adicionar o nó, clique em “comando Bootstrap”;
2. Copie o comando de instalação;
3. Execute-o como root no nó remoto:

   ```bash
   curl -fsSL https://your-panel/install.sh | bash
   ```

4. O Agente baixa, registra e inicia automaticamente.

### 5.4 Detalhes do nó

Clique em um nó para abrir sua página de detalhes:

- **Visão geral**: informações básicas, status do agente, status do certificado
- **Centro de protocolos**: configuração do processo principal
- **Proxy**: configuração Caddy/Nginx
- **Processos principais**: iniciar/parar/reiniciar
- **WARP**: configuração WARP
- **Domínios diretos**: gerenciamento de domínio
- **Limite de taxa**: modelagem de tráfego
- **Certificados**: vinculações de certificado
- **Usuários online**: usuários online em tempo real

### 5.5 Ações do nó

- **Configuração de sincronização**: envie a configuração do painel para o nó
- **Forçar reconexão**: interrompa a conexão do Agente e reconecte
- **Atualizar agente**: enviar uma atualização do agente
- **Revogar certificado**: revoga o certificado do Agente do nó

---

## 6. Centro de protocolo

O centro de protocolo configura parâmetros de protocolo para os processos principais.

### 6.1 Núcleos suportados

| Núcleo | Descrição |
|------|-------------|
| Núcleo Xray | VLESS / VMess / Trojan / Shadowsocks e mais |
| sing-box | Plataforma modular de proxy universal com suporte TUN; carrega Hysteria2 / TUIC e outros protocolos QUIC |

### 6.2 Módulos de configuração

- **Log**: log de acesso, log de erros, modo de saída
- **API**: endereço de escuta gRPC API, lista de serviços
- **DNS**: servidores DNS, estratégia de consulta, hosts estáticos
- **Roteamento**: regras de roteamento, balanceamento de carga, roteamento VLESS
- **Entradas**: lista de entrada, configurações de protocolo, transporte, TLS
- **Saídas**: lista de saída, configurações de protocolo, transporte
- **Transporte**: WebSocket, gRPC, XHTTP, TUN, etc.
- **TLS**: certificados, ALPN, impressão digital uTLS, ECH
- **Reality**: gerenciamento de destino, pares de chaves, IDs curtos
- **Verificação de integridade**: sonda URL, intervalo, simultaneidade
- **FakeDNS**: pool de IP, tamanho do pool
- **Métricas**: ouvinte de métricas do Prometheus
- **Política**: tempos limite de conexão, buffers, estatísticas
- **Dados geográficos**: atualização automática, arquivos de recursos
- **Limites de versão**: versão mínima/máxima

### 6.3 Configuração de entrada

1. Selecione o núcleo e o protocolo;
2. Preencha o básico:
   - Etiqueta
   - Endereço e porta de escuta
   - Tipo de protocolo
3. Definir parâmetros de protocolo (UUID, senha, etc.);
4. Escolha o transporte (TCP, WebSocket, gRPC, etc.);
5. Configurar TLS (certificado, ALPN);
6. Salve e aplique ao núcleo.

### 6.4 Configuração de saída

1. Adicione uma saída;
2. Defina a tag e o protocolo;
3. Defina o endereço e a porta do servidor;
4. Configurar transporte e TLS;
5. Salve e aplique.

### 6.5 Compartilhar links

1. Clique em “Compartilhar link” na lista de entrada;
2. Escolha um formato:
   - Compartilhe URI
   - Mihomo YAML
   - sing-box JSON
   - Xray JSON
3. Copie o link ou gere um código QR.

---

## 7. Gerenciamento de proxy

O gerenciamento de proxy configura o proxy reverso Caddy ou Nginx.

### 7.1 Tipos de proxy

- **Caddie**: automático HTTPS, HTTP/2, HTTP/3
- **Nginx**: alto desempenho, configuração flexível

### 7.2 Configuração do local

1. Abra “Gerenciamento de proxy”;
2. Escolha o tipo de proxy (Caddy/Nginx);
3. Adicione um site:
   - Domínio
   - A montante
   - Porto local
4. Salve.

### 7.3 Campos

- **upstream**: endereço de backend, por ex. `127.0.0.1:8082`
- **local_port**: porta de escuta local, por exemplo. `8443`
- **portal_forward**: regra de encaminhamento do portal
- **subscription_forward**: regra de encaminhamento de assinatura

---

## 8. Recursos de roteamento

Os recursos de roteamento gerenciam regras de roteamento de assinatura.

### 8.1 Recursos

- Gerenciar regras de roteamento
- Configurar políticas de roteamento
- Suporta vários formatos de roteamento

### 8.2 Uso

1. Abra “Recursos de roteamento”;
2. Adicione uma regra de roteamento;
3. Defina o nome, tipo e condições da regra;
4. Salve e aplique.

---

## 9. Central de assinaturas

O centro de assinaturas gerencia links e terminais de assinatura.

### 9.1 Formatos de assinatura

1. Abra “Centro de assinaturas” → “Formatos de assinatura”;
2. Adicione um formato:
   - Nome do formato
   - Formato de saída (Base64, YAML, JSON, etc.)
   - Protocolos incluídos
3. Salve.

### 9.2 Terminais de assinatura

1. Abra “Centro de assinaturas” → “Endpoints de assinatura”;
2. Adicione um ponto de extremidade:
   - Nome do terminal
   - Endereço de escuta
   - Caminho
   - Formato associado
3. Salve.

### 9.3 Links de assinatura

- Obtenha uma assinatura em `http://your-domain/sub/:token`
- Use parâmetros como `?format=yaml` para selecionar um formato

---

## 10. Gestão de clientes

O gerenciamento de clientes lida com os usuários finais.

### 10.1 Adicionando um cliente

1. Abra “Gestão de clientes”;
2. Clique em “Adicionar cliente”;
3. Preencha:
   - E-mail
   - Senha
   - Plano
   - Cota de tráfego
   - Prazo de validade
4. Salve.

### 10.2 Ações do cliente

- **Editar**: alterar detalhes do cliente
- **Suspender**: suspende o serviço
- **Retomar**: restaurar um cliente suspenso
- **Redefinir tráfego**: redefinir o tráfego usado
- **Redefinir senha**: redefinir a senha do cliente
- **Renovar**: prolongar a data de validade
- **Upgrade/downgrade**: alterar o plano
- **Excluir**: remova o cliente

### 10.3 Detalhes da assinatura

Clique em um cliente para ver:

- Link de assinatura
- Uso de tráfego
- Histórico de pedidos
- Histórico de transações

---

## 11. Gestão do plano

O gerenciamento de planos configura os planos que você vende.

### 11.1 Modelos de plano

1. Abra “Gerenciamento de planos” → “Modelos de planos”;
2. Adicione um modelo:
   - Nome do modelo
   - Categoria (padrão/premium/empresarial/teste)
   - Renovação automática
   - Configurações de teste
3. Salve.

### 11.2 Variantes do plano

1. Adicione uma variante ao modelo:
   - Nome da variante (por exemplo, mensal, anual)
   - Preço (em centavos)
   - Duração (dias)
   - Tráfego (GB, `0` = ilimitado)
   - Limite de simultaneidade
   - Limite de taxa (Mbps)
2. Salvar.

### 11.3 Complementos de tráfego

1. Abra “Complementos de tráfego”;
2. Adicione um complemento:
   - Nome
   - Preço
   - Trânsito
   - Modo de expiração (dias estendidos/igual ao plano)
3. Salve.

### 11.4 Cupons

1. Abra “Cupons”;
2. Adicione um cupom:
   - Código
   - Tipo (porcentagem/valor fixo)
   - Valor com desconto
   - Limite de uso
   - Período de validade
   - Planos aplicáveis
3. Salve.

---

## 12. Gerenciamento de recarga

O gerenciamento de recarga lida com sites de recarga, pedidos e cartões de recarga.

### 12.1 Locais de recarga

1. Abra “Gerenciamento de recarga” → “Sites de recarga”;
2. Adicione um site:
   - Nome do site
   - Métodos de pagamento suportados (Alipay, WeChat, USDT, Stripe, etc.)
   - Moeda
   - Taxa de câmbio
3. Salvar;
4. Verifique o site (arquivo HTTP ou verificação DNS).

### 12.2 Pedidos de recarga

- Ver todos os pedidos de recarga
- Filtrar por status (pendente, pago, com falha, expirado, reembolsado)
- Ver detalhes do pedido
- Emitir reembolsos administrativos

### 12.3 Cartões de recarga

1. Abra “Cartões de recarga”;
2. Adicione um cartão:
   - Tipo (valor, plano, complemento de tráfego)
   - Valor nominal/plano
   - Prazo de validade
3. Ou gere em massa:
   - Quantidade (até 1000)
   - Prefixo do número do cartão
   - Formato do número do cartão
4. Exporte como CSV.

### 12.4 Gestão de saldo

- Ver saldos de clientes
- Ajustar saldos (aumentar/diminuir/definir)
- Ajustar quantidades congeladas
- Ver histórico de transações

### 12.5 Revendedores

- Adicionar revendedores
- Definir taxas de comissão
- Ver comissão acumulada
- Retirar comissão

---

## 13. Gerenciamento de ingressos

O gerenciamento de tickets lida com solicitações de suporte ao cliente.

### 13.1 Lista de ingressos

- Ver todos os ingressos
- Filtrar por status (aberto, respondido, fechado)
- Filtrar por categoria (técnico, faturamento, conta, outro)
- Filtrar por prioridade (baixa, normal, alta, urgente)

### 13.2 Ações de tickets

- **Ver / responder**: abra um ticket e responda
- **Atribuir**: atribua um ticket a um administrador
- **Fechar**: fecha um ticket resolvido

---

## 14. Gerenciamento de certificados

O gerenciamento de certificados lida com certificados TLS.

### 14.1 Emissão de certificado

1. Abra “Gerenciamento de certificados”;
2. Clique em “Emitir certificado”;
3. Preencha:
   - Lista de domínios (um por linha)
   - Tipo de certificado (domínio/IP)
   - Tipo de chave (ECC/RSA)
   - Método de validação (DNS-01/HTTP-01)
   - Provedor DNS
   - Local de emissão (host/nó do painel)
4. Clique em “Emitir”.

### 14.2 Ações de certificado

- **Renovar**: renovar manualmente
- **Implantar**: implantar em um nó
- **Distribuir**: enviar para todos os nós associados
- **Migrar**: mover referências de um certificado antigo para um novo
- **Baixar**: baixar `cert` / `fullchain` / `privkey`

### 14.3 Carregando um certificado

1. Clique em “Carregar certificado”;
2. Escolha o método:
   - Cole o conteúdo
   - Carregar arquivos
3. Carregue a cadeia de certificados e a chave privada;
4. Salvar.

### 14.4 Configuração EAB

- Gerenciar credenciais EAB para Google Public CA/ZeroSSL
- Obtenha EAB automaticamente
- Anexe-o automaticamente ao emitir

---

## 15. Cotas de tráfego

As cotas de tráfego monitoram o tráfego mensal de cada nó.

### 15.1 Lista de cotas

- Ver configurações de cota para todos os nós
- Mostrar tamanho da cota, tráfego usado e permissão restante
- Mostrar porcentagem de uso

### 15.2 Uso do ciclo

- Ver detalhes de uso do ciclo por nó
- Mostrar tráfego de entrada/saída
- Mostrar hora de início/término do ciclo

### 15.3 Redefinir registro

- Ver registros de redefinição de tráfego
- Mostrar o ciclo de reinicialização, fonte de disparo e operador

---

## 16. Proxies encadeados

Proxies encadeados configuram links de múltiplas saídas.

### 16.1 Criando um link

1. Abra “Proxies encadeados”;
2. Clique em “Novo link”;
3. Selecione o nó de entrada;
4. Selecione a entrada de entrada;
5. Selecione nós de saída (múltiplos permitidos);
6. Selecione uma entrada para cada saída;
7. Definir prioridade/peso;
8. Confirmar.

### 16.2 Gerenciamento de links

- Veja a visão geral da topologia
- Editar um link (prioridade, peso, tag de saída)
- Excluir um link
- Verifique o estado de saúde

### 16.3 Nós externos

- Importar nós externos (v2ray URI, Base64 URI, assinatura URL)
- Registrar entradas externas
- Gerenciar nós externos

---

## 17. Usuários on-line

Os usuários online mostram o status online em tempo real.

### 17.1 O que mostra

- Assinaturas on-line
- Dispositivos/IPs on-line
- Nós on-line
- Taxa em tempo real
- Tráfego total

### 17.2 Filtragem

- Filtrar por nó
- Pesquise por email/cliente/nó/IP

---

## 18. Gerenciamento de usuários

O gerenciamento de usuários gerencia os usuários do painel.

### 18.1 Funções

| Função | Descrição |
|------|-------------|
| Superadministrador (`superAdmin`) | Privilégios mais altos |
| Administrador (`admin`) | Privilégios de gerenciamento |
| Operador (`operator`) | Privilégios operacionais |
| Visualizador (`viewer`) | Somente leitura |

### 18.2 Adicionando um usuário

1. Abra “Gerenciamento de usuários”;
2. Clique em “Adicionar usuário”;
3. Preencha:
   - Nome de usuário
   - E-mail
   - Senha
   - Função
4. Salve.

### 18.3 Ações do usuário

- **Editar**: alterar detalhes do usuário
- **Excluir**: remove o usuário

---

## 19. Site do portal

O site de portal configura o portal do cliente.

### 19.1 Geral

- Nome do site
- Descrição do site
- Inscrições abertas
- Caminho do portal
- Porta do portal

### 19.2 Aparência

- Título do site
- Logotipo URL
- Cores primárias, de fundo, de texto e de destaque
- Fonte

### 19.3 Navegação

- Adicionar páginas de navegação
- Configure o nome da página, ícone e link externo
- Gerenciar itens de navegação

### 19.4 Módulos

- Adicionar/editar módulos
- Configure o título, a descrição e o ícone do módulo
- Suporta Markdown, rich text e HTML personalizado

### 19.5 Idiomas

- Adicionar idiomas
- Gerenciar cópia
- Importação/exportação JSON

### 19.6 Modelos

- Alternar modelos
- Personalizar HTML/CSS/JS
- Redefinir para o modelo padrão

---

## 20. Configurações do sistema

As configurações do sistema configuram os parâmetros do painel.

### 20.1 Painel

- Endereço e porta de escuta
- Diretório de dados
- Fuso horário
- Porta de assinatura
- Limite de exportação
- Inscrições abertas
- Modo painel (completo/somente gerenciamento)

### 20.2 Segurança

- Gerenciamento de chave de criptografia
- Gerenciamento de chaves JWT
- Lista negra de IP
- Domínios permitidos Passkey

### 20.3 OAuth

- GitHub OAuth
- Google OAuth
- Configuração de retorno de chamada URL

### 20.4 Notificações

- Notificações por e-mail (SMTP)
- Notificações Telegram
- Modelos de notificação
- Configurações de monitoramento

### 20.5 Backup e restauração

- Crie um backup
- Lista de backup
- Baixe um backup
- Restaurar um backup
- Backups agendados
- Armazenamento remoto (FTP/S3/WebDAV)

### 20.6 Desbloqueio automático

- Ativar desbloqueio automático
- Recarga
- Status aplicáveis

---

## 21. Consola de operações

O console de operações gerencia instantâneos de configuração e aplica trabalhos.

### 21.1 Instantâneos de configuração

- Crie um instantâneo
- Listar instantâneos
- Reverter para um instantâneo
- Excluir um instantâneo

### 21.2 Candidatar-se a empregos

- Ver empregos recentes
- Aplicar configuração agora (DB → tempo de execução)
- Pré-verificação (validar conflitos de porta e configuração JSON)
- Cancelar um trabalho
- Tentar novamente um trabalho

### 21.3 Validação de nó

- Selecione um nó e valide
- Veja o resultado da validação
- Validar todos os nós

### 21.4 Parada de emergência

- Parada de emergência do proxy (grava uma configuração de entrada vazia)
- Pare imediatamente todo o tráfego de entrada

---

## 22. Monitoramento de banco de dados

O monitoramento do banco de dados mostra a integridade do banco de dados.

### 22.1 O que mostra

- Mecanismo de banco de dados
- Tamanho do banco de dados
- Conectividade
- Latência
- Status do pool de conexões
- Estatísticas de cache
- Estatísticas de consulta

### 22.2 Mecanismos suportados

- SQLite
- MySQL
- PostgreSQL

---

## 23. Registro de auditoria

O log de auditoria registra operações.

### 23.1 O que mostra

- Categoria
- Tipo de evento
- Resultado
- IP de origem
- Usuário
- Detalhes

### 23.2 Registro de operação do nó

- Ver registros de operação do nó
- Filtrar por nó

---

## 24. Gerenciamento de registros

O gerenciamento de logs mostra os logs do sistema.

### 24.1 Tipos de registros

- Registros do painel
- Registros de nós
- Registros principais
- Registros de proxy

### 24.2 Ações

- Ver registros
- Baixar registros
- Limpar registros

---

## 25. Extensões de operações

As extensões de operações configuram extensões opcionais.

### 25.1 Recursos

- Extensões personalizadas
- Configuração de extensão
- Gerenciamento de extensão

---

## 26. Perguntas frequentes

### 26.1 O painel não inicia

- Verifique se a porta já está em uso
- Verifique se `.env` é válido
- Verifique os registros: `journalctl -u corefusion -f`; sem systemd, `tail -f <install-dir>/data/logs/panel.log`

### 26.2 Nó off-line

- Verifique se o Agente está em execução: `systemctl status corefusion-agent`
- Verifique a conectividade da rede
- Verifique as regras de firewall (porta `18443`)

### 26.3 Falha na emissão do certificado

- Verifique a resolução DNS
- Verifique se a porta 80 está acessível
- Verifique a configuração do provedor DNS
- Verifique os logs do certificado

### 26.4 A assinatura não funciona

- Verifique o link de assinatura
- Verifique se o nó está online
- Verifique se o processo principal está em execução
- Verifique as regras do firewall

### 26.5 Falha no retorno de chamada de recarga

- Verifique o retorno de chamada URL
- Verifique se a chave do site corresponde
- Verifique a lista de permissões de IP
- Verifique os registros de pedidos

### 26.6 A página do portal está inacessível

- Verifique se a porta do portal está aberta
- Verifique a configuração do caminho do portal
- **Importante**: certifique-se de que o proxy reverso encaminhe o domínio do portal para a **porta do mecanismo do portal**, não para a porta do painel principal — o encaminhamento para a porta errada mostra uma página em branco ou o painel em vez do portal. Verifique `upstream`/`local_port` no registro do site proxy.
- Abra `http://<portal-domain>:<portal-port>` diretamente para verificar o próprio mecanismo do portal
- Verifique a configuração do proxy reverso
- Verifique os logs do portal

---

## Apêndice

### A. Portas padrão

| Porto | Objetivo |
|------|---------|
| Atribuído pelo assistente de configuração (porta livre aleatória por padrão) | Painel de administração (alterável) |
| 18443 | Agente gRPC (alterável) |
| 0 (desativado por padrão) | Porta dedicada do mecanismo de assinatura (vazia ou `0` compartilha a porta principal) |
| 0 (desativado por padrão) | Porta dedicada do mecanismo do portal (vazia ou `0` compartilha a porta principal) |
| 80/443 | Proxy reverso (Caddy / Nginx) HTTP / HTTPS |

### B. Caminhos padrão

| Caminho | Objetivo |
|------|---------|
| `/` | Painel de administração (o portal também fica em `/` quando nenhum mecanismo de portal dedicado está configurado) |
| `/sub/:token` | Busca de assinatura (quando nenhuma porta de assinatura dedicada está configurada) |
| `/p` | Página do portal (quando nenhuma porta de portal dedicada está configurada) |
| `/:token` | Busca de assinatura (com uma porta de assinatura dedicada, sem o prefixo `/sub`) |

### C. Ferramenta de linha de comando

O binário do painel suporta estes subcomandos (nenhum argumento inicia o servidor):

```bash
# Start the panel (same as ./corefusion server)
./corefusion

# Run database migrations
./corefusion migrate

# Offline administration (when you cannot sign in)
./corefusion admin reset-password -u admin -p 'new-password'
./corefusion admin 2fa reset -u admin
./corefusion admin list

# Version / help
./corefusion version
./corefusion help
```

> Sem uma unidade systemd, reinicie parando o processo antigo e iniciando-o novamente com `nohup`; os registros estão em `<install-dir>/data/logs/panel.log`.

---

> **Versão do documento**: v1.0
> **Última atualização**: 23/10/2026
> **Aplica-se a**: CoreFusion v1.0+

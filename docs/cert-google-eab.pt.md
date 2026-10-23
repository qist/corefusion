<!-- source: cert-google-eab.md | locale: pt | updated: 2026-10-23 -->
# Guia de CA pública Google (emissão automática de EAB)

> Idiomas: [简体中文](cert-google-eab.md) · [English](cert-google-eab.en.md) · [繁體中文](cert-google-eab.zh-TW.md) · [Русский](cert-google-eab.ru.md) · [日本語](cert-google-eab.ja.md) · [한국어](cert-google-eab.ko.md) · [Deutsch](cert-google-eab.de.md) · [Français](cert-google-eab.fr.md) · [Español](cert-google-eab.es.md) · Português · [Türkçe](cert-google-eab.tr.md) · [Bahasa Indonesia](cert-google-eab.id.md) · [ไทย](cert-google-eab.th.md) · [Tiếng Việt](cert-google-eab.vi.md) · [العربية](cert-google-eab.ar.md) · [فارسی](cert-google-eab.fa.md)

> Aplica-se a: painel v0.12.101+ | Página: **Certificados → Configuração EAB**
> Código: `internal/service/certificate/eab_google.go`, `internal/api/handlers/certificates/eab_config.go`

## 1. Por que Google requer EAB

| CA | EAB obrigatório para ACME? | Como obter EAB |
|---|---|---|
| Vamos criptografar | ❌ Não | — |
| ZeroSSL | ✅ Sim | **Troque um email por um** (`api.zerossl.com/acme/eab-credentials-email`, igual a acme.sh); o painel pode buscá-lo automaticamente |
| **Google CA pública** | ✅ **Sim** | **Deve ser criado com autorização do GCP** (um e-mail não é suficiente) → o foco desta página |

Em outras palavras, Google **não pode evitar a "autorização"**: um arquivo JSON de conta de serviço ou um token de acesso temporário.

## 2. Regras rígidas de Google (leia primeiro)

| Regra | Descrição |
|---|---|
| **Use dentro de 7 dias** | Uma chave EAB **expira** se não for usada dentro de 7 dias e deve ser recriada |
| **Apenas uma conta** | Uma chave EAB registra exatamente uma conta ACME; após o registro a chave **é invalidada** e não pode ser reutilizada |
| **As contas ACME não expiram** | A conta ACME criada com ele é **de longa duração** → **renovações não precisam de EAB** |
| ⚠ **Excluir o projeto GCP elimina todas as contas** | A exclusão do projeto invalida imediatamente **todas as contas ACME** dele e **a renovação se torna impossível** |

> Conclusão: **EAB é uma "credencial de integração" única; a conta ACME é o que dura.**
> O painel armazena contas em `data/acme/accounts/<ca>.json` — **faça backup deste diretório**.

## 3. Três caminhos (comece com o caminho A – sem busca de menu)

> Os pontos de entrada do Cloud Console mudaram várias vezes, então os links diretos e o Cloud Shell vêm em primeiro lugar; caminhos de menu são substitutos.

### Caminho A (mais fácil, recomendado primeiro): um comando no GCP Cloud Shell

Cloud Shell é um terminal de navegador com **gcloud pré-instalado e já conectado**; sem configuração local e sem menus.

1. Abra <https://console.cloud.google.com/cloudshell> (ou o ícone `>_` no canto superior direito do console)
2. Selecione o projeto (o ID do projeto fica visível no seletor de projetos na parte superior):
   ```bash
   gcloud config set project <your-project-id>
   ```
3. Habilite a CA Pública API (**equivalente** a habilitá-la na Biblioteca API):
   ```bash
   gcloud services enable publicca.googleapis.com
   ```
4. Crie o EAB (**estas são as duas strings que você precisa**):
   ```bash
   gcloud publicca external-account-keys create
   ```
 Na saída: `keyId` → **EAB-KID** do painel; `b64MacKey` → a **EAB-HMAC-Key** do painel
5. De volta ao painel: **Certificados → Configuração EAB → Adicionar configuração EAB → CA: Google CA pública** →
 insira os dois valores em `EAB-KID` / `EAB-HMAC-Key` → salve ✓

> ⚠ Custo: Google As chaves EAB são **7 dias, únicas** ✗ → após 7 dias (ou ao trocar de conta) **repita a etapa 4**.
> Para evitar isso, use o caminho B (armazene uma conta de serviço uma vez e deixe o painel solicitar novamente automaticamente).

### Caminho B (configuração única e depois automática): chave da conta de serviço JSON

**① Habilite a CA pública API** (link direto é mais fácil)
<https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=YOUR_PROJECT_ID> → clique em “Ativar”
Fallback: ☰ → **APIs e serviços → Biblioteca** → pesquisar `Public CA` → abrir → “Ativar”

**② Crie uma conta de serviço**
<https://console.cloud.google.com/iam-admin/serviceaccounts?project=YOUR_PROJECT_ID>
→ “**+ Criar conta de serviço**” → qualquer nome (por exemplo, `corefusion-eab`) → “Criar e continuar” → você pode **pular** a etapa da função → “Concluído”
Alternativo: ☰ → **IAM e administrador → Contas de serviço**

**③ Conceda a função mínima** (mais fácil na mesma página)
Clique na conta recém-criada → **guia “Permissões”** → “Conceder acesso” → novo principal: o **e-mail da conta de serviço**
→ pesquise a função para `publicca` → escolha **“Criador de chave de conta externa da CA pública”** → salve
Fallback: ☰ → **IAM e Admin → IAM** → “Conceder acesso” → mesmo e-mail + função
⚠ **Não conceder ao Proprietário/Editor** ✗

**④ Baixe a chave JSON**
Contas de serviço → abra a conta → **guia “Chaves”** → “**Adicionar chave**” → “**Criar nova chave**” → **JSON** → criar
(o navegador baixa um `xxx.json`; ou linha `⋮` → “Gerenciar chaves” → mesmo fluxo)

**⑤ De volta ao painel**: insira o **ID do projeto** e cole **todo o conteúdo** do JSON em “Arquivo de chave da conta de serviço” → clique em “Solicitar EAB automaticamente”
→ depois o painel **solicita novamente o EAB automaticamente** quando ele expira ✓, não há mais visitas ao GCP ✓

### Caminho C (somente teste único): cole um token de acesso

No Cloud Shell:
```bash
gcloud auth print-access-token
```
Cole a saída no “Token de acesso temporário” do painel mais o ID do projeto → clique em “Solicitar EAB automaticamente”
⚠ Válido por cerca de 1 hora e **não armazenado** ✓ → apenas verificação, sem nova solicitação automática ✗

### Folha de dicas quando você não consegue encontrar um ponto de entrada (alteração dos menus do console)

| Tarefa | Link direto (mais confiável) | Alternativo: menu |
|---|---|---|
| Habilite o CA público API | `https://console.cloud.google.com/apis/library/publicca.googleapis.com?project=<PROJECT_ID>` | ☰ → APIs e serviços → Biblioteca → pesquisar `Public CA` |
| Contas de serviço | `https://console.cloud.google.com/iam-admin/serviceaccounts?project=<PROJECT_ID>` | ☰ → IAM e administrador → Contas de serviço |
| Função de concessão | `https://console.cloud.google.com/iam-admin/iam?project=<PROJECT_ID>` | ☰ → IAM e administrador → IAM → Conceder acesso |
| Nuvem Shell | `https://console.cloud.google.com/cloudshell` | Ícone `>_` no canto superior direito |

> Dica: substitua `<PROJECT_ID>` / `YOUR_PROJECT_ID` pelo ID real do projeto (como `my-project-123456`) para ir direto para a página.

## 4. Configurando o painel (duas maneiras)

Vá para **Certificados → Configuração EAB → Adicionar configuração EAB** e escolha **CA pública Google**:

### Opção A (recomendada): cole a conta de serviço JSON

| Campo | Valor |
|---|---|
| Código do projeto do GCP | O ID do projeto da seção 3, por ex. `my-project-123456` |
| Conta de serviço JSON | Cole **todo o conteúdo** de `corefusion-eab.json` |
| Token de acesso temporário | Deixe vazio |
| Nome | Gerado automaticamente como `Google Public CA - <project-id>`; editável |

Clique em **Solicitar EAB automaticamente** → em caso de sucesso a chave KID/HMAC é preenchida e salva; posteriormente basta escolher `CA = Google Public CA` no momento da emissão (EAB está associado por CA; sem seleção manual).

> A conta de serviço JSON é **criptografada na configuração EAB do banco de dados do painel** (`json:"-"`, **nunca retornada ao frontend**).
> O objetivo é **solicitar novamente uma chave automaticamente após o vencimento de 7 dias** em vez de visitas manuais ao GCP.

### Opção B: token de acesso temporário (único)

```bash
gcloud auth print-access-token        # valid ~1 hour
```

Cole-o em “Token de acesso temporário”, preencha o ID do projeto, clique em **Solicitar EAB automaticamente**.
⚠ Este token **não é armazenado** e deve ser obtido novamente após expirar → multa para teste único, não para uso de longo prazo.

### Equivalente em linha de comando (opcional, para comparação)

```bash
gcloud publicca external-account-keys create
# returns keyId / b64MacKey, corresponding to ACME's --eab-kid / --eab-hmac-key
```

## 4.5 Por que JSON em vez de "uma única chave"?

Esta é a pergunta mais comum. Separe os dois tipos de “chave”:

| O que você pode esperar | O que é realmente necessário |
|---|---|
| **Chave API** (a sequência `AIza…`) | ❌ **Inútil** — uma chave API é apenas um identificador público e **não carrega identidade**; a criação de EAB precisa de **permissões do IAM**, portanto, são necessárias credenciais de afirmação de identidade |
| — | ✅ **Chave da conta de serviço**: o **arquivo** baixado do GCP “IAM → Contas de serviço → Chaves → Criar nova chave → **JSON**” |

Ponto principal: **o próprio arquivo JSON é "uma chave"** — o GCP empacota a identidade da conta (`client_email`) mais a chave privada (`private_key`) em um arquivo. O painel assina um JWT com ele → troca por um token de acesso OAuth2 → chama o CA público API.
Então “cole JSON” ≠ preenchendo vários campos; **cole o arquivo inteiro** (o painel também preenche campos opcionais como `type` / `token_uri`).

**O verdadeiro EAB são as duas strings** (`EAB-KID` + `EAB-HMAC-Key`), que são o **resultado** da solicitação; o painel os preenche automaticamente.

### Não quer entregar o arquivo? Duas alternativas

| Método | Como | Custo |
|---|---|---|
| **Token de acesso temporário** | `gcloud auth print-access-token` → cole em “Token de acesso temporário” | Validade de aproximadamente 1 hora, **não armazenado** → apenas único, sem nova solicitação automática |
| **Manual KID/HMAC** | Execute `gcloud publicca external-account-keys create` localmente → insira `keyId` / `b64MacKey` em “EAB-KID / EAB-HMAC-Key” | Nenhuma credencial entregue ✓; mas Google EAB é **7 dias, uma vez** → repetir após o vencimento |

> O painel recomenda a conta de serviço JSON por um motivo: **EAB expira após 7 dias** e somente uma conta de serviço armazenada permite **nova solicitação automática** sem trabalho manual.

## 4.6 FAQ: posso usar uma chave API?

**Não.** Esta não é uma opção de implementação — **Google a rejeita explicitamente** (verificado em ambos os sentidos):

```bash
# 1) Query parameter
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys?key=AIza..." -d '{}'
# 2) x-goog-api-key header recommended by the official docs
curl -X POST "https://publicca.googleapis.com/v1/projects/<ID>/locations/global/externalAccountKeys" \
     -H "x-goog-api-key: AIza..." -d '{}'
```

Ambos retornam:

```json
{"error":{"code":401,"status":"UNAUTHENTICATED",
  "message":"API keys are not supported by this API. Expected OAuth2 access token
             or other authentication credentials that assert a principal."}}
```

Para efeito de comparação, um **(falso) OAuth Bearer** retorna
`Request had invalid authentication credentials. Expected OAuth 2 access token...` —
portanto, o OAuth é reconhecido enquanto **as chaves API são rejeitadas imediatamente** ✗.

**Por que**: criar EAB é uma **operação administrativa** que requer saber “quem você é e se possui `roles/publicca.externalAccountKeyCreator`” ✗
— uma chave API identifica apenas um projeto/aplicativo ✓ e **não expressa uma identidade principal** ✓.
(As "Práticas recomendadas para gerenciamento de chaves API" de Google recomendam **IAM + credenciais de conta de serviço de curta duração** para cenários administrativos ✓ — a mesma abordagem usada aqui ✓.)

**Portanto, existem apenas três caminhos** ✓ (ver §3): conta de serviço JSON ✓ / token de acesso temporário ✓ /
ou **nenhuma credencial** — execute `gcloud publicca external-account-keys create` no Cloud Shell ✓
e insira `keyId` / `b64MacKey` no painel ✓ (**use isto se o GCP IAM bloquear você** ✓).

## 5. Registro e renovações automáticas (importante)

```
Panel: CA = Google Public CA
  ├─ usable EAB present in the config → use it
  └─ missing/expired → request a new one on the spot with the stored service-account JSON (auto-registration)
        │
First issuance on the node/panel ── registers the ACME account with EAB (one-time!)
        │
Account persisted: data/acme/accounts/google.json (panel) / <Agent data_dir>/acme/accounts/google.json (node)
        │
Every later renewal ── uses the account key only; **EAB is never needed again**
```

Portanto:

- **A primeira emissão é registrada automaticamente**; nenhuma etapa manual do GCP é necessária;
- As renovações **não** consomem EAB (desde que o arquivo da conta exista e o diretório de dados permaneça estável);
- ⚠ O diretório de dados ACME do lado do agente é um caminho absoluto fixo (padrão `/var/lib/corefusion-agent/acme`);
 **não altere casualmente**, ou será tratada como uma nova conta → outra chave EAB necessária.

## 6. Solução de problemas

| Sintoma/erro | Causa | Correção |
|---|---|---|
| `PERMISSION_DENIED` (403) | A conta de serviço não possui `roles/publicca.externalAccountKeyCreator` | Volte para a seção 3 ④ |
| `SERVICE_DISABLED` (403) | `publicca.googleapis.com` não habilitado para o projeto | Seção 3② |
| `NOT_FOUND` (404) | ID do projeto/projeto excluído incorreto | Verifique o ID do projeto |
| Emissão de relatórios `externalAccountRequired` | Esta CA não possui EAB | Solicite/configure EAB primeiro no painel |
| Emissão de relatórios `Accounts must have at least one contact.` | Nenhum e-mail foi fornecido durante o registro | Defina o e-mail da conta ACME na página do certificado (o painel usa a conta padrão) |
| Renovação pede EAB novamente/registro falha | Arquivo da conta ACME perdido (diretório de dados alterado/limpo) | **Restaurar o backup**; sem ele, solicite um novo EAB usando a conta de serviço |
| O emissor muda de Google para Let's Encrypt | Comando de renovação sem CA (bug antigo, corrigido na v0.12.101) | Atualizar e reeditar |

## 7. Recomendações de segurança e operações

1. **Privilégio mínimo**: conceda à conta de serviço apenas `roles/publicca.externalAccountKeyCreator`, nunca Proprietário/Editor.
2. **Não exclua o projeto**: a exclusão do projeto GCP invalida todas as suas contas ACME → os certificados não podem ser renovados.
3. **Sempre faça backup** `data/acme/accounts/` (da própria identidade da conta) e da configuração EAB do banco de dados do painel.
4. **Não entre em pânico com a expiração do EAB**: enquanto o arquivo da conta ACME existir, as renovações não serão afetadas; um novo EAB só é necessário para registrar uma nova conta.
5. Se a chave da conta de serviço tiver vazado: exclua essa chave no GCP e cole um novo JSON no painel.

## 8. Endpoints relacionados (para scripts)

| Ponto final | Descrição |
|---|---|
| `POST /api/v1/eab-configs/fetch-google` | Crie um EAB usando autorização do GCP. Corpo: `{project_id, service_account_json?, access_token?}` |
| `POST /api/v1/eab-configs/fetch-zerossl` | Obtenha um ZeroSSL EAB com um e-mail. Corpo: `{email}` |
| `GET  /api/v1/eab-configs` | Listar configurações EAB (**sem** a chave HMAC ou conta de serviço JSON; somente `has_mac_key` / `has_google_sa`) |

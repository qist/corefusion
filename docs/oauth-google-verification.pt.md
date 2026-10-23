<!-- source: oauth-google-verification.md | locale: pt | updated: 2026-10-23 -->
# Guia de verificação OAuth Google

> Idiomas: [简体中文](oauth-google-verification.md) · [English](oauth-google-verification.en.md) · [繁體中文](oauth-google-verification.zh-TW.md) · [Русский](oauth-google-verification.ru.md) · [日本語](oauth-google-verification.ja.md) · [한국어](oauth-google-verification.ko.md) · [Deutsch](oauth-google-verification.de.md) · [Français](oauth-google-verification.fr.md) · [Español](oauth-google-verification.es.md) · Português · [Türkçe](oauth-google-verification.tr.md) · [Bahasa Indonesia](oauth-google-verification.id.md) · [ไทย](oauth-google-verification.th.md) · [Tiếng Việt](oauth-google-verification.vi.md) · [العربية](oauth-google-verification.ar.md) · [فارسی](oauth-google-verification.fa.md)

> Escopo: login Google para o portal CoreFusion (`internal/service/oauth`, escopos: `openid`, `email`, `profile`).
> Comparação GitHub no final (GitHub não possui fluxo de verificação).

## 1. Modo de teste versus modo de produção

A tela de consentimento do OAuth Google tem dois estados de publicação:

| Estado | Quem pode fazer login | Verificação necessária |
|---|---|---|
| **Testando** | Somente contas Google na lista de “usuários de teste” | Não |
| **Em produção** | Qualquer conta Google | **Sim** (veja abaixo) |

Assim que “Configurações do sistema → OAuth” estiver habilitado, a página de login do portal mostra o botão Google para cada visitante;
se o aplicativo ainda estiver em teste, os visitantes que não estiverem na lista de usuários de teste verão
`Error 403: access_denied` (aplicativo não verificado/você não é um usuário de teste).
Para oferecer login Google publicamente, **mude o aplicativo para produção e passe na verificação**.

## 2. Requisitos rígidos para verificação

A revisão do Google (verificação básica para escopos não restritos) requer:

1. **Página inicial**: um URL acessível publicamente no domínio autorizado (mesmo domínio ou subdomínio).
2. **Link da Política de Privacidade**: deve ser **acessível publicamente** (sem login) e explicar quais dados você coleta e como você os usa/armazena/exclui. Para login Google, deve cobrir pelo menos:
 os dados da conta Google coletados (e-mail, nome, avatar), a finalidade (criar/fazer login na conta do portal), nenhuma venda a terceiros,
 e como os usuários podem excluir suas contas e dados.
3. **Link dos Termos de Serviço**: acessível publicamente, descrevendo as regras de serviço.
4. **Domínios autorizados**: cadastre o domínio do app na tela de consentimento (ex.: `example.com`); os links home/privacy/terms devem estar nele ou em um subdomínio.
5. **Logotipo do aplicativo (opcional, mas recomendado)**: 120×120px.
6. **Justificativa de escopo**: este projeto usa apenas `openid / email / profile` (não restrito, não sensível),
 que se enquadra na verificação básica — sem justificativa por escopo e **sem avaliação de segurança de terceiros**.
 (Escopos restritos, como acesso total ao Gmail, exigiriam isso; este projeto não usa nenhum.)

Entrada de verificação: [Google Cloud Console](https://console.cloud.google.com/) →
**APIs e serviços → Tela de consentimento do OAuth → Publicar aplicativo → Enviar para verificação**.
A verificação básica geralmente é concluída em alguns dias; você pode operar no modo de teste enquanto isso.

## 3. O que configurar para este projeto

### 3.1 Redirecionamento autorizado URI

O retorno de chamada é gerado dinamicamente a partir do domínio que inicia a entrada (consulte `Authorize` em `internal/api/handlers/oauth/handler.go`):

```
https://<portal-domain>/api/v1/client-auth/oauth/google/callback
```

- O login foi iniciado a partir do domínio do portal (por exemplo, `https://p.example.com`) → registrar `https://p.example.com/...`;
- Iniciado a partir do domínio do painel `/p` → registrar `https://panel.example.com/...`.
- Google permite **múltiplos** URIs de redirecionamento; registrar ambos os domínios.
- ⚠ Google exige que os URIs de redirecionamento sejam **https** (exceto `http://localhost`);
 o proxy reverso deve encaminhar `X-Forwarded-Host` / `X-Forwarded-Proto`, caso contrário o painel construirá um
 http ou redirecionamento interno_uri → Google rejeita (`redirect_uri_mismatch`).

Após definir “Configurações do sistema → OAuth → Domínio de retorno de chamada do portal”, o retorno de chamada URL mostrado na página é aquele a ser colado em Google.

### 3.2 Links de Política de Privacidade/Termos no portal

O cartão de login inclui links “Política de Privacidade · Termos de Serviço” (mostrados depois de configurados) e ambos os documentos têm **páginas independentes**
(a caixa de diálogo e a página independente compartilham o mesmo conteúdo):

- Domínio do painel principal: `https://<panel-domain>/p/legal/privacy`, `/p/legal/terms`
- Domínio do portal dedicado (implantação raiz): `https://<portal-domain>/legal/privacy`, `/legal/terms`
- Suporta `?lang=xx`; sem ele, utiliza-se o `Accept-Language` do navegador e, em seguida, o padrão do portal.
- As páginas são **renderizadas pelo servidor** (sem JS), o que auxilia o rastreador de verificação do Google.

**Configuração (de qualquer forma; links vencem)**:

1. **Modo de conteúdo (recomendado, funciona imediatamente)**: editor do portal → Configurações → Geral → Documentos de conformidade →
 escolha “Editar idioma” e cole os modelos §4 em “Conteúdo da política de privacidade” / “Conteúdo dos termos de serviço”
 (**Markdown, editado por idioma**) → depois de salvar, o cartão de login mostra links que abrem uma **caixa de diálogo**;
 o “link independente” do editor pode ser inserido diretamente na tela de consentimento do Google.
2. **Modo link externo**: digite “Política de privacidade URL” / “Termos de serviço URL” (apontando para as páginas que você hospeda) → abre em uma nova aba.

O conteúdo e os links são entregues via `/api/v1/public/portal-config` (`privacy_policy_content` /
`terms_of_service_content` são **mapas Markdown por idioma** `{"zh-CN":"...","en":"..."}`;
`privacy_policy_url` / `terms_of_service_url` são links externos opcionais). **Eles devem estar acessíveis sem login** ✓.

### 3.3 Interruptor lateral do painel

`System settings → OAuth`: ative Google e insira o ID do cliente/segredo do cliente (o segredo é criptografado com AES-256-GCM em repouso).
O botão Google aparece na página de login do portal somente quando tudo está configurado (`/api/v1/client-auth/oauth/providers` retorna habilitado).

## 4. Modelos de Política de Privacidade/Termos

> Modelos mínimos de conformidade que você pode adaptar. Antes de usar: substitua os espaços reservados `【】`, adicione/remova cláusulas para o seu negócio,
> e fazer com que a entidade operacional os revise. Esses modelos não são aconselhamento jurídico.

### 4.1 Política de Privacidade (modelo)

```markdown
# Privacy Policy

Last updated: [DATE]

[OPERATOR NAME] ("we") operates [PORTAL NAME] ("the Service"). We respect your privacy;
this policy explains what information we collect and how we use it.

## 1. Information we collect
- **Account information**: the email address and password you provide at registration (stored only as a one-way hash);
- **Google sign-in information**: when you choose "Sign in with Google", we receive, with your authorization,
  your Google account email, display name and avatar URL (scopes: openid, email, profile);
- **Service data**: subscription configuration, traffic usage, device IP allow-list, tickets and order records.

## 2. How we use information
Only to: create and sign in to your account, provide the proxy subscription service, bill you and provide support,
and for security controls (sign-in protection, IP allow-list). We **do not** sell your personal information.

## 3. Third-party services
- **Google**: receives account information within the authorized scope only when you actively use "Sign in with Google";
- Payment providers: receive order information only when you initiate a payment.

## 4. Storage and security
Data is stored on our servers; credentials are encrypted at rest and all traffic uses TLS.

## 5. Your rights
You can update your profile/password and end sessions in the portal at any time; to delete your account and all data,
contact [CONTACT EMAIL] and we will process the request within [N] business days.

## 6. Policy changes
Material changes will be announced in the portal notice board.
```

### 4.2 Termos de Serviço (modelo)

```markdown
# Terms of Service

Last updated: [DATE]

## 1. Service
The Service provides [PROXY / NETWORK ACCESS] to registered users; specific plans are described on the store page.

## 2. Accounts
- You are responsible for the security of your account and password; for accounts created with Google sign-in, credentials are managed by Google;
- You may not lend or transfer your account, or use the Service for any unlawful purpose (including but not limited to:
  unauthorized access to others' systems, distributing illegal content, or circumventing mandatory legal requirements).

## 3. Payment and refunds
Plans are billed [monthly / by usage]; traffic and device limits follow the plan description;
[REFUND POLICY: e.g. full refund within 7 days if unused].

## 4. Changes and termination
We may suspend or terminate the Service due to force majeure or your breach of these terms; deleting an account deletes the related data.

## 5. Disclaimer
The Service is provided "as is"; to the extent permitted by law, we are not liable for indirect losses caused by use of the Service.

## 6. Miscellaneous
These terms are governed by the laws of [JURISDICTION]. Contact: [CONTACT EMAIL].
```

## 5. Lista de verificação de pré-lançamento

- [ ] Tela de consentimento do OAuth: nome do aplicativo/logotipo/página inicial/política de privacidade/termos todos presentes; domínio registrado
- [ ] Páginas de Política de Privacidade e Termos **acessíveis sem fazer login** (página personalizada do portal ou página estática independente)
- [] Ambos os links visíveis no cartão/rodapé de login do portal
- [] redirecionar URI corresponde ao domínio real e é https
- [] Publicar aplicativo → enviar para verificação; a restrição do usuário de teste é suspensa após a aprovação
- [] Configurações do painel OAuth: enabled=true com Client ID/Secret preenchido (o endpoint do provedor retorna enabled:true)

## 6. Comparação GitHub

GitHub Os aplicativos OAuth **não** têm estado de publicação ou fluxo de verificação: crie o aplicativo, defina o retorno de chamada URL e ele funcionará publicamente.
Observe que um aplicativo OAuth GitHub aceita apenas **um** retorno de chamada de autorização URL (correspondência de prefixo); para vários domínios,
use a abordagem multi-URI do Google ou crie um aplicativo separado por domínio do portal.

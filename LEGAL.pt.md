<!-- source: LEGAL.md | locale: pt | updated: 2026-10-23 -->
# Notas Legais e de Conformidade

> Esta tradução é fornecida apenas para conveniência. Em caso de divergência, prevalece o original em chinês.

> Idiomas: [简体中文](LEGAL.md) · [English](LEGAL.en.md) · [繁體中文](LEGAL.zh-TW.md) · [Русский](LEGAL.ru.md) · [日本語](LEGAL.ja.md) · [한국어](LEGAL.ko.md) · [Deutsch](LEGAL.de.md) · [Français](LEGAL.fr.md) · [Español](LEGAL.es.md) · Português · [Türkçe](LEGAL.tr.md) · [Bahasa Indonesia](LEGAL.id.md) · [ไทย](LEGAL.th.md) · [Tiếng Việt](LEGAL.vi.md) · [العربية](LEGAL.ar.md) · [فارسی](LEGAL.fa.md)

> Este documento explica a licença do CoreFusion, a política de uso comercial, as obrigações de terceiros e os pontos de conformidade que os usuários devem observar em suas próprias jurisdições.
>
> **Este documento não constitui aconselhamento jurídico.** A conformidade depende da região de implantação, dos clientes e do modelo operacional; consulte um advogado local qualificado.
>
> *Esta tradução é fornecida por conveniência. Se for diferente do original chinês, a versão chinesa prevalecerá.*

## 1. Licença do projeto

CoreFusion é lançado sob a **Licença Pública Geral GNU Affero v3.0 (AGPL-3.0)**; consulte [LICENSE](./LICENSE) para obter o texto completo.

```
Copyright (C) 2026 qist

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
```

## 2. Uso comercial e hospedagem comercial: expressamente permitido

**Qualquer indivíduo ou organização pode usar CoreFusion para fins comerciais, incluindo, mas não limitado a:**

- Auto-hospedagem e implantação interna;
- Executar um serviço pago de proxy/nó para terceiros;
- Fornecer SaaS, hospedagem, operações gerenciadas ou integração de sistemas a terceiros mediante pagamento de uma taxa;
- Desenvolvimento secundário, customização e distribuição (incluindo entrega com hardware ou como parte de uma solução).

**O uso comercial e a hospedagem comercial acima não requerem autorização separada e nenhuma taxa de licença ou divisão de receita.**

As únicas obrigações são as da própria AGPL-3.0, principalmente duas:

1. **Ao distribuir binários** (§4–§6): forneça aos destinatários a Fonte Correspondente (fonte correspondente completa) ou uma oferta efetiva por escrito sob §6; retenha os avisos de direitos autorais e o texto da licença.
2. **Ao fornecer uma versão modificada através de uma rede** (§13): se você publicar uma versão modificada e permitir que terceiros interajam com ela através de uma rede, você deverá **oferecer a esses usuários a fonte correspondente da versão modificada gratuitamente** (por exemplo, um link de download/repositório da fonte em destaque no painel).

Notas adicionais:

- A implantação de uma cópia **não modificada** e o atendimento aos usuários não aciona, por si só, a obrigação de fonte §13; a distribuição de binários ainda se enquadra no item 1.
- Não é proibido cobrar dinheiro; A AGPL rege a **disponibilidade da fonte**, não o modelo de negócios. Você não pode recusar o §13 para uma versão modificada simplesmente porque opera comercialmente ou com código fechado.
- Se você deseja uma **licença comercial de código fechado não sujeita à AGPL**, entre em contato com o detentor dos direitos autorais separadamente. Observe que este projeto vincula componentes GPL-3.0 de terceiros (consulte a seção 3); qualquer versão de código fechado deve removê-los primeiro.

## 3. Componentes de terceiros e redistribuição

Os principais componentes de terceiros e suas licenças estão listados em [THIRD_PARTY_LICENSES.md](LEGAL.pt.md).

Notas especiais:

- Por meio do `github.com/xtls/xray-core`, este projeto vincula indiretamente **GPL-3.0 ou posterior** `github.com/sagernet/sing` e `sing-shadowsocks`. Portanto **a distribuição de binários do painel ou do Agente também requer o cumprimento das obrigações de fonte correspondente da GPL-3.0**.
- Em tempo de execução, o painel pode baixar ou invocar programas independentes, como Xray-core, sing-box e Caddy. Eles **não são distribuídos neste repositório**, mas se você enviá-los com seus produtos, você deve cumprir suas licenças (especialmente a GPL-3.0 do sing-box: forneça sua fonte correspondente). Hysteria2 e TUIC são protocolos transportados por sing-box, não binários independentes.
- Os dados GeoIP/GeoSite agrupados (GeoLite2, `geoip.dat`, `geosite.dat`) estão sujeitos às suas próprias licenças de dados; verifique os termos antes do uso comercial.

## 4. Conformidade regional e responsabilidade do usuário

CoreFusion é um **software de uso geral**: ele não fornece nenhum proxy ou serviço de rede, não fornece nós, largura de banda ou credenciais e não cobra taxas. As operações são de responsabilidade do implantador.

Os implantadores/operadores devem determinar se seus negócios exigem licenças, registros ou avaliações de segurança em sua jurisdição, incluindo, entre outros:

- **China continental**: licenças comerciais de telecomunicações de valor agregado (ICP/IDC/ISP, etc.), arquivamento de ICP, MLPS, avaliação de segurança de transferência transfronteiriça de dados e obrigações sob a Lei de Segurança Cibernética, Lei de Segurança de Dados e Lei de Proteção de Informações Pessoais. Não use este software para fins que violem as leis locais.
- **UE/Reino Unido**: GDPR / GDPR do Reino Unido, privacidade eletrônica, obrigações do processador/controlador da Lei de Serviços Digitais (DSA), registro e conformidade com cookies.
- **Estados Unidos**: leis estaduais de privacidade, controles de exportação de criptografia (EAR) e triagem de sanções.
- **Outras regiões**: telecomunicações locais, comunicações criptografadas, gerenciamento de conteúdo, localização de dados e regras de proteção ao consumidor.

Os usuários concordam em não usar este software para:

- Atividades que violem a lei aplicável ou regras de controle/sanções de exportação;
- Infringir propriedade intelectual, privacidade ou outros direitos legais;
- Distribuir conteúdo ilegal, ataques à rede, fraude ou outras atividades criminosas.

**Qualquer responsabilidade legal decorrente do uso ou uso indevido deste software é do usuário.**

## 5. Proteção de dados e privacidade

O painel pode armazenar contas, detalhes de contato, pedidos, retornos de pagamento, assinaturas, estatísticas de tráfego e registros de login/operação. Como processador/controlador, os operadores devem:

- Fornecer aos usuários uma política de privacidade e obter o consentimento necessário;
- Seguir a minimização de dados e definir períodos de retenção razoáveis ​​para logs e estatísticas (o projeto fornece políticas de limpeza de logs que os operadores devem configurar);
- Suporte a solicitações de acesso, correção e exclusão de informações pessoais;
- Aplicar controle de acesso e criptografia a bancos de dados, backups, chaves privadas de certificados e credenciais de pagamento;
- Conclua avaliações de transferência de dados transfronteiriças ao implantar além-fronteiras.

## 6. Isenção de responsabilidade

Este software é fornecido “como está” de acordo com AGPL-3.0 §§15–17, sem qualquer garantia expressa ou implícita, incluindo comercialização, adequação a uma finalidade específica e não violação. O detentor dos direitos autorais não é responsável por danos diretos, indiretos, incidentais ou consequenciais decorrentes do uso deste software.

## 7. Contato

- Projeto: <https://github.com/qist/corefusion>
- Dúvidas sobre licenciamento comercial/conformidade: use os problemas do projeto ou os dados de contato publicados na página do repositório.

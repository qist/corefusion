<!-- source: LEGAL.md | locale: fr | updated: 2026-10-23 -->
# Notes juridiques et de conformité

> Cette traduction est fournie à titre indicatif. En cas de divergence, le texte original chinois fait foi.

> Langues : [简体中文](LEGAL.md) · [English](LEGAL.en.md) · [繁體中文](LEGAL.zh-TW.md) · [Русский](LEGAL.ru.md) · [日本語](LEGAL.ja.md) · [한국어](LEGAL.ko.md) · [Deutsch](LEGAL.de.md) · Français · [Español](LEGAL.es.md) · [Português](LEGAL.pt.md) · [Türkçe](LEGAL.tr.md) · [Bahasa Indonesia](LEGAL.id.md) · [ไทย](LEGAL.th.md) · [Tiếng Việt](LEGAL.vi.md) · [العربية](LEGAL.ar.md) · [فارسی](LEGAL.fa.md)

> Ce document explique la licence de CoreFusion, la politique d'utilisation commerciale, les obligations des tiers et les points de conformité que les utilisateurs doivent surveiller dans leurs propres juridictions.
>
> **Ce document ne constitue pas un avis juridique.** La conformité dépend de votre région de déploiement, de vos clients et de votre modèle opérationnel ; consultez un avocat local qualifié.
>
> *Cette traduction est fournie pour plus de commodité. Si elle diffère de l'original chinois, la version chinoise prévaut.*

## 1. Licence de projet

CoreFusion est publié sous la **Licence publique générale GNU Affero v3.0 (AGPL-3.0)** ; voir [LICENSE](./LICENSE) pour le texte intégral.

```
Copyright (C) 2026 qist

This program is free software: you can redistribute it and/or modify
it under the terms of the GNU Affero General Public License as published
by the Free Software Foundation, either version 3 of the License, or
(at your option) any later version.
```

## 2. Utilisation commerciale et hébergement commercial : expressément autorisés

**Tout individu ou organisation peut utiliser CoreFusion à des fins commerciales, y compris, mais sans s'y limiter :**

- Auto-hébergement et déploiement interne ;
- Exécuter un service proxy/nœud payant pour des tiers ;
- Fournir des services SaaS, d'hébergement, d'opérations gérées ou d'intégration de systèmes à des tiers moyennant des frais ;
- Développement secondaire, personnalisation et distribution (y compris la livraison avec du matériel ou dans le cadre d'une solution).

**L'utilisation commerciale et l'hébergement commercial ci-dessus ne nécessitent aucune autorisation distincte ni aucun frais de licence ni partage des revenus.**

Les seules obligations sont celles de l'AGPL-3.0 elle-même, principalement deux :

1. **Lors de la distribution de binaires** (§4–§6) : fournir aux destinataires la source correspondante (source correspondante complète) ou une offre écrite effective en vertu du §6 ; conserver les mentions de droits d'auteur et le texte de la licence.
2. **Lors de la fourniture d'une version modifiée sur un réseau** (§13) : si vous publiez une version modifiée et laissez des tiers interagir avec elle sur un réseau, vous devez **offrir gratuitement à ces utilisateurs la source correspondante de la version modifiée** (par exemple, un lien de téléchargement/dépôt de source bien visible dans le panneau).

Notes complémentaires :

- Le déploiement d'une copie **non modifiée** et le service aux utilisateurs ne déclenchent pas, en soi, l'obligation de source §13 ; la distribution de fichiers binaires relève toujours du point 1.
- Facturer de l’argent n’est pas interdit ; AGPL régit la **disponibilité des sources**, et non le modèle économique. Vous ne pouvez pas refuser le §13 pour une version modifiée simplement parce que vous opérez commercialement ou en source fermée.
- Si vous souhaitez une **licence commerciale à source fermée non soumise à l'AGPL**, contactez le détenteur des droits d'auteur séparément. Notez que ce projet lie des composants GPL-3.0 tiers (voir section 3) ; toute version fermée doit d'abord les supprimer.

## 3. Composants tiers et redistribution

Les principaux composants tiers et leurs licences sont répertoriés dans [THIRD_PARTY_LICENSES.md](LEGAL.fr.md).

Remarques spéciales :

- Grâce à `github.com/xtls/xray-core`, ce projet relie indirectement **GPL-3.0 ou version ultérieure** `github.com/sagernet/sing` et `sing-shadowsocks`. Par conséquent, **la distribution de binaires de panneau ou d'agent nécessite également de satisfaire aux obligations de source correspondante de la GPL-3.0**.
- Au moment de l'exécution, le panneau peut télécharger ou appeler des programmes autonomes tels que Xray-core, sing-box et Caddy. Ceux-ci ne sont **pas distribués dans ce référentiel**, mais si vous les expédiez avec vos livrables vous devez respecter leurs licences (notamment GPL-3.0 de sing-box : fournir sa source correspondante). Hysteria2 et TUIC sont des protocoles portés par sing-box, et non des binaires autonomes.
- Les données GeoIP/GeoSite groupées (GeoLite2, `geoip.dat`, `geosite.dat`) sont soumises à leurs propres licences de données ; vérifier les termes avant utilisation commerciale.

## 4. Conformité régionale et responsabilité des utilisateurs

CoreFusion est un **logiciel à usage général** : il ne fournit aucun service proxy ou réseau, ne fournit aucun nœud, bande passante ou informations d'identification et ne perçoit aucun frais. Les opérations relèvent de la responsabilité du déployeur.

Les déployeurs/opérateurs doivent déterminer si leur entreprise nécessite des licences, des dépôts ou des évaluations de sécurité dans leur juridiction, y compris, mais sans s'y limiter :

- **Chine continentale** : licences commerciales de télécommunications à valeur ajoutée (ICP/IDC/ISP, etc.), dépôt ICP, MLPS, évaluation de la sécurité du transfert de données transfrontalier et obligations en vertu de la loi sur la cybersécurité, de la loi sur la sécurité des données et de la loi sur la protection des informations personnelles. N'utilisez pas ce logiciel à des fins qui violent les lois locales.
- **UE/Royaume-Uni** : RGPD/RGPD du Royaume-Uni, ePrivacy, obligations du processeur/contrôleur du Digital Services Act (DSA), conformité en matière de journalisation et de cookies.
- **États-Unis** : lois des États sur la confidentialité, contrôles des exportations par chiffrement (EAR) et contrôle des sanctions.
- **Autres régions** : télécommunications locales, communications cryptées, gestion de contenu, localisation des données et règles de protection des consommateurs.

Les utilisateurs s'engagent à ne pas utiliser ce logiciel pour :

- Activités qui violent la loi applicable ou les règles de contrôle/sanctions à l’exportation ;
- Violer la propriété intellectuelle, la vie privée ou d'autres droits légaux ;
- Distribution de contenu illégal, attaques de réseau, fraude ou autres activités criminelles.

**Toute responsabilité légale découlant de l'utilisation ou de la mauvaise utilisation de ce logiciel incombe à l'utilisateur.**

## 5. Protection des données et confidentialité

Le panneau peut stocker des comptes, des coordonnées, des commandes, des rappels de paiement, des abonnements, des statistiques de trafic et des journaux de connexion/opération. En tant que sous-traitant/contrôleur, l’opérateur doit :

- Fournir aux utilisateurs une politique de confidentialité et obtenir le consentement nécessaire ;
- Suivre la minimisation des données et définir des périodes de conservation raisonnables pour les journaux et les statistiques (le projet fournit des politiques de nettoyage des journaux que les opérateurs doivent configurer) ;
- Demandes d’assistance pour accéder, corriger et supprimer des informations personnelles ;
- Appliquer le contrôle d'accès et le cryptage aux bases de données, aux sauvegardes, aux clés privées de certificat et aux informations d'identification de paiement ;
- Effectuez des évaluations du transfert de données transfrontalier lors du déploiement au-delà des frontières.

## 6. Avis de non-responsabilité

Ce logiciel est fourni « tel quel » sous AGPL-3.0 §§15-17, sans aucune garantie expresse ou implicite, y compris la qualité marchande, l'adéquation à un usage particulier et la non-contrefaçon. Le détenteur des droits d'auteur n'est pas responsable des dommages directs, indirects, accessoires ou consécutifs résultant de l'utilisation de ce logiciel.

## 7. Contacter

- Projet : <https://github.com/qist/corefusion>
- Questions de licence commerciale/conformité : utilisez les problèmes du projet ou les coordonnées publiées sur la page du référentiel.

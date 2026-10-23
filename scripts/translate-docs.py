#!/usr/bin/env python3
"""Machine-translate the English docs into another locale.

The English edition is treated as the source of truth for structure; the
Chinese original remains the canonical source recorded in the metadata.
Code spans, paths, links and HTML are replaced by tokens before translation
so the Markdown structure survives.
"""
import argparse
import json
import os
import re
import sys
import time

import requests

ROOT = '/opt/corefusion'

LOCALE_NAMES = {
    'zh-CN': '简体中文',
    'en': 'English',
    'zh-TW': '繁體中文',
    'ru': 'Русский',
    'ja': '日本語',
    'ko': '한국어',
    'de': 'Deutsch',
    'fr': 'Français',
    'es': 'Español',
    'pt': 'Português',
    'tr': 'Türkçe',
    'id': 'Bahasa Indonesia',
    'th': 'ไทย',
    'vi': 'Tiếng Việt',
    'ar': 'العربية',
    'fa': 'فارسی',
}

BAR_PREFIX = {
    'zh-CN': '语言：',
    'en': 'Languages:',
    'zh-TW': '語言：',
    'ru': 'Языки:',
    'ja': '言語：',
    'ko': '언어:',
    'de': 'Sprachen:',
    'fr': 'Langues :',
    'es': 'Idiomas:',
    'pt': 'Idiomas:',
    'tr': 'Diller:',
    'id': 'Bahasa:',
    'th': 'ภาษา:',
    'vi': 'Ngôn ngữ:',
    'ar': 'اللغات:',
    'fa': 'زبان\u200cها:',
}

DISCLAIMER = {
    'ru': 'Этот перевод предоставлен для удобства. В случае расхождений преимущественную силу имеет китайский оригинал.',
    'ja': 'この翻訳は便宜上のものです。相違がある場合は中国語の原文が優先されます。',
    'ko': '이 번역은 편의를 위해 제공됩니다. 차이가 있을 경우 중국어 원문이 우선합니다.',
    'de': 'Diese Übersetzung dient nur der Orientierung. Bei Abweichungen ist der chinesische Originaltext maßgeblich.',
    'fr': 'Cette traduction est fournie à titre indicatif. En cas de divergence, le texte original chinois fait foi.',
    'es': 'Esta traducción se ofrece únicamente para facilitar la consulta. En caso de discrepancia, prevalece el original en chino.',
    'pt': 'Esta tradução é fornecida apenas para conveniência. Em caso de divergência, prevalece o original em chinês.',
    'tr': 'Bu çeviri yalnızca kolaylık sağlamak içindir. Tutarsızlık olması durumunda Çince asıl metin geçerlidir.',
    'id': 'Terjemahan ini disediakan untuk kemudahan. Jika ada perbedaan, teks asli bahasa Mandarin yang berlaku.',
    'th': 'คำแปลนี้จัดทำขึ้นเพื่อความสะดวก หากมีความแตกต่าง ให้ยึดต้นฉบับภาษาจีนเป็นหลัก',
    'vi': 'Bản dịch này chỉ nhằm mục đích tham khảo. Nếu có sai khác, bản gốc tiếng Trung sẽ được ưu tiên.',
    'ar': 'هذه الترجمة مقدمة للتسهيل فقط. في حال وجود اختلاف، تكون الأولوية للنص الأصلي بالصينية.',
    'fa': 'این ترجمه فقط برای سهولت ارائه شده است. در صورت اختلاف، متن اصلی چینی معتبر است.',
}

METHODS = {'GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS', 'CONNECT', 'TRACE',
           'GET/PUT', 'GET/POST', 'POST/PUT', 'GET/DELETE'}

PREFIX_RE = re.compile(r'^(\s*(?:#{1,6}\s+|>\s?|[-*+]\s+|\d+[.)]\s+)+)(.*)$')
SEP_RE = re.compile(r'^\s*\|?[\s:|-]+\|[\s:|-]*$')
LINK_EN_RE = re.compile(r'\]\(([^)#]+)\.en\.md(#[^)]*)?\)')

URL = 'https://clients5.google.com/translate_a/t?client=dict-chrome-ex&sl=en&tl={}'

BRAND_RE = re.compile(
    r'(?i)\b(CoreFusion|Telegram|GitHub|GitLab|Docker|Nginx|Xray|sing-box|VLESS|VMess|'
    r'Trojan|Shadowsocks|Hysteria2|TUIC|WireGuard|Cloudflare|ZeroSSL|Google|Apple|'
    r'Windows|Linux|macOS|Android|iOS|systemd|SQLite|MySQL|PostgreSQL|Redis|WebDAV|'
    r'FTP|SMTP|JWT|TOTP|WebAuthn|Passkey|REALITY|X25519|MLDSA65|ECH|ACME|EAB|WAF|'
    r'CDN|DNS|HTTP|HTTPS|API|CLI|YAML|JSON|CSV|UUID|URL|URI|TCP|UDP|QUIC|TLS|SNI|'
    r'S3|SLA|SSH|SFTP)\b'
)


class Protector:
    def __init__(self):
        self.map = {}
        self.n = 0

    def tok(self, s):
        self.n += 1
        t = 'ZXQ%04dQZ' % self.n
        self.map[t] = s
        return t

    def protect(self, text):
        text = re.sub(r'`[^`\n]*`', lambda m: self.tok(m.group(0)), text)
        text = re.sub(r'!\[[^\]\n]*\]\([^)\n]*\)', lambda m: self.tok(m.group(0)), text)
        text = re.sub(r'\[([^\]\n]+)\]\(([^)\n]+)\)',
                      lambda m: self.tok('[') + m.group(1) + self.tok(']') + self.tok('(' + m.group(2) + ')'), text)
        text = re.sub(r'<https?://[^>\s]+>', lambda m: self.tok(m.group(0)), text)
        text = re.sub(r'https?://[^\s)>\]]+', lambda m: self.tok(m.group(0)), text)
        text = re.sub(r'<!--.*?-->', lambda m: self.tok(m.group(0)), text)
        text = re.sub(r'<[^>\n]+>', lambda m: self.tok(m.group(0)), text)
        text = re.sub(r'(?<![\w/])(?:[A-Za-z0-9_.-]+/)*[A-Za-z0-9_.-]+\.(?:md|sh|go|json|js|ts|yaml|yml|toml|conf|service|log|env)\b', lambda m: self.tok(m.group(0)), text)
        text = re.sub(r'\bzh-(?:CN|TW)\b', lambda m: self.tok(m.group(0)), text)
        text = re.sub(BRAND_RE, lambda m: self.tok(m.group(0)), text)
        return text

    def restore(self, text):
        idx = {t[3:7]: v for t, v in self.map.items()}
        text = re.sub(r'[A-Z]{3}\s?(\d{4})\s?[A-Z]{2}',
                      lambda m: idx.get(m.group(1), m.group(0)), text)
        for _ in range(4):
            changed = False
            for t, v in self.map.items():
                if t in text:
                    text = text.replace(t, v)
                    changed = True
            if not changed:
                break
        text = re.sub(r'\]\s+\(', '](', text)
        text = re.sub(r'[ \t]{2,}', ' ', text)
        return text


SESSION = requests.Session()
SESSION.headers.update({
    'User-Agent': 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 '
                  '(KHTML, like Gecko) Chrome/120 Safari/537.36',
})


def google(text, locale, tries=4):
    last = None
    for attempt in range(tries):
        try:
            r = SESSION.post(URL.format(locale), data={'q': text}, timeout=30)
            if r.status_code == 200:
                data = r.json()
                if isinstance(data, list) and data:
                    if len(data) == 1 and isinstance(data[0], str):
                        return data[0]
                    if all(isinstance(x, str) for x in data):
                        return '\n'.join(data)
            last = 'HTTP %s' % r.status_code
        except Exception as exc:  # noqa: BLE001
            last = str(exc)
        time.sleep(1.5 * (attempt + 1))
    raise RuntimeError('translate failed: %s' % last)


def split_line(line):
    """Return (prefix, body) for a prose line."""
    m = PREFIX_RE.match(line)
    if m:
        return m.group(1), m.group(2)
    return '', line


def parse(path):
    lines = open(path, encoding='utf-8').read().splitlines()
    if lines and lines[0].startswith('<!-- source:'):
        lines = lines[1:]
    lines = [l for l in lines if not re.match(r'^> (Languages|语言|語言|Языки|言語|언어|Sprachen|Langues|Idiomas|Diller|Bahasa|ภาษา|Ngôn ngữ|اللغات|زبان)', l)]
    while lines and lines[0].strip() == '':
        lines.pop(0)

    out = []
    in_code = False
    in_html = False
    for line in lines:
        if line.strip().startswith('```'):
            in_code = not in_code
            out.append(('raw', line))
            continue
        if in_code:
            out.append(('raw', line))
            continue
        if line.strip() == '':
            out.append(('raw', line))
            continue
        if line.strip().startswith('<!--'):
            out.append(('raw', line))
            continue
        if line.lstrip().startswith('<') and line.rstrip().endswith('>'):
            out.append(('raw', line))
            continue

        if line.lstrip().startswith('|'):
            if SEP_RE.match(line):
                out.append(('raw', line))
                continue
            cells = line.strip().strip('|').split('|')
            parsed = []
            for cell in cells:
                core = cell.strip()
                if core == '':
                    parsed.append((False, ''))
                elif core in METHODS or core.startswith('/') or core.startswith('http'):
                    parsed.append((False, core))
                elif re.fullmatch(r'`[^`]*`', core):
                    parsed.append((False, core))
                elif re.fullmatch(r'[-–—\d.%/:]+', core):
                    parsed.append((False, core))
                elif core in LOCALE_NAMES or re.fullmatch(r'(?:zh-CN|zh-TW|[a-z]{2})\s+✅', core) or re.fullmatch(r'[A-Za-z0-9_./-]+\.(md|sh|go|json|js|ts|yaml|yml|toml|conf|service|log|env)', core):
                    parsed.append((False, core))
                else:
                    parsed.append((True, core))
            out.append(('table', parsed))
            continue

        prefix, body = split_line(line)
        if body.strip() == '' or re.fullmatch(r'[-*_]{3,}', body.strip()):
            out.append(('raw', line))
            continue
        out.append(('prose', prefix, body))
    return out


def translate_doc(src, dst, locale, job):
    parsed = parse(src)
    prot = Protector()
    units = []

    for idx, item in enumerate(parsed):
        if item[0] == 'prose':
            _, prefix, body = item
            prot_body = prot.protect(body)
            units.append((idx, None, prot_body))
            parsed[idx] = ('prose', prefix, prot_body)
        elif item[0] == 'table':
            cells = item[1]
            new = []
            for cidx, (flag, core) in enumerate(cells):
                if flag:
                    prot_core = prot.protect(core)
                    units.append((idx, cidx, prot_core))
                    new.append((True, prot_core))
                else:
                    new.append((False, core))
            parsed[idx] = ('table', new)

    # Batch units into requests.
    translated = {}
    pos = 0
    while pos < len(units):
        chunk = []
        size = 0
        while pos < len(units) and len(chunk) < 30 and size < 4000:
            u = units[pos]
            chunk.append(u)
            size += len(u[2]) + 1
            pos += 1
        text = '\n'.join(u[2] for u in chunk)
        try:
            result = google(text, locale)
            parts = result.split('\n')
            if len(parts) != len(chunk):
                raise RuntimeError('line mismatch %d != %d' % (len(parts), len(chunk)))
        except Exception as exc:  # noqa: BLE001
            print('  fallback %s: %s' % (job, exc), file=sys.stderr)
            parts = []
            for u in chunk:
                parts.append(google(u[2], locale))
                time.sleep(0.2)
        for u, part in zip(chunk, parts):
            translated[(u[0], u[1])] = part
        time.sleep(0.25)

    out_lines = []
    for idx, item in enumerate(parsed):
        if item[0] == 'raw':
            out_lines.append(item[1])
        elif item[0] == 'prose':
            _, prefix, _ = item
            body = prot.restore(translated[(idx, None)])
            out_lines.append(prefix + body if prefix else body)
        else:
            cells = []
            for cidx, (flag, core) in enumerate(item[1]):
                if flag:
                    cells.append(prot.restore(translated[(idx, cidx)]))
                else:
                    cells.append(core)
            out_lines.append('| ' + ' | '.join(cells) + ' |')

    text = '\n'.join(out_lines).strip('\n') + '\n'
    text = LINK_EN_RE.sub(lambda m: '](%s.%s.md%s)' % (m.group(1), locale, m.group(2) or ''), text)
    return normalize_terms(fix_anchors(text), locale)


def slug(value):
    value = value.lower().strip()
    value = re.sub(r'[^\w\s-]', '', value, flags=re.UNICODE)
    return re.sub(r'\s+', '-', value)


def fix_anchors(text):
    lines = text.splitlines()
    first = None
    for i, line in enumerate(lines):
        if re.match(r'^\s*(?:\d+\.\s+|[-*]\s+)\[.+?\]\(#[^)]*\)$', line):
            first = i
            break
    if first is None:
        return text
    bullet = lines[first].lstrip().startswith('-')
    hprefix = '### ' if bullet else '## '
    hlen = len(hprefix)
    heads = []
    for line in lines[first:]:
        if line.startswith(hprefix):
            heads.append(line[hlen:].strip())
    toc_idx = [i for i, line in enumerate(lines[first:], start=first)
               if re.match(r'^\s*(?:\d+\.\s+|[-*]\s+)\[.+?\]\(#[^)]*\)$', line)]
    if not heads or not toc_idx:
        return text
    for pos, i in enumerate(toc_idx):
        if pos >= len(heads):
            break
        m = re.match(r'^(\s*(?:\d+\.\s+|[-*]\s+))\[(.+?)\]\(#[^)]*\)$', lines[i])
        if m:
            label = re.sub(r'^\d+\.\s*', '', heads[pos])
            lines[i] = '%s[%s](#%s)' % (m.group(1), label, slug(heads[pos]))
    return '\n'.join(lines)


def normalize_terms(text, locale):
    if locale == 'ja':
        out = []
        for line in text.splitlines():
            if line.lstrip().startswith('|'):
                line = re.sub(r'\|\s*公共\s*\|', '| 公開 |', line)
                line = re.sub(r'\|\s*サインインしました\s*\|', '| 要サインイン |', line)
            out.append(line)
        return '\n'.join(out)
    if locale != 'ru':
        return text
    out = []
    for line in text.splitlines():
        if line.lstrip().startswith('|'):
            line = re.sub(r'\|\s*Общественный\s*\|', '| Публичный |', line)
            line = re.sub(r'\|\s*Вошёл в систему\s*\|', '| Требуется вход |', line)
            line = re.sub(r'\|\s*Админ\s*\|', '| Администратор |', line)
            line = re.sub(r'\|\s*Общественный · ограниченный\s*\|', '| Публичный · с ограничением |', line)
            line = line.replace('Другие регионы', 'Другие языки')
            line = line.replace('другие регионы', 'другие языки')
        out.append(line)
    return '\n'.join(out)


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument('locale')
    ap.add_argument('--only', default=None)
    args = ap.parse_args()
    locale = args.locale

    files = [
        'README',
        'LEGAL',
        'THIRD_PARTY_LICENSES',
        'docs/README',
        'docs/USER-GUIDE',
        'docs/api/README',
        'docs/api/subscription',
        'docs/api/portal-api',
        'docs/api/charge-callback',
        'docs/api/admin-api',
        'docs/cert-google-eab',
        'docs/oauth-google-verification',
        'docs/notification-templates',
    ]
    if args.only:
        files = [f for f in files if args.only in f]

    for stem in files:
        src = os.path.join(ROOT, stem + '.en.md')
        zh = os.path.join(ROOT, stem + '.md')
        dst = os.path.join(ROOT, '%s.%s.md' % (stem, locale))
        if not os.path.exists(src):
            print('skip missing', src)
            continue
        print('translate', src, '->', dst)
        body = translate_doc(src, dst, locale, stem)

        base = os.path.basename(stem)
        h1, _, rest = body.partition('\n')
        rest = rest.lstrip('\n')
        header = [
            '<!-- source: %s.md | locale: %s | updated: 2026-10-23 -->' % (base, locale),
            h1,
            '',
        ]
        if stem == 'LEGAL':
            header += ['> ' + DISCLAIMER[locale], '']
        header += ['> %s %s' % (BAR_PREFIX[locale], bar_text(stem, locale)), '', '']
        body = '\n'.join(header) + rest
        open(dst, 'w', encoding='utf-8').write(body)
        print('  wrote', len(body.splitlines()), 'lines')


def bar_text(stem, current):
    parts = []
    for loc in ['zh-CN', 'en', 'zh-TW', 'ru', 'ja', 'ko', 'de', 'fr', 'es', 'pt', 'tr', 'id', 'th', 'vi', 'ar', 'fa']:
        name = LOCALE_NAMES[loc]
        if loc == current:
            parts.append(name)
            continue
        path = '%s.%s.md' % (stem, loc) if loc != 'zh-CN' else stem + '.md'
        if not os.path.exists(os.path.join(ROOT, path)):
            continue
        parts.append('[%s](%s)' % (name, os.path.basename(path)))
    return ' · '.join(parts)


if __name__ == '__main__':
    main()

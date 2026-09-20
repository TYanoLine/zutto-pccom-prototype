#!/usr/bin/env python3
"""Extract non-expressive protocol evidence from archived Erika-K BBS logs.

The collector stores third-party bodies outside Git. This analyzer reads those
normalized files and emits only compact, source-attributed behavioral evidence:
prompts, commands, menu/control vocabulary, login/session markers, and article
structure. It intentionally does not copy article/chat bodies into its output.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import re
from collections import Counter, defaultdict
from dataclasses import dataclass, asdict
from pathlib import Path
from typing import Iterable

PROMPT_RE = re.compile(r"^(?P<prefix>.{0,120}?)(?P<prompt>(?:MAIN|CALL|BOARD|MAIL|FILE|CHAT|JUNK|MODE|[A-Z][A-Z0-9_\\-]{1,20})[^\n]{0,80}?->)\s*(?P<input>.*)$", re.I)
BOARD_PROMPT_RE = re.compile(r"^(?P<prefix>\([^)]*\)\s*)?BOARD>M:MENU(?:\s+\?:HELP)?\s*->\s*(?P<input>.*)$", re.I)
MENU_COMMAND_RE = re.compile(r"\(\s*(?P<command>[A-Z][A-Z0-9_/-]{0,15})\s*\)")
BRACKET_COMMAND_RE = re.compile(r"[\[〖](?P<key>[^\]〗]{1,16})[\]〗]")
APPEND_EVENT_RE = re.compile(r"^APPEND\s+(?P<board>\d+)\s+(?P<article>\d+)\s*$", re.I)
ARTICLE_LIST_RE = re.compile(
    r"^(?P<board>\d{1,3})\s*(?:--|\s+)\s*(?P<article>\d+)\s+"
    r"(?P<date>\d{2}/\d{2}/\d{2})\s+(?P<time>\d{2}:\d{2})\s+"
    r"(?P<author>\S+)(?:\s+(?P<append_count>\d+))?(?:\s+.*)?$"
)
TIMESTAMP_RE = re.compile(r"^(?P<date>\d{2}/\d{2}/\d{2})\s+(?P<time>\d{2}:\d{2}:\d{2})\s+(?P<author>\S+)(?:\s+.*)?$")
VERSION_RE = re.compile(r"ERIKA[- ]K\s*(?:VER(?:SION)?\.?\s*)?(?P<version>\d+(?:\.\d+)*)", re.I)

MARKERS = {
    "login_id_prompt": ("Input Your ID:", "YOUR ID:"),
    "password_prompt": ("PASSWORD:", "Password:"),
    "previous_access": ("前回アクセス", "previous access"),
    "disconnect": ("NO CARRIER", "Disconnected"),
    "chat_on": ("CHAT ON", "## chat on"),
    "who_surface": ("アクセス状況(WHO)", "WHO)"),
    "pause_key": ("Ｓキー", "Sキー"),
    "abort_key": ("Ｚキー", "Zキー"),
    "append_label": ("ｱﾍﾟ", "アペ", "APPEND"),
}

SENSITIVE_LINE_HINTS = (
    "振込先", "口座", "住所", "電話番号", "TEL", "PHONE", "@", "http://", "https://"
)

@dataclass(frozen=True)
class Evidence:
    source: str
    line: int
    kind: str
    value: str
    line_sha256: str


def line_hash(line: str) -> str:
    return hashlib.sha256(line.encode("utf-8", "replace")).hexdigest()[:16]


def safe_value(kind: str, line: str, match: re.Match[str] | None = None) -> str:
    """Return compact behavioral evidence, never arbitrary message bodies."""
    if kind == "prompt" and match:
        prompt = (match.groupdict().get("prompt") or line.rsplit("->", 1)[0] + "->").strip()
        entered = (match.groupdict().get("input") or "").strip()
        return f"{prompt} {entered}".strip()[:180]
    if kind == "append_event" and match:
        return f"APPEND {match.group('board')} {match.group('article')}"
    if kind == "article_list" and match:
        ac = match.group("append_count") or "0"
        return f"board={match.group('board')} article={match.group('article')} numeric_field_after_author={ac}"
    if kind == "article_timestamp":
        return "article-or-append timestamp line"
    if kind == "version" and match:
        return match.group(0)[:80]
    if any(h.lower() in line.lower() for h in SENSITIVE_LINE_HINTS):
        return "[redacted behavioral marker]"
    return line.strip()[:180]


def add(out: list[Evidence], source: str, lineno: int, kind: str, value: str, raw_line: str) -> None:
    out.append(Evidence(source=source, line=lineno, kind=kind, value=value, line_sha256=line_hash(raw_line)))


def analyze_file(path: Path, root: Path) -> list[Evidence]:
    source = path.relative_to(root).as_posix()
    out: list[Evidence] = []
    lines = path.read_text(encoding="utf-8", errors="replace").splitlines()
    for lineno, line in enumerate(lines, 1):
        stripped = line.strip()
        if not stripped:
            continue

        vm = VERSION_RE.search(stripped)
        if vm:
            add(out, source, lineno, "version", safe_value("version", stripped, vm), line)

        pm = BOARD_PROMPT_RE.match(stripped) or PROMPT_RE.match(stripped)
        if pm:
            add(out, source, lineno, "prompt", safe_value("prompt", stripped, pm), line)

        am = APPEND_EVENT_RE.match(stripped)
        if am:
            add(out, source, lineno, "append_event", safe_value("append_event", stripped, am), line)

        lm = ARTICLE_LIST_RE.match(stripped)
        if lm:
            add(out, source, lineno, "article_list", safe_value("article_list", stripped, lm), line)

        tm = TIMESTAMP_RE.match(stripped)
        if tm:
            add(out, source, lineno, "article_timestamp", safe_value("article_timestamp", stripped, tm), line)

        for mm in MENU_COMMAND_RE.finditer(stripped):
            add(out, source, lineno, "direct_command", mm.group("command").upper(), line)

        if any(token in stripped for token in ("読む", "戻る", "HELP", "ﾒﾆｭｰ", "メニュー", "サービス", "電報", "書く", "実行", "変更")):
            for bm in BRACKET_COMMAND_RE.finditer(stripped):
                key = bm.group("key").strip()
                if 1 <= len(key) <= 16:
                    add(out, source, lineno, "menu_key", key, line)

        for marker, needles in MARKERS.items():
            if any(n.lower() in stripped.lower() for n in needles):
                add(out, source, lineno, "marker", marker, line)

    unique = {(e.source, e.line, e.kind, e.value): e for e in out}
    return list(unique.values())


def iter_logs(root: Path) -> Iterable[Path]:
    for p in sorted(root.rglob("*.txt")):
        if "erika-k" in p.as_posix().lower():
            yield p


def confidence(files: int) -> str:
    if files >= 2:
        return "confirmed-repeated"
    return "station-specific-or-single-source"


def render_markdown(evidence: list[Evidence], file_count: int) -> str:
    kinds: dict[str, list[Evidence]] = defaultdict(list)
    for e in evidence:
        kinds[e.kind].append(e)

    values_by_kind: dict[str, Counter[str]] = {
        kind: Counter(e.value for e in rows) for kind, rows in kinds.items()
    }
    files_by_value: dict[tuple[str, str], set[str]] = defaultdict(set)
    for e in evidence:
        files_by_value[(e.kind, e.value)].add(e.source)

    lines = [
        "# 絵理香K版ログ由来・機械抽出仕様",
        "",
        "> Generated from locally archived normalized logs. Third-party message bodies are not reproduced.",
        "",
        f"- analyzed files: {file_count}",
        f"- evidence records: {len(evidence)}",
        "",
        "## Evidence policy",
        "",
        "- `confirmed-repeated`: 同じ構文・機能が2ファイル以上で観測された。",
        "- `station-specific-or-single-source`: 1ファイルのみ。K版共通仕様とは断定しない。",
        "- 行番号と短い SHA-256 を保持し、元ログで再検証できるようにする。",
        "",
    ]

    headings = [
        ("version", "Version identifiers"),
        ("prompt", "Prompt / state surfaces"),
        ("direct_command", "Direct commands"),
        ("menu_key", "Menu/control keys"),
        ("marker", "Behavioral markers"),
        ("append_event", "Append semantics"),
        ("article_list", "Board/article list structure"),
    ]
    for kind, title in headings:
        lines += [f"## {title}", ""]
        counter = values_by_kind.get(kind, Counter())
        if not counter:
            lines += ["- No observations.", ""]
            continue
        for value, count in counter.most_common():
            fs = sorted(files_by_value[(kind, value)])
            conf = confidence(len(fs))
            examples = [e for e in kinds[kind] if e.value == value][:3]
            refs = ", ".join(f"`{e.source}:{e.line}`#{e.line_sha256}" for e in examples)
            lines.append(f"- **{value}** — {count} observations / {len(fs)} file(s) — `{conf}` — {refs}")
        lines.append("")

    return "\n".join(lines) + "\n"


def main() -> int:
    ap=argparse.ArgumentParser(); ap.add_argument("--archive",type=Path,required=True); ap.add_argument("--json",type=Path,required=True); ap.add_argument("--markdown",type=Path,required=True); a=ap.parse_args()
    paths=list(iter_logs(a.archive)); evidence=[]
    for p in paths: evidence.extend(analyze_file(p,a.archive))
    a.json.parent.mkdir(parents=True,exist_ok=True); a.markdown.parent.mkdir(parents=True,exist_ok=True)
    a.json.write_text(json.dumps({"schema_version":1,"analyzed_files":[p.relative_to(a.archive).as_posix() for p in paths],"evidence":[asdict(e) for e in evidence]},ensure_ascii=False,indent=2)+"\n",encoding="utf-8")
    a.markdown.write_text(render_markdown(evidence,len(paths)),encoding="utf-8")
    print(f"analyzed {len(paths)} file(s), {len(evidence)} evidence record(s)")
    return 0

if __name__=="__main__": raise SystemExit(main())

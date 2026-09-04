#!/usr/bin/env python3
"""Polite, reproducible collector for Japanese PC communications sources.

Third-party bodies are written only below --output. Repository-safe metadata can
be copied out separately after review. This collector intentionally has no
recursive crawler.
"""
from __future__ import annotations

import argparse, csv, hashlib, html, json, re, shutil, sys, time, urllib.error, urllib.parse, urllib.request
from charset_normalizer import from_bytes
from datetime import datetime, timezone
from html.parser import HTMLParser
from pathlib import Path
from urllib.robotparser import RobotFileParser
from zipfile import ZIP_DEFLATED, ZipFile

UA = "zutto-pccom-primary-source-archiver/1.0 (historical research; single connection)"

SOURCES = [
    ("pekeroku-index", "pekeroku", "grassroots/pekeroku/raw/index.html", "https://www.asahi-net.or.jp/~uv2s-oob/x68/", "later-preservation", "html", True),
    *[(f"pekeroku-{period}-{name}", "pekeroku", f"grassroots/pekeroku/raw/{period}/{name}.bbs", f"https://www.asahi-net.or.jp/~uv2s-oob/x68/{period}/{name}.bbs", "confirmed-original", "bbs-log", True)
      for period, names in (("88_89", "game sf av food h gnu pc music jr theme play x68000"), ("90", "free game sf av food h gnu pc music jr theme play x68000 hard")) for name in names.split()],
    ("midnight-top", "Midnight Driving", "grassroots/midnight-driving/raw/index.html", "https://webmid.kinet.ne.jp/", "station-specific", "html", True),
    ("midnight-guide", "Midnight Driving", "grassroots/midnight-driving/raw/wtsbbs/index.html", "https://webmid.kinet.ne.jp/mid/manual/wtsbbs/", "station-specific", "manual-html", True),
    *[(f"mash-{name.lower()}", "Midnight Driving", f"grassroots/midnight-driving/raw/{name}-utf8.txt", f"https://webmid.kinet.ne.jp/mid/manual/wtsbbs/manual/MASHMAN/{name}-utf8.txt", "confirmed-manual", "manual", False) for name in ("USER", "COMMANDS", "APPENDIX", "HNOTES")],
    ("minkymoon-30years", "Minkymoon Network", "grassroots/minkymoon/raw/minkymoonnetwork-30years.html", "https://minkymoon.jp/2022/10/09/minkymoonnetwork-30years/", "later-preservation", "html", True),
    ("bickle-index", "Bickle Net", "grassroots/bickle/raw/index.html", "https://bm98.yaneu.com/bickle/", "later-preservation", "html", True),
    *[(f"bickle-{i}", "Bickle Net", f"grassroots/bickle/raw/bick{i}.html", f"https://bm98.yaneu.com/bickle/bick{i}.html", "station-specific", "html", True) for i in range(1,18)],
    *[(f"minkymoon-image-{i}", "Minkymoon Network", f"grassroots/minkymoon/captures/{name}", f"https://minkymoon.jp/wordpress/wp-content/uploads/2022/10/{name}", "confirmed-original-log-in-retrospective", "image", True) for i,name in enumerate(("MMN-login-9801.jpg","MMN-main.jpg","MMN-kaisen.jpg","MMN-freetalk.png","MMN-acesslog.png","MMN-logout-9801.jpg"),1)],
    ("yokohama-totsuka", "Yokohama Totsuka BBS", "grassroots/yokohama-totsuka/raw/MemoryOfYTBBS2.html", "https://www.jh1ifz.com/aboutComputer/MemoryOfYTBBS2.html", "later-preservation", "html", True),
    ("hanzou-index", "Hanzou BBS", "grassroots/hanzou-bbs/captures/index.html", "https://www.hanzou.jp/hanzoubbs/index.php", "confirmed-live-session", "html", True),
    ("tcm-top", "T.C.M.network", "grassroots/tcm-network/raw/index.html", "https://tcm.jp/", "station-specific", "html", True),
    ("tcm-guide", "T.C.M.network", "grassroots/tcm-network/raw/guide.html", "https://tcm.jp/member/guide.html", "confirmed-manual", "manual-html", True),
    ("tokyo-bbs-index", "Tokyo BBS", "grassroots/tokyo-bbs/raw/category-320155-1.html", "https://mubou.seesaa.net/category/320155-1.html", "discovery-only", "html", True),
    ("jipdec-1986", "JIPDEC", "commercial-reference/jipdec/J0001147.pdf", "https://www.jipdec.or.jp/archives/publications/J0001147.pdf", "confirmed-contemporary-report", "pdf", False),
    ("discovery-hally", "source index", "discovery/source-indexes/hally-20050612.html", "https://hally.hatenadiary.com/entry/20050612/p1", "discovery-only", "index", True),
    ("aaa-top", "AAA", "grassroots/aaa/raw/index.html", "https://www.aaa-int.jp/", "discovery-only", "html", True),
    ("sohonzan", "Kisekae Sohonzan", "grassroots/other/sohonzan/raw/index.html", "https://emk.name/sohonzan/shz.cgi", "discovery-only", "html", True),
    ("pcvan-keiei", "PC-VAN Keiei SIG", "commercial-reference/pc-van/keieisig/index.html", "https://www.zenko3.com/keieisig/", "later-preservation", "html", True),
    ("pcvan-rumic", "PC-VAN Rumic World SIG", "commercial-reference/pc-van/rumic/index.html", "https://www.rumic.org/forum/", "later-preservation", "html", True),
    ("nifty-ftrain", "NIFTY-Serve FTRAIN", "commercial-reference/nifty/logtty_setumei.html", "https://www.railforum.jp/ftrain/public/logtty_setumei.html", "later-preservation", "html", True),
    ("vector-log-tools", "Vector", "commercial-reference/software/vector-log/index.html", "https://www.vector.co.jp/vpack/filearea/dos/net/comm/log/", "later-preservation", "software-index", True),
]

class Links(HTMLParser):
    def __init__(self): super().__init__(); self.urls=[]
    def handle_starttag(self, tag, attrs):
        if tag in {"a", "img"}:
            d=dict(attrs); u=d.get("href") or d.get("src")
            if u: self.urls.append(u)

def now(): return datetime.now(timezone.utc).isoformat().replace("+00:00", "Z")
def sha256(p):
    h=hashlib.sha256()
    with p.open("rb") as f:
        for b in iter(lambda:f.read(1024*1024), b""): h.update(b)
    return h.hexdigest()

def encoding(data, content_type):
    m=re.search(r"charset=([\w.-]+)", content_type or "", re.I)
    candidates=([m.group(1)] if m else [])+["utf-8", "cp932", "euc_jp", "iso2022_jp"]
    for enc in candidates:
        try: data.decode(enc); return enc, "strict-decode"
        except (UnicodeDecodeError, LookupError): pass
    best=from_bytes(data).best()
    if best and best.encoding:
        return best.encoding, "charset-normalizer"
    return "unknown", "undetermined"

def text_from_html(s):
    s=re.sub(r"(?is)<(script|style).*?>.*?</\1>", "", s)
    s=re.sub(r"(?i)<br\s*/?>|</p\s*>|</div\s*>|</li\s*>|</tr\s*>", "\n", s)
    s=re.sub(r"(?s)<[^>]+>", "", s)
    return html.unescape(s).replace("\r\n", "\n").replace("\r", "\n")

def allowed(url, cache):
    u=urllib.parse.urlsplit(url); root=f"{u.scheme}://{u.netloc}"
    if root not in cache:
        rp=RobotFileParser(); rp.set_url(root+"/robots.txt")
        try: rp.read(); cache[root]=rp
        except Exception: cache[root]=None
    return True if cache[root] is None else cache[root].can_fetch(UA, url)

def fetch(item, root, robots, delay):
    sid, network, rel, url, ev, kind, station=item; dest=root/rel
    base={"source_id":sid,"network_name":network,"host_program":"mmm/MASH" if sid.startswith("mash-") or sid.startswith("midnight-") else "NET-COCK" if sid.startswith("tcm-") else "KTBBS" if sid.startswith("hanzou-") else "unknown","period":"1993-1997-priority" if network not in {"JIPDEC","Yokohama Totsuka BBS"} else "1986" if network=="JIPDEC" else "1985-1987","source_url":url,"retrieved_at":now(),"material_type":kind,"evidence_class":ev,"station_specific":station,"original_filename":Path(urllib.parse.urlsplit(url).path).name or "index.html","rights_notes":"Research preservation; do not redistribute third-party body via GitHub.","pii_risk":"high" if kind=="bbs-log" else "medium" if network not in {"JIPDEC","Vector"} else "low","notes":""}
    if not allowed(url, robots): return {**base,"status":"skipped-robots","notes":"robots.txt disallows this URL"}
    req=urllib.request.Request(url, headers={"User-Agent":UA,"Accept":"*/*"})
    try:
        time.sleep(delay)
        with urllib.request.urlopen(req, timeout=45) as r:
            data=r.read(); headers=dict(r.headers.items()); status=getattr(r,"status",200)
        dest.parent.mkdir(parents=True, exist_ok=True); dest.write_bytes(data)
        hp=dest.with_name(dest.name+".http-headers.txt"); hp.write_text(f"HTTP status: {status}\n"+"\n".join(f"{k}: {v}" for k,v in headers.items())+"\n", encoding="utf-8")
        digest=sha256(dest); dest.with_name(dest.name+".sha256").write_text(f"{digest}  {dest.name}\n", encoding="ascii")
        enc, method=encoding(data, headers.get("Content-Type",""))
        if kind in {"html","manual-html","index","software-index","bbs-log","manual"} and enc!="unknown":
            text=data.decode(enc); norm=root/rel.replace("/raw/","/normalized/")
            norm=norm.with_suffix(".txt") if kind in {"html","manual-html","index","software-index"} else norm
            norm.parent.mkdir(parents=True, exist_ok=True)
            norm.write_text(text_from_html(text) if "html" in kind or kind in {"index","software-index"} else text, encoding="utf-8", newline="\n")
        return {**base,"status":"downloaded","content_type":headers.get("Content-Type",""),"content_length":len(data),"original_encoding":enc,"encoding_method":method,"sha256":digest,"http_last_modified":headers.get("Last-Modified",""),"http_etag":headers.get("ETag",""),"local_path":rel.replace("\\","/"),"line_count":data.count(b"\n")+bool(data)}
    except urllib.error.HTTPError as e: return {**base,"status":"missing" if e.code==404 else "blocked","http_status":e.code,"notes":str(e)}
    except Exception as e: return {**base,"status":"blocked","notes":f"{type(e).__name__}: {e}"}

def write_outputs(root, records):
    (root/"manifest.json").write_text(json.dumps(records,ensure_ascii=False,indent=2)+"\n",encoding="utf-8")
    fields=sorted({k for r in records for k in r})
    with (root/"manifest.csv").open("w",newline="",encoding="utf-8-sig") as f:
        w=csv.DictWriter(f,fields); w.writeheader(); w.writerows(records)
    bodies=[p for p in root.rglob("*") if p.is_file() and not p.name.endswith((".sha256",".http-headers.txt")) and p.name not in {"checksums.sha256","manifest.json","manifest.csv"}]
    (root/"checksums.sha256").write_text("".join(f"{sha256(p)}  {p.relative_to(root).as_posix()}\n" for p in bodies),encoding="ascii")
    counts={s:sum(r["status"]==s for r in records) for s in ("downloaded","captured","discovery-only","blocked","missing","skipped-rights","skipped-robots")}
    total=sum((root/r["local_path"]).stat().st_size for r in records if r.get("status")=="downloaded")
    report="# Acquisition report\n\n# Summary\n\n"+"\n".join(f"- {k}: {v}" for k,v in counts.items())+f"\n- Raw downloaded bytes: {total}\n\n# Highest-value sources\n\n1. ぺけろく教 .bbs corpus — SYSOPがFDから復元した実ログ。\n2. MASH/mmm manuals — 明示配布されたコマンド・利用者マニュアル。\n3. JIPDEC 1986 report — 同時代の全国動向・局一覧。\n4. ミンキームーン当時画面 — 後年記事中に保存された1995年実ログ画面。\n5. びっくるネット過去ログ — INDEX実hrefから取得した局固有ログ。\n\n# Failed / blocked\n\n"+(("\n".join(f"- {r['source_url']} — {r['status']} — {r.get('http_status','')} {r.get('notes','')}" for r in records if r["status"]!="downloaded")) or "- None in the bounded static acquisition set.")+"\n\n# New discoveries\n\n- Bickle index exposes `bick1.html` through `bick17.html`; all were acquired without URL guessing beyond the observed href set.\n- Minkymoon article directly embeds six period-screen/log images; these are classified separately from the retrospective prose.\n\n# Historical observations\n\n- **Confirmed (contemporary report):** JIPDEC PDF section 2.2, \"わが国におけるパソコン通信の動向\", begins on PDF page 81 (viewer index P80). The individual/group BBS list is on PDF page 83 (viewer index P82), and includes 横浜戸塚BBS. The section continues through PDF page 91 before section 2.3 begins on page 92.\n- **Station-specific:** preserved BBS menus, board names, and banners must not be treated as host-program defaults.\n- **Later preservation:** Minkymoon prose is retrospective; its explicitly identified contemporary log images are separately classified.\n\n# Next actions\n\n- Perform browser-only, read-only guest-flow captures for Mapletown, Midnight Driving, Hanzou BBS, and T.C.M.; do not register or write.\n- Inspect Tokyo BBS article links and Wayback candidates manually before bounded acquisition.\n- Run corpus format analysis after validating date/article delimiters per source; avoid user profiling.\n"
    (root/"acquisition-report.md").write_text(report,encoding="utf-8")
    (root/"README.md").write_text("# Zutto PC Communication primary-source research archive\n\nResearch/private preservation artifact. Third-party bodies must not be committed to GitHub or redistributed as training data. See `manifest.json` for provenance, rights notes, and status. Raw bytes are unchanged; normalized files are UTF-8 derivatives.\n",encoding="utf-8")

def main():
    ap=argparse.ArgumentParser(); ap.add_argument("--output",type=Path,required=True); ap.add_argument("--delay",type=float,default=2.0); a=ap.parse_args()
    a.output.mkdir(parents=True,exist_ok=True); robots={}; records=[]
    for i,item in enumerate(SOURCES,1):
        print(f"[{i}/{len(SOURCES)}] {item[0]}",flush=True); records.append(fetch(item,a.output,robots,a.delay))
    write_outputs(a.output,records)
    zip_path=a.output.parent/(a.output.name+"-2026-08-31.zip")
    with ZipFile(zip_path,"w",ZIP_DEFLATED) as z:
        for p in a.output.rglob("*"):
            if p.is_file(): z.write(p,p.relative_to(a.output.parent))
    (zip_path.parent/(zip_path.name+".sha256")).write_text(f"{sha256(zip_path)}  {zip_path.name}\n",encoding="ascii")
    print(zip_path)

if __name__=="__main__": main()

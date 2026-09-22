import asyncio
import json
import time
import urllib.parse
import urllib.request

import websockets

WS = "wss://zutto-pccom-prototype.onrender.com/ws"
HTTP = "https://zutto-pccom-prototype.onrender.com"
PHONE = "0920000196"
BOARD = "20/1"
OLD_CORE = {"MARI","YUKI","NORI","KAZU","TAKU","NEKO","KEN","MAKO","TOMO","AKI","RYO","HIRO","SACHI","JUN","MIDNIGHT"}


def export(board=None, full=False):
    q = {"phone": PHONE}
    if board:
        q["board"] = board
    if full:
        q["full"] = "1"
    url = HTTP + "/api/debug/export?" + urllib.parse.urlencode(q)
    with urllib.request.urlopen(url, timeout=30) as r:
        return json.load(r)


async def connect_ws():
    last = None
    for attempt in range(1, 13):
        try:
            with urllib.request.urlopen(HTTP + "/health", timeout=20) as r:
                print("health", r.status)
            return await websockets.connect(
                WS,
                open_timeout=30,
                close_timeout=10,
                origin="https://zutto-pccom-prototype-liart.vercel.app",
            )
        except Exception as exc:
            last = exc
            print("ws retry", attempt, type(exc).__name__, exc)
            await asyncio.sleep(5)
    raise last


async def recv_type(ws, wanted, timeout=180):
    end = time.monotonic() + timeout
    while time.monotonic() < end:
        msg = json.loads(await asyncio.wait_for(ws.recv(), timeout=end-time.monotonic()))
        if msg.get("type") == wanted:
            return msg
    raise AssertionError("timeout waiting for " + wanted)


async def dial(ws):
    for attempt in range(1, 31):
        await ws.send(json.dumps({"type":"dial","phone":PHONE,"attempt":attempt}))
        result = await recv_type(ws, "dial_result", 30)
        if result.get("result") == "connect":
            terminal = await recv_type(ws, "terminal", 30)
            assert "YOUR ID:" in terminal.get("text",""), terminal
            return
    raise AssertionError("could not connect")


async def line(ws, value, timeout=180):
    await ws.send(json.dumps({"type":"line","line":value}))
    msg = await recv_type(ws, "terminal", timeout)
    return msg.get("text","")


async def main():
    ws = await connect_ws()
    try:
        await dial(ws)
        state = export(full=True)
        assert state["counts"]["total_host_posts"] == 0, state["counts"]
        assert len(state["personas"]) == 326, len(state["personas"])
        handles = [p["handle"] for p in state["personas"]]
        assert len({h.lower() for h in handles}) == 326
        print("membership=326 unique=326")

        out = await line(ws, "GUEST", 30)
        assert "WELCOME TO HAKATA CANAL NET" in out
        out = await line(ws, "1", 30)
        assert "ボード／フォーラムメニュー" in out
        out = await line(ws, "20", 30)
        assert "アミューズメントフォーラム" in out

        started = time.monotonic()
        out = await line(ws, "1", 240)
        elapsed = time.monotonic() - started
        assert "ＧＡＭＥ" in out
        assert "--- MSG はありません ---" not in out, out
        print(f"GAME first render waited {elapsed:.2f}s and returned populated index")

        data = export(BOARD, full=True)
        posts = [p for p in data["posts"] if p.get("intent",{}).get("action") == "bbs-world-catchup"]
        roots = [p for p in posts if not p.get("parent_id")]
        assert len(roots) >= 3, roots
        assert all((p.get("body") or "") == "" for p in posts), posts
        authors = [p["author"] for p in roots]
        print("authors:", authors)
        print("non-core authors:", [a for a in authors if a not in OLD_CORE])
        print("subjects:")
        for p in roots:
            print(" -", p["subject"])
        assert len(set(authors)) >= 3, authors

        first = roots[0]
        bodyout = await line(ws, str(first["id"]), 240)
        print("article read output:", repr(bodyout[:1200]))
        assert "SUBJ:" in bodyout and first["subject"] in bodyout
        after = export(BOARD, full=True)
        matching = [p for p in after["posts"] if p["id"] == first["id"]]
        assert matching and (matching[0].get("body") or "").strip(), matching
        untouched = [p for p in after["posts"] if p["id"] != first["id"] and p.get("intent",{}).get("action") == "bbs-world-catchup"]
        assert all((p.get("body") or "") == "" for p in untouched), untouched
        print("lazy body verified: selected article materialized, others stayed header-only")

        await ws.send(json.dumps({"type":"hangup"}))
    finally:
        await ws.close()
    print("POPULATION_TITLEFIRST_PRODUCTION_SMOKE_OK")

asyncio.run(main())

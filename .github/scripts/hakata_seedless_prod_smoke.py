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
ACTION = "bbs-world-catchup"


def export(board=None, full=False):
    q = {"phone": PHONE}
    if board:
        q["board"] = board
    if full:
        q["full"] = "1"
    url = HTTP + "/api/debug/export?" + urllib.parse.urlencode(q)
    with urllib.request.urlopen(url, timeout=20) as r:
        return json.load(r)


async def connect_ws():
    last = None
    for attempt in range(1, 13):
        try:
            with urllib.request.urlopen(HTTP + "/health", timeout=20) as r:
                print(f"health attempt {attempt}: {r.status}")
            return await websockets.connect(
                WS,
                open_timeout=20,
                close_timeout=10,
                origin="https://zutto-pccom-prototype-liart.vercel.app",
            )
        except Exception as exc:
            last = exc
            print(f"websocket attempt {attempt} failed: {type(exc).__name__}: {exc}")
            await asyncio.sleep(5)
    raise last


async def recv_type(ws, wanted, timeout=30):
    end = time.monotonic() + timeout
    seen = []
    while time.monotonic() < end:
        msg = json.loads(await asyncio.wait_for(ws.recv(), timeout=end - time.monotonic()))
        seen.append(msg)
        if msg.get("type") == wanted:
            return msg
    raise AssertionError(f"did not receive {wanted}; seen={seen}")


async def dial(ws):
    for attempt in range(1, 31):
        await ws.send(json.dumps({"type": "dial", "phone": PHONE, "attempt": attempt}))
        result = await recv_type(ws, "dial_result", 30)
        if result.get("result") == "connect":
            terminal = await recv_type(ws, "terminal", 30)
            assert "YOUR ID:" in terminal.get("text", ""), terminal
            return result
        if result.get("result") not in ("busy", "no_carrier"):
            raise AssertionError(f"unexpected dial result: {result}")
    raise AssertionError("could not connect after 30 attempts")


async def line(ws, text):
    await ws.send(json.dumps({"type": "line", "line": text}))
    msg = await recv_type(ws, "terminal", 40)
    return msg.get("text", "")


async def enter_game_board(ws):
    out = await line(ws, "GUEST")
    assert "WELCOME TO HAKATA CANAL NET" in out, out
    out = await line(ws, "1")
    assert "ボード／フォーラムメニュー" in out, out
    out = await line(ws, "20")
    assert "アミューズメントフォーラム" in out, out
    out = await line(ws, "1")
    assert "ＧＡＭＥ" in out, out
    return out


async def wait_batch(timeout=210):
    deadline = time.monotonic() + timeout
    last = None
    while time.monotonic() < deadline:
        last = export(BOARD, full=True)
        generated = [
            p for p in last["posts"]
            if p.get("intent", {}).get("action") == ACTION
        ]
        if len(generated) >= 5:
            return last, generated
        await asyncio.sleep(5)
    raise AssertionError(f"batch did not materialize; last={last}")


def assert_empty_host(label):
    data = export(full=True)
    assert data["counts"]["total_host_posts"] == 0, data["counts"]
    assert data["counts"]["returned_posts"] == 0, data["counts"]
    assert len(data["personas"]) == 15, len(data["personas"])
    print(f"{label}: posts=0 personas={len(data['personas'])}")


def describe(label, posts):
    roots = [p for p in posts if not p.get("parent_id")]
    authors = {p["author"] for p in roots}
    print(f"{label}: generated={len(posts)} roots={len(roots)} authors={sorted(authors)}")
    assert len(posts) == 5, len(posts)
    assert len(roots) == 5, roots
    assert len(authors) >= 3, authors
    for p in sorted(posts, key=lambda x: x["id"]):
        print(f"  id={p['id']} author={p['author']} subject={p['subject']!r}")
        assert p["subject"].strip()
        assert not p["subject"].startswith("Re:")
    normalized = ["".join(p["subject"].lower().split()) for p in roots]
    assert len(normalized) == len(set(normalized)), normalized


async def main():
    ws = await connect_ws()
    try:
        await dial(ws)
        assert_empty_host("first CONNECT")

        # Login/menu navigation itself must not fan out generation over all empty boards.
        out = await line(ws, "GUEST")
        assert "WELCOME TO HAKATA CANAL NET" in out
        out = await line(ws, "1")
        assert "ボード／フォーラムメニュー" in out
        await asyncio.sleep(2)
        assert_empty_host("after login + root board menu")

        out = await line(ws, "20")
        assert "アミューズメントフォーラム" in out
        out = await line(ws, "1")
        assert "ＧＡＭＥ" in out
        assert "--- MSG はありません ---" in out, out

        _, first = await wait_batch()
        describe("first fresh GAME batch", first)
        first_ids = {p["id"] for p in first}

        out = await line(ws, "BX")
        for p in first:
            assert p["subject"][:28] in out or p["subject"] in out, (p["subject"], out)

        await ws.send(json.dumps({"type": "hangup"}))
        await recv_type(ws, "carrier", 20)

        await dial(ws)
        assert_empty_host("second CONNECT reset")

        await enter_game_board(ws)
        _, second = await wait_batch()
        describe("second fresh GAME batch", second)
        second_ids = {p["id"] for p in second}
        assert first_ids.isdisjoint(second_ids), (first_ids, second_ids)

        await ws.send(json.dumps({"type": "hangup"}))
        await recv_type(ws, "carrier", 20)
    finally:
        await ws.close()

    print("SEEDLESS_HAKATA_PRODUCTION_SMOKE_OK")


asyncio.run(main())

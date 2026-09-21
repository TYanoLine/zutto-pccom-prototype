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


def generated(data):
    return [p for p in data["posts"] if p.get("intent", {}).get("action") == ACTION]


async def recv_type(ws, wanted, timeout=20):
    end = time.monotonic() + timeout
    seen = []
    while time.monotonic() < end:
        msg = json.loads(await asyncio.wait_for(ws.recv(), timeout=end - time.monotonic()))
        seen.append(msg)
        if msg.get("type") == wanted:
            return msg, seen
    raise AssertionError(f"did not receive {wanted}; seen={seen}")


async def dial(ws):
    for attempt in range(1, 31):
        await ws.send(json.dumps({"type": "dial", "phone": PHONE, "attempt": attempt}))
        result, _ = await recv_type(ws, "dial_result", 20)
        if result.get("result") == "connect":
            terminal, _ = await recv_type(ws, "terminal", 20)
            assert "YOUR ID:" in terminal.get("text", ""), terminal
            return result
        if result.get("result") not in ("busy", "no_carrier"):
            raise AssertionError(f"unexpected dial result: {result}")
    raise AssertionError("could not connect after 30 attempts")


async def line(ws, text):
    await ws.send(json.dumps({"type": "line", "line": text}))
    msg, _ = await recv_type(ws, "terminal", 30)
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


async def wait_batch(timeout=210):
    deadline = time.monotonic() + timeout
    last = None
    while time.monotonic() < deadline:
        last = export(BOARD, full=True)
        posts = generated(last)
        if len(posts) >= 3:
            return last, posts
        await asyncio.sleep(5)
    raise AssertionError(f"batch did not materialize; last={last}")


def describe(label, posts):
    roots = [p for p in posts if not p.get("parent_id")]
    print(f"{label}: generated={len(posts)} roots={len(roots)}")
    for p in sorted(posts, key=lambda x: x["id"]):
        body = (p.get("body") or "").replace("\r", " ").replace("\n", " ")[:120]
        print(
            f"  id={p['id']} parent={p.get('parent_id', 0)} "
            f"author={p['author']} subject={p['subject']!r} body={body!r}"
        )
    assert len(roots) >= 3, roots
    normalized = ["".join(r["subject"].lower().split()) for r in roots]
    assert len(normalized) == len(set(normalized)), normalized


async def main():
    async with websockets.connect(WS, open_timeout=20, close_timeout=10) as ws:
        await dial(ws)

        # CONNECT itself must clear the previous generated sample.
        after_connect = export(BOARD, full=True)
        assert generated(after_connect) == [], generated(after_connect)
        print("first CONNECT reset verified: generated=0")

        await enter_game_board(ws)
        first_data, first = await wait_batch()
        describe("first batch", first)
        first_ids = {p["id"] for p in first}

        await ws.send(json.dumps({"type": "hangup"}))
        await recv_type(ws, "carrier", 20)

        await dial(ws)
        second_reset = export(BOARD, full=True)
        assert generated(second_reset) == [], generated(second_reset)
        print("second CONNECT reset verified: generated=0")

        await enter_game_board(ws)
        second_data, second = await wait_batch()
        describe("second batch", second)
        second_ids = {p["id"] for p in second}
        assert first_ids.isdisjoint(second_ids), (first_ids, second_ids)

        await ws.send(json.dumps({"type": "hangup"}))
        await recv_type(ws, "carrier", 20)

    print("PRODUCTION_SMOKE_OK")


asyncio.run(main())

#!/usr/bin/env python3
"""
chaos_all.py — Combined Weeks 7–8 chaos test runner

Runs all three chaos scenarios in sequence and reports a pass/fail summary
for each.  To detect success the script watches the Go node's REST API —
if a bundle that should arrive actually appears at node2, it's a PASS.

Usage:
    # Make sure both dtnd nodes are running and poller.py is NOT running
    # (this script does its own REST polling)
    python3 chaos_all.py

Prerequisites:
    pip3 install requests cbor2

What it tests:
    1. Out-of-order  — 4-chunk bundle, chunks shuffled before send
    2. Packet loss   — 4-chunk bundle, 30% of chunks dropped
    3. Interleaved   — two 3-chunk bundles sent alternating
    4. Control       — normal send, verifies baseline still works after chaos

For tests 1 and 3: the bundle SHOULD arrive (all chunks sent, just reordered).
For test 2:        the bundle MAY OR MAY NOT arrive depending on which chunks
                   are dropped.  What matters is the Go server doesn't crash
                   and logs the incomplete bundle cleanly.

The script polls node2's REST API (port 8082) to verify delivery.
"""

import argparse
import random
import socket
import struct
import time
import sys

try:
    import cbor2
    import requests
except ImportError:
    print("ERROR: missing dependencies.  Run:  pip3 install requests cbor2")
    sys.exit(1)

HEADER_FMT  = ">IBBH"
HEADER_SIZE = struct.calcsize(HEADER_FMT)
MAX_PAYLOAD = 200 - HEADER_SIZE

NODE1_REST = "http://localhost:8081"
NODE2_REST = "http://localhost:8082"

RESULTS: list[tuple[str, str, str]] = []   # (test_name, expected, actual)


# ── Helpers ───────────────────────────────────────────────────────────────────

def make_bundle_bytes(label: str, size: int) -> bytes:
    filler = (f"CHAOS_{label}_" * 100)[:size]
    return cbor2.dumps({
        "src": "dtn://node1/inbox", "dst": "dtn://node2/inbox",
        "label": label, "data": filler, "ts": time.time(),
    })


def chunk_bytes(data: bytes, bundle_id: int) -> list[bytes]:
    slices = [data[i : i + MAX_PAYLOAD] for i in range(0, len(data), MAX_PAYLOAD)]
    total = len(slices)
    return [
        struct.pack(HEADER_FMT, bundle_id, idx, total, len(p)) + p
        for idx, p in enumerate(slices)
    ]


def send_packets(packets: list[bytes], host: str, port: int, gap: float = 0.05):
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    for pkt in packets:
        sock.sendto(pkt, (host, port))
        time.sleep(gap)
    sock.close()


def register_endpoint(rest_base: str, endpoint: str) -> str | None:
    try:
        r = requests.post(f"{rest_base}/rest/register",
                          json={"endpoint_id": endpoint}, timeout=5)
        data = r.json()
        return data.get("uuid")
    except Exception as e:
        print(f"  [register] ERROR: {e}")
        return None


def fetch_bundles(rest_base: str, uuid: str) -> list[dict]:
    try:
        r = requests.post(f"{rest_base}/rest/fetch",
                          json={"uuid": uuid}, timeout=5)
        data = r.json()
        return data.get("bundles", [])
    except Exception:
        return []


def poll_for_bundle(uuid: str, label: str, timeout: int = 20) -> bool:
    """Poll node2 REST until a bundle with matching label arrives or timeout."""
    import base64
    deadline = time.time() + timeout
    while time.time() < deadline:
        bundles = fetch_bundles(NODE2_REST, uuid)
        for b in bundles:
            raw = b.get("payloadBlock", {}).get("data", "")
            try:
                decoded = base64.b64decode(raw)
                obj = cbor2.loads(decoded)
                if obj.get("label") == label:
                    return True
            except Exception:
                pass
        time.sleep(1)
    return False


def header(title: str):
    print(f"\n{'═'*60}")
    print(f"  {title}")
    print(f"{'═'*60}")


def record(test: str, expected: str, passed: bool):
    actual = "PASS ✅" if passed else "FAIL ✗"
    RESULTS.append((test, expected, actual))
    print(f"\n  Result: {actual}")


# ── Test 1: Out-of-order ──────────────────────────────────────────────────────

def test_ooo(port: int, uuid: str):
    header("TEST 1 — Out-of-order chunk delivery")
    print("  Sends a 4-chunk bundle with chunks shuffled.\n"
          "  Expect: bundle reassembles correctly.")

    raw = make_bundle_bytes("OOO", 600)
    bid = random.randint(1, 0xFFFFFFFF)
    chunks = chunk_bytes(raw, bid)
    assert len(chunks) >= 3, "Need at least 3 chunks — increase size"

    shuffled = list(chunks)
    random.shuffle(shuffled)
    order = [struct.unpack_from("B", p, 4)[0] for p in shuffled]
    print(f"  bundle_id=0x{bid:08x}  chunks={len(chunks)}  order={order}")

    send_packets(shuffled, "127.0.0.1", port)
    print("  Waiting up to 20s for bundle to appear at node2 REST...")
    passed = poll_for_bundle(uuid, "OOO", timeout=20)
    record("Out-of-order", "bundle arrives", passed)


# ── Test 2: Packet loss ───────────────────────────────────────────────────────

def test_loss(port: int, loss_rate: float = 0.30):
    header(f"TEST 2 — Packet loss ({loss_rate*100:.0f}%)")
    print(f"  Sends a 4-chunk bundle, dropping ~{loss_rate*100:.0f}% of chunks.\n"
          f"  Expect: server.go logs incomplete bundle, does NOT crash.")

    raw = make_bundle_bytes("LOSS", 600)
    bid = random.randint(1, 0xFFFFFFFF)
    chunks = chunk_bytes(raw, bid)

    sent = dropped = 0
    packets_to_send = []
    for pkt in chunks:
        if random.random() < loss_rate:
            idx = struct.unpack_from("B", pkt, 4)[0]
            print(f"  ✗ DROPPED chunk {idx+1}/{len(chunks)}")
            dropped += 1
        else:
            idx = struct.unpack_from("B", pkt, 4)[0]
            print(f"  ✓ queued  chunk {idx+1}/{len(chunks)}")
            packets_to_send.append(pkt)
            sent += 1

    send_packets(packets_to_send, "127.0.0.1", port)
    print(f"  Sent {sent}/{len(chunks)} chunks.  "
          f"Waiting 5s then checking server is still alive...")
    time.sleep(5)

    # "Alive" check — can we still reach node1's REST?
    try:
        r = requests.get(f"{NODE1_REST}/rest/status", timeout=3)
        alive = True
    except Exception:
        # /rest/status may not exist — that's fine, just try register
        try:
            requests.post(f"{NODE1_REST}/rest/register",
                          json={"endpoint_id": "dtn://node1/_probe"}, timeout=3)
            alive = True
        except Exception:
            alive = False

    if dropped == 0:
        print("  (No chunks dropped this run — re-run for a real loss test)")
        record("Packet loss (no crash)", "server alive", alive)
    else:
        print(f"  Server alive: {alive}")
        record("Packet loss (no crash)", "server alive", alive)

    if dropped > 0:
        print(f"  NOTE: Wait ~60s for the bundle timeout to fire in server.go.")
        print(f"  Look for a log line like: 'incomplete bundle discarded bundle_id=0x{bid:08x}'")


# ── Test 3: Interleaved bundles ───────────────────────────────────────────────

def test_interleaved(port: int, uuid: str):
    header("TEST 3 — Interleaved bundles (A and B mixed)")
    print("  Sends chunks from two bundles alternating: A1, B1, A2, B2, ...\n"
          "  Expect: BOTH bundles reassemble correctly.")

    raw_a = make_bundle_bytes("INTER_A", 400)
    raw_b = make_bundle_bytes("INTER_B", 400)
    bid_a = random.randint(1, 0xFFFFFFFF)
    bid_b = random.randint(1, 0xFFFFFFFF)
    while bid_b == bid_a:
        bid_b = random.randint(1, 0xFFFFFFFF)

    chunks_a = chunk_bytes(raw_a, bid_a)
    chunks_b = chunk_bytes(raw_b, bid_b)

    # Alternating interleave
    interleaved = []
    for i in range(max(len(chunks_a), len(chunks_b))):
        if i < len(chunks_a):
            interleaved.append(chunks_a[i])
        if i < len(chunks_b):
            interleaved.append(chunks_b[i])

    print(f"  Bundle A: 0x{bid_a:08x}  {len(chunks_a)} chunks")
    print(f"  Bundle B: 0x{bid_b:08x}  {len(chunks_b)} chunks")
    print(f"  Total interleaved packets: {len(interleaved)}")

    send_packets(interleaved, "127.0.0.1", port)

    print("  Waiting up to 25s for BOTH bundles to appear at node2 REST...")
    a_ok = poll_for_bundle(uuid, "INTER_A", timeout=25)
    b_ok = poll_for_bundle(uuid, "INTER_B", timeout=25)

    print(f"  Bundle A arrived: {a_ok}")
    print(f"  Bundle B arrived: {b_ok}")
    record("Interleaved (both arrive)", "both bundles", a_ok and b_ok)


# ── Test 4: Control (baseline after chaos) ────────────────────────────────────

def test_control(port: int, uuid: str):
    header("TEST 4 — Control (clean send after chaos)")
    print("  Sends a normal in-order bundle with no drops.\n"
          "  Expect: delivery succeeds (verifies system recovered from chaos).")

    raw = make_bundle_bytes("CTRL", 300)
    bid = random.randint(1, 0xFFFFFFFF)
    chunks = chunk_bytes(raw, bid)
    print(f"  bundle_id=0x{bid:08x}  chunks={len(chunks)}")

    send_packets(chunks, "127.0.0.1", port)
    print("  Waiting up to 20s...")
    passed = poll_for_bundle(uuid, "CTRL", timeout=20)
    record("Control (clean send)", "bundle arrives", passed)


# ── Main ──────────────────────────────────────────────────────────────────────

def main():
    parser = argparse.ArgumentParser(description="Combined chaos test runner (Weeks 7–8)")
    parser.add_argument("--target-port", type=int, default=5006,
                        help="Meshtastic CLA UDP port of node2 (default: 5006)")
    parser.add_argument("--loss-rate", type=float, default=0.30,
                        help="Loss rate for Test 2 (default: 0.30)")
    parser.add_argument("--skip-rest", action="store_true",
                        help="Skip REST delivery checks (useful if nodes not running)")
    parser.add_argument("--seed", type=int, default=None)
    args = parser.parse_args()

    if args.seed is not None:
        random.seed(args.seed)

    print("╔══════════════════════════════════════════════════════════╗")
    print("║         DTN7 Meshtastic CLA — Weeks 7–8 Chaos Suite      ║")
    print("╚══════════════════════════════════════════════════════════╝")
    print(f"\nTarget: 127.0.0.1:{args.target_port}")
    print(f"REST:   node1={NODE1_REST}  node2={NODE2_REST}")

    uuid = None
    if not args.skip_rest:
        print("\nRegistering on node2 REST API...")
        uuid = register_endpoint(NODE2_REST, "dtn://node2/chaos_inbox")
        if not uuid:
            print("ERROR: Could not register on node2.  Is node2 running?")
            print("       Re-run with --skip-rest to skip delivery checks.")
            sys.exit(1)
        print(f"node2 UUID: {uuid}")

    # ── Run tests ──
    test_ooo(args.target_port, uuid)
    time.sleep(2)

    test_loss(args.target_port, args.loss_rate)
    time.sleep(2)

    test_interleaved(args.target_port, uuid)
    time.sleep(2)

    test_control(args.target_port, uuid)

    # ── Summary ──
    print(f"\n{'═'*60}")
    print("  SUMMARY")
    print(f"{'═'*60}")
    all_pass = True
    for name, expected, result in RESULTS:
        icon = "✅" if "PASS" in result else "✗"
        print(f"  {icon}  {name:<35}  {result}")
        if "FAIL" in result:
            all_pass = False

    print()
    if all_pass:
        print("  All tests passed.  Reassembly engine handles chaos correctly.")
        print("  You are ready for Week 9 hardware integration. ✅")
    else:
        print("  Some tests failed.  Check server.go logs for clues.")
        print("  Common issues:")
        print("    - Bundle timeout not yet implemented → add it to server.go")
        print("    - CBOR decode error → reassembly concatenation order wrong")
        print("    - Server crashed → check dtnd log for panic/goroutine dump")


if __name__ == "__main__":
    main()
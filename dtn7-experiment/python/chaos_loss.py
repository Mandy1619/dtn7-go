#!/usr/bin/env python3
"""
chaos_loss.py — Random packet loss simulation (Weeks 7–8)

Sends a multi-chunk bundle to the Go receiver, dropping each chunk with a
configurable probability.  The receiver's bundle timeout (60s) should fire
for incomplete bundles and discard them cleanly without crashing.

Usage:
    python3 chaos_loss.py [--loss RATE] [--target-port PORT] [--size BYTES]

Examples:
    python3 chaos_loss.py --loss 0.10   # 10% drop rate
    python3 chaos_loss.py --loss 0.30   # 30% drop rate
    python3 chaos_loss.py --loss 0.50   # 50% drop rate (severe)
    python3 chaos_loss.py --loss 0.00   # 0% — baseline, all chunks delivered

What to check:
    - 0% loss  → bundle reassembles correctly (control run)
    - 10% loss → some bundles reassemble, some time out; no crash
    - 30% loss → most bundles time out; server.go logs incomplete bundles
    - 100% loss (--loss 1.0) → all dropped; timeout fires for every bundle
"""

import argparse
import random
import socket
import struct
import time
import cbor2

HEADER_FMT  = ">IBBH"
HEADER_SIZE = struct.calcsize(HEADER_FMT)   # 8
MAX_PAYLOAD = 200 - HEADER_SIZE              # 192


def make_fake_bundle_bytes(message: str) -> bytes:
    payload = {
        "src":  "dtn://node1/inbox",
        "dst":  "dtn://node2/inbox",
        "data": message,
        "ts":   time.time(),
    }
    return cbor2.dumps(payload)


def chunk_bytes(data: bytes, bundle_id: int) -> list[bytes]:
    chunks = []
    slices = [data[i : i + MAX_PAYLOAD] for i in range(0, len(data), MAX_PAYLOAD)]
    total = len(slices)
    for idx, payload in enumerate(slices):
        header = struct.pack(HEADER_FMT, bundle_id, idx, total, len(payload))
        chunks.append(header + payload)
    return chunks


def send_with_loss(chunks: list[bytes], host: str, port: int,
                   loss_rate: float, seed: int | None = None):
    rng = random.Random(seed)
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    total = len(chunks)

    sent = dropped = 0
    bundle_id = struct.unpack_from(">I", chunks[0])[0]

    print(f"\n[chaos_loss] bundle_id=0x{bundle_id:08x}  "
          f"chunks={total}  loss_rate={loss_rate*100:.0f}%")
    print(f"[chaos_loss] Sending to {host}:{port}\n")

    for pkt in chunks:
        _, chunk_idx, total_chunks, payload_len = struct.unpack_from(HEADER_FMT, pkt)
        if rng.random() < loss_rate:
            print(f"  ✗ DROPPED  chunk {chunk_idx + 1}/{total_chunks}  "
                  f"payload_len={payload_len}")
            dropped += 1
        else:
            print(f"  ✓ sent     chunk {chunk_idx + 1}/{total_chunks}  "
                  f"payload_len={payload_len}")
            sock.sendto(pkt, (host, port))
            sent += 1
        time.sleep(0.05)

    sock.close()

    print(f"\n[chaos_loss] Summary: {sent} sent, {dropped} dropped "
          f"out of {total} total chunks")

    if dropped == 0:
        print("[chaos_loss] All chunks delivered — expect reassembly SUCCESS.")
    elif sent == 0:
        print("[chaos_loss] All chunks dropped — expect timeout + discard in server.go.")
    else:
        print(f"[chaos_loss] Partial delivery — expect timeout + discard in server.go "
              f"(bundle timeout is 60s).")
        print("[chaos_loss] Watch the dtnd log for an 'incomplete bundle' or 'timeout' message.")


def run_sweep(host: str, port: int, base_size: int, rates: list[float],
              seed: int | None = None):
    """Run multiple loss-rate trials in sequence, with a pause between each."""
    print("=" * 60)
    print(f"[chaos_loss] SWEEP MODE — running {len(rates)} trials")
    print("=" * 60)

    for rate in rates:
        filler = ("LOSS_TEST_" * 100)[:base_size]
        raw = make_fake_bundle_bytes(filler)
        bundle_id = random.randint(1, 0xFFFFFFFF)
        chunks = chunk_bytes(raw, bundle_id)

        print(f"\n{'─'*60}")
        send_with_loss(chunks, host, port, rate, seed=seed)

        if rate < rates[-1]:
            wait = 65   # just over the 60s bundle timeout in server.go
            print(f"\n[chaos_loss] Waiting {wait}s for timeout to clear "
                  f"before next trial...")
            time.sleep(wait)

    print("\n[chaos_loss] Sweep complete.")


def main():
    parser = argparse.ArgumentParser(description="Chaos test: random packet loss")
    parser.add_argument("--target-port", type=int, default=5006)
    parser.add_argument("--target-host", default="127.0.0.1")
    parser.add_argument("--loss", type=float, default=0.30,
                        help="Fraction of chunks to drop, 0.0–1.0 (default: 0.30)")
    parser.add_argument("--size", type=int, default=500,
                        help="Approximate payload size in bytes (default: 500 → ~3-4 chunks)")
    parser.add_argument("--sweep", action="store_true",
                        help="Run a sweep at 0%%, 10%%, 30%%, 50%% loss rates in sequence")
    parser.add_argument("--seed", type=int, default=None,
                        help="Random seed for reproducible drop decisions")
    args = parser.parse_args()

    if args.sweep:
        run_sweep(args.target_host, args.target_port, args.size,
                  rates=[0.0, 0.10, 0.30, 0.50], seed=args.seed)
        return

    if not 0.0 <= args.loss <= 1.0:
        parser.error("--loss must be between 0.0 and 1.0")

    filler = ("LOSS_TEST_" * 100)[:args.size]
    raw = make_fake_bundle_bytes(filler)
    bundle_id = random.randint(1, 0xFFFFFFFF)
    print(f"[chaos_loss] Payload: {len(raw)} bytes  bundle_id=0x{bundle_id:08x}")

    chunks = chunk_bytes(raw, bundle_id)
    print(f"[chaos_loss] Split into {len(chunks)} chunk(s)")

    if len(chunks) < 2:
        print("[chaos_loss] WARNING: only 1 chunk — loss test is trivial.  Use --size 400+.")

    send_with_loss(chunks, args.target_host, args.target_port, args.loss,
                   seed=args.seed)


if __name__ == "__main__":
    main()
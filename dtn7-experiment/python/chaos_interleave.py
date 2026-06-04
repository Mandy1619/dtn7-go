#!/usr/bin/env python3
"""
chaos_interleave.py — Interleaved bundle delivery test (Weeks 7–8)

Sends chunks from two (or more) bundles mixed together in the same UDP
stream.  The Go receiver's reassembly buffer is keyed by bundle_id — each
bundle should be tracked separately and both should reassemble correctly.

Usage:
    python3 chaos_interleave.py [--target-port PORT] [--bundles N]
                                [--size BYTES] [--strategy STRATEGY]

Strategies:
    alternating  — chunk A1, B1, A2, B2, A3, B3, ... (default)
    burst        — all of A, then all of B (control — should always work)
    random       — fully randomised interleave
    reverse      — second bundle's chunks arrive before the first bundle's

What to verify:
    - Both bundles appear in the Go receiver's log as 'reassembled'
    - Neither bundle's payload is corrupted by the other's chunks
    - No crash or hang in server.go
    - If chunk_listener.py is running on a third terminal, you'll see
      two distinct bundle_ids in its output
"""

import argparse
import random
import socket
import struct
import time
import cbor2

HEADER_FMT  = ">IBBH"
HEADER_SIZE = struct.calcsize(HEADER_FMT)
MAX_PAYLOAD = 200 - HEADER_SIZE


def make_fake_bundle_bytes(label: str, size: int) -> bytes:
    filler = (f"BUNDLE_{label}_DATA_" * 100)[:size]
    payload = {
        "src":   "dtn://node1/inbox",
        "dst":   "dtn://node2/inbox",
        "label": label,
        "data":  filler,
        "ts":    time.time(),
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


def interleave_alternating(chunk_lists: list[list[bytes]]) -> list[bytes]:
    """Round-robin: take one chunk from each bundle in turn."""
    result = []
    max_len = max(len(c) for c in chunk_lists)
    for i in range(max_len):
        for cl in chunk_lists:
            if i < len(cl):
                result.append(cl[i])
    return result


def interleave_random(chunk_lists: list[list[bytes]],
                      seed: int | None = None) -> list[bytes]:
    """Fully random interleave."""
    all_chunks = [pkt for cl in chunk_lists for pkt in cl]
    rng = random.Random(seed)
    rng.shuffle(all_chunks)
    return all_chunks


def interleave_burst(chunk_lists: list[list[bytes]]) -> list[bytes]:
    """All of bundle A, then all of B, … (no interleaving — control case)."""
    result = []
    for cl in chunk_lists:
        result.extend(cl)
    return result


def interleave_reverse(chunk_lists: list[list[bytes]]) -> list[bytes]:
    """Send the last bundle's chunks first, then the first bundle's chunks."""
    result = []
    for cl in reversed(chunk_lists):
        result.extend(cl)
    return result


STRATEGIES = {
    "alternating": interleave_alternating,
    "random":      interleave_random,
    "burst":       interleave_burst,
    "reverse":     interleave_reverse,
}


def send_interleaved(packets: list[bytes], host: str, port: int,
                     bundle_ids: list[int]):
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    id_set = set(bundle_ids)
    seen_ids: dict[int, list[int]] = {bid: [] for bid in id_set}

    print(f"\n[chaos_interleave] Sending {len(packets)} total packets to {host}:{port}")
    print(f"[chaos_interleave] Bundle IDs: "
          + ", ".join(f"0x{bid:08x}" for bid in bundle_ids))
    print()

    for pkt in packets:
        bid, chunk_idx, total_chunks, payload_len = struct.unpack_from(HEADER_FMT, pkt)
        seen_ids[bid].append(chunk_idx)
        label = bundle_ids.index(bid) + 1   # 1-indexed for display
        print(f"  [B{label}] chunk {chunk_idx + 1}/{total_chunks}  "
              f"bundle_id=0x{bid:08x}  payload_len={payload_len}")
        sock.sendto(pkt, (host, port))
        time.sleep(0.05)

    sock.close()

    print("\n[chaos_interleave] Transmission complete.")
    for label, bid in enumerate(bundle_ids, 1):
        chunks_sent = sorted(seen_ids[bid])
        print(f"  B{label} (0x{bid:08x}): sent chunks {chunks_sent}")
    print("\n[chaos_interleave] Expected: BOTH bundles reassembled correctly in server.go.")
    print("[chaos_interleave] Verify: no 'CBOR decode error', no missing bundle.")


def main():
    parser = argparse.ArgumentParser(description="Chaos test: interleaved bundles")
    parser.add_argument("--target-port", type=int, default=5006)
    parser.add_argument("--target-host", default="127.0.0.1")
    parser.add_argument("--bundles", type=int, default=2,
                        help="Number of concurrent bundles to interleave (default: 2)")
    parser.add_argument("--size", type=int, default=450,
                        help="Approx payload size per bundle in bytes (default: 450 → ~3 chunks)")
    parser.add_argument("--strategy", choices=list(STRATEGIES.keys()),
                        default="alternating",
                        help="Interleave strategy (default: alternating)")
    parser.add_argument("--seed", type=int, default=None,
                        help="Random seed (used by 'random' strategy)")
    args = parser.parse_args()

    if args.bundles < 2:
        parser.error("--bundles must be at least 2")

    # Generate distinct bundle_ids — use different values so the receiver
    # can distinguish them without collision
    bundle_ids = []
    while len(bundle_ids) < args.bundles:
        bid = random.randint(1, 0xFFFFFFFF)
        if bid not in bundle_ids:
            bundle_ids.append(bid)

    chunk_lists = []
    for i, bid in enumerate(bundle_ids):
        label = chr(ord("A") + i)
        raw = make_fake_bundle_bytes(label, args.size)
        chunks = chunk_bytes(raw, bid)
        print(f"[chaos_interleave] Bundle {label}: {len(raw)} bytes → "
              f"{len(chunks)} chunks  id=0x{bid:08x}")
        chunk_lists.append(chunks)

    fn = STRATEGIES[args.strategy]
    if args.strategy == "random":
        packets = fn(chunk_lists, seed=args.seed)
    else:
        packets = fn(chunk_lists)

    print(f"[chaos_interleave] Strategy: {args.strategy}  "
          f"total packets: {len(packets)}")

    send_interleaved(packets, args.target_host, args.target_port, bundle_ids)


if __name__ == "__main__":
    main()
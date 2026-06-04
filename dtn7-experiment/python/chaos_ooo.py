
#!/usr/bin/env python3
"""
chaos_ooo.py — Out-of-order chunk delivery test (Weeks 7–8)
 
Builds a synthetic BPv7 bundle payload, splits it into chunks using the
agreed 8-byte header format, shuffles them, and sends them to the Go
receiver. The reassembler should reconstruct the bundle correctly regardless
of arrival order.
 
Usage:
    python3 chaos_ooo.py [--target-port PORT] [--message TEXT] [--size BYTES]
 
Defaults:
    --target-port 5006   (node2's Meshtastic CLA listener)
    --message     None   (use --size instead)
    --size        400    (bytes of payload to generate, forces multi-chunk)
 
The script prints every chunk it sends, in what order, and whether the
bundle_id it used is visible in the chunk_listener output on the other side.
"""
 
import argparse
import random
import socket
import struct
import time
import cbor2
 
# ── Chunk header ──────────────────────────────────────────────────────────────
# Offset  Size  Field
#      0     4  bundle_id     uint32 big-endian
#      4     1  chunk_idx     uint8, 0-indexed
#      5     1  total_chunks  uint8
#      6     2  payload_len   uint16 big-endian
HEADER_FMT   = ">IBBH"
HEADER_SIZE  = struct.calcsize(HEADER_FMT)   # 8
MAX_PAYLOAD  = 200 - HEADER_SIZE              # 192
 
 
def make_fake_bundle_bytes(message: str) -> bytes:
    """Produce a minimal CBOR blob that looks like a bundle payload.
 
    We don't have the full bpv7 Go library here, so we encode a dict that
    represents the payload block data.  The Go receiver will attempt to
    UnmarshalCbor — if you want a valid round-trip you must use sender.py
    instead.  This script is for testing the reassembly engine, not the
    CBOR decoder.
    """
    payload = {
        "src": "dtn://node1/inbox",
        "dst": "dtn://node2/inbox",
        "data": message,
        "ts": time.time(),
    }
    return cbor2.dumps(payload)
 
 
def chunk_bytes(data: bytes, bundle_id: int) -> list[bytes]:
    """Split data into ≤192-byte chunks with the 8-byte header prepended."""
    chunks = []
    slices = [data[i : i + MAX_PAYLOAD] for i in range(0, len(data), MAX_PAYLOAD)]
    total = len(slices)
    for idx, payload in enumerate(slices):
        header = struct.pack(HEADER_FMT, bundle_id, idx, total, len(payload))
        chunks.append(header + payload)
    return chunks
 
 
def send_chunks_ooo(chunks: list[bytes], host: str, port: int, seed: int | None = None):
    """Shuffle chunks and send them with a small delay between each."""
    shuffled = list(chunks)
    rng = random.Random(seed)
    rng.shuffle(shuffled)
 
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    total = len(chunks)
    print(f"\n[chaos_ooo] Sending {total} chunks OUT OF ORDER to {host}:{port}")
    print(f"[chaos_ooo] Original order: {list(range(total))}")
 
    order = []
    for pkt in shuffled:
        idx = struct.unpack_from("B", pkt, 4)[0]
        order.append(idx)
 
    print(f"[chaos_ooo] Shuffled order: {order}")
    print()
 
    for pkt in shuffled:
        bundle_id, chunk_idx, total_chunks, payload_len = struct.unpack_from(HEADER_FMT, pkt)
        print(f"  → sending chunk {chunk_idx + 1}/{total_chunks}  "
              f"bundle_id=0x{bundle_id:08x}  payload_len={payload_len}")
        sock.sendto(pkt, (host, port))
        time.sleep(0.05)   # 50ms gap — fast enough for localhost
 
    sock.close()
    print(f"\n[chaos_ooo] All {total} chunks sent (out of order).")
    print("[chaos_ooo] The Go receiver should reassemble them correctly.")
    print("[chaos_ooo] Check server.go logs — look for 'bundle reassembled' or similar.")
 
 
def main():
    parser = argparse.ArgumentParser(description="Chaos test: out-of-order chunks")
    parser.add_argument("--target-port", type=int, default=5006,
                        help="UDP port of the receiving Meshtastic CLA (default: 5006)")
    parser.add_argument("--target-host", default="127.0.0.1")
    parser.add_argument("--message", default=None,
                        help="Message string to encode.  Overrides --size.")
    parser.add_argument("--size", type=int, default=400,
                        help="Approximate payload size in bytes (default: 400 → ~3 chunks)")
    parser.add_argument("--bundle-id", type=int, default=None,
                        help="Override bundle_id (default: random)")
    parser.add_argument("--seed", type=int, default=None,
                        help="Random seed for reproducible shuffle order")
    args = parser.parse_args()
 
    if args.message:
        raw = make_fake_bundle_bytes(args.message)
    else:
        # Fill to requested size with recognisable repeated content
        filler = ("CHAOS_OOO_" * 50)[:args.size]
        raw = make_fake_bundle_bytes(filler)
 
    bundle_id = args.bundle_id if args.bundle_id is not None else random.randint(1, 0xFFFFFFFF)
    print(f"[chaos_ooo] Bundle payload: {len(raw)} bytes  bundle_id=0x{bundle_id:08x}")
 
    chunks = chunk_bytes(raw, bundle_id)
    print(f"[chaos_ooo] Split into {len(chunks)} chunk(s) of ≤{MAX_PAYLOAD} bytes each")
 
    if len(chunks) < 2:
        print("[chaos_ooo] WARNING: only 1 chunk — out-of-order test is trivial.")
        print("[chaos_ooo]   Use --size 400 or larger to generate multiple chunks.")
 
    send_chunks_ooo(chunks, args.target_host, args.target_port, seed=args.seed)
 
 
if __name__ == "__main__":
    main()
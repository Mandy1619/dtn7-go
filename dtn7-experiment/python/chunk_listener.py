#!/usr/bin/env python3
"""
UDP listener on port 5005.
Parses the 8-byte chunk header from each incoming datagram.

Chunk header format (agreed):
  [bundle_id: 4 bytes][chunk_idx: 1 byte][total_chunks: 1 byte][payload_len: 2 bytes]

Usage: python3 chunk_listener.py
"""

import socket
import struct
import cbor2

UDP_PORT    = 5006
MAX_PACKET  = 200
HEADER_SIZE = 8


def parse_header(data):
    if len(data) < HEADER_SIZE:
        return None
    bundle_id, chunk_idx, total_chunks, payload_len = struct.unpack(">IBBH", data[:HEADER_SIZE])
    payload = data[HEADER_SIZE:HEADER_SIZE + payload_len]
    return {
        "bundle_id":    bundle_id,
        "chunk_idx":    chunk_idx,
        "total_chunks": total_chunks,
        "payload_len":  payload_len,
        "payload":      payload
    }


def main():
    sock = socket.socket(socket.AF_INET, socket.SOCK_DGRAM)
    sock.bind(("0.0.0.0", UDP_PORT))
    print(f"Listening for chunks on UDP port {UDP_PORT}...")
    print(f"Header format: [bundle_id:4][chunk_idx:1][total_chunks:1][payload_len:2]")
    print("Ctrl+C to stop\n")

    # buffer: bundle_id -> list of (chunk_idx, payload)
    buffers = {}

    while True:
        try:
            data, addr = sock.recvfrom(MAX_PACKET)
            chunk = parse_header(data)
            if not chunk:
                print(f"Received malformed packet ({len(data)} bytes) from {addr}")
                continue

            bid   = chunk["bundle_id"]
            idx   = chunk["chunk_idx"]
            total = chunk["total_chunks"]

            print(f"  chunk {idx+1}/{total}  bundle_id={bid:#010x}  payload_len={chunk['payload_len']}")

            if bid not in buffers:
                buffers[bid] = {}
            buffers[bid][idx] = chunk["payload"]

            # check if all chunks arrived
            if len(buffers[bid]) == total:
                full = b"".join(buffers[bid][i] for i in range(total))
                print(f"\n  ✓ All {total} chunks received for bundle {bid:#010x}")
                print(f"    Total payload: {len(full)} bytes")
                try:
                    bundle = cbor2.loads(full)
                    print(f"    CBOR decoded OK — type: {type(bundle)}")
                    print(f"    Raw structure: {bundle}")
                except Exception as e:
                    print(f"    CBOR decode failed: {e}")
                    print(f"    Raw bytes (hex): {full.hex()[:80]}...")
                print()
                del buffers[bid]

        except KeyboardInterrupt:
            print("\nStopped.")
            break


if __name__ == "__main__":
    main()

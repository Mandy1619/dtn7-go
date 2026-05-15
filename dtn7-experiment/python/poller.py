#!/usr/bin/env python3
"""
Registers on node2 and polls for incoming bundles every 2 seconds.
Prints the decoded payload when a bundle arrives.

Usage: python3 poller.py
"""

import requests
import time
import base64
import json

NODE2_REST = "http://localhost:8082"
ENDPOINT_ID = "dtn://node2/inbox"
POLL_INTERVAL = 2  # seconds


def register():
    resp = requests.post(
        f"{NODE2_REST}/rest/register",
        json={"endpoint_id": ENDPOINT_ID}
    )
    data = resp.json()
    if data.get("error"):
        print(f"Registration error: {data['error']}")
        exit(1)
    uuid = data["uuid"]
    print(f"Registered on node2 as {ENDPOINT_ID}")
    print(f"UUID: {uuid}")
    return uuid


def fetch(uuid):
    resp = requests.post(
        f"{NODE2_REST}/rest/fetch",
        json={"uuid": uuid}
    )
    return resp.json()


def print_bundle(bundle):
    primary = bundle.get("primaryBlock", {})
    payload = bundle.get("payloadBlock", {})

    src = primary.get("source", "unknown")
    dst = primary.get("destination", "unknown")
    ts  = primary.get("creationTimestamp", {}).get("date", "unknown")

    raw_data = payload.get("data", "")
    try:
        decoded = base64.b64decode(raw_data).decode("utf-8")
    except Exception:
        decoded = f"(binary) {raw_data}"

    print("-" * 50)
    print(f"  From     : {src}")
    print(f"  To       : {dst}")
    print(f"  Sent at  : {ts}")
    print(f"  Payload  : {decoded}")
    print("-" * 50)


def main():
    uuid = register()
    print(f"Polling every {POLL_INTERVAL}s... (Ctrl+C to stop)\n")

    seen = set()

    while True:
        try:
            result = fetch(uuid)
            bundles = result.get("bundles", [])
            for bundle in bundles:
                # Use creation timestamp + source as a unique key to avoid reprinting
                primary = bundle.get("primaryBlock", {})
                key = (
                    primary.get("source"),
                    str(primary.get("creationTimestamp"))
                )
                if key not in seen:
                    seen.add(key)
                    print_bundle(bundle)
        except requests.exceptions.ConnectionError:
            print("Cannot reach node2 — is dtnd running?")
        except Exception as e:
            print(f"Error: {e}")

        time.sleep(POLL_INTERVAL)


if __name__ == "__main__":
    main()

#!/usr/bin/env python3
"""
Sends a bundle from node1 to node2.

HOW IT WORKS:
  1. Register with dtnd's REST API to get a session UUID
  2. Ask dtnd to build and send a bundle with our message as the payload
  3. DTN7 takes care of the rest: routing, chunking over LoRa, store-and-forward

Usage: python3 sender.py "your message here"
       python3 sender.py            (uses a default message)
"""

import requests
import sys
import json

NODE1_REST = "http://localhost:8081"
SOURCE_ENDPOINT = "dtn://node1/inbox"
DEST_ENDPOINT   = "dtn://node2/inbox"


def register():     #Register with dtnd to get a UUID. dtnd uses the UUID to associate our session with the source endpoint. We must register before we can send or receive bundles.
    resp = requests.post(
        f"{NODE1_REST}/rest/register",
        json={"endpoint_id": SOURCE_ENDPOINT}
    )
    data = resp.json()
    if data.get("error"):
        print(f"Registration error: {data['error']}")
        exit(1)
    return data["uuid"]


def send(uuid, message):    #Ask dtnd to create and send a bundle. dtnd calls the BundleBuilder internally, then hands the bundle to the routing engine, which forwards it via the Meshtastic CLA.
    resp = requests.post(
        f"{NODE1_REST}/rest/build",
        json={
            "uuid": uuid,
            "arguments": {
                "destination": DEST_ENDPOINT,
                "source": SOURCE_ENDPOINT,
                "creation_timestamp_now": 1,
                "lifetime": "24h",
                "payload_block": message
            }
        }
    )
    return resp.json()


def main():
    message = " ".join(sys.argv[1:]) if len(sys.argv) > 1 else "Hello from Python sender!"

    print(f"Registering on node1...")
    uuid = register()
    print(f"UUID: {uuid}")

    print(f"Sending: \"{message}\"")
    result = send(uuid, message)

    if result.get("error"):
        print(f"Send error: {result['error']}")
    else:
        print("Bundle sent successfully.")
        print("Wait ~10 seconds, then check poller.py on node2.")


if __name__ == "__main__":
    main()

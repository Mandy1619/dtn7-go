#!/usr/bin/env python3
"""
Meshtastic bridge — listens on a Unix socket for outbound chunks from the
Go CLA, forwards them over LoRa, and writes received packets back.

Usage:
    python3 meshtastic_bridge.py /dev/ttyUSB0 /tmp/mesh_bridge.sock
"""
import sys, socket, os, threading, time, struct
import meshtastic.serial_interface
from pubsub import pub

SERIAL_PORT = sys.argv[1]      # e.g. /dev/ttyUSB0
SOCK_PATH   = sys.argv[2]      # e.g. /tmp/mesh_node1.sock

# ---- Meshtastic interface ----
iface = meshtastic.serial_interface.SerialInterface(SERIAL_PORT)
time.sleep(2)

# ---- Unix socket server (Go writes chunks here) ----
if os.path.exists(SOCK_PATH):
    os.remove(SOCK_PATH)
srv = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
srv.bind(SOCK_PATH)
srv.listen(1)
print(f"Bridge ready on {SOCK_PATH}", flush=True)

# ---- Receive from radio → write to Go CLA ----
receive_conn = None
receive_lock = threading.Lock()

def on_receive(packet, interface):
    data = packet.get("decoded", {}).get("payload", b"")
    if data and receive_conn:
        with receive_lock:
            # length-prefix so Go knows where one packet ends
            receive_conn.sendall(struct.pack(">H", len(data)) + data)

pub.subscribe(on_receive, "meshtastic.receive.data")

# ---- Accept connection from Go CLA ----
conn, _ = srv.accept()
receive_conn = conn
print("Go CLA connected", flush=True)

# ---- Read outbound chunks from Go CLA → send over radio ----
while True:
    hdr = conn.recv(2)
    if not hdr:
        break
    length = struct.unpack(">H", hdr)[0]
    data = conn.recv(length)
    if data:
        iface.sendData(data, portNum=256, wantAck=False)
        time.sleep(0.5)   # EU868 duty cycle gap

iface.close()
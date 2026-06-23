package meshtastic //Package meshtastic  implementing a DTN7 CLA that sends and recieves BPv7 bundles over Meshtastic LoRa radio
//Key: DTN7 speaks BUNDLES and LoRa in packets. THis CLA sits between them and handles the translation.

// Transport is a simple 3-method interface that hides HOW data is sent.
//MeshtasticTransport: sends over Unix socket to the python sidecar
//UDPTransport: No longer need -Simulation testing [Removed]
// 
// client.go and server.go call only these 3 methods — they never know or care whether the underlying link is UDP or real LoRa radio
// to switch from UDP to Real LoRa, change only in node.toml
type Transport interface {
    SendPacket(data []byte) error      // send one chunk packet (≤200 bytes)
    ReceivePacket() ([]byte, error)    // block until one chunk packet arrives
    Close() error                     //close shuts down the transport and releases any resources
}


//   [DTN7 core] ──calls──► [MeshtasticClient.Send(bundle)]
//                                    │
//                                    ▼
//                           serialize to CBOR
//                           split into chunks
//                           send via Transport
//                                    │
//                          ┌─────────┴──────────┐
//                          │                    │
//                   UDPTransport         MeshtasticTransport
//                  (testing/sim)         (real LoRa hardware)
//
//   [Transport.ReceivePacket()] ──► reassemble chunks ──► [DTN7 core]
//
// To switch from simulation to real hardware: change one line in node.toml --TODO--
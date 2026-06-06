package meshtastic

// Transport is the send/receive contract that both the UDP simulation
// and the real Meshtastic sidecar must satisfy.
// client.go and server.go call only these two methods — they never
// touch a UDP socket or a Unix socket directly.
type Transport interface {
    SendPacket(data []byte) error      // send one chunk packet (≤200 bytes)
    ReceivePacket() ([]byte, error)    // block until one chunk packet arrives
    Close() error
}
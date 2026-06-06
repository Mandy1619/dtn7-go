package meshtastic

import (
    "encoding/binary"
    "net"
)

// MeshtasticTransport implements Transport by talking to the Python
// meshtastic_bridge.py sidecar over a Unix domain socket.
type MeshtasticTransport struct {
    conn net.Conn    // Unix socket connection to the sidecar
}

func NewMeshtasticTransport(socketPath string) (*MeshtasticTransport, error) {
    conn, err := net.Dial("unix", socketPath)
    if err != nil {
        return nil, err
    }
    return &MeshtasticTransport{conn: conn}, nil
}

func (t *MeshtasticTransport) SendPacket(data []byte) error {
    // length-prefix so the sidecar knows where one packet ends
    hdr := make([]byte, 2)
    binary.BigEndian.PutUint16(hdr, uint16(len(data)))
    _, err := t.conn.Write(append(hdr, data...))
    return err
}

func (t *MeshtasticTransport) ReceivePacket() ([]byte, error) {
    // read the 2-byte length prefix the sidecar sends back
    hdr := make([]byte, 2)
    if _, err := io.ReadFull(t.conn, hdr); err != nil {
        return nil, err
    }
    length := binary.BigEndian.Uint16(hdr)
    buf := make([]byte, length)
    _, err := io.ReadFull(t.conn, buf)
    return buf, err
}

func (t *MeshtasticTransport) Close() error {
    return t.conn.Close()
}
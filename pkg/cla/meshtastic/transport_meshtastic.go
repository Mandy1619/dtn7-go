package meshtastic

import (
    "encoding/binary"
    "io"
    "net"
    "time"
    "fmt"
    "log"
)

// MeshtasticTransport implements Transport by talking to the Python
// meshtastic_bridge.py sidecar over a Unix domain socket.
type MeshtasticTransport struct {
    conn net.Conn
}

func NewMeshtasticTransport(socketPath string) (*MeshtasticTransport, error) {
    var conn net.Conn
    var err error

    // Retry for up to 30 seconds — sidecar may still be initializing
    for i := 0; i < 30; i++ {
        conn, err = net.Dial("unix", socketPath)
        if err == nil {
            break
        }
        log.WithField("socket", socketPath).Info("Waiting for Meshtastic sidecar...")
        time.Sleep(1 * time.Second)
    }
    if err != nil {
        return nil, fmt.Errorf("Error creating Meshtastic transport: %w", err)
    }

    return &MeshtasticTransport{conn: conn}, nil
}

func (t *MeshtasticTransport) SendPacket(data []byte) error {
    hdr := make([]byte, 2)
    binary.BigEndian.PutUint16(hdr, uint16(len(data)))
    _, err := t.conn.Write(append(hdr, data...))
    return err
}

func (t *MeshtasticTransport) ReceivePacket() ([]byte, error) {
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
    if t.conn == nil {
        return nil
    }
    return t.conn.Close()
}
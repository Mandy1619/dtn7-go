package meshtastic

import (
    "encoding/binary"
    "io"
    "net"
    "time"
    "fmt"
    log "github.com/sirupsen/logrus"
)

// MeshtasticTransport implements Transport by talking to the Python meshtastic_bridge.py over a Unix domain socket.

//Why Python instead of pure go: The Meshtastic Python library has all the handling of device protocol, channel configuration, etc. Instead of reimplementing that in Go, this approach is used. The UNix socket adds near-zero latency compared to  LoRa 

//Every packet is length-prefixed so the reader knows exactly how many bytes belong to each packet

//Usage in node.toml: 
//   [[Listener]]
//   type    = "meshtastic"
//   address = "/tmp/mesh_node1.sock"
//   peer_id = "dtn://node2/" 
type MeshtasticTransport struct {
    conn net.Conn
}

//NewMeshtasticTransport dials the UNix Socket. Retry: 30secs
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

//SendPacket writes [2-byte length][data] to the UNix Socket. The meshtastic_bridge.py reads this and calls ifacesendData() over LoRa

func (t *MeshtasticTransport) SendPacket(data []byte) error {
    hdr := make([]byte, 2)
    binary.BigEndian.PutUint16(hdr, uint16(len(data)))
    _, err := t.conn.Write(append(hdr, data...))
    return err
}

//REcievePacket reads the 2-byte length prefix, then reads exactly that many bytes. Blocks until a packet arrives from the radio via the meshtastic_bridge.py
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

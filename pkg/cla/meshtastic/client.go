// SPDX-License-Identifier: GPL-3.0-or-later
package meshtastic

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sync"

	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
)

const (
	maxPayloadBytes = 192 // 200 byte LoRa MTU minus 8 byte header
	headerSize      = 8
)

// MeshtasticClient sends bundles to a peer Meshtastic node via UDP simulation.
// Implements cla.ConvergenceSender.
type MeshtasticClient struct {
	peerAddress    string         // e.g. "127.0.0.1:5005"
	peerEndpointID bpv7.EndpointID
	conn           *net.UDPConn
	mu             sync.Mutex
	active         bool
}

func NewMeshtasticClient(peerAddress string, peerEndpointID bpv7.EndpointID) *MeshtasticClient {
	return &MeshtasticClient{
		peerAddress:    peerAddress,
		peerEndpointID: peerEndpointID,
	}
}

func (c *MeshtasticClient) Activate() error {
	addr, err := net.ResolveUDPAddr("udp", c.peerAddress)
	if err != nil {
		return fmt.Errorf("meshtastic client: resolve %s: %w", c.peerAddress, err)
	}
	conn, err := net.DialUDP("udp", nil, addr)
	if err != nil {
		return fmt.Errorf("meshtastic client: dial %s: %w", c.peerAddress, err)
	}
	c.conn = conn
	c.active = true
	log.WithField("peer", c.peerAddress).Info("Meshtastic client activated (UDP sim)")
	return nil
}

func (c *MeshtasticClient) Send(bndl *bpv7.Bundle) error {
    // 1. CBOR-marshal the bundle
    var buf bytes.Buffer
    if err := cboring.Marshal(bndl, &buf); err != nil {
        return err
    }
    data := buf.Bytes()

    // 2. Derive bundle_id from creation timestamp
    ts := bndl.PrimaryBlock.CreationTimestamp.DtnTime()
    bundleID := uint32(ts & 0xFFFFFFFF)

    // 3. Chunk and send
    chunkSize := 192
    chunks := splitIntoChunks(data, chunkSize)
    total := uint8(len(chunks))

    conn, err := net.Dial("udp", "localhost:5005")
    if err != nil { return err }
    defer conn.Close()

    for i, payload := range chunks {
        header := make([]byte, 8)
        binary.BigEndian.PutUint32(header[0:4], bundleID)
        header[4] = uint8(i)
        header[5] = total
        binary.BigEndian.PutUint16(header[6:8], uint16(len(payload)))
        packet := append(header, payload...)
        conn.Write(packet)
        // TODO Week 9: add 500ms duty cycle gap here when --duty-cycle flag is set
    }
    return nil
}

func (c *MeshtasticClient) GetPeerEndpointID() bpv7.EndpointID { return c.peerEndpointID }
func (c *MeshtasticClient) Active() bool                       { return c.active }
func (c *MeshtasticClient) Address() string {
	return fmt.Sprintf("meshtastic://%s", c.peerAddress)
}
func (c *MeshtasticClient) Close() error {
	c.active = false
	if c.conn != nil {
		return c.conn.Close()
	}
	return nil
}

var _ cla.ConvergenceSender = (*MeshtasticClient)(nil)
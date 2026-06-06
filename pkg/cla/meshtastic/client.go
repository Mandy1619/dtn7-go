// SPDX-License-Identifier: GPL-3.0-or-later
package meshtastic

import (
	"bytes"
	"encoding/binary"
	"fmt"
	//"net"
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
	transport      Transport
	mu             sync.Mutex
	active         bool
}

func NewMeshtasticClient(transport Transport, peerEndpointID bpv7.EndpointID) *MeshtasticClient {
	return &MeshtasticClient{
		transport:    transport,
		peerEndpointID: peerEndpointID,
	}
}

func (c *MeshtasticClient) Activate() error {
	c.active = true
    log.Info("Meshtastic client activated")
    return nil
}

func (c *MeshtasticClient) Send(bndl *bpv7.Bundle) error {
	// 1. CBOR-marshal the bundle to bytes
	var buf bytes.Buffer
	if err := bndl.MarshalCbor(&buf); err != nil {
		return fmt.Errorf("meshtastic: cbor marshal: %w", err)
	}
	bundleBytes := buf.Bytes()

	// 2. Derive a 4-byte bundle_id from the creation timestamp
	bundleID := uint32(bndl.ID().Timestamp[0] & 0xFFFFFFFF)

	// 3. Split into 192-byte chunks
	var chunks [][]byte
	for len(bundleBytes) > 0 {
		end := maxPayloadBytes
		if end > len(bundleBytes) {
			end = len(bundleBytes)
		}
		chunks = append(chunks, bundleBytes[:end])
		bundleBytes = bundleBytes[end:]
	}
	totalChunks := len(chunks)

	log.WithFields(log.Fields{
		"bundle_id":    fmt.Sprintf("%#010x", bundleID),
		"total_chunks": totalChunks,
		"total_bytes":  buf.Len(),
	}).Info("Meshtastic client: sending bundle")

	c.mu.Lock()
	defer c.mu.Unlock()

	// 4. Send each chunk over the already-open UDP connection from Activate()
	for i, payload := range chunks {
		header := make([]byte, headerSize)
		binary.BigEndian.PutUint32(header[0:4], bundleID)
		header[4] = uint8(i)
		header[5] = uint8(totalChunks)
		binary.BigEndian.PutUint16(header[6:8], uint16(len(payload)))

		packet := append(header, payload...)
		if err := c.transport.SendPacket(packet); err != nil {
			return fmt.Errorf("meshtastic: chunk %d/%d write: %w", i+1, totalChunks, err)
		}
		log.WithFields(log.Fields{
			"chunk":     fmt.Sprintf("%d/%d", i+1, totalChunks),
			"bundle_id": fmt.Sprintf("%#010x", bundleID),
		}).Debug("Meshtastic client: sent chunk")
		// TODO Week 9: time.Sleep(500 * time.Millisecond) when --duty-cycle flag is set
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
    return c.transport.Close()
}

var _ cla.ConvergenceSender = (*MeshtasticClient)(nil)
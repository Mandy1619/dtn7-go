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

//Chunk Header layout: 8 bytes total:
//
//4	bundle_id	- lower32 bits of the bundle's creation timestamp
//1	chunk_idx	- which chunk this is (0-indexed)
//1	total_chunks	- how many chunks this bundle was split into
//2	payload_len	- how many data bytes follow the header

//The header uses 8 bytes leaving 192 bytes for data
const (
	maxPayloadBytes = 192 // 200 byte LoRa MTU minus 8 byte header
	headerSize      = 8
)

// MeshtasticClient is the SENDING side of the Meshtastic CLA sends bundles to a peer Meshtastic node via UDP simulation.
// Implements cla.ConvergenceSender.
type MeshtasticClient struct {
	peerEndpointID bpv7.EndpointID //DTN address of the node we are sending to
	transport      Transport //send chunks (UDP or H/w)
	mu             sync.Mutex //prevents two goroutines sending chunks at the same time
	active         bool
}

func NewMeshtasticClient(transport Transport, peerEndpointID bpv7.EndpointID) *MeshtasticClient {
	return &MeshtasticClient{
		transport:    transport,
		peerEndpointID: peerEndpointID,
	}
}

//Activate is called by the DTN7 Manager after registering this CLA. The Transport is already open, we just mark ourselves as active.
func (c *MeshtasticClient) Activate() error {
	c.active = true
    log.Info("Meshtastic client activated")
    return nil
}

//Send() is the imp part of this CLA. called by DTN7 whenever a bundle needs to be forwarded to the peer node.

// Flow:      bundle -> CBOR bytes -> split into ≤192-byte chunks -> send each chunk
func (c *MeshtasticClient) Send(bndl *bpv7.Bundle) error {
	// 1. Serialise the bundle into CBOR Bytes (CBOR is the binary encoding format defined by the BPv7 standard)
	var buf bytes.Buffer
	if err := bndl.MarshalCbor(&buf); err != nil {
		return fmt.Errorf("meshtastic: cbor marshal: %w", err)
	}
	bundleBytes := buf.Bytes()

	// 2. Derive a 4-byte bundle_id from the creation timestamp
	//This lets the reciever know which chunks belong together
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

	log.WithFields(log.Fields{    //visible in logs
		"bundle_id":    fmt.Sprintf("%#010x", bundleID),
		"total_chunks": totalChunks,
		"total_bytes":  buf.Len(),
	}).Info("Meshtastic client: sending bundle")

	c.mu.Lock()
	defer c.mu.Unlock()

	// 4. Send each chunk over
	for i, payload := range chunks {
		//build 8-byte header
		header := make([]byte, headerSize)
		binary.BigEndian.PutUint32(header[0:4], bundleID) //which bundle
		header[4] = uint8(i) //chunk position
		header[5] = uint8(totalChunks) //total count
		binary.BigEndian.PutUint16(header[6:8], uint16(len(payload))) //data size

		packet := append(header, payload...)
		if err := c.transport.SendPacket(packet); err != nil {
			return fmt.Errorf("meshtastic: chunk %d/%d write: %w", i+1, totalChunks, err)
		}
		log.WithFields(log.Fields{
			"chunk":     fmt.Sprintf("%d/%d", i+1, totalChunks),
			"bundle_id": fmt.Sprintf("%#010x", bundleID),
		}).Debug("Meshtastic client: sent chunk")
		
	}
	return nil
}

// The following methods satisfy the cla.ConvergenceSender interface.
func (c *MeshtasticClient) GetPeerEndpointID() bpv7.EndpointID { return c.peerEndpointID }
func (c *MeshtasticClient) Active() bool                       { return c.active }
func (c *MeshtasticClient) Address() string {
	return fmt.Sprintf("meshtastic://peer/%s", c.peerEndpointID)
}
func (c *MeshtasticClient) Close() error {
    c.active = false
    return c.transport.Close()
}

var _ cla.ConvergenceSender = (*MeshtasticClient)(nil) //Compile-time check - MeshtatsticClient must satisfy cla.ConvergenceSender. If not, the build fails with error message

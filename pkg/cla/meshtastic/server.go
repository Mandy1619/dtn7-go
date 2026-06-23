// SPDX-License-Identifier: GPL-3.0-or-later
package meshtastic

import (
	"bytes"
	"encoding/binary"
	"fmt"
	//"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
)

// reassemblyBuffer holds chunks we ahve recieved so far for one bundle. When all chunks arrive, we concatenate them and decode the bundle
type reassemblyBuffer struct {
	chunks      map[uint8][]byte //key=chunk_idx, value = payload bytes
	totalChunks uint8
	lastSeen    time.Time // updated each time a new chunk arrives; used for timeout
}

// MeshtasticServer is the RECIEVING side of the MeshtasticCLA
//It runs a bg goroutine that reads incoming chunk packets, reassembles them into complete bundles, and hands each bundle to DTN7
// Implements cla.ConvergenceListener and cla.ConvergenceReceiver.
type MeshtasticServer struct {
	endpointID     bpv7.EndpointID
	receiveCallback func(*bpv7.Bundle) // DTN7 calls this when a bundle arrives
	transport           Transport
	running        bool
	active         bool
	stopCh         chan struct{}
	reassembly      map[uint32]*reassemblyBuffer // reassembly holds in-flight bundles: bundle_id -> buffer of chunks received so far. Using a map keyed by chunk_idx means out-of-order arrival is handled automatically.
    mu              sync.Mutex // protects reassembly map (accessed by receive loop + timeout goroutine)
}

func NewMeshtasticServer(transport Transport, endpointID bpv7.EndpointID, receiveCallback func(*bpv7.Bundle)) *MeshtasticServer {
    return &MeshtasticServer{
        transport:       transport,
        endpointID:      endpointID,
        receiveCallback: receiveCallback,
        stopCh:          make(chan struct{}),
        reassembly:      make(map[uint32]*reassemblyBuffer),
    }
}

// Start launches two background goroutines:
func (s *MeshtasticServer) Start() error {
    s.running = true
    log.Info("Meshtastic server starting")
    go s.receiveLoop() //1. receiveLoop — reads packets and reassembles bundles
    go func() {
        ticker := time.NewTicker(15 * time.Second) //2. timeout cleaner — discards incomplete bundles after 60 seconds (defined here only)
        defer ticker.Stop()
        for {
            select {
            case <-s.stopCh:
                return
            case <-ticker.C:
                now := time.Now()
                s.mu.Lock()
                for id, rb := range s.reassembly {
                    if now.Sub(rb.lastSeen) > 60*time.Second {
                        log.WithFields(log.Fields{
                            "bundle_id":    fmt.Sprintf("%#010x", id),
                            "chunks_have":  len(rb.chunks),
                            "chunks_total": rb.totalChunks,
                        }).Warn("Meshtastic: incomplete bundle timed out — discarding")
                        delete(s.reassembly, id)
                    }
                }
                s.mu.Unlock()
            }
        }
    }()

    return nil
}

// receiveLoop runs forever, reading one chunk packet at a time.
// For each packet it:
//  1. Parses the 8-byte header
//  2. Stores the payload in the reassembly buffer for that bundle
//  3. If all chunks have arrived, decodes the bundle and calls receiveCallback
func (s *MeshtasticServer) receiveLoop() {
    
    //exiting when Close() is called
    for {
        select {
        case <-s.stopCh:
            return
        default:
        }

        // Block until a packet arrives
        data, err := s.transport.ReceivePacket()
        if err != nil {
            if s.running {
                log.WithError(err).Warn("Meshtastic server: recieve error")
            }
            continue
        }

        // Validate minimum packet size
        if len(data) < headerSize {
            log.Warnf("Meshtastic server: short packet (%d bytes), skipping", len(data))
            continue
        }

        // Parse the 8-byte chunk header
        bundleID    := binary.BigEndian.Uint32(data[0:4])
        chunkIdx    := data[4]
        totalChunks := data[5]
        payloadLen  := binary.BigEndian.Uint16(data[6:8])
        
        // Copy the payload bytes (everything after the header)
        payload := make([]byte, payloadLen)
        copy(payload, data[headerSize:headerSize+int(payloadLen)])

        log.WithFields(log.Fields{
            "bundle_id": fmt.Sprintf("%#010x", bundleID),
            "chunk":     fmt.Sprintf("%d/%d", chunkIdx+1, totalChunks),
        }).Debug("Meshtastic server: received chunk")

        // Store this chunk and check if the bundle is now complete
        s.mu.Lock()

        // Create a new buffer if this is the first chunk for this bundle
        if s.reassembly[bundleID] == nil {
            s.reassembly[bundleID] = &reassemblyBuffer{
                chunks:      make(map[uint8][]byte),
                totalChunks: totalChunks,
            }
        }
        s.reassembly[bundleID].chunks[chunkIdx] = payload
        s.reassembly[bundleID].lastSeen = time.Now()   // <- the lastSeen update

        // Check if all chunks for this bundle have arrived
        rb := s.reassembly[bundleID]
        complete := uint8(len(rb.chunks)) == rb.totalChunks

        // If complete, collect the bytes and remove from map before unlocking
        var full bytes.Buffer
        if complete {
            for i := uint8(0); i < rb.totalChunks; i++ {
                full.Write(rb.chunks[i]) // concatenate in order: chunk 0, 1, 2, ...
            }
            delete(s.reassembly, bundleID)
        }
        s.mu.Unlock()

         // Decode and deliver the bundle outside the lock
        if complete {
            var bundle bpv7.Bundle
            if err := bundle.UnmarshalCbor(&full); err != nil {
                log.WithError(err).Errorf("Meshtastic server: CBOR decode failed for bundle %#010x", bundleID)
                continue
            }
            log.WithField("bundle_id", bundle.ID().String()).Info("Meshtastic server: bundle reassembled, handing to DTN7")
            s.receiveCallback(&bundle)
        }
    }
}

// Interface method implementations

func (s *MeshtasticServer) Close() error {
    s.running = false
    s.active = false
    close(s.stopCh)
    return s.transport.Close()
}

func (s *MeshtasticServer) Running() bool                    { return s.running }
func (s *MeshtasticServer) Active() bool                     { return s.active }
func (s *MeshtasticServer) Activate() error                  { s.active = true; return nil }
func (s *MeshtasticServer) GetEndpointID() bpv7.EndpointID   { return s.endpointID }
func (s *MeshtasticServer) Address() string {
	return fmt.Sprintf("meshtastic://server")
}

// Compile-time checks: MeshtasticServer must satisfy both interfaces.
var _ cla.ConvergenceListener = (*MeshtasticServer)(nil)
var _ cla.ConvergenceReceiver = (*MeshtasticServer)(nil)
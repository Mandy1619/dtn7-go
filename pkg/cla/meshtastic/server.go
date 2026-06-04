// SPDX-License-Identifier: GPL-3.0-or-later
package meshtastic

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"sync"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
)

// MeshtasticServer listens for incoming chunked bundles on a UDP socket.
// Implements cla.ConvergenceListener and cla.ConvergenceReceiver.
type MeshtasticServer struct {
	listenAddr     string // e.g. "0.0.0.0:5006" or "sim"
	endpointID     bpv7.EndpointID
	receiveCallback func(*bpv7.Bundle)
	conn           *net.UDPConn
	running        bool
	active         bool
	stopCh         chan struct{}
	reassembly      map[uint32]*reassemblyBuffer
    mu              sync.Mutex 
}

func NewMeshtasticServer(address string, endpointID bpv7.EndpointID, receiveCallback func(*bpv7.Bundle)) *MeshtasticServer {
	return &MeshtasticServer{
		listenAddr:      address,
		endpointID:      endpointID,
		receiveCallback: receiveCallback,
		stopCh:          make(chan struct{}),
		reassembly:      make(map[uint32]*reassemblyBuffer),
	}
}

func (s *MeshtasticServer) Start() error {
    addr, err := net.ResolveUDPAddr("udp", s.listenAddr)
    if err != nil {
        return fmt.Errorf("meshtastic server: resolve %s: %w", s.listenAddr, err)
    }
    conn, err := net.ListenUDP("udp", addr)
    if err != nil {
        return fmt.Errorf("meshtastic server: listen %s: %w", s.listenAddr, err)
    }
    s.conn = conn
    s.running = true
    log.WithField("addr", s.listenAddr).Info("Meshtastic server listening (UDP sim)")

    go s.receiveLoop()

    go func() {
        ticker := time.NewTicker(15 * time.Second)
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

// reassemblyBuffer holds chunks for a bundle in progress
type reassemblyBuffer struct {
	chunks      map[uint8][]byte
	totalChunks uint8
	lastSeen    time.Time // updated each time a new chunk arrives
}

func (s *MeshtasticServer) receiveLoop() {
    buf := make([]byte, 200)

    for {
        select {
        case <-s.stopCh:
            return
        default:
        }

        n, _, err := s.conn.ReadFromUDP(buf)
        if err != nil {
            if s.running {
                log.WithError(err).Warn("Meshtastic server: UDP read error")
            }
            continue
        }

        if n < headerSize {
            log.Warnf("Meshtastic server: short packet (%d bytes), skipping", n)
            continue
        }

        bundleID    := binary.BigEndian.Uint32(buf[0:4])
        chunkIdx    := buf[4]
        totalChunks := buf[5]
        payloadLen  := binary.BigEndian.Uint16(buf[6:8])

        payload := make([]byte, payloadLen)
        copy(payload, buf[headerSize:headerSize+int(payloadLen)])

        log.WithFields(log.Fields{
            "bundle_id": fmt.Sprintf("%#010x", bundleID),
            "chunk":     fmt.Sprintf("%d/%d", chunkIdx+1, totalChunks),
        }).Debug("Meshtastic server: received chunk")

        s.mu.Lock()
        if s.reassembly[bundleID] == nil {
            s.reassembly[bundleID] = &reassemblyBuffer{
                chunks:      make(map[uint8][]byte),
                totalChunks: totalChunks,
            }
        }
        s.reassembly[bundleID].chunks[chunkIdx] = payload
        s.reassembly[bundleID].lastSeen = time.Now()   // ← the lastSeen update

        rb := s.reassembly[bundleID]
        complete := uint8(len(rb.chunks)) == rb.totalChunks
        var full bytes.Buffer
        if complete {
            for i := uint8(0); i < rb.totalChunks; i++ {
                full.Write(rb.chunks[i])
            }
            delete(s.reassembly, bundleID)
        }
        s.mu.Unlock()

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

func (s *MeshtasticServer) Close() error {
	s.running = false
	s.active = false
	close(s.stopCh)
	if s.conn != nil {
		return s.conn.Close()
	}
	return nil
}

func (s *MeshtasticServer) Running() bool                    { return s.running }
func (s *MeshtasticServer) Active() bool                     { return s.active }
func (s *MeshtasticServer) Activate() error                  { s.active = true; return nil }
func (s *MeshtasticServer) GetEndpointID() bpv7.EndpointID   { return s.endpointID }
func (s *MeshtasticServer) Address() string {
	return fmt.Sprintf("meshtastic://%s", s.listenAddr)
}

var _ cla.ConvergenceListener = (*MeshtasticServer)(nil)
var _ cla.ConvergenceReceiver = (*MeshtasticServer)(nil)
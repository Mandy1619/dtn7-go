// SPDX-License-Identifier: GPL-3.0-or-later

package meshtastic

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
)

// MeshtasticServer listens for incoming bundles from a Meshtastic device
// (or the UDP simulator in Weeks 5–6).
//
// It implements:
//   - cla.ConvergenceListener  (Start, Running, Address, Close)
//   - cla.ConvergenceReceiver  (Activate, Active, Address, Close, GetEndpointID)
type MeshtasticServer struct {
	address    string
	endpointID bpv7.EndpointID
	receive    func(*bpv7.Bundle)

	running bool
	active  bool
}

// NewMeshtasticServer creates a new server stub.
// address is a string like "sim" (UDP sim) or "/dev/ttyUSB0" (real hardware).
// endpointID is the DTN endpoint this node owns.
// receive is the callback supplied by the CLA manager; call it when a bundle arrives.
func NewMeshtasticServer(address string, endpointID bpv7.EndpointID, receive func(*bpv7.Bundle)) *MeshtasticServer {
	return &MeshtasticServer{
		address:    address,
		endpointID: endpointID,
		receive:    receive,
	}
}

// ── ConvergenceListener ──────────────────────────────────────────────────────

func (s *MeshtasticServer) Start() error {
	log.WithField("address", s.Address()).Info("Meshtastic server Start() called (stub)")
	s.running = true
	return nil
}

func (s *MeshtasticServer) Running() bool {
	return s.running
}

// ── ConvergenceReceiver ──────────────────────────────────────────────────────

func (s *MeshtasticServer) Activate() error {
	log.WithField("address", s.Address()).Info("Meshtastic server Activate() called (stub)")
	s.active = true
	return nil
}

func (s *MeshtasticServer) Active() bool {
	return s.active
}

func (s *MeshtasticServer) GetEndpointID() bpv7.EndpointID {
	return s.endpointID
}

// ── Convergence (shared) ─────────────────────────────────────────────────────

func (s *MeshtasticServer) Address() string {
	return fmt.Sprintf("meshtastic://%s", s.address)
}

func (s *MeshtasticServer) Close() error {
	log.WithField("address", s.Address()).Info("Meshtastic server Close() called (stub)")
	s.running = false
	s.active = false
	return nil
}

// compile-time interface checks
var _ cla.ConvergenceListener = (*MeshtasticServer)(nil)
var _ cla.ConvergenceReceiver = (*MeshtasticServer)(nil)

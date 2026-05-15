// SPDX-License-Identifier: GPL-3.0-or-later

package meshtastic

import (
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
)

// MeshtasticClient sends bundles to a peer Meshtastic node.
//
// It implements cla.ConvergenceSender.
type MeshtasticClient struct {
	address        string
	peerEndpointID bpv7.EndpointID

	active bool
}

// NewMeshtasticClient creates a new client stub pointing at peerAddress.
func NewMeshtasticClient(address string, peerEndpointID bpv7.EndpointID) *MeshtasticClient {
	return &MeshtasticClient{
		address:        address,
		peerEndpointID: peerEndpointID,
	}
}

// ── ConvergenceSender ────────────────────────────────────────────────────────

func (c *MeshtasticClient) Send(bundle *bpv7.Bundle) error {
	log.WithField("address", c.Address()).Info("Meshtastic client Send() called (stub — bundle dropped)")
	// TO-DO: marshal bundle to CBOR, split into 200-byte chunks, send over UDP/serial
	return nil
}

func (c *MeshtasticClient) GetPeerEndpointID() bpv7.EndpointID {
	return c.peerEndpointID
}

// ── Convergence (shared) ─────────────────────────────────────────────────────

func (c *MeshtasticClient) Activate() error {
	log.WithField("address", c.Address()).Info("Meshtastic client Activate() called (stub)")
	c.active = true
	return nil
}

func (c *MeshtasticClient) Active() bool {
	return c.active
}

func (c *MeshtasticClient) Address() string {
	return fmt.Sprintf("meshtastic://%s", c.address)
}

func (c *MeshtasticClient) Close() error {
	log.WithField("address", c.Address()).Info("Meshtastic client Close() called (stub)")
	c.active = false
	return nil
}

// compile-time interface check
var _ cla.ConvergenceSender = (*MeshtasticClient)(nil)

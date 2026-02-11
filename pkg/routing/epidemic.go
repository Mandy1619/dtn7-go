// SPDX-FileCopyrightText: 2019, 2022, 2026 Markus Sommer
// SPDX-FileCopyrightText: 2019, 2020 Alvar Penning
//
// SPDX-License-Identifier: GPL-3.0-or-later

package routing

import (
	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
	"github.com/dtn7/dtn7-go/pkg/store"
)

// EpidemicRouting is an implementation of an Algorithm and behaves in a
// flooding-based epidemic way.
type EpidemicRouting struct{}

// NewEpidemicRouting creates a new EpidemicRouting Algorithm interacting
// with the given Core.
func NewEpidemicRouting() *EpidemicRouting {
	log.Debug("Initialised epidemic routing")

	return &EpidemicRouting{}
}

// NotifyNewBundle does nothing for this algorithm
func (er *EpidemicRouting) NotifyNewBundle(_ *store.BundleDescriptor, _ *bpv7.Bundle) {}

// NotifyReceivedBundle does nothing for this algorithm
func (er *EpidemicRouting) NotifyReceivedBundle(_ *store.BundleDescriptor, _ *bpv7.Bundle) {}

func (er *EpidemicRouting) SelectPeersForForwarding(descriptor *store.BundleDescriptor, peers []cla.ConvergenceSender) ([]cla.ConvergenceSender, *bpv7.Bundle) {
	peers = filterPeers(descriptor, peers)
	log.WithFields(log.Fields{
		"bundle": descriptor,
		"peers":  peers,
	}).Debug("EpidemicRouting selected peers for outgoing bundle")
	return peers, nil
}

func (_ *EpidemicRouting) NotifyPeerAppeared(_ bpv7.EndpointID) {}

func (_ *EpidemicRouting) NotifyPeerDisappeared(_ bpv7.EndpointID) {}

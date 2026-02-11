// SPDX-FileCopyrightText: 2023, 2025 Markus Sommer
//
// SPDX-License-Identifier: GPL-3.0-or-later

// Package routing provides and interface & implementations for routing algorithms.
//
// Since there should only be a single Algorithm active at any time, this package employs the singleton pattern.
// Use `InitialiseAlgorithm` and `GetAlgorithmSingleton.`
package routing

import (
	"fmt"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
	"github.com/dtn7/dtn7-go/pkg/store"
)

type AlgorithmEnum uint32

const (
	AlgorithmEpidemic     AlgorithmEnum = 1
	AlgorithmSprayAndWait AlgorithmEnum = 2
)

func AlgorithmEnumFromString(name string) (AlgorithmEnum, error) {
	switch name = strings.ToLower(name); name {
	case "epidemic":
		return AlgorithmEpidemic, nil
	case "spray&wait":
		return AlgorithmSprayAndWait, nil
	default:
		return 0, fmt.Errorf("%s is not a valid algorithm name", name)
	}
}

// Algorithm is an interface to specify routing algorithms for delay-tolerant networks.
type Algorithm interface {
	// NotifyNewBundle notifies an Algorithm about new bundles.
	// Called when a new bundle was created on this node
	// Whether an algorithm acts on this information or ignores it, is an implementation matter.
	NotifyNewBundle(descriptor *store.BundleDescriptor, bundle *bpv7.Bundle)

	// NotifyReceivedBundle notifies an Algorithm about a bundle received from another node
	// Whether an algorithm acts on this information or ignores it, is an implementation matter.
	NotifyReceivedBundle(descriptor *store.BundleDescriptor, bundle *bpv7.Bundle)

	// SelectPeersForForwarding returns an array of ConvergenceSender for a requested bundle.
	// dtnd will attempt to forward the bundle to all the selected peers.
	// If the routing algorithm needs to make any modifications to the bundle, it should load the bundle, make modifications and then return the pointer.
	// If the pointer is nil, then the processing pipeline will load the bundle from disk.
	SelectPeersForForwarding(descriptor *store.BundleDescriptor, peers []cla.ConvergenceSender) ([]cla.ConvergenceSender, *bpv7.Bundle)

	// NotifyPeerAppeared notifies the Algorithm about a new peer.
	NotifyPeerAppeared(peer bpv7.EndpointID)

	// NotifyPeerDisappeared notifies the Algorithm about the
	// disappearance of a peer.
	NotifyPeerDisappeared(peer bpv7.EndpointID)
}

var algorithmSingleton Algorithm

func InitialiseAlgorithm(algorithm Algorithm) {
	if algorithmSingleton != nil {
		log.Fatalf("Attempting to initialise an already initialised algorithm. This must never happen!")
	}
	if algorithm == nil {
		log.Fatalf("Attempting to initialise algorithm with nil. This must never happen!")
	}

	algorithmSingleton = algorithm
}

// GetAlgorithmSingleton returns the routing algorithm singleton-instance.
// Attempting to call this function before algorithm initialisation will cause the program to panic.
func GetAlgorithmSingleton() Algorithm {
	if algorithmSingleton == nil {
		log.Fatalf("Attempting to access an uninitialised algorithm. This must never happen!")
	}
	return algorithmSingleton
}

func ShutdownAlgorithm() {
	algorithmSingleton = nil
}

// filterPeers filters the nodes which already received a Bundle.
// It returns a list of unused ConvergenceSenders.
func filterPeers(bundleDescriptor *store.BundleDescriptor, clas []cla.ConvergenceSender) (filtered []cla.ConvergenceSender) {
	filtered = make([]cla.ConvergenceSender, 0, len(clas))

	sentEids, err := bundleDescriptor.GetKnownHolders()
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": bundleDescriptor,
			"error":  err,
		}).Debug("Error getting bundle's known holders")
		return []cla.ConvergenceSender{}
	}

	for _, cs := range clas {
		skip := false

		for _, eid := range sentEids {
			if cs.GetPeerEndpointID() == eid {
				skip = true
				break
			}
		}

		if !skip {
			filtered = append(filtered, cs)
		}
	}

	return
}

// getFilteredPeers returns a slice ov ConvergenceSenders which connect to nodes that are not known to already hold the bundle
func getFilteredPeers(bundleDescriptor *store.BundleDescriptor) []cla.ConvergenceSender {
	return filterPeers(bundleDescriptor, cla.GetManagerSingleton().GetSenders())
}

// uniquePeers filters a list of ConvergenceSenders for uniqueness.
// Sometimes you may have multiple CLAs which connect ot the same peer, and you may or may not want to send a bundle across all parallel links.
func uniquePeers(peers []cla.ConvergenceSender) []cla.ConvergenceSender {
	endpoints := make(map[bpv7.EndpointID]bool)
	unique := make([]cla.ConvergenceSender, 0, len(peers))
	for _, sender := range peers {
		_, present := endpoints[sender.GetPeerEndpointID()]
		if !present {
			endpoints[sender.GetPeerEndpointID()] = true
			unique = append(unique, sender)
		}
	}
	return unique
}

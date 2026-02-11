package routing

import (
	"math/rand"

	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
	"github.com/dtn7/dtn7-go/pkg/store"
)

// SprayAndWait routing algorithm (https://doi.org/10.1145/1080139.1080143)
// Implements both basic and binary mode
type SprayAndWait struct {
	// copies is the default value that is attached to bundles that don't have a spray&wait extension block
	copies uint64
	// whether to run in binary-mode
	binary bool
}

func NewSprayAndWait(copies uint64, binary bool) *SprayAndWait {
	router := SprayAndWait{
		copies: copies,
		binary: binary,
	}
	return &router
}

// NotifyPeerAppeared does nothing for this algorithm
func (router *SprayAndWait) NotifyPeerAppeared(_ bpv7.EndpointID) {}

// NotifyPeerDisappeared does nothing for this algorithm
func (router *SprayAndWait) NotifyPeerDisappeared(_ bpv7.EndpointID) {}

func (router *SprayAndWait) NotifyNewBundle(descriptor *store.BundleDescriptor, _ *bpv7.Bundle) {
	log.WithField("bundle", descriptor.ID()).Debug("Spray&Wait handling new bundle")
	setCopies(descriptor, router.copies)
}

func (router *SprayAndWait) NotifyReceivedBundle(descriptor *store.BundleDescriptor, bundle *bpv7.Bundle) {
	log.WithField("bundle", descriptor.ID()).Debug("Spray&Wait handling received bundle")
	log.WithField("bundle", descriptor.ID()).Debug("Checking for BinarySprayBlock")
	block, err := bundle.ExtensionBlockByType(bpv7.BlockTypeBinarySprayBlock)
	if err != nil {
		log.WithField("bundle", descriptor.ID()).Debug("Bundle has no BinarySprayBlock, using copies 0")
		setCopies(descriptor, 0)
	} else {
		copies := block.Value.(*bpv7.BinarySprayBlock).RemainingCopies()
		log.WithFields(log.Fields{
			"bundle": descriptor.ID(),
			"copies": copies,
		}).Debug("Bundle has BinarySprayBlock, using received copies")
		setCopies(descriptor, copies)
	}
}

func (router *SprayAndWait) SelectPeersForForwarding(descriptor *store.BundleDescriptor) ([]cla.ConvergenceSender, *bpv7.Bundle) {
	log.WithField("bundle", descriptor.ID()).Debug("Spray&Wait selecting peers for forwarding")
	var copies uint64
	data, ok := descriptor.GetMiscData("spray_and_wait/copies")
	if !ok {
		log.WithField("bundle", descriptor.ID()).Debug("Bundle had no saved copies, assuming default")
		copies = router.copies
		setCopies(descriptor, copies)
	} else {
		copies = data.(uint64)
	}

	if !(copies > 0) {
		log.WithField("bundle", descriptor.ID()).Debug("No bundle copies left to replicate")
		return []cla.ConvergenceSender{}, nil
	}

	peers := getFilteredPeers(descriptor)
	nPeers := uint64(len(peers))
	if !(nPeers > 0) {
		log.WithField("bundle", descriptor.ID()).Debug("No suitable peers connected")
		return []cla.ConvergenceSender{}, nil
	}

	if router.binary {
		return router.selectBinarySpray(descriptor, copies, peers)
	} else {
		return router.selectBasicSpray(descriptor, copies, peers, nPeers), nil
	}
}

// selectBasicSpray runs algorithm in basic mode
// The originating node will spray the configured number od copies to other nodes, but other nodes don't replicate the bundle themselves
// A second forwarding hop only happen through direct transmission (when a carrying node encounters the recipient)
func (router *SprayAndWait) selectBasicSpray(descriptor *store.BundleDescriptor, copies uint64, peers []cla.ConvergenceSender, nPeers uint64) []cla.ConvergenceSender {
	log.WithField("bundle", descriptor.ID()).Debug("Spray&Wait running in basic mode")
	var remainingCopies uint64
	var selectedPeers []cla.ConvergenceSender
	if nPeers <= copies {
		log.WithField("bundle", descriptor.ID()).Debug("Fewer peers than remaining copies, sending to everyone")
		remainingCopies = copies - nPeers
		selectedPeers = peers
	} else {
		log.WithField("bundle", descriptor.ID()).Debug("More peers than remaining copies")
		remainingCopies = 0
		selectedPeers = peers[0:copies]
	}

	setCopies(descriptor, remainingCopies)
	log.WithFields(log.Fields{
		"bundle":           descriptor.ID(),
		"remaining copies": remainingCopies,
		"selected peers":   selectedPeers,
	}).Debug("Spray&Wait selected peers for forwarding")
	return selectedPeers
}

// selectBinarySpray runs the algorithm in binary mode
// The originating node starts with l copies, and every time it forwards the bundle, it is tagged with n/2 copies, while the transmitting node keeps the other n/2 for itself
// Since we need to modify the bundle and attach an appropriate extension block, we can only choose one peer per routing invocation.
func (router *SprayAndWait) selectBinarySpray(descriptor *store.BundleDescriptor, copies uint64, peers []cla.ConvergenceSender) ([]cla.ConvergenceSender, *bpv7.Bundle) {
	log.WithField("bundle", descriptor.ID()).Debug("Spray&Wait running in binary mode")
	sendCopies := copies / 2
	retainedCopies := copies / 2
	// if the number of copies is odd, we retain one more than we give away
	if (copies % 2) != 0 {
		retainedCopies += 1
	}

	log.WithFields(log.Fields{
		"bundle":        descriptor.ID(),
		"send copies":   sendCopies,
		"retain copies": retainedCopies,
	}).Debug("Spray&Wait: new copies")

	bundle, err := descriptor.Load()
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": descriptor.ID(),
			"error":  err,
		}).Error("Error loading bundle")
		return []cla.ConvergenceSender{}, nil
	}

	// remove all previous BinarySprayBlocks and attach our new one
	blocks, err := bundle.ExtensionBlocksByType(bpv7.BlockTypeBinarySprayBlock)
	if err == nil {
		for _, block := range blocks {
			bundle.RemoveExtensionBlockByBlockNumber(block.BlockNumber)
		}
	}

	block := bpv7.NewCanonicalBlock(0, 0, bpv7.NewBinarySprayBlock(sendCopies))
	err = bundle.AddExtensionBlock(block)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": descriptor.ID(),
			"error":  err,
		}).Error("Error adding block to bundle")
		return []cla.ConvergenceSender{}, nil
	}

	// pick a peer at random
	peer := []cla.ConvergenceSender{peers[rand.Intn(len(peers))]}
	log.WithFields(log.Fields{
		"bundle": descriptor.ID(),
		"peer":   peer[0],
	}).Debug("Binary Spray&Wait selected peer for forwarding")

	setCopies(descriptor, retainedCopies)

	return peer, bundle
}

func setCopies(descriptor *store.BundleDescriptor, copies uint64) {
	err := descriptor.SetMiscData("spray_and_wait/copies", copies)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": descriptor.ID(),
			"error":  err,
		}).Error("Spray&Wait could not set bundle copies")
	}
}

package routing

import (
	"testing"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
)

const (
	initialCopies  uint64 = 10
	receivedCopies uint64 = 5
)

func TestSprayBasic_NotifyNewBundle(t *testing.T) {
	router := NewSprayAndWait(initialCopies, false)
	setup(t, router)
	defer teardown(t)

	descriptor, bundle := testBundle(t)

	router.NotifyNewBundle(descriptor, bundle)

	data, ok := descriptor.GetMiscData(sprayBundleCopiesKey)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	copies := data.(uint64)

	if copies != initialCopies {
		t.Fatalf("Spray&Wait did not set correct initial copies. Wanted: %d, got: %d", initialCopies, copies)
	}
}

func TestSprayBinary_NotifyNewBundle(t *testing.T) {
	router := NewSprayAndWait(initialCopies, true)
	setup(t, router)
	defer teardown(t)

	descriptor, bundle := testBundle(t)

	router.NotifyNewBundle(descriptor, bundle)

	data, ok := descriptor.GetMiscData(sprayBundleCopiesKey)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	copies := data.(uint64)

	if copies != initialCopies {
		t.Fatalf("Spray&Wait did not set correct initial copies. Wanted: %d, got: %d", initialCopies, copies)
	}
}

func TestSprayBasic_NotifyReceivedBundle(t *testing.T) {
	router := NewSprayAndWait(initialCopies, false)
	setup(t, router)
	defer teardown(t)

	// test bundle with no BinarySprayBlock
	descriptor, bundle := testBundle(t)

	router.NotifyReceivedBundle(descriptor, bundle)

	copies, ok := getSprayCopies(descriptor)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	if copies != 0 {
		t.Fatalf("Spray&Wait did not set correct initial copies. Wanted: %d, got: %d", 0, copies)
	}

	// test bundle with BinarySprayBlock
	descriptor, bundle = testBundle(t)
	block := bpv7.NewCanonicalBlock(0, 0, bpv7.NewBinarySprayBlock(receivedCopies))
	err := bundle.AddExtensionBlock(block)
	if err != nil {
		t.Fatalf("Error adding BinarySprayBlock to bundle: %s", err)
	}

	router.NotifyReceivedBundle(descriptor, bundle)

	copies, ok = getSprayCopies(descriptor)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	if copies != receivedCopies {
		t.Fatalf("Spray&Wait did not set correct initial copies. Wanted: %d, got: %d", receivedCopies, copies)
	}
}

func TestSprayBinary_NotifyReceivedBundle(t *testing.T) {
	router := NewSprayAndWait(initialCopies, true)
	setup(t, router)
	defer teardown(t)

	// test bundle with no BinarySprayBlock
	descriptor, bundle := testBundle(t)

	router.NotifyReceivedBundle(descriptor, bundle)

	copies, ok := getSprayCopies(descriptor)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	if copies != 0 {
		t.Fatalf("Spray&Wait did not set correct initial copies. Wanted: %d, got: %d", 0, copies)
	}

	// test bundle with BinarySprayBlock
	descriptor, bundle = testBundle(t)
	block := bpv7.NewCanonicalBlock(0, 0, bpv7.NewBinarySprayBlock(receivedCopies))
	err := bundle.AddExtensionBlock(block)
	if err != nil {
		t.Fatalf("Error adding BinarySprayBlock to bundle: %s", err)
	}

	router.NotifyReceivedBundle(descriptor, bundle)

	copies, ok = getSprayCopies(descriptor)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	if copies != receivedCopies {
		t.Fatalf("Spray&Wait did not set correct initial copies. Wanted: %d, got: %d", receivedCopies, copies)
	}
}

func TestSprayBasic_SelectPeersForForwarding(t *testing.T) {
	router := NewSprayAndWait(initialCopies, false)
	setup(t, router)
	defer teardown(t)

	peers := generatePeers(15)

	descriptor, bundle := testBundle(t)
	router.NotifyNewBundle(descriptor, bundle)

	// feeding it 5 peers, should select all 5 and reduce the remaining copies by 5
	selectedPeers, modifiedBundle := router.SelectPeersForForwarding(descriptor, peers[:5])
	if modifiedBundle != nil {
		t.Fatal("Basic Spray routing should not modify bundle")
	}

	if len(selectedPeers) != 5 {
		t.Fatal("Spray&Wait selected wrong number of peers")
	}

	copies, ok := getSprayCopies(descriptor)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	if copies != 5 {
		t.Fatal("Bundle should have 5 copies left")
	}

	// add previously selected peers to bundle known holders
	for _, peer := range selectedPeers {
		err := descriptor.AddKnownHolder(peer.GetPeerEndpointID())
		if err != nil {
			t.Fatalf("Error adding peer to known holders: %s", err)
		}
	}

	// feeding it 8, of which 5 are the same as before, should just select the 3 new peers and reduce the remaining copies by 3
	selectedPeers, modifiedBundle = router.SelectPeersForForwarding(descriptor, peers[:8])
	if modifiedBundle != nil {
		t.Fatal("Basic Spray routing should not modify bundle")
	}

	if len(selectedPeers) != 3 {
		t.Fatalf("Spray&Wait selected wrong number of peers. Expected 3, got %d", len(selectedPeers))
	}

	copies, ok = getSprayCopies(descriptor)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	if copies != 2 {
		t.Fatal("Bundle should have 2 copies left")
	}

	// add previously selected peers to bundle known holders
	for _, peer := range selectedPeers {
		err := descriptor.AddKnownHolder(peer.GetPeerEndpointID())
		if err != nil {
			t.Fatalf("Error adding peer to known holders: %s", err)
		}
	}

	// feeding it all peers should now only select 2 and drop the remaining copies to 0
	selectedPeers, modifiedBundle = router.SelectPeersForForwarding(descriptor, peers)
	if modifiedBundle != nil {
		t.Fatal("Basic Spray routing should not modify bundle")
	}

	if len(selectedPeers) != 2 {
		t.Fatalf("Spray&Wait selected wrong number of peers. Expected 3, got %d", len(selectedPeers))
	}

	copies, ok = getSprayCopies(descriptor)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	if copies != 0 {
		t.Fatal("Bundle should have 0 copies left")
	}
}

func TestSprayBinary_SelectPeersForForwarding(t *testing.T) {
	router := NewSprayAndWait(initialCopies, true)
	setup(t, router)
	defer teardown(t)

	peers := generatePeers(15)

	descriptor, bundle := testBundle(t)
	router.NotifyNewBundle(descriptor, bundle)

	// calls should always just return a single peer, who will receive half of the copies
	selectedPeers, modifiedBundle := router.SelectPeersForForwarding(descriptor, peers)
	if modifiedBundle == nil {
		t.Fatal("Binary Spray routing should modify bundle")
	}

	if len(selectedPeers) != 1 {
		t.Fatal("Binary spray should only select 1 peer")
	}

	err := descriptor.AddKnownHolder(peers[0].GetPeerEndpointID())
	if err != nil {
		t.Fatalf("Error adding peer to known holders: %s", err)
	}

	copies, ok := getSprayCopies(descriptor)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	if copies != 5 {
		t.Fatal("Bundle should have 5 copies left")
	}

	block, err := modifiedBundle.ExtensionBlockByType(bpv7.BlockTypeBinarySprayBlock)
	if err != nil {
		t.Fatal("Bundle should have BinarySprayBlock")
	}

	attachedCopies := block.Value.(*bpv7.BinarySprayBlock).RemainingCopies()
	if attachedCopies != 5 {
		t.Fatal("Bundle should have 5 copies attached")
	}

	// do it again
	selectedPeers, modifiedBundle = router.SelectPeersForForwarding(descriptor, peers)
	if modifiedBundle == nil {
		t.Fatal("Binary Spray routing should modify bundle")
	}

	if len(selectedPeers) != 1 {
		t.Fatal("Binary spray should only select 1 peer")
	}

	err = descriptor.AddKnownHolder(peers[0].GetPeerEndpointID())
	if err != nil {
		t.Fatalf("Error adding peer to known holders: %s", err)
	}

	copies, ok = getSprayCopies(descriptor)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	if copies != 3 {
		t.Fatal("Bundle should have 3 copies left")
	}

	block, err = modifiedBundle.ExtensionBlockByType(bpv7.BlockTypeBinarySprayBlock)
	if err != nil {
		t.Fatal("Bundle should have BinarySprayBlock")
	}

	attachedCopies = block.Value.(*bpv7.BinarySprayBlock).RemainingCopies()
	if attachedCopies != 2 {
		t.Fatal("Bundle should have 2 copies attached")
	}

	// and again
	selectedPeers, modifiedBundle = router.SelectPeersForForwarding(descriptor, peers)
	if modifiedBundle == nil {
		t.Fatal("Binary Spray routing should modify bundle")
	}

	if len(selectedPeers) != 1 {
		t.Fatal("Binary spray should only select 1 peer")
	}

	err = descriptor.AddKnownHolder(peers[0].GetPeerEndpointID())
	if err != nil {
		t.Fatalf("Error adding peer to known holders: %s", err)
	}

	copies, ok = getSprayCopies(descriptor)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	if copies != 2 {
		t.Fatal("Bundle should have 2 copies left")
	}

	block, err = modifiedBundle.ExtensionBlockByType(bpv7.BlockTypeBinarySprayBlock)
	if err != nil {
		t.Fatal("Bundle should have BinarySprayBlock")
	}

	attachedCopies = block.Value.(*bpv7.BinarySprayBlock).RemainingCopies()
	if attachedCopies != 1 {
		t.Fatal("Bundle should have 1 copies attached")
	}

	// and again
	selectedPeers, modifiedBundle = router.SelectPeersForForwarding(descriptor, peers)
	if modifiedBundle == nil {
		t.Fatal("Binary Spray routing should modify bundle")
	}

	if len(selectedPeers) != 1 {
		t.Fatal("Binary spray should only select 1 peer")
	}

	err = descriptor.AddKnownHolder(peers[0].GetPeerEndpointID())
	if err != nil {
		t.Fatalf("Error adding peer to known holders: %s", err)
	}

	copies, ok = getSprayCopies(descriptor)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	if copies != 1 {
		t.Fatal("Bundle should have 1 copies left")
	}

	block, err = modifiedBundle.ExtensionBlockByType(bpv7.BlockTypeBinarySprayBlock)
	if err != nil {
		t.Fatal("Bundle should have BinarySprayBlock")
	}

	attachedCopies = block.Value.(*bpv7.BinarySprayBlock).RemainingCopies()
	if attachedCopies != 1 {
		t.Fatal("Bundle should have 1 copies attached")
	}

	// finally, there should be no copies left and forwarding should stop
	selectedPeers, modifiedBundle = router.SelectPeersForForwarding(descriptor, peers)
	if modifiedBundle != nil {
		t.Fatal("No copies left, so there should be no modified bundle")
	}

	if len(selectedPeers) != 0 {
		t.Fatal("No copies lest, so there should be no forwarding")
	}

	copies, ok = getSprayCopies(descriptor)
	if !ok {
		t.Fatalf("Could not retrieve bundle copies")
	}

	if copies != 1 {
		t.Fatal("Bundle should have 1 copies left")
	}
}

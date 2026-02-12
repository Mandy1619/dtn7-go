package routing

import (
	"fmt"
	"os"
	"testing"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/cla"
	"github.com/dtn7/dtn7-go/pkg/cla/dummy_cla"
	"github.com/dtn7/dtn7-go/pkg/store"
)

func setup(t *testing.T) {
	nodeID := bpv7.EndpointID{EndpointType: bpv7.DtnEndpoint{
		NodeName:  "test",
		Demux:     "",
		IsDtnNone: false,
	}}
	err := store.InitialiseStore(nodeID, "/tmp/dtn7-test")
	if err != nil {
		t.Fatal(err)
	}
}

func teardown(t *testing.T) {
	err := store.ShutdownStore()
	if err != nil {
		t.Fatal(err)
	}
	err = os.RemoveAll("/tmp/dtn7-test")
	if err != nil {
		t.Fatal(err)
	}
}

func TestEpidemicRouting_SelectPeersForForwarding(t *testing.T) {
	setup(t)
	defer teardown(t)

	nPeers := 10
	peers := make([]cla.ConvergenceSender, nPeers)
	for i := range peers {
		eid := bpv7.EndpointID{
			EndpointType: bpv7.DtnEndpoint{
				NodeName:  fmt.Sprintf("test_%d", i),
				Demux:     "",
				IsDtnNone: false,
			},
		}
		peers[i] = dummy_cla.NewSuperDummyCLA(eid)
	}

	bndl, err := bpv7.Builder().
		CRC(bpv7.CRC32).
		Source("dtn://source/").
		Destination("dtn://destination/").
		CreationTimestampEpoch().
		Lifetime("10m").
		HopCountBlock(64).
		BundleAgeBlock(0).
		PayloadBlock([]byte("hello world!")).
		Build()
	if err != nil {
		t.Fatalf("Error during bundle creation %s", err)
	}

	descriptor, err := store.GetStoreSingleton().InsertBundle(bndl)
	if err != nil {
		t.Fatalf("Error inserting bundle into store %s", err)
	}

	router := NewEpidemicRouting()

	// send to half of peers
	selectedPeers, modified := router.SelectPeersForForwarding(descriptor, peers[:5])
	if modified != nil {
		t.Fatal("Epidemic routing should not modify bundle")
	}

	if len(selectedPeers) != 5 {
		t.Fatal("Epidemic did not select correct number of peers")
	}

	for i := range selectedPeers {
		if selectedPeers[i] != peers[i] {
			t.Fatal("Epidemic did not select correct peer")
		}
	}

	// add previously selected peers to bundle known holders
	for _, peer := range selectedPeers {
		err = descriptor.AddKnownHolder(peer.GetPeerEndpointID())
		if err != nil {
			t.Fatalf("Error adding peer to known holders: %s", err)
		}
	}

	// giving the same peers again should select none
	selectedPeers, modified = router.SelectPeersForForwarding(descriptor, peers[:5])
	if modified != nil {
		t.Fatal("Epidemic routing should not modify bundle")
	}

	if len(selectedPeers) != 0 {
		t.Fatal("Epidemic should not select known holders")
	}

	// giving all peers should only select new peers
	selectedPeers, modified = router.SelectPeersForForwarding(descriptor, peers)
	if modified != nil {
		t.Fatal("Epidemic routing should not modify bundle")
	}

	if len(selectedPeers) != 5 {
		t.Fatal("Epidemic did not select correct number of peers")
	}

	for i := range selectedPeers {
		if selectedPeers[i] != peers[i+5] {
			t.Fatal("Epidemic did not select correct peer")
		}
	}
}

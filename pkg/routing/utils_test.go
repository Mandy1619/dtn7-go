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

func setup(t *testing.T, router Algorithm) {
	nodeID := bpv7.EndpointID{EndpointType: bpv7.DtnEndpoint{
		NodeName:  "test",
		Demux:     "",
		IsDtnNone: false,
	}}
	err := store.InitialiseStore(nodeID, "/tmp/dtn7-test")
	if err != nil {
		t.Fatal(err)
	}
	InitialiseAlgorithm(router)
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
	ShutdownAlgorithm()
}

func generatePeers(n uint) []cla.ConvergenceSender {
	peers := make([]cla.ConvergenceSender, n)
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
	return peers
}

func testBundle(t *testing.T) (*store.BundleDescriptor, *bpv7.Bundle) {
	bundle, err := bpv7.Builder().
		CRC(bpv7.CRC32).
		Source("dtn://source/").
		Destination("dtn://destination/").
		CreationTimestampNow().
		Lifetime("10m").
		HopCountBlock(64).
		BundleAgeBlock(0).
		PayloadBlock([]byte("hello world!")).
		Build()
	if err != nil {
		t.Fatalf("Error during bundle creation %s", err)
	}

	descriptor, err := store.GetStoreSingleton().InsertBundle(bundle)
	if err != nil {
		t.Fatalf("Error inserting bundle into store %s", err)
	}

	return descriptor, bundle
}

package quicl

import (
	"testing"

	"github.com/dtn7/dtn7-go/pkg/bpv7"
)

func generateSampleBundle(t *testing.T) *bpv7.Bundle {
	bndl, err := bpv7.Builder().
		CRC(bpv7.CRC32).
		Source("dtn://myself/").
		Destination("dtn://dest/").
		CreationTimestampEpoch().
		Lifetime("10m").
		HopCountBlock(64).
		BundleAgeBlock(0).
		PayloadBlock([]byte("hello world!")).
		Build()
	if err != nil {
		t.Fatalf("Error during bundle creation %s", err)
	}
	return bndl
}

package store

import (
	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"pgregory.net/rapid"
)

var randomizedBundleGenerator = rapid.Custom(func(t *rapid.T) *bpv7.Bundle {
	bndl, err := bpv7.Builder().
		CRC(bpv7.CRC32).
		Source(rapid.StringMatching(bpv7.DtnEndpointRegexpNotNone).Draw(t, "sourceID")).
		Destination(rapid.StringMatching(bpv7.DtnEndpointRegexpFull).Draw(t, "destinationID")).
		CreationTimestampNow().
		Lifetime("10m").
		HopCountBlock(64).
		BundleAgeBlock(0).
		PayloadBlock([]byte(rapid.String().Draw(t, "payload"))).
		Build()
	if err != nil {
		t.Fatalf("Error during bundle creation %s", err)
	}
	return bndl
})

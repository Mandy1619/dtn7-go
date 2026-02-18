package processing

import (
	log "github.com/sirupsen/logrus"

	"github.com/dtn7/dtn7-go/pkg/application_agent"
	"github.com/dtn7/dtn7-go/pkg/bpv7"
	"github.com/dtn7/dtn7-go/pkg/routing"
	"github.com/dtn7/dtn7-go/pkg/store"
)

func receiveAsync(bundle *bpv7.Bundle) {
	isAdminRecord := bundle.IsAdministrativeRecord()
	if isAdminRecord {
		handleAdministrativeRecord(bundle)
	} else {
		handleBundle(bundle, false)
	}
}

// handleBundle handles a bundle which does not contain an administrative record
func handleBundle(bundle *bpv7.Bundle, new bool) {
	bundleDescriptor, err := store.GetStoreSingleton().InsertBundle(bundle)
	if err != nil {
		log.WithFields(log.Fields{
			"bundle": bundle.ID(),
			"error":  err,
		}).Error("Error storing new bundle")
		return
	}

	application_agent.GetManagerSingleton().DeliverBundle(bundleDescriptor)

	if new {
		routing.GetAlgorithmSingleton().NotifyNewBundle(bundleDescriptor, bundle)
	} else {
		routing.GetAlgorithmSingleton().NotifyReceivedBundle(bundleDescriptor, bundle)
	}

	if dispatch, err := bundleDescriptor.HasConstraint(store.DispatchPending); err == nil && dispatch {
		log.WithField("bundle", bundleDescriptor).Debug("Forwarding received bundle")
		BundleForwarding(bundleDescriptor)
	}
}

// handleAdministrativeRecord handles reception logic for bundles which contains an administrative record
func handleAdministrativeRecord(bundle *bpv7.Bundle) {
	if routing.GetAlgorithmSingleton().NotifyReceivedAdministrativeRecord(bundle) && application_agent.GetManagerSingleton().DeliverAdministrativeRecord(bundle) {
		bundleDescriptor, err := store.GetStoreSingleton().InsertBundle(bundle)
		if err != nil {
			log.WithFields(log.Fields{
				"bundle": bundle.ID(),
				"error":  err,
			}).Error("Error storing new bundle")
			return
		}

		BundleForwarding(bundleDescriptor)
	}
}

// ReceiveBundle is to be called when a bundle is received from another node
func ReceiveBundle(bundle *bpv7.Bundle) {
	go receiveAsync(bundle)
}

// NewBundle is to be called if a bundle was created on this node
func NewBundle(bundle *bpv7.Bundle) {
	go handleBundle(bundle, true)
}

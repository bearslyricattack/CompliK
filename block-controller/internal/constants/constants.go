package constants

const (
	BlockRequestFinalizer = "core.clawcloud.run/finalizer"

	LockedStatus = "locked"
	ActiveStatus = "active"

	StatusLabel                = "clawcloud.run/status"
	UnlockTimestampLabel       = "clawcloud.run/unlock-timestamp"
	OriginalReplicasAnnotation = "core.clawcloud.run/original-replicas"
	OriginalSuspendAnnotation  = "core.clawcloud.run/original-suspend"

	ResourceQuotaName = "block-controller-quota"
)

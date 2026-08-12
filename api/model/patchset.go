package model

// Patchset statuses, as stored in patchset.status. These were generated from
// the GraphQL schema; they are hand-written now that there is none.
type PatchsetStatus string

const (
	PatchsetStatusUnknown       PatchsetStatus = "UNKNOWN"
	PatchsetStatusProposed      PatchsetStatus = "PROPOSED"
	PatchsetStatusNeedsRevision PatchsetStatus = "NEEDS_REVISION"
	PatchsetStatusSuperseded    PatchsetStatus = "SUPERSEDED"
	PatchsetStatusApproved      PatchsetStatus = "APPROVED"
	PatchsetStatusRejected      PatchsetStatus = "REJECTED"
	PatchsetStatusApplied       PatchsetStatus = "APPLIED"
)

func (e PatchsetStatus) IsValid() bool {
	switch e {
	case PatchsetStatusUnknown, PatchsetStatusProposed, PatchsetStatusNeedsRevision,
		PatchsetStatusSuperseded, PatchsetStatusApproved, PatchsetStatusRejected,
		PatchsetStatusApplied:
		return true
	}
	return false
}

func (e PatchsetStatus) String() string {
	return string(e)
}

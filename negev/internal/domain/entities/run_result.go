package entities

type RunResult struct {
	PortsChanged int
	VLANsCreated int
	VLANsDeleted int
	PortsFailed  int
	Errors       []error
}

func (r RunResult) HasChanges() bool {
	return r.PortsChanged > 0 || r.VLANsCreated > 0 || r.VLANsDeleted > 0
}

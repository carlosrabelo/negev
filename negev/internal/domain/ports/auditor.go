package ports

// PortVLANAudit is one SET_PORT_VLAN audit record.
type PortVLANAudit struct {
	Timestamp  string
	User       string
	Target     string
	SwitchName string
	Port       string
	MAC        string
	OldVLAN    int
	NewVLAN    int
	Action     string
	Sandbox    bool
	Status     string
}

// Auditor appends immutable audit records for VLAN changes.
type Auditor interface {
	AppendPortVLAN(entry PortVLANAudit) error
}

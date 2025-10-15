package entities

// Device stores discovered device information on a port
type Device struct {
	Vlan      string
	Mac       string
	MacFull   string
	Interface string
}

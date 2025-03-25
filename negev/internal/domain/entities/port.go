package entities

type Port struct {
	Interface string
	Vlan      string
	IsRouted  bool
	IsTrunk   bool
}

package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
)

const (
	DebounceTime = 10 * time.Second // Minimum time between VLAN configurations
)

// TrapState holds the state of the last processed trap for debouncing
type TrapState struct {
	lastTrapTime time.Time
	lastVlan     string
	mutex        sync.Mutex
}

// trapStates stores trap states by switch and port
var trapStates = make(map[string]*TrapState)
var trapStateMutex sync.Mutex

// RunSNMP starts the daemon to listen for SNMP traps and configure VLANs
func RunSNMP(cfg *Config, verbose, extra bool) error {
	// Map to look up switch configurations by IP
	switchMap := make(map[string]SwitchConfig)
	for _, sw := range cfg.Switches {
		sw.Verbose = verbose // Apply verbosity to all switch configurations
		sw.Extra = extra     // Apply raw output display
		switchMap[sw.Target] = sw
	}

	// Initialize the SNMP trap listener
	listener := gosnmp.NewTrapListener()
	listener.Params = &gosnmp.GoSNMP{
		Port:      uint16(cfg.SnmpPort),
		Community: cfg.SnmpCommunity,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(5) * time.Second,
		Transport: "udp", // Ensure IPv4
	}

	// Define handler to process traps
	listener.OnNewTrap = func(packet *gosnmp.SnmpPacket, addr *net.UDPAddr) {
		// Log all received traps
		if verbose {
			fmt.Printf("DEBUG: Received trap from %s, OID=%s\n", addr.IP.String(), packet.Variables[0].Name)
		}

		// Check if the sender's IP is in the list of switches
		switchCfg, exists := switchMap[addr.IP.String()]
		if !exists {
			log.Printf("Trap from %s not registered in YAML", addr.IP.String())
			return
		}

		// Log trap reception only if verbose is enabled
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Trap received from %s\n", addr.IP.String())
		}

		// Log all trap variables
		var macAddress string
		var port uint16
		var operation uint8
		for _, variable := range packet.Variables {
			oid := variable.Name
			valueStr := fmt.Sprintf("%v", variable.Value)
			if switchCfg.Extra {
				fmt.Printf("Switch output: Trap variable: OID=%s, Value=%s\n", oid, valueStr)
			}

			// Process only cmnMacChangedNotification traps
			if oid == ".1.3.6.1.6.3.1.1.4.1.0" && variable.Value == ".1.3.6.1.4.1.9.9.215.2.0.1" {
				// Look for cmnHistMacChangedMsg
				for _, v := range packet.Variables {
					if strings.HasPrefix(v.Name, ".1.3.6.1.4.1.9.9.215.1.1.8.1.2") {
						if bytes, ok := v.Value.([]byte); ok && len(bytes) >= 11 {
							operation = bytes[0]
							// Extract VLAN, MAC, port
							vlan := binary.BigEndian.Uint16(bytes[1:3])
							macBytes := bytes[3:9]
							macAddress = fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x",
								macBytes[0], macBytes[1], macBytes[2], macBytes[3], macBytes[4], macBytes[5])
							port = binary.BigEndian.Uint16(bytes[9:11])
							if switchCfg.Verbose {
								fmt.Printf("DEBUG: Processed trap: MAC=%s, VLAN=%d, dot1dBasePort=%d, Operation=%d\n", macAddress, vlan, port, operation)
							}
						} else {
							log.Printf("Error: Invalid or too short cmnHistMacChangedMsg: %v", v.Value)
							return
						}
					}
				}

				if macAddress == "" {
					log.Printf("Error: Could not extract MAC from trap")
					return
				}

				// Key to track trap state
				trapKey := fmt.Sprintf("%s:%d", switchCfg.Target, port)

				// Get or create trap state
				trapStateMutex.Lock()
				state, exists := trapStates[trapKey]
				if !exists {
					state = &TrapState{}
					trapStates[trapKey] = state
				}
				trapStateMutex.Unlock()

				// Check debounce
				state.mutex.Lock()
				defer state.mutex.Unlock()
				now := time.Now()
				if now.Sub(state.lastTrapTime) < DebounceTime && state.lastVlan != "" {
					if switchCfg.Verbose {
						fmt.Printf("DEBUG: Ignoring trap for %s due to debounce (last VLAN: %s, time since last trap: %v)\n", trapKey, state.lastVlan, now.Sub(state.lastTrapTime))
					}
					return
				}

				// Process based on operation
				if operation == 1 { // MAC learnt
					// Configure VLAN for the MAC and dot1dBasePort via SNMP
					err := configureVlanForTrap(switchCfg, *cfg, macAddress, port)
					if err != nil {
						log.Printf("Error configuring VLAN for MAC %s with dot1dBasePort %d on switch %s: %v", macAddress, port, switchCfg.Target, err)
					} else {
						state.lastTrapTime = now
						state.lastVlan = switchCfg.DefaultVlan
					}
				} else if operation == 2 { // MAC removed
					// Revert VLAN to no_data_vlan
					err := revertVlanForTrap(switchCfg, *cfg, macAddress, port)
					if err != nil {
						log.Printf("Error reverting VLAN for MAC %s with dot1dBasePort %d on switch %s: %v", macAddress, port, switchCfg.Target, err)
					} else {
						state.lastTrapTime = now
						state.lastVlan = switchCfg.NoDataVlan
						// Add delay to ensure application before new traps
						time.Sleep(2 * time.Second)
					}
				} else {
					if switchCfg.Verbose {
						fmt.Printf("DEBUG: Ignoring trap with unknown operation %d\n", operation)
					}
				}
			}
		}
	}

	// Start the listener on the specified IP and port
	listenerAddress := fmt.Sprintf("%s:%d", cfg.ServerIP, cfg.SnmpPort)
	fmt.Printf("SNMP daemon started, listening for traps on %s with community %s...\n", listenerAddress, cfg.SnmpCommunity)
	err := listener.Listen(listenerAddress)
	if err != nil {
		return fmt.Errorf("failed to start SNMP listener on %s: %v", listenerAddress, err)
	}

	// Keep the program running
	select {}
}

// isTrunkPort checks if the port with the specified ifIndex is in trunk mode
func isTrunkPort(client *gosnmp.GoSNMP, ifIndex int, verbose bool) (bool, error) {
	oid := fmt.Sprintf(".1.3.6.1.4.1.9.9.68.1.2.2.1.1.%d", ifIndex) // vmVlanType
	result, err := client.Get([]string{oid})
	if err != nil {
		return false, fmt.Errorf("failed to query vmVlanType for ifIndex %d: %v", ifIndex, err)
	}

	for _, v := range result.Variables {
		if v.Name == oid && v.Type == gosnmp.Integer {
			mode, ok := v.Value.(int)
			if !ok {
				return false, fmt.Errorf("vmVlanType %v is not an integer for ifIndex %d", v.Value, ifIndex)
			}
			if verbose {
				fmt.Printf("DEBUG: Port mode for ifIndex %d: %d (1=access, 2=trunk, 3=multiVlan)\n", ifIndex, mode)
			}
			return mode == 2, nil // Return true if trunk
		}
	}

	return false, fmt.Errorf("vmVlanType not found for ifIndex %d", ifIndex)
}

// configureVlanForTrap configures the VLAN for a MAC based on dot1dBasePort via SNMP
func configureVlanForTrap(switchCfg SwitchConfig, cfg Config, mac string, port uint16) error {
	normMac := normalizeMac(mac)
	macPrefix := normMac[:6]

	// Check if the MAC is excluded
	for _, excludeMac := range switchCfg.ExcludeMacs {
		if normMac == excludeMac {
			if switchCfg.Verbose {
				fmt.Printf("DEBUG: Ignoring MAC %s with dot1dBasePort %d due to exclusion\n", mac, port)
			}
			return nil
		}
	}

	// Initialize SNMP client
	client := &gosnmp.GoSNMP{
		Target:    switchCfg.Target,
		Port:      161,
		Community: cfg.SnmpCommunity,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(5) * time.Second,
	}
	err := client.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to switch %s via SNMP: %v", switchCfg.Target, err)
	}
	defer client.Conn.Close()

	// Get ifIndex with retries
	var ifIndex int
	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ifIndex, err = getIfIndexFromPort(client, port, switchCfg.Verbose)
		if err == nil {
			break
		}
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Attempt %d of %d failed to get ifIndex for dot1dBasePort %d: %v\n", attempt, maxRetries, port, err)
		}
		if attempt < maxRetries {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		// Fallback to ifTable
		iface := fmt.Sprintf("GigabitEthernet1/0/%d", port)
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Falling back to ifTable with inferred interface %s\n", iface)
		}
		ifIndex, err = getIfIndexFromIfTable(client, iface, switchCfg.Verbose)
		if err != nil {
			return fmt.Errorf("failed to get ifIndex for dot1dBasePort %d or interface %s: %v", port, iface, err)
		}
	}

	// Check if the port is trunk
	isTrunk, err := isTrunkPort(client, ifIndex, switchCfg.Verbose)
	if err != nil {
		log.Printf("Warning: Failed to check if port is trunk for ifIndex %d: %v", ifIndex, err)
		// Continue, as we cannot confirm the port mode
	}
	if isTrunk {
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Ignoring trunk interface with dot1dBasePort %d (ifIndex %d)\n", port, ifIndex)
		}
		return nil
	}

	// Determine target VLAN
	targetVlan := switchCfg.MacToVlan[macPrefix]
	if switchCfg.Verbose {
		fmt.Printf("DEBUG: MAC prefix %s mapped to VLAN %s in MacToVlan\n", macPrefix, targetVlan)
	}
	if targetVlan == "" {
		targetVlan = switchCfg.DefaultVlan
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: No VLAN mapping for %s with dot1dBasePort %d, using switch default_vlan %s\n", mac, port, targetVlan)
			fmt.Printf("DEBUG: Final default_vlan value for switch %s: %s\n", switchCfg.Target, targetVlan)
		}
	}

	// Configure VLAN via SNMP (using CISCO-VLAN-MEMBERSHIP-MIB)
	vNum, err := parseVlanNumber(targetVlan)
	if err != nil {
		return fmt.Errorf("invalid VLAN %s: %v", targetVlan, err)
	}

	// Attempt to configure VLAN with retries
	const setRetries = 3
	for attempt := 1; attempt <= setRetries; attempt++ {
		oid := fmt.Sprintf(".1.3.6.1.4.1.9.9.68.1.2.2.1.2.%d", ifIndex)
		pdu := gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.Integer,
			Value: vNum,
		}
		result, err := client.Set([]gosnmp.SnmpPDU{pdu})
		if err == nil {
			if switchCfg.Verbose {
				fmt.Printf("DEBUG: SET result for VLAN %s (attempt %d): %v\n", targetVlan, attempt, result)
			}
			break
		}
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Attempt %d of %d failed to configure VLAN %s for dot1dBasePort %d (ifIndex %d): %v\n", attempt, setRetries, targetVlan, port, ifIndex, err)
		}
		if attempt < setRetries {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to configure VLAN %s for dot1dBasePort %d (ifIndex %d) after %d attempts: %v", targetVlan, port, ifIndex, setRetries, err)
	}

	// Verify if the VLAN was configured correctly
	currentVlan, err := getCurrentVlan(client, ifIndex, switchCfg.Verbose)
	if err != nil {
		log.Printf("Warning: Failed to verify current VLAN for dot1dBasePort %d (ifIndex %d): %v", port, ifIndex, err)
	} else if currentVlan != vNum {
		log.Printf("Warning: Configured VLAN (%s) does not match current VLAN (%d) for dot1dBasePort %d (ifIndex %d)", targetVlan, currentVlan, port, ifIndex)
	} else {
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Verified: VLAN %s configured correctly for dot1dBasePort %d (ifIndex %d)\n", targetVlan, port, ifIndex)
		}
	}

	// Display message when VLAN is changed
	fmt.Printf("VLAN %s configured for interface with dot1dBasePort %d (ifIndex %d) on switch %s\n", targetVlan, port, ifIndex, switchCfg.Target)

	if switchCfg.Verbose {
		fmt.Printf("DEBUG: Configured VLAN %s for dot1dBasePort %d (ifIndex %d) via SNMP\n", targetVlan, port, ifIndex)
	}

	return nil
}

// revertVlanForTrap reverts the VLAN to no_data_vlan when a device is disconnected
func revertVlanForTrap(switchCfg SwitchConfig, cfg Config, mac string, port uint16) error {
	// Determine quarantine VLAN
	targetVlan := switchCfg.NoDataVlan
	if switchCfg.Verbose {
		fmt.Printf("DEBUG: no_data_vlan value for switch %s: %s\n", switchCfg.Target, targetVlan)
		fmt.Printf("DEBUG: Reverting to no_data_vlan %s for MAC %s with dot1dBasePort %d\n", targetVlan, mac, port)
	}

	// Initialize SNMP client
	client := &gosnmp.GoSNMP{
		Target:    switchCfg.Target,
		Port:      161,
		Community: cfg.SnmpCommunity,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(5) * time.Second,
	}
	err := client.Connect()
	if err != nil {
		return fmt.Errorf("failed to connect to switch %s via SNMP: %v", switchCfg.Target, err)
	}
	defer client.Conn.Close()

	// Get ifIndex with retries
	var ifIndex int
	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		ifIndex, err = getIfIndexFromPort(client, port, switchCfg.Verbose)
		if err == nil {
			break
		}
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Attempt %d of %d failed to get ifIndex for dot1dBasePort %d: %v\n", attempt, maxRetries, port, err)
		}
		if attempt < maxRetries {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		// Fallback to ifTable
		iface := fmt.Sprintf("GigabitEthernet1/0/%d", port)
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Falling back to ifTable with inferred interface %s\n", iface)
		}
		ifIndex, err = getIfIndexFromIfTable(client, iface, switchCfg.Verbose)
		if err != nil {
			return fmt.Errorf("failed to get ifIndex for dot1dBasePort %d or interface %s: %v", port, iface, err)
		}
	}

	// Check if the port is trunk
	isTrunk, err := isTrunkPort(client, ifIndex, switchCfg.Verbose)
	if err != nil {
		log.Printf("Warning: Failed to check if port is trunk for ifIndex %d: %v", ifIndex, err)
		// Continue, as we cannot confirm the port mode
	}
	if isTrunk {
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Ignoring trunk interface with dot1dBasePort %d (ifIndex %d) for reversion\n", port, ifIndex)
		}
		return nil
	}

	// Configure VLAN via SNMP (using CISCO-VLAN-MEMBERSHIP-MIB)
	vNum, err := parseVlanNumber(targetVlan)
	if err != nil {
		return fmt.Errorf("invalid no_data_vlan %s: %v", targetVlan, err)
	}

	// Attempt to configure VLAN with retries
	const setRetries = 5
	for attempt := 1; attempt <= setRetries; attempt++ {
		oid := fmt.Sprintf(".1.3.6.1.4.1.9.9.68.1.2.2.1.2.%d", ifIndex)
		pdu := gosnmp.SnmpPDU{
			Name:  oid,
			Type:  gosnmp.Integer,
			Value: vNum,
		}
		result, err := client.Set([]gosnmp.SnmpPDU{pdu})
		if err == nil {
			if switchCfg.Verbose {
				fmt.Printf("DEBUG: SET result for no_data_vlan %s (attempt %d): %v\n", targetVlan, attempt, result)
			}
			break
		}
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Attempt %d of %d failed to configure no_data_vlan %s for dot1dBasePort %d (ifIndex %d): %v\n", attempt, setRetries, targetVlan, port, ifIndex, err)
		}
		if attempt < maxRetries {
			time.Sleep(2 * time.Second)
		}
	}
	if err != nil {
		return fmt.Errorf("failed to revert to no_data_vlan %s for dot1dBasePort %d (ifIndex %d) after %d attempts: %v", targetVlan, port, ifIndex, setRetries, err)
	}

	// Verify if the VLAN was configured correctly
	currentVlan, err := getCurrentVlan(client, ifIndex, switchCfg.Verbose)
	if err != nil {
		log.Printf("Warning: Failed to verify current VLAN for dot1dBasePort %d (ifIndex %d): %v", port, ifIndex, err)
	} else if currentVlan != vNum {
		log.Printf("Warning: Configured VLAN (%s) does not match current VLAN (%d) for dot1dBasePort %d (ifIndex %d)", targetVlan, currentVlan, port, ifIndex)
	} else {
		if switchCfg.Verbose {
			fmt.Printf("DEBUG: Verified: no_data_vlan %s configured correctly for dot1dBasePort %d (ifIndex %d)\n", targetVlan, port, ifIndex)
		}
	}

	// Display message when VLAN is changed
	fmt.Printf("Reverted to no_data_vlan %s on interface with dot1dBasePort %d (ifIndex %d) on switch %s\n", targetVlan, port, ifIndex, switchCfg.Target)

	if switchCfg.Verbose {
		fmt.Printf("DEBUG: Reverted to no_data_vlan %s for dot1dBasePort %d (ifIndex %d) via SNMP\n", targetVlan, port, ifIndex)
	}

	return nil
}

// getIfIndexFromPort retrieves the ifIndex corresponding to the dot1dBasePort
func getIfIndexFromPort(client *gosnmp.GoSNMP, port uint16, verbose bool) (int, error) {
	oid := fmt.Sprintf(".1.3.6.1.2.1.17.1.4.1.2.%d", port) // dot1dBasePortToIfIndex
	result, err := client.Get([]string{oid})
	if err != nil {
		return 0, fmt.Errorf("failed to query dot1dBasePortTable: %v", err)
	}

	for _, v := range result.Variables {
		if v.Name == oid && v.Type == gosnmp.Integer {
			ifIndex, ok := v.Value.(int)
			if !ok {
				return 0, fmt.Errorf("ifIndex %v is not an integer", v.Value)
			}
			if verbose {
				fmt.Printf("DEBUG: Found ifIndex %d for dot1dBasePort %d\n", ifIndex, port)
			}
			return ifIndex, nil
		}
	}

	return 0, fmt.Errorf("dot1dBasePort %d not found in dot1dBasePortTable", port)
}

// getIfIndexFromIfTable retrieves the ifIndex for an interface using the ifTable
func getIfIndexFromIfTable(client *gosnmp.GoSNMP, iface string, verbose bool) (int, error) {
	oids := []string{
		".1.3.6.1.2.1.2.2.1.2",    // ifDescr
		".1.3.6.1.2.1.31.1.1.1.1", // ifName
	}
	var ifIndex int
	for _, baseOid := range oids {
		err := client.Walk(baseOid, func(pdu gosnmp.SnmpPDU) error {
			if pdu.Type == gosnmp.OctetString {
				ifName := string(pdu.Value.([]byte))
				if verbose {
					fmt.Printf("DEBUG: Found interface: OID=%s, Name=%s\n", pdu.Name, ifName)
				}
				// Partial match to handle format variations
				if strings.Contains(strings.ToLower(ifName), strings.ToLower(strings.ReplaceAll(iface, "GigabitEthernet1/0/", "Gi1/0/"))) ||
					strings.Contains(strings.ToLower(ifName), strings.ToLower(iface)) {
					parts := strings.Split(pdu.Name, ".")
					if len(parts) > 0 {
						var err error
						ifIndex, err = strconv.Atoi(parts[len(parts)-1])
						if err != nil {
							return fmt.Errorf("failed to extract ifIndex from OID %s: %v", pdu.Name, err)
						}
						return fmt.Errorf("found") // Stop the Walk
					}
				}
			}
			return nil
		})
		if err != nil && err.Error() == "found" {
			if ifIndex != 0 {
				return ifIndex, nil
			}
		} else if err != nil {
			log.Printf("Warning: Error querying %s: %v", baseOid, err)
		}
	}

	return 0, fmt.Errorf("interface %s not found in ifTable", iface)
}

// getCurrentVlan retrieves the current VLAN of an interface via SNMP
func getCurrentVlan(client *gosnmp.GoSNMP, ifIndex int, verbose bool) (int, error) {
	oid := fmt.Sprintf(".1.3.6.1.4.1.9.9.68.1.2.2.1.2.%d", ifIndex)
	result, err := client.Get([]string{oid})
	if err != nil {
		return 0, fmt.Errorf("failed to query current VLAN: %v", err)
	}

	for _, v := range result.Variables {
		if v.Name == oid && v.Type == gosnmp.Integer {
			vlan, ok := v.Value.(int)
			if !ok {
				return 0, fmt.Errorf("current VLAN %v is not an integer", v.Value)
			}
			if verbose {
				fmt.Printf("DEBUG: Current VLAN for ifIndex %d: %d\n", ifIndex, vlan)
			}
			return vlan, nil
		}
	}

	return 0, fmt.Errorf("current VLAN not found for ifIndex %d", ifIndex)
}

// parseVlanNumber converts a VLAN string to a number
func parseVlanNumber(vlan string) (int, error) {
	vlanNum, err := strconv.Atoi(vlan)
	if err != nil {
		return 0, fmt.Errorf("invalid VLAN number: %v", err)
	}
	if vlanNum < 1 || vlanNum > 4094 {
		return 0, fmt.Errorf("VLAN number %d is out of allowed range (1-4094)", vlanNum)
	}
	return vlanNum, nil
}

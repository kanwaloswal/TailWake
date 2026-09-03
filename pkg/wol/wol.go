// Package wol implements the standard Wake-on-LAN (WoL) protocol over UDP sockets.
//
// How Wake-on-LAN Works:
// A Wake-on-LAN "Magic Packet" is a raw network frame composed of:
// 1. A 6-byte synchronization stream of 0xFF (hex FF FF FF FF FF FF).
// 2. The target machine's 6-byte hardware MAC address repeated 16 consecutive times.
// Total payload size: 6 + (6 * 16) = 102 bytes.
//
// When the network interface card (NIC) of a sleeping machine receives this payload
// on its Ethernet/Wi-Fi interface, it signals the motherboard to wake up the system.
package wol

import (
	"bytes"
	"fmt"
	"net"
	"time"
)

// MagicPacket represents a 102-byte Wake-on-LAN magic packet structure.
type MagicPacket [102]byte

// BuildMagicPacket constructs a 102-byte magic packet buffer for a target MAC address.
// Acceptable MAC formats include "AA:BB:CC:DD:EE:FF", "aa-bb-cc-dd-ee-ff", or "aabbccddeeff".
func BuildMagicPacket(macAddrStr string) (MagicPacket, error) {
	var packet MagicPacket

	// Parse human-readable MAC string into a 6-byte slice using Go's standard net package
	hwAddr, err := net.ParseMAC(macAddrStr)
	if err != nil {
		return packet, fmt.Errorf("invalid MAC address '%s': %w", macAddrStr, err)
	}

	if len(hwAddr) != 6 {
		return packet, fmt.Errorf("MAC address must be 6 bytes (got %d bytes for '%s')", len(hwAddr), macAddrStr)
	}

	// 1. Fill first 6 bytes with 0xFF (synchronization header)
	for i := 0; i < 6; i++ {
		packet[i] = 0xFF
	}

	// 2. Repeat target MAC address 16 times in succession
	for i := 1; i <= 16; i++ {
		copy(packet[i*6:(i+1)*6], hwAddr)
	}

	return packet, nil
}

// SendMagicPacket transmits the WoL magic packet to the target broadcast address and port.
// - targetMAC: e.g. "AA:BB:CC:DD:EE:FF"
// - broadcastIP: e.g. "255.255.255.255" or "192.168.1.255"
// - port: standard WoL UDP port is 9 (or 7)
func SendMagicPacket(targetMAC string, broadcastIP string, port int) error {
	// Construct the 102-byte packet payload
	packet, err := BuildMagicPacket(targetMAC)
	if err != nil {
		return err
	}

	if broadcastIP == "" {
		broadcastIP = "255.255.255.255"
	}
	if port <= 0 {
		port = 9
	}

	// Resolve target broadcast UDP address
	addrStr := fmt.Sprintf("%s:%d", broadcastIP, port)
	udpAddr, err := net.ResolveUDPAddr("udp", addrStr)
	if err != nil {
		return fmt.Errorf("failed to resolve UDP broadcast address '%s': %w", addrStr, err)
	}

	// Create an unbound UDP socket connection
	// We bind to a local ephemeral port (0) and send to the broadcast address
	conn, err := net.DialUDP("udp", nil, udpAddr)
	if err != nil {
		return fmt.Errorf("failed to open UDP socket for %s: %w", addrStr, err)
	}
	defer conn.Close()

	// Set socket deadline to prevent hangs
	_ = conn.SetDeadline(time.Now().Add(2 * time.Second))

	// Write packet bytes to network socket
	n, err := conn.Write(packet[:])
	if err != nil {
		return fmt.Errorf("failed to transmit magic packet to %s: %w", addrStr, err)
	}

	if n != len(packet) {
		return fmt.Errorf("short write: sent %d of %d bytes", n, len(packet))
	}

	return nil
}

// ValidateMAC checks if a string is a valid 6-byte MAC address.
func ValidateMAC(macAddrStr string) bool {
	hw, err := net.ParseMAC(macAddrStr)
	return err == nil && len(hw) == 6
}

// VerifyPacketContent checks if a magic packet payload matches the expected MAC address.
func VerifyPacketContent(packet [102]byte, macAddrStr string) bool {
	hwAddr, err := net.ParseMAC(macAddrStr)
	if err != nil || len(hwAddr) != 6 {
		return false
	}

	// Check header (first 6 bytes must be 0xFF)
	for i := 0; i < 6; i++ {
		if packet[i] != 0xFF {
			return false
		}
	}

	// Check 16 MAC repetitions
	for i := 1; i <= 16; i++ {
		if !bytes.Equal(packet[i*6:(i+1)*6], hwAddr) {
			return false
		}
	}

	return true
}

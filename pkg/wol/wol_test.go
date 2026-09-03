package wol

import (
	"testing"
)

func TestBuildMagicPacket(t *testing.T) {
	macStr := "AA:BB:CC:DD:EE:FF"
	packet, err := BuildMagicPacket(macStr)
	if err != nil {
		t.Fatalf("unexpected error building magic packet: %v", err)
	}

	if !VerifyPacketContent(packet, macStr) {
		t.Errorf("magic packet content verification failed for MAC %s", macStr)
	}
}

func TestInvalidMAC(t *testing.T) {
	invalidMACs := []string{
		"invalid-mac",
		"AA:BB:CC:DD:EE",       // 5 bytes
		"AA:BB:CC:DD:EE:FF:GG", // invalid hex
		"",
	}

	for _, mac := range invalidMACs {
		if ValidateMAC(mac) {
			t.Errorf("expected ValidateMAC('%s') to be false, got true", mac)
		}
		_, err := BuildMagicPacket(mac)
		if err == nil {
			t.Errorf("expected BuildMagicPacket('%s') to return error, got nil", mac)
		}
	}
}

package crazyflie

import (
	"encoding/binary"
	"github.com/mikehamer/crazyserver/crtp"
	"log"
	"strings"
)

// ---- BOOTLOADER REQUEST: GET INFO ----
type BootloaderRequestInit struct{}

func (p *BootloaderRequestInit) Port() crtp.Port {
	return crtp.PortLink
}

func (p *BootloaderRequestInit) Channel() crtp.Channel {
	return 0x03
	// crtp.HeaderBytes(crtp.PortLink, 0x03) make the byte 0xFF (if link=3 is used). This is important for bootloader communications!
}

func (p *BootloaderRequestInit) Bytes() []byte {
	return []byte{0xFE, 0xFF, 1, 2, 4, 5, 6, 7, 8, 9, 10, 11, 12} // and 0xFF is prepended by the sending code, based on the port and channel above
}

// ---- BOOTLOADER REQUEST: REBOOT TO FIRMWARE ----
type BootloaderRequestRebootToFirmware struct{}

func (p *BootloaderRequestRebootToFirmware) Port() crtp.Port {
	return crtp.PortLink
}

func (p *BootloaderRequestRebootToFirmware) Channel() crtp.Channel {
	return 0x03
	// crtp.HeaderBytes(crtp.PortLink, 0x03) make the byte 0xFF (if link=3 is used). This is important for bootloader communications!
}

func (p *BootloaderRequestRebootToFirmware) Bytes() []byte {
	return []byte{0xFE, 0xF0, 0x01} // and 0xFF is prepended by the sending code, based on the port and channel above
}

// ---- BOOTLOADER REQUEST: REBOOT TO BOOTLOADER ----
type BootloaderRequestRebootToBootloader struct{}

func (p *BootloaderRequestRebootToBootloader) Port() crtp.Port {
	return crtp.PortLink
}

func (p *BootloaderRequestRebootToBootloader) Channel() crtp.Channel {
	return 0x03
	// crtp.HeaderBytes(crtp.PortLink, 0x03) make the byte 0xFF (if link=3 is used). This is important for bootloader communications!
}

func (p *BootloaderRequestRebootToBootloader) Bytes() []byte {
	return []byte{0xFE, 0xF0, 0x00} // and 0xFF is prepended by the sending code, based on the port and channel above
}

// ---- BOOTLOADER RESPONSE: ADDRESS ----
type BootloaderResponseAddress struct {
	NewAddress uint64
}

func (p *BootloaderResponseAddress) Port() crtp.Port {
	return crtp.PortGreedy
}

func (p *BootloaderResponseAddress) Channel() crtp.Channel {
	return 0x00 // doesn't matter when using greedy port
}

func (p *BootloaderResponseAddress) LoadFromBytes(b []byte) error {
	if b[0] != 0xFF || len(b) < 7 { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	log.Println("Got bootloader response bytes: ", b)

	p.NewAddress = uint64(b[3]) | (uint64(b[4]) << 8) | (uint64(b[5]) << 16) | (uint64(b[6]) << 24) | (uint64(0xb1) << 32)

	return nil
}

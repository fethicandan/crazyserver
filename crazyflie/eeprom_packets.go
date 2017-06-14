package crazyflie

import (
	"github.com/mikehamer/crazyserver/crtp"
)

// ---- EEPROM REQUEST: READ ----
type MemRequestRead struct {
	Target uint8
	Offset uint32
	Length uint8
}

func (p *MemRequestRead) Port() crtp.Port {
	return crtp.PortMem
}

func (p *MemRequestRead) Channel() crtp.Channel {
	return 0x01
}

func (p *MemRequestRead) Bytes() []byte {
	packet := make([]byte, 6)
	packet[0] = p.Target
	copy(packet[1:5], uint32ToBytes(p.Offset))
	packet[5] = p.Length
	return packet
}

// ---- EEPROM RESPONSE: READ ----
type MemResponseRead struct {
	Target uint8
	Offset uint32
	Length uint8
	Data   []byte
}

func (p *MemResponseRead) Port() crtp.Port {
	return crtp.PortMem
}

func (p *MemResponseRead) Channel() crtp.Channel {
	return 0x01
}

func (p *MemResponseRead) LoadFromBytes(b []byte) error {
	if b[1] != p.Target || bytesToUint32(b[2:6]) != p.Offset { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	if b[6] != 0 || len(b[7:]) != int(p.Length) {
		return ErrorMemReadFailed
	}

	p.Data = b[7:]

	return nil
}

// ---- EEPROM REQUEST: WRITE ----
type MemRequestWrite struct {
	Target uint8
	Offset uint32
	Data   []byte
}

func (p *MemRequestWrite) Port() crtp.Port {
	return crtp.PortMem
}

func (p *MemRequestWrite) Channel() crtp.Channel {
	return 0x02
}

func (p *MemRequestWrite) Bytes() []byte {
	packet := make([]byte, 5+len(p.Data))
	packet[0] = p.Target
	copy(packet[1:5], uint32ToBytes(p.Offset))
	copy(packet[5:], p.Data)
	return packet
}

// ---- EEPROM RESPONSE: WRITE ----
type MemResponseWrite struct {
	Target uint8
	Offset uint32
}

func (p *MemResponseWrite) Port() crtp.Port {
	return crtp.PortMem
}

func (p *MemResponseWrite) Channel() crtp.Channel {
	return 0x02
}

func (p *MemResponseWrite) LoadFromBytes(b []byte) error {
	if b[1] != p.Target || bytesToUint32(b[2:6]) != p.Offset { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	if b[6] != 0 {
		return ErrorMemWriteFailed
	}

	return nil
}

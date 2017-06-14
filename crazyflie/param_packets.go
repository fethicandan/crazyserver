package crazyflie

import (
	"encoding/binary"
	"strings"

	"github.com/mikehamer/crazyserver/crtp"
)

// ---- PARAM REQUEST: GET INFO ----
type ParamRequestGetInfo struct{}

func (p *ParamRequestGetInfo) Port() crtp.Port {
	return crtp.PortParam
}

func (p *ParamRequestGetInfo) Channel() crtp.Channel {
	return 0
}

func (p *ParamRequestGetInfo) Bytes() []byte {
	return []byte{0x01}
}

// ---- PARAM RESPONSE: GET INFO ----
type ParamResponseGetInfo struct {
	Count int    //= int(resp[2])
	CRC   uint32 //= binary.LittleEndian.Uint32(resp[3 : 3+4])
}

func (p *ParamResponseGetInfo) Port() crtp.Port {
	return crtp.PortParam
}

func (p *ParamResponseGetInfo) Channel() crtp.Channel {
	return 0
}

func (p *ParamResponseGetInfo) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if b[1] != 0x01 { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	p.Count = int(b[2])
	p.CRC = binary.LittleEndian.Uint32(b[3 : 3+4])

	return nil
}

// ---- PARAM REQUEST: GET ITEM ----
type ParamRequestReadMeta struct{ ID uint8 }

func (p *ParamRequestReadMeta) Port() crtp.Port {
	return crtp.PortParam
}

func (p *ParamRequestReadMeta) Channel() crtp.Channel {
	return 0
}

func (p *ParamRequestReadMeta) Bytes() []byte {
	return []byte{0x00, p.ID}
}

// ---- PARAM RESPONSE: GET ITEM ----
type ParamResponseReadMeta struct {
	ID       uint8
	Datatype byte
	ReadOnly bool
	Name     string
}

func (p *ParamResponseReadMeta) Port() crtp.Port {
	return crtp.PortParam
}

func (p *ParamResponseReadMeta) Channel() crtp.Channel {
	return 0
}

func (p *ParamResponseReadMeta) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if b[1] != 0x00 || b[2] != p.ID { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	p.Datatype = b[3] & 0x0F
	p.ReadOnly = b[3]&(1<<6) != 0
	// group := resp[3]&(1<<7) != 0

	str := strings.Split(string(b[4:]), "\x00")
	groupName := str[0]
	varName := str[1]
	p.Name = groupName + "." + varName

	return nil
}

// ---- PARAM REQUEST: READ VALUE ----
type ParamRequestReadValue struct {
	ID uint8
}

func (p *ParamRequestReadValue) Port() crtp.Port {
	return crtp.PortParam
}

func (p *ParamRequestReadValue) Channel() crtp.Channel {
	return 1
}

func (p *ParamRequestReadValue) Bytes() []byte {
	return []byte{p.ID}
}

// ---- PARAM RESPONSE: READ VALUE ----
type ParamResponseReadValue struct {
	ID   uint8
	Data []uint8
}

func (p *ParamResponseReadValue) Port() crtp.Port {
	return crtp.PortParam
}

func (p *ParamResponseReadValue) Channel() crtp.Channel {
	return 1
}

func (p *ParamResponseReadValue) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if b[1] != p.ID { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	p.Data = b[2:]
	return nil
}

// ---- PARAM REQUEST: WRITE VALUE ----
type ParamRequestWriteValue struct {
	ID   uint8
	Data []byte
}

func (p *ParamRequestWriteValue) Port() crtp.Port {
	return crtp.PortParam
}

func (p *ParamRequestWriteValue) Channel() crtp.Channel {
	return 2
}

func (p *ParamRequestWriteValue) Bytes() []byte {
	return append([]byte{p.ID}, p.Data...)
}

// ---- PARAM RESPONSE: WRITE VALUE ----
type ParamResponseWriteValue struct {
	ID uint8
}

func (p *ParamResponseWriteValue) Port() crtp.Port {
	return crtp.PortParam
}

func (p *ParamResponseWriteValue) Channel() crtp.Channel {
	return 2
}

func (p *ParamResponseWriteValue) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if b[1] != p.ID { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	// value confirmation = b[2:]
	return nil
}

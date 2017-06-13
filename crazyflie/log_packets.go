package crazyflie

import (
	"encoding/binary"
	"github.com/mikehamer/crazyserver/crtp"
	"strings"
)

// ---- LOG REQUEST: GET INFO ----
type LogRequestGetInfo struct{}

func (p *LogRequestGetInfo) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogRequestGetInfo) Channel() crtp.Channel {
	return 0
}

func (p *LogRequestGetInfo) Bytes() []byte {
	return []byte{0x01}
}

// ---- LOG RESPONSE: GET INFO ----
type LogResponseGetInfo struct {
	Count     int    //= int(resp[2])
	CRC       uint32 //= binary.LittleEndian.Uint32(resp[3 : 3+4])
	MaxPacket uint8  //= uint8(resp[7])
	MaxOps    uint8  //= uint8(resp[8])
}

func (p *LogResponseGetInfo) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogResponseGetInfo) Channel() crtp.Channel {
	return 0
}

func (p *LogResponseGetInfo) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if b[1] != 0x01 { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	p.Count = int(b[2])
	p.CRC = binary.LittleEndian.Uint32(b[3 : 3+4])
	p.MaxPacket = uint8(b[7])
	p.MaxOps = uint8(b[8])

	return nil
}

// ---- LOG REQUEST: GET ITEM ----
type LogRequestGetItem struct{ ID uint8 }

func (p *LogRequestGetItem) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogRequestGetItem) Channel() crtp.Channel {
	return 0
}

func (p *LogRequestGetItem) Bytes() []byte {
	return []byte{0x00, p.ID}
}

// ---- LOG RESPONSE: GET ITEM ----
type LogResponseGetItem struct {
	ID       uint8  //:= uint8(resp[2])
	Datatype byte   //:= resp[3]
	Name     string //:= groupName + "." + varName
}

func (p *LogResponseGetItem) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogResponseGetItem) Channel() crtp.Channel {
	return 0
}

func (p *LogResponseGetItem) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if b[1] != 0x00 || b[2] != p.ID { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	p.Datatype = b[3]

	str := strings.Split(string(b[4:]), "\x00")
	groupName := str[0]
	varName := str[1]
	p.Name = groupName + "." + varName

	return nil
}

// ---- LOG REQUEST: BLOCK CLEAR ALL ----
type LogRequestBlockClearAll struct{}

func (p *LogRequestBlockClearAll) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogRequestBlockClearAll) Channel() crtp.Channel {
	return 1
}

func (p *LogRequestBlockClearAll) Bytes() []byte {
	return []byte{0x05}
}

// ---- LOG RESPONSE: BLOCK CLEAR ALL ----
type LogResponseBlockClearAll struct{}

func (p *LogResponseBlockClearAll) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogResponseBlockClearAll) Channel() crtp.Channel {
	return 1
}

func (p *LogResponseBlockClearAll) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if b[1] != 0x05 { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}
	// this is just an acknowledgement packet, so no need to do anything other than detect its reception
	return nil
}

// ---- LOG REQUEST: BLOCK ADD ----
type LogRequestBlockAdd struct {
	BlockID           uint8
	VariableIDs       []uint8
	VariableDatatypes []uint8
}

func (p *LogRequestBlockAdd) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogRequestBlockAdd) Channel() crtp.Channel {
	return 1
}

func (p *LogRequestBlockAdd) Bytes() []byte {
	packet := make([]byte, 2*len(p.VariableIDs)+2)
	packet[0] = 0x00             // control create block
	packet[1] = uint8(p.BlockID) // logblock id
	for i := 0; i < len(p.VariableIDs); i++ {
		packet[2+2*i] = p.VariableDatatypes[i]
		packet[2+2*i+1] = p.VariableIDs[i]
	}
	return packet
}

// ---- LOG RESPONSE: BLOCK ADD ----
type LogResponseBlockAdd struct {
	BlockID uint8
}

func (p *LogResponseBlockAdd) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogResponseBlockAdd) Channel() crtp.Channel {
	return 1
}

func (p *LogResponseBlockAdd) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if b[1] != 0x00 || b[2] != p.BlockID { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	errorCode := b[3]

	switch errorCode {
	case 0:
		return nil
	case 2:
		return ErrorLogBlockOrItemNotFound
	case 7:
		return ErrorLogBlockTooLong
	case 12:
		return ErrorLogBlockNoMemory
	default:
		return ErrorUnknown
	}
}

// ---- LOG REQUEST: BLOCK DELETE ----
type LogRequestBlockDelete struct {
	BlockID uint8
}

func (p *LogRequestBlockDelete) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogRequestBlockDelete) Channel() crtp.Channel {
	return 1
}

func (p *LogRequestBlockDelete) Bytes() []byte {
	return []byte{0x02, p.BlockID}
}

// ---- LOG RESPONSE: BLOCK DELETE ----
type LogResponseBlockDelete struct {
	BlockID uint8
}

func (p *LogResponseBlockDelete) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogResponseBlockDelete) Channel() crtp.Channel {
	return 1
}

func (p *LogResponseBlockDelete) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if b[1] != 0x02 || b[2] != p.BlockID { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	errorCode := b[3]

	switch errorCode {
	case 0:
		return nil
	case 2:
		return ErrorLogBlockOrItemNotFound
	case 7:
		return ErrorLogBlockTooLong
	case 12:
		return ErrorLogBlockNoMemory
	default:
		return ErrorUnknown
	}
}

// ---- LOG REQUEST: BLOCK START ----
type LogRequestBlockStart struct {
	BlockID uint8
	Period  uint8
}

func (p *LogRequestBlockStart) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogRequestBlockStart) Channel() crtp.Channel {
	return 1
}

func (p *LogRequestBlockStart) Bytes() []byte {
	return []byte{0x03, p.BlockID, p.Period}
}

// ---- LOG RESPONSE: BLOCK START ----
type LogResponseBlockStart struct {
	BlockID uint8
}

func (p *LogResponseBlockStart) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogResponseBlockStart) Channel() crtp.Channel {
	return 1
}

func (p *LogResponseBlockStart) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if b[1] != 0x03 || b[2] != p.BlockID { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	errorCode := b[3]

	switch errorCode {
	case 0:
		return nil
	case 2:
		return ErrorLogBlockOrItemNotFound
	case 7:
		return ErrorLogBlockTooLong
	case 12:
		return ErrorLogBlockNoMemory
	default:
		return ErrorUnknown
	}
}

// ---- LOG REQUEST: BLOCK Stop ----
type LogRequestBlockStop struct {
	BlockID uint8
}

func (p *LogRequestBlockStop) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogRequestBlockStop) Channel() crtp.Channel {
	return 1
}

func (p *LogRequestBlockStop) Bytes() []byte {
	return []byte{0x04, p.BlockID}
}

// ---- LOG RESPONSE: BLOCK Stop ----
type LogResponseBlockStop struct {
	BlockID uint8
}

func (p *LogResponseBlockStop) Port() crtp.Port {
	return crtp.PortLog
}

func (p *LogResponseBlockStop) Channel() crtp.Channel {
	return 1
}

func (p *LogResponseBlockStop) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if b[1] != 0x04 || b[2] != p.BlockID { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	errorCode := b[3]

	switch errorCode {
	case 0:
		return nil
	case 2:
		return ErrorLogBlockOrItemNotFound
	case 7:
		return ErrorLogBlockTooLong
	case 12:
		return ErrorLogBlockNoMemory
	default:
		return ErrorUnknown
	}
}

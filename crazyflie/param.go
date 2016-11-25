package crazyflie

import (
	"encoding/binary"
	"log"
	"strings"
	"time"
)

// PARAM_UINT8  (0x00 | (0x00<<2) | (0x01<<3)) = 0x8
// PARAM_UINT16 (0x01 | (0x00<<2) | (0x01<<3)) = 0x9
// PARAM_UINT32 (0x02 | (0x00<<2) | (0x01<<3)) = 0xA
// PARAM_INT8   (0x00 | (0x00<<2) | (0x00<<3)) = 0x0
// PARAM_INT16  (0x01 | (0x00<<2) | (0x00<<3)) = 0x1
// PARAM_INT32  (0x02 | (0x00<<2) | (0x00<<3)) = 0x2
// PARAM_FLOAT  (0x02 | (0x01<<2) | (0x00<<3)) = 0x6

var paramTypeToValue = map[uint8](func([]byte) interface{}){
	0x8: bytesToUint8,
	0x9: bytesToUint16,
	0xA: bytesToUint32,
	0x0: bytesToInt8,
	0x1: bytesToInt16,
	0x2: bytesToInt32,
	0x6: bytesToFloat32,
}

var paramTypeToSize = map[uint8]uint8{
	0x8: 1,
	0x9: 2,
	0xA: 4,
	0x0: 1,
	0x1: 2,
	0x2: 4,
	0x6: 4,
}

type paramItem struct {
	id       uint8
	datatype uint8
	readonly bool
}

func (cf *Crazyflie) paramSystemInit() {
	cf.paramNameToIndex = make(map[string]paramItem)
	cf.paramIndexToName = make(map[uint8]string)
}

func (cf *Crazyflie) ParamTOCGetInfo() (int, uint32, error) {

	// the packet to initialize the transaction
	packet := []byte{crtp(crtpPortParam, 0), 0x01}

	// the function which matches and acts on the response packet
	callbackTriggered := make(chan bool)
	callback := func(resp []byte) {
		header := crtpHeader(resp[0])

		// should check the header port and channel like this (rather than check the hex value of resp[0]) since the link bits might vary(?)
		if header.port() == crtpPortParam && header.channel() == 0 && resp[1] == 0x01 {

			cf.paramCount = int(resp[2])
			cf.paramCRC = binary.LittleEndian.Uint32(resp[3 : 3+4])

			callbackTriggered <- true
		}
	}

	// add the callback to the list
	e := cf.responseCallbacks[crtpPortParam].PushBack(callback)
	defer cf.responseCallbacks[crtpPortParam].Remove(e) // and remove it once we're done

	cf.commandQueue <- packet // schedule transmission of the packet

	select {
	case <-callbackTriggered:
		return cf.paramCount, cf.paramCRC, nil
	case <-time.After(time.Duration(500) * time.Millisecond):
		return 0, 0, ErrorNoResponse
	}
}

func (cf *Crazyflie) ParamTOCGetList() error {
	count, crc, err := cf.ParamTOCGetInfo()
	if err != nil {
		return err
	}
	// TODO: load crc from cache
	_ = count
	_ = crc

	// the packet to initialize the transaction
	packet := []byte{crtp(crtpPortParam, 0), 0x00, 0x00}

	// the function which matches and acts on the response packet
	callbackTriggered := make(chan bool)
	callback := func(resp []byte) {
		header := crtpHeader(resp[0])

		// should check the header port and channel like this (rather than check the hex value of resp[0]) since the link bits might vary(?)
		if header.port() == crtpPortParam && header.channel() == 0 && resp[1] == 0x00 {
			id := uint8(resp[2])
			datatype := resp[3] & 0x0F
			readonly := resp[3]&(1<<6) != 0
			// group := resp[3]&(1<<7) != 0

			str := strings.Split(string(resp[4:]), "\x00")
			groupName := str[0]
			varName := str[1]
			name := groupName + "." + varName

			// log.Printf("%s -> id: %d, group: %t, dtype: %X, readonly: %t", name, id, group, datatype, readonly)

			cf.paramNameToIndex[name] = paramItem{id, datatype, readonly}
			cf.paramIndexToName[id] = name

			callbackTriggered <- true
		}
	}

	// add the callback to the list
	e := cf.responseCallbacks[crtpPortParam].PushBack(callback)
	defer cf.responseCallbacks[crtpPortParam].Remove(e) // and remove it once we're done

	for i := 0; i < cf.paramCount; {
		packet[2] = uint8(i)      // the parameter we want to read
		cf.commandQueue <- packet // schedule transmission of the packet

		select {
		case <-callbackTriggered:
			i++
		case <-time.After(time.Duration(500) * time.Millisecond):
			// no increment
		}
	}

	log.Printf("Loaded Param TOC Size %d with CRC %X", cf.paramCount, cf.paramCRC)
	return nil
}

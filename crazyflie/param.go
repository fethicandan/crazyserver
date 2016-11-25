package crazyflie

import (
	"encoding/binary"
	"log"
	"strings"
	"time"
)

var paramTypeToValue = map[uint8](func([]byte) interface{}){
	1: bytesToUint8,
	2: bytesToUint16,
	3: bytesToUint32,
	4: bytesToInt8,
	5: bytesToInt16,
	6: bytesToInt32,
	7: bytesToFloat32,
	8: bytesToFloat16,
}

var paramTypeToSize = map[uint8]uint8{
	1: 1,
	2: 2,
	3: 4,
	4: 1,
	5: 2,
	6: 4,
	7: 4,
	8: 2,
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

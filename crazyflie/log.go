package crazyflie

import (
	"encoding/binary"
	"log"
	"strings"
	"time"
)

var logTypeToValue = map[uint8](func([]byte) interface{}){
	1: bytesToUint8,
	2: bytesToUint16,
	3: bytesToUint32,
	4: bytesToInt8,
	5: bytesToInt16,
	6: bytesToInt32,
	7: bytesToFloat32,
	8: bytesToFloat16,
}

type logItem struct {
	id       uint8
	datatype uint8
}

func (cf *Crazyflie) GetLogTOCInfo() error {

	// the packet to initialize the transaction
	packet := []byte{crtp(crtpPortLog, 0), 0x01}

	// the function which matches and acts on the response packet
	callbackTriggered := make(chan bool)
	callback := func(resp []byte) {
		header := crtpHeader(resp[0])

		// should check the header port and channel like this (rather than check the hex value of resp[0]) since the link bits might vary(?)
		if header.port() == crtpPortLog && header.channel() == 0 && resp[1] == 0x01 {

			cf.logCount = int(resp[2])
			cf.logCRC = binary.LittleEndian.Uint32(resp[3 : 3+4])
			cf.logMaxPacket = uint8(resp[7])
			cf.logMaxOps = uint8(resp[8])

			callbackTriggered <- true
		}
	}

	// add the callback to the list
	e := cf.responseCallbacks[crtpPortLog].PushBack(callback)
	defer cf.responseCallbacks[crtpPortLog].Remove(e) // and remove it once we're done

	cf.commandQueue <- packet // schedule transmission of the packet

	select {
	case <-callbackTriggered:
		log.Printf("TOC Size %d with CRC %d, (%d, %d)", cf.logCount, cf.logCRC, cf.logMaxPacket, cf.logMaxOps)
		return nil
	case <-time.After(time.Duration(500) * time.Millisecond):
		return ErrorNoResponse
	}
}

func (cf *Crazyflie) GetLogTOCList() error {
	cf.GetLogTOCInfo()

	// the packet to initialize the transaction
	packet := []byte{crtp(crtpPortLog, 0), 0x00, 0x00}

	// the function which matches and acts on the response packet
	callbackTriggered := make(chan bool)
	callback := func(resp []byte) {
		header := crtpHeader(resp[0])

		// should check the header port and channel like this (rather than check the hex value of resp[0]) since the link bits might vary(?)
		if header.port() == crtpPortLog && header.channel() == 0 && resp[1] == 0x00 {
			id := uint8(resp[2])
			datatype := resp[3]

			str := strings.Split(string(resp[4:]), "\x00")
			groupName := str[0]
			varName := str[1]
			name := groupName + "." + varName

			cf.logNameToIndex[name] = logItem{id, datatype}
			cf.logIndexToName[id] = name

			log.Printf("%d -> %s (%d)", id, name, datatype)

			callbackTriggered <- true
		}
	}

	// add the callback to the list
	e := cf.responseCallbacks[crtpPortLog].PushBack(callback)
	defer cf.responseCallbacks[crtpPortLog].Remove(e) // and remove it once we're done

	for i := 0; i < cf.logCount; {
		packet[2] = uint8(i)      // the log variable we want to read
		cf.commandQueue <- packet // schedule transmission of the packet

		select {
		case <-callbackTriggered:
			i++
		case <-time.After(time.Duration(500) * time.Millisecond):
			// no increment
		}
	}
	return nil
}

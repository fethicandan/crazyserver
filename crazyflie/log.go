package crazyflie

import (
	"encoding/binary"
	"log"
	"math"
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

var logTypeToSize = map[uint8]uint8{
	1: 1,
	2: 2,
	3: 4,
	4: 1,
	5: 2,
	6: 4,
	7: 4,
	8: 2,
}

type logItem struct {
	id       uint8
	datatype uint8
}

type logBlock struct {
	id        int
	period    time.Duration
	variables []logItem
}

func (cf *Crazyflie) initLogSystem() {
	cf.responseCallbacks[crtpPortLog].PushBack(cf.handleLogBlock)
}

func (cf *Crazyflie) handleLogBlock(resp []byte) {
	header := crtpHeader(resp[0])

	if header.port() == crtpPortLog && header.channel() == 2 {
		blockid := int(resp[1])
		//timestamp := uint32(resp[2]) | (uint32(resp[3]) << 8) | (uint32(resp[4]) << 16)

		block, ok := cf.logBlocks[blockid]
		if !ok {
			// we are getting told about an unknown block
			// TODO: send a block cancellation?
			log.Printf("warning: unknown block id=%d", blockid)
			return
		}

		idx := 5 // first index of element
		log.Printf("Log Block %d", blockid)
		for i := 0; i < len(block.variables) && idx < len(resp); i++ {
			variable := block.variables[i]
			datasize := int(logTypeToSize[variable.datatype])
			data := logTypeToValue[variable.datatype](resp[idx : idx+datasize])
			log.Printf("%s = %v", cf.logIndexToName[variable.id], data)
			idx += datasize
		}

		if idx != len(resp) {
			log.Printf("warning: block %d has strange size %d (expect %d)", blockid, idx, len(resp))
		}

	}
}

func (cf *Crazyflie) GetLogTOCInfo() error {

	// the packet to initialize the transaction
	packet := []byte{crtp(crtpPortLog, 0), 0x01}

	// the function which matches and acts on the response packet
	callbackTriggered := make(chan int)
	callback := func(resp []byte) {
		header := crtpHeader(resp[0])

		// should check the header port and channel like this (rather than check the hex value of resp[0]) since the link bits might vary(?)
		if header.port() == crtpPortLog && header.channel() == 0 && resp[1] == 0x01 {

			cf.logCount = int(resp[2])
			cf.logCRC = binary.LittleEndian.Uint32(resp[3 : 3+4])
			cf.logMaxPacket = uint8(resp[7])
			cf.logMaxOps = uint8(resp[8])

			callbackTriggered <- cf.logCount
		}
	}

	// add the callback to the list
	e := cf.responseCallbacks[crtpPortLog].PushBack(callback)
	defer cf.responseCallbacks[crtpPortLog].Remove(e) // and remove it once we're done

	cf.commandQueue <- packet // schedule transmission of the packet

	select {
	case count := <-callbackTriggered:
		log.Printf("TOC Size %d with CRC %d, (%d, %d)", count, cf.logCRC, cf.logMaxPacket, cf.logMaxOps)
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

func (cf *Crazyflie) RequestLogBlock(period time.Duration, variables []string) (int, error) {
	blockid := 0

	if len(variables) > 30 {
		return 0, ErrorLogBlockTooLong
	}

	// find a free logblock id
	for ; blockid < 256; blockid++ {
		if _, ok := cf.logBlocks[blockid]; !ok {
			break // if the block id hasn't yet been allocated
		}
	}

	if blockid >= 256 {
		return 0, ErrorLogBlockNoMemory
	}

	// create and populate the block object
	block := logBlock{
		blockid,
		time.Duration(math.Floor(period.Seconds()*100.0+0.5)*10.0) * time.Millisecond, // nearest multiple of 10ms
		make([]logItem, len(variables)),
	}

	for i := 0; i < len(variables); i++ {
		val, ok := cf.logNameToIndex[variables[i]]
		if !ok {
			return 0, ErrorLogItemNotFound
		}
		block.variables[i] = val
	}

	// request block creation
	packet := make([]byte, len(variables)+3)
	packet[0] = crtp(crtpPortLog, 1)
	packet[1] = 0x00           // control create block
	packet[2] = uint8(blockid) // logblock id
	for i := 0; i < len(variables); i++ {
		packet[3+i] = block.variables[i].id
	}

	// callback on logblock creation
	callbackTriggered := make(chan error)
	callback := func(resp []byte) {
		header := crtpHeader(resp[0])

		// should check the header port and channel like this (rather than check the hex value of resp[0]) since the link bits might vary(?)
		if header.port() == crtpPortLog && header.channel() == 1 && resp[1] == 0x00 && resp[2] == uint8(blockid) {
			errNum := resp[3]
			switch errNum {
			case 0:
				callbackTriggered <- nil
			case 2:
				callbackTriggered <- ErrorLogItemNotFound
			case 7:
				callbackTriggered <- ErrorLogBlockTooLong
			case 12:
				callbackTriggered <- ErrorLogBlockNoMemory
			default:
				callbackTriggered <- ErrorUnknown
			}
		}
	}

	// add the callback to the list
	e := cf.responseCallbacks[crtpPortLog].PushBack(callback)
	defer cf.responseCallbacks[crtpPortLog].Remove(e) // and remove it once we're done

	// request creation of the log block
	cf.commandQueue <- packet

	select {
	case err := <-callbackTriggered:
		if err != nil {
			return 0, err
		}
	case <-time.After(time.Duration(500) * time.Millisecond):
		return 0, ErrorNoResponse
	}

	cf.logBlocks[blockid] = block
	return blockid, nil
}

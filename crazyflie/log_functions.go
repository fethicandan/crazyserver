package crazyflie

import (
	"log"
	"math"
	"time"

	"github.com/mikehamer/crazyserver/cache"
	"github.com/mikehamer/crazyserver/crtp"
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
	ID       uint8
	Datatype uint8
}

type logBlock struct {
	ID        uint8
	Variables []logItem
}

func (cf *Crazyflie) logSystemInit() {
	cf.logNameToIndex = make(map[string]logItem)
	cf.logIndexToName = make(map[uint8]string)
	cf.logBlocks = make(map[uint8]logBlock)

	cf.responseCallbacks[crtp.PortLog].PushBack(cf.handleLogBlock)
}

func (cf *Crazyflie) handleLogBlock(resp []byte) {
	header := crtp.Header(resp[0])

	if header.Port() == crtp.PortLog && header.Channel() == 2 {
		blockid := resp[1]
		//timestamp := uint32(resp[2]) | (uint32(resp[3]) << 8) | (uint32(resp[4]) << 16)

		block, ok := cf.logBlocks[blockid]
		if !ok {
			// we are getting told about an unknown block
			// TODO: send a block cancellation?
			log.Printf("warning: unknown block id=%d", blockid)
			return
		}

		idx := 5 // first index of element
		for i := 0; i < len(block.Variables) && idx < len(resp); i++ {
			variable := block.Variables[i]
			datasize := int(logTypeToSize[variable.Datatype])
			data := logTypeToValue[variable.Datatype](resp[idx : idx+datasize])
			log.Printf("%s = %v", cf.logIndexToName[variable.ID], data)
			idx += datasize
		}
		log.Print("-----\n")

		if idx != len(resp) {
			log.Printf("warning: block %d has strange size %d (expect %d)", blockid, idx, len(resp))
		}

	}
}

func (cf *Crazyflie) logTOCGetInfo() (int, uint32, error) {
	request := &LogRequestGetInfo{}
	response := &LogResponseGetInfo{}

	if err := cf.PacketSendAndAwaitResponse(request, response, DEFAULT_RESPONSE_TIMEOUT); err != nil {
		return 0, 0, err
	}

	// if successful, response now includes the data we want
	cf.logCount = response.Count
	cf.logCRC = response.CRC
	cf.logMaxPacket = response.MaxPacket
	cf.logMaxOps = response.MaxOps

	return cf.logCount, cf.logCRC, nil
}

func (cf *Crazyflie) LogTOCGetList() error {
	_, crc, err := cf.logTOCGetInfo()
	if err != nil {
		return err
	}

	err = cache.LoadLog(crc, &cf.logNameToIndex)
	if err == nil {
		for k, v := range cf.logNameToIndex {
			cf.logIndexToName[v.ID] = k
		}
		log.Printf("Uncached Log TOC Size %d with CRC %X", len(cf.logNameToIndex), crc)
		return nil
	}

	for i := 0; i < cf.logCount; i++ {

		request := &LogRequestGetItem{uint8(i)}
		response := &LogResponseGetItem{ID: uint8(i)}

		var err error
		for attempts := 0; attempts < 5; attempts++ {
			err = cf.PacketSendAndAwaitResponse(request, response, DEFAULT_RESPONSE_TIMEOUT)
			if err == nil {
				break
			}
		}
		if err != nil {
			return err
		}

		cf.logNameToIndex[response.Name] = logItem{response.ID, response.Datatype}
		cf.logIndexToName[response.ID] = response.Name

		//log.Printf("%d/%d -> %s (%d)", response.ID, cf.logCount, response.Name, response.Datatype)
	}

	log.Printf("Loaded Log TOC Size %d with CRC %X", cf.logCount, cf.logCRC)

	err = cache.SaveLog(crc, &cf.logNameToIndex)
	if err != nil {
		log.Printf("Error while caching: %s", err)
	}

	return nil
}

func (cf *Crazyflie) LogBlockClearAll() error {
	request := &LogRequestBlockClearAll{}
	response := &LogResponseBlockClearAll{}

	return cf.PacketSendAndAwaitResponse(request, response, DEFAULT_RESPONSE_TIMEOUT)
}

func (cf *Crazyflie) LogBlockAdd(variables []string) (int, error) {
	if len(variables) > 30 {
		return 0, ErrorLogBlockTooLong
	}

	// find a free logblock id
	blockid := 0
	for ; blockid < 256; blockid++ {
		if _, ok := cf.logBlocks[uint8(blockid)]; !ok {
			break // if the block id hasn't yet been allocated
		}
	}

	if blockid >= 256 {
		return 0, ErrorLogBlockNoMemory
	}

	// create and populate the block object
	block := logBlock{
		uint8(blockid),
		make([]logItem, len(variables)),
	}

	variableTypes := make([]byte, len(variables))
	variableIDs := make([]byte, len(variables))

	for i := 0; i < len(variables); i++ {
		val, ok := cf.logNameToIndex[variables[i]]
		if !ok {
			return 0, ErrorLogBlockOrItemNotFound
		}
		block.Variables[i] = val
		variableTypes[i] = val.Datatype
		variableIDs[i] = val.ID
	}

	request := &LogRequestBlockAdd{
		block.ID,
		variableIDs,
		variableTypes,
	}
	response := &LogResponseBlockAdd{block.ID}

	if err := cf.PacketSendAndAwaitResponse(request, response, DEFAULT_RESPONSE_TIMEOUT); err != nil {
		return -1, err
	}

	// packet was successfully parsed without error
	cf.logBlocks[block.ID] = block
	return blockid, nil
}

func (cf *Crazyflie) LogBlockStart(blockid uint8, period time.Duration) error {
	_, ok := cf.logBlocks[blockid]
	if !ok {
		return ErrorLogBlockOrItemNotFound
	}

	quantizedPeriod := uint8(math.Floor(period.Seconds()*100.0 + 0.5)) // nearest multiple of 10ms

	if quantizedPeriod == 0 {
		return ErrorLogBlockPeriodTooShort
	}

	request := &LogRequestBlockStart{blockid, quantizedPeriod}
	response := &LogResponseBlockStart{blockid}
	return cf.PacketSendAndAwaitResponse(request, response, DEFAULT_RESPONSE_TIMEOUT)
}

func (cf *Crazyflie) LogBlockDelete(blockid uint8) error {
	request := &LogRequestBlockDelete{blockid}
	response := &LogResponseBlockDelete{blockid}

	err := cf.PacketSendAndAwaitResponse(request, response, DEFAULT_RESPONSE_TIMEOUT)
	delete(cf.logBlocks, blockid) // a noop if it doesn't exist
	return err
}

func (cf *Crazyflie) LogBlockStop(blockid uint8) error {
	_, ok := cf.logBlocks[blockid]
	if !ok {
		return ErrorLogBlockOrItemNotFound
	}

	request := &LogRequestBlockStop{blockid}
	response := &LogResponseBlockStop{blockid}
	return cf.PacketSendAndAwaitResponse(request, response, DEFAULT_RESPONSE_TIMEOUT)

}

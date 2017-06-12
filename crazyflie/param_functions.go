package crazyflie

import (
	"encoding/binary"
	"log"
	"strings"
	"time"

	"github.com/mikehamer/crazyserver/cache"
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

var paramTypeToBytes = map[uint8](func(interface{}) []byte){
	0x8: uint8ToBytes,
	0x9: uint16ToBytes,
	0xA: uint32ToBytes,
	0x0: int8ToBytes,
	0x1: int16ToBytes,
	0x2: int32ToBytes,
	0x6: float32ToBytes,
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

var paramTypeToName = map[uint8]string{
	0x8: "uint8",
	0x9: "uint16",
	0xA: "uint32",
	0x0: "int8",
	0x1: "int16",
	0x2: "int32",
	0x6: "float",
}

type paramItem struct {
	ID       uint8
	Datatype uint8
	Readonly bool
}

type ParamTocItem struct {
	Group  string
	Name   string
	Type   string
	Access string // "RW" or "RO"
}

func (cf *Crazyflie) paramSystemInit() {
	cf.paramNameToIndex = make(map[string]paramItem)
	cf.paramIndexToName = make(map[uint8]string)
}

func (cf *Crazyflie) paramTOCGetInfo() (int, uint32, error) {
	request := &ParamRequestGetInfo{}
	response := &ParamResponseGetInfo{}

	if err := cf.PacketSendAndAwaitResponse(request, response, 100*time.Millisecond); err != nil {
		return 0, 0, err
	}

	// if successful, response now includes the data we want
	cf.paramCount = response.Count
	cf.paramCRC = response.CRC

	return cf.logCount, cf.logCRC, nil
}

func (cf *Crazyflie) ParamTOCGetList() error {
	_, crc, err := cf.paramTOCGetInfo()
	if err != nil {
		return err
	}

	err = cache.LoadParam(crc, &cf.paramNameToIndex)
	if err == nil {
		for k, v := range cf.paramNameToIndex {
			cf.paramIndexToName[v.ID] = k
		}
		log.Printf("Uncached Param TOC Size %d with CRC %X", len(cf.paramNameToIndex), crc)
		return nil
	}

	for i := 0; i < cf.paramCount; i++ {

		request := &ParamRequestReadMeta{uint8(i)}
		response := &ParamResponseReadMeta{ID: uint8(i)}

		for attempts := 0; attempts < 5; attempts++ {
			if err := cf.PacketSendAndAwaitResponse(request, response, 100*time.Millisecond); err != nil {
				return err
			}
		}

		cf.paramNameToIndex[response.Name] = paramItem{response.ID, response.Datatype, response.ReadOnly}
		cf.paramIndexToName[response.ID] = response.Name
	}

	log.Printf("Loaded Param TOC Size %d with CRC %X", cf.paramCount, cf.paramCRC)

	err = cache.SaveParam(crc, &cf.paramNameToIndex)
	if err != nil {
		log.Printf("Error while caching: %s", err)
	}

	return nil
}

func (cf *Crazyflie) ParamGetList() []string {
	list := make([]string, cf.paramCount)

	for name, idx := range cf.paramNameToIndex {
		list[idx.ID] = name
	}

	return list
}

func (cf *Crazyflie) ParamGetToc() []ParamTocItem {
	list := make([]ParamTocItem, cf.paramCount)

	for name, idx := range cf.paramNameToIndex {
		splitName := strings.Split(name, ".")
		list[idx.ID].Group = splitName[0]
		list[idx.ID].Name = splitName[1]
		list[idx.ID].Type = paramTypeToName[idx.Datatype]
		if idx.Readonly {
			list[idx.ID].Access = "RO"
		} else {
			list[idx.ID].Access = "RW"
		}
	}

	return list
}

func (cf *Crazyflie) ParamRead(name string) (interface{}, error) {
	param, ok := cf.paramNameToIndex[name]
	if !ok {
		return nil, ErrorParamNotFound
	}

	request := &ParamRequestReadValue{ID: param.ID}
	response := &ParamResponseReadValue{ID: param.ID}

	if err := cf.PacketSendAndAwaitResponse(request, response, 100*time.Millisecond); err != nil {
		return nil, err
	}

	return paramTypeToValue[param.Datatype](response.Data), nil
}

func (cf *Crazyflie) ParamWriteFromFloat64(name string, valf float64) error {
	param, ok := cf.paramNameToIndex[name]
	if !ok {
		return ErrorParamNotFound
	}

	var val interface{}

	switch param.Datatype {
	case 0x8:
		val = uint8(valf)
	case 0x9:
		val = uint16(valf)
	case 0xA:
		val = uint32(valf)
	case 0x0:
		val = int8(valf)
	case 0x1:
		val = int16(valf)
	case 0x2:
		val = int32(valf)
	case 0x6:
		val = float32(valf)
	}

	return cf.ParamWrite(name, val)
}

func (cf *Crazyflie) ParamWrite(name string, val interface{}) error {
	param, ok := cf.paramNameToIndex[name]
	if !ok {
		return ErrorParamNotFound
	}

	request := &ParamRequestWriteValue{param.ID, paramTypeToBytes[param.Datatype](val)}
	response := &ParamResponseWriteValue{param.ID}

	return cf.PacketSendAndAwaitResponse(request, response, 100*time.Millisecond)
}

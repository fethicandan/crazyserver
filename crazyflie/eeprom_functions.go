package crazyflie

import (
	"fmt"
	"time"
)

const (
	memIdEEPROM = 0
	memIdLED    = 1
)

const (
	memLength        = 16
	memOffsetStart   = 5 // the start of the writable EEPROM, to which other offsets are relative
	memOffsetChannel = 0
	memOffsetSpeed   = 1
	memOffsetAddress = 10
)

const (
	memMagicChecksum = 48 + 120 + 66 + 67 + 1 // 0xBC in ASCII is the EEPROM magic number, and 1 is the EEPROM version
)

func (cf *Crazyflie) memSystemInit() {
	cf.memoryContents = make([]byte, memLength)
}

func (cf *Crazyflie) MemCommitChannel(channel uint8) {
	cf.memoryContents[memOffsetChannel] = channel
}

func (cf *Crazyflie) MemCommitSpeed(crtpSpeed uint8) {
	cf.memoryContents[memOffsetSpeed] = crtpSpeed
}

func (cf *Crazyflie) MemCommitAddress(address uint64) {
	lower := uint32(address & 0xFFFFFFFF)
	upper := uint8(0xFF & (address >> 32))
	cf.memoryContents[memOffsetAddress] = upper
	copy(cf.memoryContents[memOffsetAddress+1:memOffsetAddress+5], uint32ToBytes(lower))
}

func (cf *Crazyflie) MemReadChannelAddress() (uint8, uint64, error) {
	err := cf.MemReadContents()
	if err != nil {
		return 0, 0, err
	}

	firmwareAddress := uint64(bytesToUint32(cf.memoryContents[memOffsetAddress+1:memOffsetAddress+5]).(uint32)) | (uint64(cf.memoryContents[memOffsetAddress]) << 32)
	firmwareChannel := uint8(cf.memoryContents[memOffsetChannel])

	return firmwareChannel, firmwareAddress, nil
}

func (cf *Crazyflie) MemPushCommits() error {
	cf.memoryContents[memLength-1] = memChecksum256(cf.memoryContents[0 : memLength-1])

	data := make([]byte, memLength)
	copy(data, cf.memoryContents)

	err := cf.memWrite(memOffsetStart, data)
	if err != nil {
		return err
	}

	err = cf.MemReadContents()
	if err != nil {
		return err
	}

	for i, v := range cf.memoryContents {
		if data[i] != v {
			fmt.Printf("written: %v != read: %v\n", data, cf.memoryContents)
			return ErrorMemWriteFailed
		}
	}

	cf.firmwareAddress = uint64(bytesToUint32(cf.memoryContents[memOffsetAddress+1:memOffsetAddress+5]).(uint32)) | (uint64(cf.memoryContents[memOffsetAddress]) << 32)
	cf.firmwareChannel = uint8(cf.memoryContents[memOffsetChannel])

	return nil
}

func (cf *Crazyflie) MemReadContents() error {
	var err error = nil

	for retries := 0; retries < 5; retries++ {
		data, err := cf.memRead(memOffsetStart, 16)
		if err != nil {
			continue
		}

		if data[memLength-1] != memChecksum256(data[0:memLength-1]) {
			err = ErrorMemReadChecksum
			continue
		}

		copy(cf.memoryContents, data)
		break
	}
	return err
}

func (cf *Crazyflie) memRead(offset uint32, length uint8) ([]byte, error) {
	if length > 24 {
		return nil, ErrorMemLengthTooLarge
	}

	request := &MemRequestRead{Target: memIdEEPROM, Offset: offset, Length: length}
	response := &MemResponseRead{Target: memIdEEPROM, Offset: offset, Length: length}

	if err := cf.PacketSendAndAwaitResponse(request, response, 500*time.Millisecond); err != nil {
		return nil, err
	}

	return response.Data, nil
}

func (cf *Crazyflie) memWrite(offset uint32, data []byte) error {
	if len(data) > 24 {
		return ErrorMemLengthTooLarge
	}

	request := &MemRequestWrite{Target: memIdEEPROM, Offset: offset, Data: data}
	response := &MemResponseWrite{Target: memIdEEPROM, Offset: offset}

	return cf.PacketSendAndAwaitResponse(request, response, 500*time.Millisecond)

}

func memChecksum256(data []byte) uint8 {
	checksum := memMagicChecksum
	for _, v := range data {
		checksum += int(v)
	}
	return uint8(checksum % 256)
}

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

func (cf *Crazyflie) MemPushCommits() error {
	cf.memoryContents[memLength-1] = memChecksum256(cf.memoryContents[0 : memLength-1])

	data := make([]byte, memLength)
	copy(data, cf.memoryContents)

	err := cf.memWrite(memOffsetStart, data)
	if err != nil {
		return err
	}

	err = cf.memReadContents()
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

	cf.RebootToFirmware()
	return nil
}

func (cf *Crazyflie) memReadContents() error {
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
	if err == nil {
		fmt.Println(cf.memoryContents)
	}
	return err
}

func (cf *Crazyflie) memRead(offset uint32, length uint8) ([]byte, error) {
	if length > 24 {
		return nil, ErrorMemLengthTooLarge
	}

	packet := make([]byte, 1+1+4+1)
	packet[0] = crtp(crtpPortMem, 1)
	packet[1] = memIdEEPROM
	copy(packet[2:6], uint32ToBytes(offset))
	packet[6] = length

	data, err := cf.PacketSendAwaitResponse(packet, 500*time.Millisecond)
	if err != nil {
		return nil, err
	}

	// assert(data[0] == packet[1])
	// assert(bytesToUint32(data[1:5]) == offset)
	// assert(len(data[6:]) == length)

	if data[5] != 0 {
		return nil, ErrorMemReadFailed
	}

	if len(data[6:]) != int(length) {
		return nil, ErrorMemReadFailed
	}

	return data[6:], nil
}

func (cf *Crazyflie) memWrite(offset uint32, data []byte) error {
	if len(data) > 24 {
		return ErrorMemLengthTooLarge
	}

	packet := make([]byte, 1+1+4+len(data))
	packet[0] = crtp(crtpPortMem, 2)
	packet[1] = memIdEEPROM
	copy(packet[2:6], uint32ToBytes(offset))
	copy(packet[6:], data)

	data, err := cf.PacketSendAwaitResponse(packet, 500*time.Millisecond)
	if err != nil {
		return err
	}

	// assert(data[0] == packet[1])
	// assert(bytesToUint32(data[1:5]) == offset)
	// assert(len(data[6:]) == length)

	if data[5] != 0 {
		return ErrorMemWriteFailed
	}

	return nil
}

func memChecksum256(data []byte) uint8 {
	checksum := memMagicChecksum
	for _, v := range data {
		checksum += int(v)
	}
	return uint8(checksum % 256)
}

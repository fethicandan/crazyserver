package crazyflie

import (
	"log"
	"time"

	"reflect"
)

type flashObj struct {
	// flash
	target         byte
	pageSize       int
	numBuffPages   int
	numFlashPages  int
	startFlashPage int
}

type TargetCPU uint8

const (
	TargetCPU_NRF51 TargetCPU = iota
	TargetCPU_STM32
)

var cpuName = map[TargetCPU]string{TargetCPU_NRF51: "NRF51", TargetCPU_STM32: "STM32"}

func (cf *Crazyflie) ReflashSTM32(data []byte, verify bool, progressChannel chan int) error {
	return cf.reflash(TargetCPU_STM32, data, verify, progressChannel)
}

func (cf *Crazyflie) ReflashNRF51(data []byte, verify bool, progressChannel chan int) error {
	return cf.reflash(TargetCPU_NRF51, data, verify, progressChannel)
}

func (cf *Crazyflie) reflash(target TargetCPU, data []byte, verify bool, progressChannel chan int) error {
	err := cf.RebootToBootloader()
	if err != nil {
		return err
	}

	flash, err := cf.flashGetInfo(target)
	if err != nil {
		return err
	}

	err = cf.flashLoadData(flash, data, progressChannel)
	if err != nil {
		return err
	}

	if verify {
		for i := 0; i < len(data); i += 16 {
			cf.flashVerifyAddress(flash, i, data)
		}
	}

	err = cf.RebootToFirmware()
	if err != nil {
		return err
	}

	return nil
}

func (cf *Crazyflie) flashGetInfo(target TargetCPU) (*flashObj, error) {
	var flash = new(flashObj)

	cpu := 0xFE | uint8(target)
	flash.target = cpu

	packet := []byte{0xFF, cpu, 0x10} // get info command

	callbackTriggered := make(chan bool)
	callback := func(resp []byte) {
		if resp[0] == 0xFF && resp[1] == cpu && resp[2] == 0x10 {
			flash.pageSize = int(bytesToUint16(resp[3:5]).(uint16))
			flash.numBuffPages = int(bytesToUint16(resp[5:7]).(uint16))
			flash.numFlashPages = int(bytesToUint16(resp[7:9]).(uint16))
			flash.startFlashPage = int(bytesToUint16(resp[9:11]).(uint16))
			callbackTriggered <- true
		}
	}

	e := cf.responseCallbacks[crtpPortGreedy].PushBack(callback)
	defer cf.responseCallbacks[crtpPortGreedy].Remove(e)

	cf.PacketSend(packet)

	select {
	case <-callbackTriggered:
		return flash, nil
	case <-time.After(500 * time.Millisecond):
		return nil, ErrorNoResponse
	}
}

func (cf *Crazyflie) flashLoadData(flash *flashObj, data []byte, progressChannel chan int) error {

	if len(data) > int(flash.numFlashPages-flash.startFlashPage)*int(flash.pageSize) {
		return ErrorFlashDataTooLarge
	}

	writeFlashError := make(chan byte)
	writeFlashCallback := func(resp []byte) {
		if resp[0] == 0xFF && resp[1] == flash.target && (resp[2] == 0x18 || resp[2] == 0x19) { // response to write flash
			errorcode := resp[4]
			writeFlashError <- errorcode
		}
	}

	e := cf.responseCallbacks[crtpPortGreedy].PushBack(writeFlashCallback)
	defer cf.responseCallbacks[crtpPortGreedy].Remove(e)

	writeFlashPacket := make([]byte, 9)
	writeFlashPacket[0] = 0xFF
	writeFlashPacket[1] = flash.target

	// write to flash command
	writeFlashPacket[2] = 0x18

	// flashing in order, always begin from buffer page 0
	writeFlashPacket[3] = 0
	writeFlashPacket[4] = 0

	dataIdx := 0                     // index into the data array
	flashIdx := flash.startFlashPage // which flash page we're currently writing

	for {
		pageIdx := 0 // which buffer page we're currently writing
		for {
			// no more data or pages to write
			if dataIdx >= len(data) || pageIdx >= flash.numBuffPages {
				break
			}

			// write as much data as the page can store, or as much as is left
			dataLen := flash.pageSize
			if dataIdx+dataLen > len(data) {
				dataLen = len(data) - dataIdx
			}

			// write the buffer page, consists of multiple packets
			cf.flashLoadBufferPage(flash, pageIdx, data[dataIdx:dataIdx+dataLen])
			progressChannel <- dataLen

			dataIdx += dataLen
			pageIdx++
		}

		if pageIdx == 0 { // no buffer pages written
			break
		}

		// begin writing the flash at page flashIdx
		writeFlashPacket[5] = byte(flashIdx & 0xFF)
		writeFlashPacket[6] = byte((flashIdx >> 8) & 0xFF)

		// here, pageIdx holds the number of buffer pages that were written
		writeFlashPacket[7] = byte(pageIdx & 0xFF)
		writeFlashPacket[8] = byte((pageIdx >> 8) & 0xFF)

		// next time, start further ahead in flash
		flashIdx += pageIdx

		// send the packet
		cf.PacketSend(writeFlashPacket)

		cf.WaitForEmptyPacketQueues()

		for flashConfirmation := false; !flashConfirmation; {
			timeout := time.After(20 * time.Millisecond)
			select {
			case errorcode := <-writeFlashError:
				if errorcode != 0 {
					// progressBar.FinishPrint(fmt.Sprintf("Write flash error %d", errorcode))
					return nil
				}
				flashConfirmation = true // breaks out of the loop
			case <-timeout:
				// Since uplink is safe we know the flash request has been executed
				// Send a flash info request to find out if the flash process is done
				flashInfoPacket := []byte{0xFF, flash.target, 0x19}
				cf.PacketSend(flashInfoPacket)
			}
		}
	}
	return nil
}

func (cf *Crazyflie) flashLoadBufferPage(flash *flashObj, bufferPageNum int, data []byte) {

	readBuffData := make(chan []byte)
	readBuffCallback := func(resp []byte) {
		if resp[0] == 0xFF && resp[1] == flash.target && resp[2] == 0x15 { // response to read flash
			readBuffData <- resp
		}
	}
	e := cf.responseCallbacks[crtpPortGreedy].PushBack(readBuffCallback)
	defer cf.responseCallbacks[crtpPortGreedy].Remove(e)

	loadBufferPacket := make([]byte, 32)
	loadBufferPacket[0] = 0xFF
	loadBufferPacket[1] = flash.target

	// load buffer page command
	loadBufferPacket[2] = 0x14

	// buffer page to load into
	loadBufferPacket[3] = byte(bufferPageNum & 0xFF)
	loadBufferPacket[4] = byte((bufferPageNum >> 8) & 0xFF)

	dataIdx := 0
	bufferPageIdx := 0

	for {
		if dataIdx >= len(data) {
			break
		}

		// address to begin at
		loadBufferPacket[5] = byte(bufferPageIdx & 0xFF)
		loadBufferPacket[6] = byte((bufferPageIdx >> 8) & 0xFF)

		dataLen := len(loadBufferPacket) - 7
		if dataIdx+dataLen > len(data) {
			dataLen = len(data) - dataIdx
		}

		if dataLen == 0 {
			break
		}

		copy(loadBufferPacket[7:7+dataLen], data[dataIdx:dataIdx+dataLen])

		cf.PacketSend(loadBufferPacket[0 : 7+dataLen])

		dataIdx += dataLen
		bufferPageIdx += dataLen
	}
}

func (cf *Crazyflie) flashVerifyAddress(flash *flashObj, flashAddress int, data []byte) bool {

	pageIdx := flashAddress / flash.pageSize
	pageAddress := flashAddress - pageIdx*flash.pageSize

	readFlashPacket := []byte{0xFF, flash.target, 0x1C, 0, 0, 0, 0}
	readFlashPacket[3] = byte((pageIdx + flash.startFlashPage) & 0xFF)
	readFlashPacket[4] = byte(((pageIdx + flash.startFlashPage) >> 8) & 0xFF)
	readFlashPacket[5] = byte(pageAddress & 0xFF)
	readFlashPacket[6] = byte((pageAddress >> 8) & 0xFF)

	readFlashData := make(chan []byte)
	readFlashCallback := func(resp []byte) {
		if resp[0] == 0xFF && resp[1] == flash.target && resp[2] == 0x1C { // response to read flash
			if !reflect.DeepEqual(resp[3:7], readFlashPacket[3:7]) {
				return // Data for the wrong address (previous duplicated request)
			}
			readFlashData <- resp
		} else {
			log.Println("Read strange data: ", resp)
		}
	}

	e := cf.responseCallbacks[crtpPortGreedy].PushBack(readFlashCallback)
	defer cf.responseCallbacks[crtpPortGreedy].Remove(e)

	var readData []byte
	for readSuccess := false; !readSuccess; {
		cf.PacketSend(readFlashPacket)

		select {
		case readData = <-readFlashData:
			dataLen := len(readData) - 7
			if flashAddress+dataLen > len(data) {
				dataLen = len(data) - flashAddress
			}

			equal := reflect.DeepEqual(readData[7:7+dataLen], data[flashAddress:flashAddress+dataLen])
			if !equal {
				log.Fatalf("Flash @ 0x%X = \n%v expecting \n%v", flashAddress, readData[7:7+dataLen], data[flashAddress:flashAddress+dataLen])
				return false
			}
			return true

		case <-time.After(20 * time.Millisecond):
			break
		}
	}

	return true
}

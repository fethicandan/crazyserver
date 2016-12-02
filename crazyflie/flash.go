package crazyflie

import (
	"fmt"
	"log"
	"time"

	"gopkg.in/cheggaaa/pb.v1"
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

func (cf *Crazyflie) ReflashSTM32(data []byte) error {
	return cf.reflash(TargetCPU_STM32, data)
}

func (cf *Crazyflie) ReflashNRF51(data []byte) error {
	return cf.reflash(TargetCPU_NRF51, data)
}

func (cf *Crazyflie) reflash(target TargetCPU, data []byte) error {
	err := cf.RebootToBootloader()
	if err != nil {
		return err
	}

	flash, err := cf.flashGetInfo(target)
	if err != nil {
		return err
	}

	flash.numBuffPages = 1
	log.Printf("Flashing %d bytes to %s (Start: %X, Size: %d, Buff: %d, Flash: %d)", len(data), cpuName[target], flash.startFlashPage, flash.pageSize, flash.numBuffPages, flash.numFlashPages)

	err = cf.flashLoadData(flash, data)
	if err != nil {
		return err
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

	cf.commandQueue <- packet

	select {
	case <-callbackTriggered:
		return flash, nil
	case <-time.After(500 * time.Millisecond):
		return nil, ErrorNoResponse
	}
}

func (cf *Crazyflie) flashLoadData(flash *flashObj, data []byte) error {

	if len(data) > int(flash.numFlashPages-flash.startFlashPage)*int(flash.pageSize) {
		return ErrorFlashDataTooLarge
	}

	progressBar := pb.New(len(data)).Prefix(fmt.Sprintf("Flashing 0x%X", cf.firmwareAddress))
	progressBar.ShowTimeLeft = true
	progressBar.SetUnits(pb.U_BYTES)
	progressBar.Start()

	writeFlashError := make(chan byte)
	writeFlashCallback := func(resp []byte) {
		if resp[0] == 0xFF && resp[1] == flash.target && (resp[2] == 0x18 || resp[2] == 0x19) { // response to write flash
			errorcode := resp[4]
			writeFlashError <- errorcode
		} else if resp[0] == 0xFF && resp[1] == flash.target {
			log.Println("Strange data: ", resp)
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
			progressBar.Add(dataLen)

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
		cf.commandQueue <- writeFlashPacket

		for flashConfirmation := false; !flashConfirmation; {
			timeout := time.After(time.Duration(1000+2*minCommunicationPeriod_ms*flash.numBuffPages*flash.pageSize/25) * time.Millisecond) // twice the time it should take to write all packets
			select {
			case errorcode := <-writeFlashError:
				if errorcode != 0 {
					progressBar.FinishPrint(fmt.Sprintf("Write flash error %d", errorcode))
					return nil
				}
				flashConfirmation = true // breaks out of the loop
			case <-timeout:
				// flashInfoPacket := []byte{0xFF, flash.target, 0x19} // for some reason doesn't work if we just send this
				cf.commandQueue <- writeFlashPacket
			}
		}
	}
	progressBar.FinishPrint(fmt.Sprintf("Finishing flashing %d bytes (%d pages)", len(data), flashIdx-flash.startFlashPage))
	return nil
}

func (cf *Crazyflie) flashLoadBufferPage(flash *flashObj, bufferPageNum int, data []byte) {
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

		dataLen := 25
		if dataIdx+dataLen > len(data) {
			dataLen = len(data) - dataIdx
		}

		if dataLen == 0 {
			break
		}

		copy(loadBufferPacket[7:7+dataLen], data[dataIdx:dataIdx+dataLen])

		cf.commandQueue <- loadBufferPacket[0 : 7+dataLen]

		dataIdx += dataLen
		bufferPageIdx += dataLen
	}
}

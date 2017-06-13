package crazyflie

import (
	"log"
	"time"

	"fmt"
	"golang.org/x/tools/go/gcimporter15/testdata"
	"reflect"
)

type flashObj struct {
	// flash
	Target         byte
	PageSize       int
	NumBuffPages   int
	NumFlashPages  int
	StartFlashPage int
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
	var flash = &flashObj{}

	cpu := 0xFE | uint8(target)
	flash.Target = cpu

	request := &FlashRequestGetInfo{Target: flash.Target}
	response := &FlashResponseGetInfo{Target: flash.Target}

	if err := cf.PacketSendAndAwaitResponse(request, response, 100*time.Millisecond); err != nil {
		return nil, err
	}

	flash.PageSize = response.PageSize
	flash.NumBuffPages = response.NumBuffPages
	flash.NumFlashPages = response.NumFlashPages
	flash.StartFlashPage = response.StartFlashPage

	return flash, nil
}

func (cf *Crazyflie) flashLoadData(flash *flashObj, data []byte, progressChannel chan int) error {

	if len(data) > int(flash.NumFlashPages-flash.StartFlashPage)*int(flash.PageSize) {
		return ErrorFlashDataTooLarge
	}

	writeFlashPacket := &FlashRequestWriteLoadedPagesToFlash{Target: flash.Target}
	flashingStatus := &FlashResponseGetFlashingStatus{Target: flash.Target}

	dataIdx := 0                     // index into the data array
	flashIdx := flash.StartFlashPage // which flash page we're currently writing

	for {
		pageIdx := 0 // which buffer page we're currently writing
		for {
			// no more data or pages to write
			if dataIdx >= len(data) || pageIdx >= flash.NumBuffPages {
				break
			}

			// write as much data as the page can store, or as much as is left
			dataLen := flash.PageSize
			if dataIdx+dataLen > len(data) {
				dataLen = len(data) - dataIdx
			}

			// write the buffer page, consists of multiple packets
			cf.flashLoadBufferPage(flash, pageIdx, data[dataIdx:dataIdx+dataLen])

			if cf.Status() == StatusNoResponse {
				return ErrorNoResponse
			}

			progressChannel <- dataLen

			dataIdx += dataLen
			pageIdx++
		}

		if pageIdx == 0 { // no buffer pages written
			break
		}

		// where to begin writing the pages
		writeFlashPacket.FlashLocation = flashIdx
		writeFlashPacket.PageCount = pageIdx

		// next time, start further ahead in flash
		flashIdx += pageIdx

		// send the packet
		awaitErrorChannel, stopAwaiting := cf.PacketStartAwaiting(flashingStatus)
		cf.PacketSend(writeFlashPacket)

		cf.PacketQueueWaitForEmpty()

		// We resend a request for information repeatedly until we get confirmation
		for flashConfirmation := false; !flashConfirmation; {
			timeout := time.After(100 * time.Millisecond)
			select {
			case err := <-awaitErrorChannel:
				if err != nil {
					stopAwaiting()
					return err
				}
				if flashingStatus.ErrorCode != 0 {
					log.Printf("Write flash error %d\n", flashingStatus.ErrorCode)
					stopAwaiting()
					return nil // TODO: return a sensible error
				}
				flashConfirmation = true // breaks out of the loop if we get a confirmation
				stopAwaiting()
			case <-timeout:
				// Since uplink is safe we know the flash request has been executed
				// Send a flash info request to find out if the flash process is done
				flashInfoPacket := &FlashRequestGetFlashingStatus{Target: flash.Target}
				cf.PacketSend(flashInfoPacket)

				if cf.Status() == StatusNoResponse {
					stopAwaiting()
					return ErrorNoResponse
				}
			}
		}
	}
	return nil
}

func (cf *Crazyflie) flashLoadBufferPage(flash *flashObj, bufferPageNum int, data []byte) {

	loadBufferPacket := &FlashRequestLoadBufferPage{Target: flash.Target, BufferPageNum: bufferPageNum}

	dataIdx := 0
	bufferPageIdx := 0

	for {
		if dataIdx >= len(data) {
			break
		}

		// how much data can we copy to the packet
		dataLen := loadBufferPacket.MaxDataSize()
		if dataIdx+dataLen > len(data) {
			dataLen = len(data) - dataIdx
		}

		if dataLen == 0 {
			break
		}

		// load the packet for this particular write
		loadBufferPacket.BufferPageIdx = bufferPageIdx
		loadBufferPacket.Data = data[dataIdx : dataIdx+dataLen]

		cf.PacketSend(loadBufferPacket)

		dataIdx += dataLen
		bufferPageIdx += dataLen
	}
}

func (cf *Crazyflie) flashVerifyAddress(flash *flashObj, flashAddress int, data []byte) (bool, error) {

	pageIdx := flashAddress / flash.PageSize
	pageAddress := flashAddress - pageIdx*flash.PageSize

	request := &FlashRequestVerifyAddress{
		flash.Target,
		pageIdx + flash.StartFlashPage,
		pageAddress,
	}

	response := &FlashResponseVerifyAddress{
		Target:      flash.Target,
		PageIndex:   pageIdx + flash.StartFlashPage,
		PageAddress: pageAddress,
	}

	readErrorChannel, stopAwaiting := cf.PacketStartAwaiting(response)
	defer stopAwaiting()

	for readSuccess := false; !readSuccess; {
		cf.PacketSend(request)

		select {
		case err := <-readErrorChannel:
			if err != nil {
				return false, err
			}

			dataLen := len(response.Data)
			if flashAddress+dataLen > len(data) {
				dataLen = len(data) - flashAddress
			}

			equal := reflect.DeepEqual(response.Data[:dataLen], data[flashAddress:flashAddress+dataLen])
			if !equal {
				log.Fatalf("Flash @ 0x%X = \n%v expecting \n%v", flashAddress, response.Data[:dataLen], data[flashAddress:flashAddress+dataLen])
				return false, nil // TODO: should we just use an error rather than a bool?
			}
			return true, nil

		case <-time.After(20 * time.Millisecond):
			break
		}

		if cf.Status() == StatusNoResponse {
			return false, ErrorNoResponse
		}
	}

	return true, nil
}

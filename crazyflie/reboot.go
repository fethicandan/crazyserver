package crazyflie

import (
	"log"
	"time"
)

//https://forum.bitcraze.io/viewtopic.php?f=9&t=1488

func (cf *Crazyflie) RebootToFirmware() (uint64, error) {
	callbackData := make(chan []byte)
	callback := func(resp []byte) {
		if resp[0] == 0xFF {
			callbackData <- resp
		}
	}

	e := cf.responseCallbacks[crtpPortGreedy].PushBack(callback)
	defer cf.responseCallbacks[crtpPortGreedy].Remove(e)

	initPacket := []byte{0xFF, 0xFE, 0xFF, 1, 2, 4, 5, 6, 7, 8, 9, 10, 11, 12} //need these extra bytes due to CF1 legacy
	rebootPacket := []byte{0xFF, 0xFE, 0xF0, 0x01}

	// important that these two packets are serviced directly after each other
	cf.commandQueue <- initPacket

	<-callbackData

	cf.commandQueue <- rebootPacket

	cf.DisconnectOnEmpty()
	<-time.After(1 * time.Second)
	return cf.firmwareAddress, cf.connect(cf.firmwareAddress, cf.firmwareChannel)
}

func (cf *Crazyflie) RebootToBootloader() (uint64, error) {
	callbackData := make(chan []byte)
	callback := func(resp []byte) {
		if resp[0] == 0xFF {
			callbackData <- resp
		}
	}

	e := cf.responseCallbacks[crtpPortGreedy].PushBack(callback)
	defer cf.responseCallbacks[crtpPortGreedy].Remove(e)

	initPacket := []byte{0xFF, 0xFE, 0xFF}
	rebootPacket := []byte{0xFF, 0xFE, 0xF0, 0x00}

	cf.commandQueue <- initPacket

	data := <-callbackData

	cf.commandQueue <- rebootPacket // initialize the reboot

	bootloaderAddress := uint64(data[3]) | (uint64(data[4]) << 8) | (uint64(data[5]) << 16) | (uint64(data[6]) << 24) | (uint64(0xb1) << 32)
	log.Printf("New Address: 0x%X", bootloaderAddress)

	cf.DisconnectOnEmpty()
	<-time.After(1 * time.Second)
	return bootloaderAddress, cf.connect(bootloaderAddress, 0)
}

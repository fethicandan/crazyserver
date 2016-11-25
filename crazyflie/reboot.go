package crazyflie

import (
	"log"
	"time"
)

//https://forum.bitcraze.io/viewtopic.php?f=9&t=1488

func (cf *Crazyflie) RebootToFirmware() {
	callbackData := make(chan []byte)
	callback := func(resp []byte) {
		if resp[0] == 0xFF {
			callbackData <- resp
		}
	}

	e := cf.responseCallbacks[crtpPortGreedy].PushBack(callback)
	defer cf.responseCallbacks[crtpPortGreedy].Remove(e)

	initPacket := []byte{0xFF, 0xFE, 0xFF}
	rebootPacket := []byte{0xFF, 0xFE, 0xF0, 0x01}

	// important that these two packets are serviced directly after each other
	cf.commandQueue <- initPacket
	cf.commandQueue <- rebootPacket

	data := <-callbackData
	log.Print(data)

	cf.Disconnect()
	<-time.After(1 * time.Second)
	cf.connect()
}

func (cf *Crazyflie) RebootToBootloader() uint64 {
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

	// important that these two packets are serviced directly after each other
	cf.commandQueue <- initPacket
	cf.commandQueue <- rebootPacket

	data := <-callbackData

	address := uint64(data[4]) | (uint64(data[5]) << 8) | (uint64(data[6]) << 16) | (uint64(data[7]) << 24) | (uint64(data[8]) << 32)
	log.Printf("New Address: 0x%X", address)

	cf.Disconnect()

	return address
}

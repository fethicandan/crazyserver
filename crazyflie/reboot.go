package crazyflie

import "time"

//https://forum.bitcraze.io/viewtopic.php?f=9&t=1488

func (cf *Crazyflie) RebootToFirmware() error {
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
	cf.PacketSend(initPacket)
	cf.PacketSend(rebootPacket)

	<-callbackData

	cf.DisconnectOnEmpty()

	<-time.After(500 * time.Millisecond)

	return cf.connect(cf.firmwareAddress, cf.firmwareChannel)
}

func (cf *Crazyflie) RebootToBootloader() error {
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

	cf.PacketSend(initPacket)
	cf.PacketSend(rebootPacket) // initialize the reboot

	data := <-callbackData

	bootloaderAddress := uint64(data[3]) | (uint64(data[4]) << 8) | (uint64(data[5]) << 16) | (uint64(data[6]) << 24) | (uint64(0xb1) << 32)

	cf.DisconnectOnEmpty()

	<-time.After(500 * time.Millisecond)

	return cf.connect(bootloaderAddress, 0)
}

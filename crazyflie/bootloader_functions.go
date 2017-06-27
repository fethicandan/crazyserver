package crazyflie

import (
	"fmt"
	"time"
)

//https://forum.bitcraze.io/viewtopic.php?f=9&t=1488

func (cf *Crazyflie) RebootToFirmware() error {
	initPacket := &BootloaderRequestInit{}
	rebootPacket := &BootloaderRequestRebootToFirmware{}
	responsePacket := &BootloaderResponseAddress{}

	// since we are sending multiple packets, we await first and send twice
	// this is exactly what is done in cf.PacketSendAndAwaitResponse, except
	// by making it explicit, we can send twice without stopping the await
	awaitErrorChannel, stopAwaiting := cf.PacketStartAwaiting(responsePacket)
	defer stopAwaiting()

	cf.PacketSend(initPacket)
	cf.PacketSend(rebootPacket) // initialize the reboot

	select {
	case err := <-awaitErrorChannel:
		if err != nil {
			return err
		}
	case <-time.After(500 * time.Millisecond):
		return ErrorNoResponse
	}

	cf.DisconnectOnEmpty()

	<-time.After(500 * time.Millisecond)

	return cf.connect(cf.firmwareChannel, cf.firmwareAddress)
}

func (cf *Crazyflie) RebootToBootloader() error {
	initPacket := &BootloaderRequestInit{}
	rebootPacket := &BootloaderRequestRebootToBootloader{}
	responsePacket := &BootloaderResponseAddress{}

	// since we are sending multiple packets, we await first and send twice
	// this is exactly what is done in cf.PacketSendAndAwaitResponse, except
	// by making it explicit, we can send twice without stopping the await
	awaitErrorChannel, stopAwaiting := cf.PacketStartAwaiting(responsePacket)
	defer stopAwaiting()

	cf.PacketSend(initPacket)
	cf.PacketSend(rebootPacket) // initialize the reboot

	select {
	case err := <-awaitErrorChannel:
		if err != nil {
			return err
		}
	case <-time.After(500 * time.Millisecond):
		return ErrorNoResponse
	}

	cf.DisconnectOnEmpty()

	<-time.After(500 * time.Millisecond)

	fmt.Printf("Connecting to %d:0x%X\n", 0, responsePacket.NewAddress)

	return cf.connect(0, responsePacket.NewAddress)
}

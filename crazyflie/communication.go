package crazyflie

import (
	"container/list"

	"sync"
	"time"

	"github.com/mikehamer/crazyserver/crazyradio"
)

const statusTimeoutDuration time.Duration = 1 * time.Second

func (cf *Crazyflie) communicationSystemInit() {
	cf.disconnect = make(chan bool)
	cf.waitGroup = &sync.WaitGroup{}
	cf.statusTimeout = time.NewTimer(statusTimeoutDuration)

	// setup the communication callbacks
	cf.responseCallbacks = map[crtpPort](*list.List){
		crtpPortConsole:  list.New(),
		crtpPortParam:    list.New(),
		crtpPortSetpoint: list.New(),
		crtpPortMem:      list.New(),
		crtpPortLog:      list.New(),
		crtpPortPosition: list.New(),
		crtpPortPlatform: list.New(),
		crtpPortLink:     list.New(),
		crtpPortEmpty1:   list.New(),
		crtpPortEmpty2:   list.New(),
		crtpPortGreedy:   list.New(),
	}

	cf.waitGroup.Add(1)
	go cf.statusTimeoutThread()
}

func (cf *Crazyflie) statusTimeoutThread() {
	defer cf.waitGroup.Done()

	for {
		select {
		case <-cf.disconnect:
			return
		case <-cf.statusTimeout.C:
			cf.status = StatusNoResponse
			cf.statusTimeout.Reset(time.Second)
		}
	}
}

func (cf *Crazyflie) PacketSend(packet []byte) {
	crazyradio.PacketSend(cf.channel, cf.address, packet)
}

func (cf *Crazyflie) PacketSendPriority(packet []byte) {
	crazyradio.PacketSendPriority(cf.channel, cf.address, packet)
}

func (cf *Crazyflie) packetCustomSendAwaitResponseOnChannelPort(packet []byte, awaitPort byte, awaitChannel byte, timeout time.Duration, sendFunction func([]byte)) ([]byte, error) {
	// the function which matches and acts on the response packet
	callbackTriggered := make(chan []byte)
	callback := func(resp []byte) {
		header := crtpHeader(resp[0])

		// should check the header port and channel like this (rather than check the hex value of resp[0]) since the link bits might vary(?)
		if header.port() == crtpPort(awaitPort) && header.channel() == awaitChannel {
			callbackTriggered <- resp[1:]
		}
	}

	// add the callback to the list
	e := cf.responseCallbacks[crtpPort(awaitPort)].PushBack(callback)
	defer cf.responseCallbacks[crtpPort(awaitPort)].Remove(e) // and remove it once we're done

	sendFunction(packet) // schedule transmission of the packet

	select {
	case data := <-callbackTriggered:
		return data, nil
	case <-time.After(timeout):
		return nil, ErrorNoResponse
	}
}

func (cf *Crazyflie) PacketSendAwaitResponseOnChannelPort(packet []byte, awaitPort byte, awaitChannel byte, timeout time.Duration) ([]byte, error) {
	return cf.packetCustomSendAwaitResponseOnChannelPort(packet, awaitPort, awaitChannel, timeout, cf.PacketSend)
}

func (cf *Crazyflie) PacketSendPriorityAwaitResponseOnChannelPort(packet []byte, awaitPort byte, awaitChannel byte, timeout time.Duration) ([]byte, error) {
	return cf.packetCustomSendAwaitResponseOnChannelPort(packet, awaitPort, awaitChannel, timeout, cf.PacketSendPriority)
}

func (cf *Crazyflie) PacketSendAwaitResponse(packet []byte, timeout time.Duration) ([]byte, error) {
	awaitPort := byte(crtpHeader(packet[0]).port())
	awaitChannel := crtpHeader(packet[0]).channel()
	return cf.PacketSendAwaitResponseOnChannelPort(packet, awaitPort, awaitChannel, timeout)
}

func (cf *Crazyflie) PacketSendPriorityAwaitResponse(packet []byte, timeout time.Duration) ([]byte, error) {
	awaitPort := byte(crtpHeader(packet[0]).port())
	awaitChannel := crtpHeader(packet[0]).channel()
	return cf.PacketSendPriorityAwaitResponseOnChannelPort(packet, awaitPort, awaitChannel, timeout)
}

// Waits for the packet queues to be empty
func (cf *Crazyflie) PacketQueueWaitForEmpty() {
	crazyradio.PacketQueueWaitForEmpty(cf.channel, cf.address)
}

func (cf *Crazyflie) responseHandler(resp []byte) {
	cf.status = StatusConnected
	cf.statusTimeout.Reset(statusTimeoutDuration)

	if len(resp) > 0 {
		header := crtpHeader(resp[0])

		if header.port() == 0xF3 || header.port() == 0xF7 {
			return // CF has nothing to report, indicate we can transmit at a lower frequency
		}

		// call any registered callbacks for this port
		for e := cf.responseCallbacks[header.port()].Front(); e != nil; e = e.Next() {
			f := e.Value.(func(r []byte))
			go f(resp) // TODO: send them copies? otherwise they can modify underlying data?
		}

		for e := cf.responseCallbacks[crtpPortGreedy].Front(); e != nil; e = e.Next() {
			f := e.Value.(func(r []byte))
			go f(resp)
		}
	}
}

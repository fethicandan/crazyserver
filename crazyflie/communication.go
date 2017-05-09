package crazyflie

import (
	"container/list"

	"sync"
	"time"

	"github.com/mikehamer/crazyserver/crtp"
)

const statusTimeoutDuration time.Duration = 0.5 * time.Second

func (cf *Crazyflie) communicationSystemInit() {
	cf.disconnect = make(chan bool)
	cf.waitGroup = &sync.WaitGroup{}
	cf.statusTimeout = time.NewTimer(statusTimeoutDuration)

	// setup the communication callbacks
	cf.responseCallbacks = map[crtp.Port](*list.List){
		crtp.PortConsole:  list.New(),
		crtp.PortParam:    list.New(),
		crtp.PortSetpoint: list.New(),
		crtp.PortMem:      list.New(),
		crtp.PortLog:      list.New(),
		crtp.PortPosition: list.New(),
		crtp.PortPlatform: list.New(),
		crtp.PortLink:     list.New(),
		crtp.PortEmpty1:   list.New(),
		crtp.PortEmpty2:   list.New(),
		crtp.PortGreedy:   list.New(),
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
			cf.statusTimeout.Reset(statusTimeoutDuration)
		}
	}
}

func (cf *Crazyflie) PacketSend(packet crtp.RequestPacketPtr) error {
	return cf.crtpDevice.PacketSend(cf.channel, cf.address, packet)
}

func (cf *Crazyflie) PacketSendPriority(packet crtp.RequestPacketPtr) error {
	return cf.crtpDevice.PacketSendPriority(cf.channel, cf.address, packet)
}

func (cf *Crazyflie) PacketSendAndAwaitResponse(requestPacket crtp.RequestPacketPtr, responsePacket crtp.ResponsePacketPtr, timeout time.Duration) error {
	return cf.packetCustomSendAndAwaitResponse(cf.PacketSend, requestPacket, responsePacket, timeout)
}

func (cf *Crazyflie) PacketSendPriorityAndAwaitResponse(requestPacket crtp.RequestPacketPtr, responsePacket crtp.ResponsePacketPtr, timeout time.Duration) error {
	return cf.packetCustomSendAndAwaitResponse(cf.PacketSendPriority, requestPacket, responsePacket, timeout)
}

func (cf *Crazyflie) packetCustomSendAndAwaitResponse(sendFunction func(packet crtp.RequestPacketPtr) error, requestPacket crtp.RequestPacketPtr, responsePacket crtp.ResponsePacketPtr, timeout time.Duration) error {
	callbackError := make(chan error)
	callback := func(resp []byte) {
		err := responsePacket.LoadFromBytes(resp)
		if err == crtp.ErrorPacketIncorrectType {
			// if the packet is not the correct one, silently fail and keep waiting
			return
		}
		callbackError <- err // otherwise propagate the error up
	}

	// add the callback to the list
	// note that this callback will be called for every CRTP packet on this port
	e := cf.responseCallbacks[responsePacket.Port()].PushBack(callback)
	// and remove it once we're done
	defer cf.responseCallbacks[responsePacket.Port()].Remove(e)

	// schedule transmission of the packet
	if err := sendFunction(requestPacket); err != nil {
		return err
	}

	select {
	case err := <-callbackError:
		return err
	case <-time.After(timeout):
		return ErrorNoResponse
	}
}

// Waits for the packet queues to be empty
func (cf *Crazyflie) WaitUntilAllPacketsHaveBeenSent() {
	cf.crtpDevice.ClientWaitUntilAllPacketsHaveBeenSent(cf.channel, cf.address)
}

func (cf *Crazyflie) responseHandler(resp []byte) {
	cf.status = StatusConnected
	cf.statusTimeout.Reset(statusTimeoutDuration)

	if len(resp) > 0 {
		header := crtp.Header(resp[0])

		if header.Port() == crtp.PortEmpty1 || header.Port() == crtp.PortEmpty2 {
			return // CF has nothing to report
		}

		// call any registered callbacks for this port
		for e := cf.responseCallbacks[header.Port()].Front(); e != nil; e = e.Next() {
			f := e.Value.(func(r []byte))
			go f(resp) // TODO: send them copies? otherwise they can modify underlying data?
		}

		for e := cf.responseCallbacks[crtp.PortGreedy].Front(); e != nil; e = e.Next() {
			f := e.Value.(func(r []byte))
			go f(resp)
		}
	}
}

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

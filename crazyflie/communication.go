package crazyflie

import (
	"container/list"
	"log"
	"time"
)

const minCommunicationPeriod_ms = 5    // milliseconds
const maxCommunicationPeriod_ms = 1000 // milliseconds
var defaultPacket = []byte{0xFF}       // a ping
var packetDequeued = make(chan bool)

func (cf *Crazyflie) communicationSystemInit() {
	cf.disconnect = make(chan bool)
	cf.disconnectOnEmpty = make(chan bool)
	cf.handlerDisconnect = make(chan bool)

	cf.packetQueue = list.New()
	cf.packetPriorityQueue = list.New()

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
}

func (cf *Crazyflie) PacketSend(packet []byte) {
	packetCopy := make([]byte, len(packet))
	copy(packetCopy, packet)
	cf.packetQueue.PushBack(packetCopy)
}

func (cf *Crazyflie) PacketSendPriority(packet []byte) {
	packetCopy := make([]byte, len(packet))
	copy(packetCopy, packet)
	cf.packetPriorityQueue.PushBack(packetCopy)
}

func (cf *Crazyflie) PacketSendImmediately(packet []byte) {
	packetCopy := make([]byte, len(packet))
	copy(packetCopy, packet)
	cf.packetPriorityQueue.PushFront(packetCopy)
}

// Waits for the packet queues to be empty
func (cf *Crazyflie) WaitForEmptyPacketQueues() {
	for {
		if cf.packetPriorityQueue.Len() == 0 && cf.packetQueue.Len() == 0 {
			return
		}
		<-packetDequeued
	}
}

func (cf *Crazyflie) PacketClearAll() {
	cf.packetQueue.Init()
	cf.packetPriorityQueue.Init()
}

func (cf *Crazyflie) communicationLoop() {
	defer func() { cf.handlerDisconnect <- true }()
	// begin transmitting quickly
	cf.lastUpdate = 0

	minPeriod := time.NewTimer(time.Duration(minCommunicationPeriod_ms) * time.Millisecond)

	for {
		var err error
		var packet []byte
		var packetElement *list.Element
		var packetList *list.List

		if cf.lastUpdate < 2000/minCommunicationPeriod_ms {
			// if we are communicating, keep communicating quickly
			cf.period = minCommunicationPeriod_ms
		} else {
			// otherwise begin exponential slowing
			cf.period *= 2
			if cf.period > maxCommunicationPeriod_ms {
				cf.period = maxCommunicationPeriod_ms
			}
		}

		// wait for one at least one minimum period so we don't spam the CF
		<-minPeriod.C

		select { // non blocking receive on the disconnect channel
		case <-cf.disconnect: // if we should disconnect
			return
		default:
			break //out of this select statement
		}

		if cf.packetPriorityQueue.Front() != nil {
			packetList = cf.packetPriorityQueue
			packetElement = packetList.Front()
			packet = packetElement.Value.([]byte)
			cf.lastUpdate = 0
		} else if cf.packetQueue.Front() != nil {
			packetList = cf.packetQueue
			packetElement = packetList.Front()
			packet = packetElement.Value.([]byte)
			cf.lastUpdate = 0
		} else { // no packets, both queues empty
			select { // non blocking receive on the disconnect channel
			case <-cf.disconnectOnEmpty: // if we should disconnect
				return
			default:
				packetList = nil
				packetElement = nil
				packet = defaultPacket // after a delay, send a ping to keep things alive
				<-time.After(time.Duration(cf.period-minCommunicationPeriod_ms) * time.Millisecond)
			}
		}

		// reset the timer such that the loop runs at the correct maximum frequency irrespective of the processing time below
		minPeriod.Stop()
		minPeriod.Reset(minCommunicationPeriod_ms)

		// we lock the radio so it has the correct address for the whole transaction
		cf.radio.Lock()

		err = cf.radio.SetChannel(cf.channel)
		if err != nil {
			cf.radio.Unlock()
			log.Printf("%X error: %s", cf.address, err)
			cf.lastUpdate++
			continue
		}

		err = cf.radio.SetAddress(cf.address)
		if err != nil {
			cf.radio.Unlock()
			log.Printf("%X error: %s", cf.address, err)
			cf.lastUpdate++
			continue
		}

		err = cf.radio.SendPacket(packet)
		if err != nil {
			cf.radio.Unlock()
			log.Printf("%X error: %s", cf.address, err)
			cf.lastUpdate++
			continue
		}

		// read the response, which we then distribute to the relevant handler
		ackReceived, resp, err := cf.radio.ReadResponse()
		cf.radio.Unlock() // want to unlock the radio ASAP such that other crazyflies can take it

		// log.Println("-> ", packet)
		// log.Println("<- ", ackReceived, resp, err)

		if err != nil {
			log.Printf("%X error: %s", cf.address, err)
			continue // if there is an error, something is wrong... should try and retransmit the packet
		}

		if !ackReceived {
			continue // if there is no response, something is wrong... should try and retransmit the packet
		}

		if packetList != nil {
			packetList.Remove(packetElement) // remove the acknowledged packet, since it was successfully transmitted

			select { // if possible (eg. if not already triggered), trigger the packetDequeued channel (used only in function WaitForEmptyPacketQueues)
			case packetDequeued <- true:
				break
			default: // if it has already been triggered, do nothing
				break
			}
		}

		if len(resp) > 0 {
			header := crtpHeader(resp[0])

			if header.port() == 0xF3 || header.port() == 0xF7 {
				cf.lastUpdate++ // CF has nothing to report, indicate we can transmit at a lower frequency
			} else {
				cf.lastUpdate = 0 // wants to tell us something, so we communicate with a max frequency again
			}

			// call any registered callbacks for this port
			for e := cf.responseCallbacks[header.port()].Front(); e != nil; e = e.Next() {
				f := e.Value.(func(r []byte))
				f(resp)
			}

			for e := cf.responseCallbacks[crtpPortGreedy].Front(); e != nil; e = e.Next() {
				f := e.Value.(func(r []byte))
				f(resp)
			}
		} else {
			// we sent an acknowledgement only packet (basically only flashing), don't throttle
			cf.lastUpdate = 0
		}
	}
}

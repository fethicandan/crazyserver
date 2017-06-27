package crazyusb

// Functions implementing the CrtpDevice interface

import "github.com/mikehamer/crazyserver/crtp"

func (cr *CrtpUsb) ClientRegister(channel uint8, address uint64, callback func([]byte)) {
	cr.clientCallbackSet(callback)
}

func (cr *CrtpUsb) ClientRemove(channel uint8, address uint64) {
	cr.clientCallbackRemove()
}

func (cr *CrtpUsb) ClientWaitUntilAllPacketsHaveBeenSent(channel uint8, address uint64) {

	for {
		if cr.priorityQueue.Len() == 0 && cr.standardQueue.Len() == 0 {
			break
		} else {
			select {
			case <-cr.packetDequeued: // block here until the radioThread indicates one of our packets has been dequeued (after which we again check for empty)
				//case <-time.After(50 * time.Millisecond):
				break
			}
		}
	}
}

func (cr *CrtpUsb) PacketSend(channel uint8, address uint64, request crtp.RequestPacketPtr) error {
	return clientPacketEnqueue(cr.standardQueue, request)
}

func (cr *CrtpUsb) PacketSendPriority(channel uint8, address uint64, request crtp.RequestPacketPtr) error {
	return clientPacketEnqueue(cr.priorityQueue, request)
}

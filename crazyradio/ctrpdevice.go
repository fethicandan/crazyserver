package crazyradio

// Functions implementing the CrtpDevice interface

import (
	"fmt"

	"github.com/mikehamer/crazyserver/crtp"
)

func (cr *Radio) ClientRegister(channel uint8, address uint64, callback func([]byte)) {
	cr.clientCallbackSet(channel, address, callback)
	cr.clientPacketQueueGet(channel, address) // initializes if non existent
	fmt.Printf("New client %d:0x%X\n", channel, address)
}

func (cr *Radio) ClientRemove(channel uint8, address uint64) {
	cr.clientCallbackRemove(channel, address)
	cr.clientPacketQueueRemove(channel, address)
}

func (cr *Radio) ClientWaitUntilAllPacketsHaveBeenSent(channel uint8, address uint64) {
	queue := cr.clientPacketQueueGet(channel, address)

	for {
		if queue.priorityQueue.Len() == 0 && queue.standardQueue.Len() == 0 {
			break
		} else {
			<-queue.packetDequeued // block here until the radioThread indicates one of our packets has been dequeued (after which we again check for empty)
		}
	}
}

func (cr *Radio) PacketSend(channel uint8, address uint64, request crtp.RequestPacketPtr) error {
	return clientPacketEnqueue(cr.clientPacketQueueGet(channel, address).standardQueue, request)
}

func (cr *Radio) PacketSendPriority(channel uint8, address uint64, request crtp.RequestPacketPtr) error {
	return clientPacketEnqueue(cr.clientPacketQueueGet(channel, address).priorityQueue, request)
}

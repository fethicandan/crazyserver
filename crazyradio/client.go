package crazyradio

import (
	"github.com/Workiva/go-datastructures/queue"
	"github.com/mikehamer/crazyserver/crtp"
)

func (cr *Radio) clientCallbackSet(channel uint8, address uint64, callback func([]byte)) {
	if _, ok := cr.callbacks[channel]; !ok {
		cr.callbacks[channel] = make(map[uint64]func([]byte))
	}
	cr.callbacks[channel][address] = callback
}

func (cr *Radio) clientCallbackRemove(channel uint8, address uint64) {
	delete(cr.callbacks[channel], address)
	if len(cr.callbacks[channel]) == 0 {
		delete(cr.callbacks, channel)
	}
}

func (cr *Radio) clientPacketQueueGet(channel uint8, address uint64) *packetQueue {
	if _, ok := cr.packetQueues[channel]; !ok {
		cr.packetQueues[channel] = make(map[uint64]*packetQueue)
	}

	channelQueues := cr.packetQueues[channel]

	if _, ok := channelQueues[address]; !ok {
		channelQueues[address] = &packetQueue{queue.New(10), queue.New(10), make(chan bool)}
	}

	return channelQueues[address]
}

func (cr *Radio) clientPacketQueueRemove(channel uint8, address uint64) {
	delete(cr.packetQueues[channel], address)
	if len(cr.packetQueues[channel]) == 0 {
		delete(cr.packetQueues, channel)
	}
}

func clientPacketEnqueue(queue *queue.Queue, request crtp.RequestPacketPtr) error {

	requestBody := request.Bytes()
	requestData := make([]byte, len(requestBody)+1)
	requestData[0] = crtp.HeaderBytes(request.Port(), request.Channel())
	if len(requestBody) > 0 {
		copy(requestData[1:], requestBody)
	}

	return queue.Put(requestData)
}

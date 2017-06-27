package crazyusb

import (
	"github.com/Workiva/go-datastructures/queue"
	"github.com/mikehamer/crazyserver/crtp"
)

func (cr *CrtpUsb) clientCallbackSet(callback func([]byte)) {
	cr.callback = callback
}

func (cr *CrtpUsb) clientCallbackRemove() {
	cr.callback = nil
}

func clientPacketEnqueue(queue *queue.Queue, request crtp.RequestPacketPtr) error {

	requestBody := request.Bytes()
	requestData := make([]byte, len(requestBody)+1)
	requestData[0] = crtp.HeaderBytes(request.Port(), request.Channel())
	copy(requestData[1:], requestBody)

	return queue.Put(requestData)
}

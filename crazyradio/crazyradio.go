package crazyradio

import (
	"container/list"
	"fmt"
	"sync"
	"time"
)

type packetQueue struct {
	standardQueue  *list.List
	priorityQueue  *list.List
	lock           *sync.Mutex
	packetDequeued chan bool
}

var radios []*RadioDevice

var packetQueues map[uint8]map[uint64]*packetQueue
var callbacks map[uint64]func([]byte)

var radioThreadShouldStop chan bool
var waitGroup *sync.WaitGroup

var defaultPacket = []byte{0xFF}

func callbackRegister(address uint64, callback func([]byte)) {
	callbacks[address] = callback
}

func callbackRemove(address uint64) {
	delete(callbacks, address)
}

func CrazyflieRegister(channel uint8, address uint64, responseCallback func([]byte)) error {

	var ackReceived = false
	var err error = nil

	// first, test the crazyflie
	for i := 0; i < 200; i++ {
		timeout := time.After(50 * time.Millisecond)

		radios[0].Lock()
		radios[0].SetChannel(channel)
		radios[0].SetAddress(address)
		radios[0].SendPacket([]byte{0xFF})             // ping the crazyflie
		ackReceived, _, err = radios[0].ReadResponse() // and see if it responds
		radios[0].Unlock()

		// if it responds, we've verified connectivity and quit the loop
		if ackReceived {
			break
		}

		// otherwise we wait for 50ms and then try again
		<-timeout
	}

	if !ackReceived || err != nil {
		fmt.Printf("Error connecting to %d/0x%X: %v", channel, address, err)

		if err != nil {
			return err
		}
		return ErrorNoResponse
	}

	packetQueueGet(channel, address) // this also initializes the packet queues
	callbackRegister(address, responseCallback)
	fmt.Printf("Connected to %d/0x%X", channel, address)
	return nil
}

func CrazyflieRemove(channel uint8, address uint64) {
	callbackRemove(address)
	packetQueueRemove(channel, address)

}

func packetQueueGet(channel uint8, address uint64) *packetQueue {
	if _, ok := packetQueues[channel]; !ok {
		packetQueues[channel] = make(map[uint64]*packetQueue)
	}
	channelQueues := packetQueues[channel]

	if _, ok := channelQueues[address]; !ok {
		channelQueues[address] = &packetQueue{list.New(), list.New(), new(sync.Mutex), make(chan bool)}
	}

	return channelQueues[address]
}

func packetQueueRemove(channel uint8, address uint64) {
	delete(packetQueues[channel], address)
	if len(packetQueues[channel]) == 0 {
		delete(packetQueues, channel)
	}
}

func PacketSend(channel uint8, address uint64, packet []byte) {
	queue := packetQueueGet(channel, address)

	packetCopy := make([]byte, len(packet))
	copy(packetCopy, packet)

	queue.lock.Lock()
	queue.standardQueue.PushBack(packetCopy)
	queue.lock.Unlock()
}

func PacketSendPriority(channel uint8, address uint64, packet []byte) {
	queue := packetQueueGet(channel, address)

	packetCopy := make([]byte, len(packet))
	copy(packetCopy, packet)

	queue.lock.Lock()
	queue.priorityQueue.PushBack(packetCopy)
	queue.lock.Unlock()
}

func PacketQueueWaitForEmpty(channel uint8, address uint64) {
	queue := packetQueueGet(channel, address)

	for {
		if queue.priorityQueue.Front() == nil && queue.standardQueue.Front() == nil {
			break
		} else {
			<-queue.packetDequeued // block here until the radioThread indicates one of our packets has been dequeued (after which we again check for empty)
		}
	}
}

func Start() error {
	callbacks = make(map[uint64]func([]byte))
	packetQueues = make(map[uint8]map[uint64]*packetQueue)

	radioThreadShouldStop = make(chan bool)
	waitGroup = &sync.WaitGroup{}

	radio, err := Open()
	if err != nil {
		return err
	}

	radios = append(radios, radio)

	waitGroup.Add(1)
	go radioThread()

	return nil
}

func Stop() {
	close(radioThreadShouldStop)
	waitGroup.Wait()
}

func radioThread() {
	defer waitGroup.Done()

	var err error
	radio := radios[0]

	for {
		// quit if we should quit
		select {
		case <-radioThreadShouldStop:
			return
		default:
			break
		}

		if len(packetQueues) == 0 {
			<-time.After(10 * time.Millisecond)
			continue
		}

		for channel, channelQueues := range packetQueues { // loop through all channels
			// quit if we should quit
			select {
			case <-radioThreadShouldStop:
				return
			default:
				break
			}

			for address, queue := range channelQueues {
				// quit if we should quit
				select {
				case <-radioThreadShouldStop:
					return
				default:
					break
				}

				queue.lock.Lock()

				var packetQueue *list.List = nil
				var packetElement *list.Element = nil
				var packet []byte

				if queue.priorityQueue.Front() != nil {
					packetQueue = queue.priorityQueue
					packetElement = packetQueue.Front()
					packet = packetElement.Value.([]byte)
				} else if queue.standardQueue.Front() != nil {
					packetQueue = queue.standardQueue
					packetElement = packetQueue.Front()
					packet = packetElement.Value.([]byte)
				} else {
					packet = defaultPacket
				}

				queue.lock.Unlock()

				radio.lock.Lock()

				radio.SetChannel(channel)
				radio.SetAddress(address)
				err = radio.SendPacket(packet)
				if err != nil {
					continue
				}

				// read the response, which we then distribute to the relevant handler
				ackReceived, resp, err := radio.ReadResponse()
				radio.lock.Unlock()

				if err != nil || !ackReceived {
					continue // if there is an error, something is wrong... should try and retransmit the packet
				}

				if packetQueue != nil {
					queue.lock.Lock()
					packetQueue.Remove(packetElement) // remove the acknowledged packet, since it was successfully transmitted
					queue.lock.Unlock()
				}

				select { // if possible (eg. if not already triggered), trigger the packetDequeued channel (used only in function WaitForEmptyPacketQueue)
				case queue.packetDequeued <- true:
					break
				default: // if it has already been triggered, do nothing
					break
				}

				// now call the crazyflie's callback (resp will have len 0 if the packet was acked with no data)
				if callback, ok := callbacks[address]; ok {
					go callback(resp)
				}
			}
		}
	}
}

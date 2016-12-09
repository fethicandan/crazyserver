package crazyradio

import (
	"container/list"
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
var radioWorkQueue chan uint8

var packetQueues map[uint8]map[uint64]*packetQueue
var callbacks map[uint64]func([]byte)

var radioThreadShouldStop chan bool
var globalWaitGroup *sync.WaitGroup
var workWaitGroup *sync.WaitGroup

var defaultPacket = []byte{0xFF}

func callbackRegister(address uint64, callback func([]byte)) {
	callbacks[address] = callback
}

func callbackRemove(address uint64) {
	delete(callbacks, address)
}

func CrazyflieRegister(channel uint8, address uint64, responseCallback func([]byte)) error {

	// setup a temporary callback for the crazyflie such that this thread is notified when
	cfCommunicating := make(chan bool)
	callbackRegister(address, func(resp []byte) {
		select {
		case cfCommunicating <- true:
		default:
		}
	})

	// initialize the packet queues for the crazyflie
	// this will cause it to be pinged in the next round (and our callback will be called)
	packetQueueGet(channel, address)

	// wait for the crazyflie to respond, or to time out

	select {
	case <-time.After(5 * time.Second):
		packetQueueRemove(channel, address)
		callbackRemove(address)
		return ErrorNoResponse
	case <-cfCommunicating:
		callbackRegister(address, responseCallback)
		return nil
	}
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
	globalWaitGroup = &sync.WaitGroup{}
	workWaitGroup = &sync.WaitGroup{}

	radios, err := OpenAllRadios()
	if err != nil {
		return err
	}

	// the queue on which radios receive their work
	radioWorkQueue = make(chan uint8, 256)

	// start a thread per radio
	for _, r := range radios {
		globalWaitGroup.Add(1)
		go radioThread(r)
	}

	// start the thread to coordinate the radios
	globalWaitGroup.Add(1)
	go coordinatorThread()

	return nil
}

func Stop() {
	close(radioThreadShouldStop)
	globalWaitGroup.Wait()

	for _, r := range radios {
		r.Close()
	}
}

func radioThread(radio *RadioDevice) {
	defer globalWaitGroup.Done()

	for {
		var channel uint8

		select {
		case <-radioThreadShouldStop:
			return // here no need to workWaitGroup.Done() since we haven't received work
		case channel = <-radioWorkQueue:
		}

	addressLoop:
		for address, queue := range packetQueues[channel] {
			// quit if we should quit
			select {
			case <-radioThreadShouldStop:
				break addressLoop // prematurely finish the work
			default:
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

			radio.SetChannel(channel)
			radio.SetAddress(address)
			err := radio.SendPacket(packet)
			if err != nil {
				continue
			}

			// read the response, which we then distribute to the relevant handler
			ackReceived, resp, err := radio.ReadResponse()

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
			}

			// now call the crazyflie's callback (resp will have len 0 if the packet was acked with no data)
			if callback, ok := callbacks[address]; ok {
				go callback(resp)
			}
		}

		workWaitGroup.Done() // signal to the coordinatorThread that we're done with the work
	}
}

func coordinatorThread() {
	defer globalWaitGroup.Done()

	for {
		// quit if we should quit
		select {
		case <-radioThreadShouldStop:
			return
		default:
		}

		if len(packetQueues) == 0 {
			<-time.After(10 * time.Millisecond)
			continue
		}

		for channel := range packetQueues { // loop through all channels
			workWaitGroup.Add(1)
			radioWorkQueue <- channel
		}
		workWaitGroup.Wait() // wait for all work to be processed, ensures that only one radio operates per channel
	}
}

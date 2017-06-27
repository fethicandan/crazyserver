package crazyradio

import (
	"sync"
	"time"

	"fmt"

	"github.com/Workiva/go-datastructures/queue"
)

type packetQueue struct {
	standardQueue  *queue.Queue
	priorityQueue  *queue.Queue
	packetDequeued chan bool
}

var defaultPacket = []byte{0xFF}
var singletonRadio *Radio = nil

type Radio struct {
	radios           []*radioDevice
	workQueue        chan uint8
	threadShouldStop chan bool

	packetQueues map[uint8]map[uint64]*packetQueue
	callbacks    map[uint8]map[uint64]func([]byte)

	globalWaitGroup *sync.WaitGroup
	workWaitGroup   *sync.WaitGroup
}

func Open() (*Radio, error) {
	if singletonRadio != nil {
		return singletonRadio, nil
	}

	radios, err := openAllRadios()
	if err != nil {
		return nil, err
	}

	singletonRadio = &Radio{
		radios:           radios,
		workQueue:        make(chan uint8, 256),
		threadShouldStop: make(chan bool),

		packetQueues: make(map[uint8]map[uint64]*packetQueue),
		callbacks:    make(map[uint8]map[uint64]func([]byte)),

		globalWaitGroup: &sync.WaitGroup{},
		workWaitGroup:   &sync.WaitGroup{},
	}

	// start a thread per radio
	for _, r := range singletonRadio.radios {
		singletonRadio.globalWaitGroup.Add(1)
		go singletonRadio.radioThread(r)
	}

	// start the thread to coordinate the radios
	singletonRadio.globalWaitGroup.Add(1)
	go singletonRadio.coordinatorThread()

	return singletonRadio, nil
}

func (cr *Radio) Close() {
	close(cr.threadShouldStop)
	cr.globalWaitGroup.Wait()

	for _, r := range cr.radios {
		r.Close()
	}

	singletonRadio = nil
}

func (cr *Radio) radioThread(radio *radioDevice) {
	defer cr.globalWaitGroup.Done()

	for {
		var channel uint8

		select {
		case <-cr.threadShouldStop:
			return // here no need to workWaitGroup.Done() since we haven't received work
		case channel = <-cr.workQueue:
		}

	addressLoop:
		for address, addressQueue := range cr.packetQueues[channel] {
			// quit if we should quit
			select {
			case <-cr.threadShouldStop:
				break addressLoop // prematurely finish the work
			default:
				//fmt.Printf("Service %d:0x%X\n", channel, address)
			}

			var currentQueue *queue.Queue = nil
			var packet []byte = defaultPacket

			if frontPacket, err := addressQueue.priorityQueue.Peek(); err == nil {
				currentQueue = addressQueue.priorityQueue
				packet = frontPacket.([]byte)
				//fmt.Printf("Priority %d:0x%X — %v\n", channel, address, packet)
			} else if frontPacket, err := addressQueue.standardQueue.Peek(); err == nil {
				currentQueue = addressQueue.standardQueue
				packet = frontPacket.([]byte)
				//fmt.Printf("Standard %d:0x%X — %v\n", channel, address, packet)
			}

			err := radio.SetChannel(channel)
			if err != nil {
				fmt.Printf("Error setting channel: %v\n", err)
				continue
			}

			err = radio.SetAddress(address)
			if err != nil {
				fmt.Printf("Error setting address: %v\n", err)
				continue
			}

			err = radio.SendPacket(packet)
			if err != nil {
				fmt.Printf("Error sending packet: %v\n", err)
				continue
			}

			// read the response, which we then distribute to the relevant handler
			ackReceived, resp, err := radio.ReadResponse()

			if err != nil || !ackReceived {
				continue // if there is an error, something is wrong... should try and retransmit the packet
			}

			if currentQueue != nil {
				currentQueue.Get(1) // remove the acknowledged packet, since it was successfully transmitted
			}

			select { // if possible (eg. if not already triggered), trigger the packetDequeued channel (used only in function WaitForEmptyPacketQueue)
			case addressQueue.packetDequeued <- true:
				break
			default: // if it has already been triggered, do nothing
			}

			// now call the crazyflie's callback (resp will have len 0 if the packet was acked with no data)
			if callback, ok := cr.callbacks[channel][address]; ok {
				go callback(resp)
			}
		}

		cr.workWaitGroup.Done() // signal to the coordinatorThread that we're done with the work
	}
}

func (cr *Radio) coordinatorThread() {
	defer cr.globalWaitGroup.Done()

	for {
		// quit if we should quit
		select {
		case <-cr.threadShouldStop:
			return
		default:
		}

		if len(cr.packetQueues) == 0 {
			<-time.After(10 * time.Millisecond)
			continue
		}

		for channel := range cr.packetQueues { // loop through all channels
			cr.workWaitGroup.Add(1)
			cr.workQueue <- channel
		}
		cr.workWaitGroup.Wait() // wait for all work to be processed, ensures that only one radio operates per channel
	}
}

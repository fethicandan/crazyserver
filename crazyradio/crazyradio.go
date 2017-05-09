package crazyradio

import (
	"sync"
	"time"

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
	radios                []*RadioDevice
	radioWorkQueue        chan uint8
	radioThreadShouldStop chan bool

	packetQueues map[uint8]map[uint64]*packetQueue
	callbacks    map[uint8]map[uint64]func([]byte)

	globalWaitGroup *sync.WaitGroup
	workWaitGroup   *sync.WaitGroup
}

func Open() (*Radio, error) {
	if singletonRadio != nil {
		return singletonRadio, nil
	}

	radios, err := OpenAllRadios()
	if err != nil {
		return nil, err
	}

	singletonRadio = &Radio{
		radios:                radios,
		radioWorkQueue:        make(chan uint8, 256),
		radioThreadShouldStop: make(chan bool),

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
	close(cr.radioThreadShouldStop)
	cr.globalWaitGroup.Wait()

	for _, r := range cr.radios {
		r.Close()
	}
}

func (cr *Radio) radioThread(radio *RadioDevice) {
	defer cr.globalWaitGroup.Done()

	for {
		var channel uint8

		select {
		case <-cr.radioThreadShouldStop:
			return // here no need to workWaitGroup.Done() since we haven't received work
		case channel = <-cr.radioWorkQueue:
		}

	addressLoop:
		for address, addressQueue := range cr.packetQueues[channel] {
			// quit if we should quit
			select {
			case <-cr.radioThreadShouldStop:
				break addressLoop // prematurely finish the work
			default:
			}

			var currentQueue *queue.Queue = nil
			var packet []byte = defaultPacket

			if !addressQueue.priorityQueue.Empty() {
				currentQueue = addressQueue.priorityQueue
				frontPacket, err := currentQueue.Peek()
				if err != nil {
					packet = frontPacket.([]byte)
				}
			} else if !addressQueue.standardQueue.Empty() {
				currentQueue = addressQueue.standardQueue
				frontPacket, err := currentQueue.Peek()
				if err != nil {
					packet = frontPacket.([]byte)
				}
			}

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
		case <-cr.radioThreadShouldStop:
			return
		default:
		}

		if len(cr.packetQueues) == 0 {
			<-time.After(10 * time.Millisecond)
			continue
		}

		for channel := range cr.packetQueues { // loop through all channels
			cr.workWaitGroup.Add(1)
			cr.radioWorkQueue <- channel
		}
		cr.workWaitGroup.Wait() // wait for all work to be processed, ensures that only one radio operates per channel
	}
}

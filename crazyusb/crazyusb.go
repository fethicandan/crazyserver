package crazyusb

import (
	"sync"

	"fmt"

	"time"

	"github.com/Workiva/go-datastructures/queue"
	"github.com/mikehamer/crazyserver/crtp"
)

var defaultPacket = []byte{}
var singletonUsb *CrtpUsb = nil

type CrtpUsb struct {
	usbDevice        *usbDevice
	threadShouldStop chan bool

	standardQueue  *queue.Queue
	priorityQueue  *queue.Queue
	packetDequeued chan bool

	callback func([]byte)

	globalWaitGroup *sync.WaitGroup
}

func Open() (*CrtpUsb, error) {
	if singletonUsb != nil {
		return nil, ErrorDeviceAlreadyOpen
	}

	dev, err := openUsbDevice()
	if err != nil {
		return nil, err
	}

	singletonUsb = &CrtpUsb{
		usbDevice: dev,

		threadShouldStop: make(chan bool),

		standardQueue:  queue.New(10),
		priorityQueue:  queue.New(10),
		packetDequeued: make(chan bool),
		callback:       nil,

		globalWaitGroup: &sync.WaitGroup{},
	}

	// start a thread per radio

	go singletonUsb.workerThread()
	go singletonUsb.readerThread()

	return singletonUsb, nil
}

func (cr *CrtpUsb) Close() {
	close(cr.threadShouldStop)
	cr.globalWaitGroup.Wait()
	cr.usbDevice.Close()

	singletonUsb = nil
}

func (cr *CrtpUsb) readerHelperThread(packetChan chan []byte) {
	cr.globalWaitGroup.Add(1)
	defer cr.globalWaitGroup.Done()

	for {
		select {
		case <-cr.threadShouldStop:
			return
		default:
			break
		}

		// read the response, which we then distribute to the relevant handler
		resp, err := cr.usbDevice.ReadResponse()

		if err != nil {
			fmt.Println("Read error", err)
		} else {
			packetChan <- resp
		}
	}
}

func (cr *CrtpUsb) readerThread() {
	// no need to ping the CF when using USB interface, we can just keep reading
	cr.globalWaitGroup.Add(1)
	defer cr.globalWaitGroup.Done()

	packetChan := make(chan []byte, 100)
	go cr.readerHelperThread(packetChan)

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cr.threadShouldStop:
			return
		case <-ticker.C:
			break
		}

		// now call the crazyflie's callback (resp will have len 0 if the packet was acked with no data)
		select {
		case resp := <-packetChan:
			if cr.callback != nil {
				go cr.callback(resp)
			}
		default:
			if cr.callback != nil {
				go cr.callback([]byte{crtp.PortEmpty1}) // if there is no packet on the channel, simulate a ping
			}
		}
	}
}

func (cr *CrtpUsb) workerThread() {
	// since we don't have to ping the CF, we can just wait until packets are enqueued, and then send them
	cr.globalWaitGroup.Add(1)
	defer cr.globalWaitGroup.Done()

	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-cr.threadShouldStop:
			return
		case <-ticker.C:
			break
		}

		var currentQueue *queue.Queue = nil
		var packet []byte = defaultPacket

		if frontPacket, err := cr.priorityQueue.Peek(); err == nil {
			currentQueue = cr.priorityQueue
			packet = frontPacket.([]byte)
			//fmt.Printf("Priority %d:0x%X — %v\n", channel, address, packet)
		} else if frontPacket, err := cr.standardQueue.Peek(); err == nil {
			currentQueue = cr.standardQueue
			packet = frontPacket.([]byte)
			//fmt.Printf("Standard %d:0x%X — %v\n", channel, address, packet)
		} else {
			continue
		}

		// currentQueue != nil
		err := cr.usbDevice.SendPacket(packet)
		if err != nil {
			fmt.Printf("Error sending packet: %v\n", err)
			continue
		}
		currentQueue.Get(1) // remove the acknowledged packet, since it was successfully transmitted

		select { // if possible (eg. if not already triggered), trigger the packetDequeued channel (used only in function WaitForEmptyPacketQueue)
		case cr.packetDequeued <- true:
			break
		default: // if it has already been triggered, do nothing
		}
	}
}

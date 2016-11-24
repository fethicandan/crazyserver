package crazyflie

import (
	"container/list"
	"time"

	"github.com/mikehamer/crazyradio/crazyradio"
)

type Crazyflie struct {
	radio   *crazyradio.RadioDevice
	address uint64

	// communication loop
	disconnect   chan bool
	commandQueue chan []byte
	lastUpdate   uint
	period       uint

	// callbacks for packet reception
	responseCallbacks map[crtpPort](*list.List)

	// console printing
	accumulatedConsolePrint string

	// log variables
	logCount       int
	logCRC         uint32
	logMaxPacket   uint8
	logMaxOps      uint8
	logNameToIndex map[string]logItem
	logIndexToName map[uint8]string

	// parameters
	paramCount int
	paramCache uint
}

func Connect(radio *crazyradio.RadioDevice, address uint64) (*Crazyflie, error) {
	cf := new(Crazyflie)
	cf.radio = radio
	cf.address = address

	// ping until response or timeout
	var err error
	var responseReceived bool

	// try to connect 10 times (5 seconds), before giving up
	for i := 0; i < 10; i++ {
		cf.radio.Lock()

		err = cf.radio.SetAddress(cf.address)
		if err != nil {
			cf.radio.Unlock()
			return nil, err
		}

		// ping the crazyflie
		err = cf.radio.SendPacket([]byte{0xFF})
		if err != nil {
			cf.radio.Unlock()
			return nil, err
		}

		// and see if it responds
		responseReceived, _, err = cf.radio.ReadResponse()
		cf.radio.Unlock()

		if err != nil {
			return nil, err
		}

		// if it responds, we've verified connectivity and quit the loop
		if responseReceived {
			break
		}

		// otherwise we wait for 500ms and then try again
		<-time.After(500 * time.Millisecond)
	}

	if responseReceived {
		// initialize the structures required for communication and packet handling
		cf.disconnect = make(chan bool)
		cf.commandQueue = make(chan []byte, 1000)

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
		}
		cf.responseCallbacks[crtpPortConsole].PushBack(cf.handleConsoleResponse)

		cf.logNameToIndex = make(map[string]logItem)
		cf.logIndexToName = make(map[uint8]string)

		// start the crazyflie's communications thread
		go cf.communicationLoop()

		// return success
		return cf, nil
	} else {
		return nil, ErrorNoResponse
	}
}

func (cf *Crazyflie) Disconnect() {
	// asynchronously (& non-blocking) stops the communications thread
	cf.disconnect <- true
	time.Sleep(minCommunicationPeriod_ms) // wait for enough time for the communications thread to exit
}

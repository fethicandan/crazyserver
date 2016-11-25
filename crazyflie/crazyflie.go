package crazyflie

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/mikehamer/crazyserver/crazyradio"
)

type Crazyflie struct {
	radio     *crazyradio.RadioDevice
	address   uint64
	firstInit sync.Once

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
	logBlocks      map[int]logBlock

	// parameters
	paramCount       int
	paramCRC         uint32
	paramNameToIndex map[string]paramItem
	paramIndexToName map[uint8]string
}

func Connect(radio *crazyradio.RadioDevice, address uint64) (*Crazyflie, error) {
	cf := new(Crazyflie)
	cf.radio = radio
	cf.address = address

	err := cf.connect()
	if err != nil {
		return nil, err
	}

	return cf, nil
}

func (cf *Crazyflie) connect() error {
	var err error
	var responseReceived bool

	// try to connect 10 times (5 seconds), before giving up
	fmt.Print("Connecting to Crazyflie.")
	for i := 0; i < 10; i++ {
		// timeout := time.After(500 * time.Millisecond)

		cf.radio.Lock()
		err = cf.radio.SetAddress(cf.address)
		err = cf.radio.SendPacket([]byte{0xFF})            // ping the crazyflie
		responseReceived, _, err = cf.radio.ReadResponse() // and see if it responds
		cf.radio.Unlock()

		// if it responds, we've verified connectivity and quit the loop
		if responseReceived {
			break
		}

		// otherwise we wait for 500ms and then try again
		// <-timeout
		fmt.Print(".")

	}
	fmt.Println(" Connected")

	if err != nil {
		return err
	}

	if responseReceived {
		cf.firstInit.Do(func() {
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
				crtpPortGreedy:   list.New(),
			}

			cf.consoleSystemInit()
			cf.logSystemInit()
			cf.paramSystemInit()
		})

		// start the crazyflie's communications thread
		go cf.communicationLoop()

		return nil
	} else {
		return ErrorNoResponse
	}
}

func (cf *Crazyflie) Disconnect() {
	// asynchronously (& non-blocking) stops the communications thread
	cf.disconnect <- true
	time.Sleep(minCommunicationPeriod_ms) // wait for enough time for the communications thread to exit
}

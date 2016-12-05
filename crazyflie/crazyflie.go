package crazyflie

import (
	"container/list"
	"fmt"
	"sync"
	"time"

	"github.com/mikehamer/crazyserver/crazyradio"
)

type Crazyflie struct {
	radio           *crazyradio.RadioDevice
	address         uint64
	firmwareAddress uint64
	channel         uint8
	firmwareChannel uint8
	firstInit       sync.Once

	// communication loop
	disconnect          chan bool
	disconnectOnEmpty   chan bool
	handlerDisconnect   chan bool
	packetQueue         *list.List
	packetPriorityQueue *list.List
	lastUpdate          uint
	period              uint

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

func Connect(radio *crazyradio.RadioDevice, address uint64, channel uint8) (*Crazyflie, error) {
	cf := new(Crazyflie)
	cf.radio = radio

	cf.address = address
	cf.channel = channel
	cf.firmwareAddress = address // we save explicitly the firmware address and channel since a restart to bootloader will overwrite the current radio settings
	cf.firmwareChannel = channel

	err := cf.connect(address, channel)
	if err != nil {
		return nil, err
	}

	return cf, nil
}

func (cf *Crazyflie) connect(address uint64, channel uint8) error {
	var err error
	var ackReceived bool

	cf.address = address
	cf.channel = channel

	for i := 0; i < 10; i++ {
		timeout := time.After(500 * time.Millisecond)

		cf.radio.Lock()
		err = cf.radio.SetChannel(cf.channel)
		err = cf.radio.SetAddress(cf.address)
		err = cf.radio.SendPacket([]byte{0xFF})       // ping the crazyflie
		ackReceived, _, err = cf.radio.ReadResponse() // and see if it responds
		cf.radio.Unlock()

		// if it responds, we've verified connectivity and quit the loop
		if ackReceived {
			break
		}

		if i == 0 {
			fmt.Print("Connecting to Crazyflie. ")
		} else {
			fmt.Print(". ")
		}

		// otherwise we wait for 500ms and then try again
		<-timeout
	}

	if !ackReceived || err != nil {
		fmt.Printf("Error connecting (response: %t, error: %v)", ackReceived, err)
		if err != nil {
			return err
		}
		return ErrorNoResponse
	}

	fmt.Println("Connected")

	if !ackReceived {
		return ErrorNoResponse
	}

	cf.firstInit.Do(func() {
		// initialize the structures required for communication and packet handling
		cf.communicationSystemInit()
		cf.consoleSystemInit()
		cf.logSystemInit()
		cf.paramSystemInit()
	})

	// start the crazyflie's communications thread
	go cf.communicationLoop()

	return nil
}

func (cf *Crazyflie) DisconnectImmediately() {
	// asynchronously (& non-blocking) stops the communications thread
	cf.disconnect <- true
	<-cf.handlerDisconnect
}

func (cf *Crazyflie) DisconnectOnEmpty() {
	// asynchronously (& non-blocking) stops the communications thread
	cf.disconnectOnEmpty <- true
	<-cf.handlerDisconnect
}

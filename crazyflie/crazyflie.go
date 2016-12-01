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
	disconnect        chan bool
	disconnectOnEmpty chan bool
	handlerDisconnect chan bool
	commandQueue      chan []byte
	lastUpdate        uint
	period            uint

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
	var responseReceived bool

	cf.address = address
	cf.channel = channel

	for i := 0; i < 10; i++ {
		timeout := time.After(500 * time.Millisecond)

		cf.radio.Lock()
		err = cf.radio.SetChannel(cf.channel)
		err = cf.radio.SetAddress(cf.address)
		err = cf.radio.SendPacket([]byte{0xFF})            // ping the crazyflie
		responseReceived, _, err = cf.radio.ReadResponse() // and see if it responds
		cf.radio.Unlock()

		// if it responds, we've verified connectivity and quit the loop
		if responseReceived {
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

	if !responseReceived || err != nil {
		fmt.Printf("Error connecting (response: %t, error: %v)", responseReceived, err)
		if err != nil {
			return err
		} else {
			return ErrorNoResponse
		}
	}

	fmt.Println("Connected")

	if responseReceived {
		cf.firstInit.Do(func() {
			// initialize the structures required for communication and packet handling
			cf.disconnect = make(chan bool)
			cf.disconnectOnEmpty = make(chan bool)
			cf.handlerDisconnect = make(chan bool)
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
	<-cf.handlerDisconnect
}

func (cf *Crazyflie) DisconnectOnEmpty() {
	// asynchronously (& non-blocking) stops the communications thread
	cf.disconnectOnEmpty <- true
	<-cf.handlerDisconnect
}

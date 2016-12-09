package crazyflie

import (
	"container/list"
	"sync"
	"time"

	"github.com/mikehamer/crazyserver/crazyradio"
)

type CrazyflieStatus uint8

const (
	StatusDisconnected CrazyflieStatus = iota
	StatusConnected
	StatusNoResponse
)

type Crazyflie struct {
	address         uint64
	firmwareAddress uint64
	channel         uint8
	firmwareChannel uint8
	status          CrazyflieStatus
	firstInit       sync.Once

	// communication loop
	disconnect    chan bool
	statusTimeout *time.Timer
	waitGroup     *sync.WaitGroup

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

func Connect(address uint64, channel uint8) (*Crazyflie, error) {
	cf := new(Crazyflie)

	cf.firmwareAddress = address // we save explicitly the firmware address and channel since a restart to bootloader will overwrite the current radio settings
	cf.firmwareChannel = channel

	err := cf.connect(address, channel)
	if err != nil {
		return nil, err
	}

	return cf, nil
}

func (cf *Crazyflie) connect(address uint64, channel uint8) error {
	cf.address = address
	cf.channel = channel
	cf.status = StatusDisconnected

	// initialize the structures required for communication and packet handling
	cf.communicationSystemInit()
	cf.consoleSystemInit()
	cf.logSystemInit()
	cf.paramSystemInit()

	return crazyradio.CrazyflieRegister(cf.channel, cf.address, cf.responseHandler)
}

func (cf *Crazyflie) Address() uint64 {
	return cf.address
}

func (cf *Crazyflie) FirmwareAddress() uint64 {
	return cf.firmwareAddress
}

func (cf *Crazyflie) Status() CrazyflieStatus {
	return cf.status
}

func (cf *Crazyflie) DisconnectImmediately() {
	// asynchronously (& non-blocking) stops the communications thread
	crazyradio.CrazyflieRemove(cf.channel, cf.address)
	close(cf.disconnect)
	cf.status = StatusDisconnected
}

func (cf *Crazyflie) DisconnectOnEmpty() {
	// asynchronously (& non-blocking) stops the communications thread
	cf.PacketQueueWaitForEmpty()
	cf.DisconnectImmediately()
}

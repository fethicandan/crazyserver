package crazyflie

import (
	"container/list"
	"sync"
	"time"

	"github.com/mikehamer/crazyserver/crtp"
	"github.com/mikehamer/crazyserver/crtpdevice"
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
	crtpDevice      crtpdevice.CrtpDevice
	status          CrazyflieStatus
	firstInit       sync.Once

	// communication loop
	disconnect    chan bool
	statusTimeout *time.Timer
	waitGroup     *sync.WaitGroup

	// callbacks for packet reception
	responseCallbacks map[crtp.Port](*list.List)

	// console printing
	accumulatedConsolePrint string

	// eeprom contents
	memoryContents []byte

	// log variables
	logCount       int
	logCRC         uint32
	logMaxPacket   uint8
	logMaxOps      uint8
	logNameToIndex map[string]logItem
	logIndexToName map[uint8]string
	logBlocks      map[uint8]logBlock

	// parameters
	paramCount       int
	paramCRC         uint32
	paramNameToIndex map[string]paramItem
	paramIndexToName map[uint8]string
}

func Connect(crtpDevice crtpdevice.CrtpDevice, channel uint8, address uint64) (*Crazyflie, error) {
	cf := &Crazyflie{
		crtpDevice:      crtpDevice,
		firmwareAddress: address, // we save explicitly the firmware address and channel since a restart to bootloader will overwrite the current radio settings
		firmwareChannel: channel,
	}

	err := cf.connect(channel, address)
	if err != nil {
		return nil, err
	}

	return cf, nil
}

// Crazyflie.connect handles connection to the crazyflie at a given address and channel
// Note that we split this functionality from the Connect function, since we can then reboot to bootloader
// (which has a different channel and address) without affecting or losing the firmware address, or having to
// create a new crazyflie object
func (cf *Crazyflie) connect(channel uint8, address uint64) error {
	cf.address = address
	cf.channel = channel
	cf.status = StatusDisconnected

	// initialize the structures required for communication and packet handling
	cf.communicationSystemInit()
	cf.consoleSystemInit()
	cf.logSystemInit()
	cf.paramSystemInit()
	cf.memSystemInit()

	// now we wait for something to happen...
	greedyResponse := &UtilityResponseGreedy{}
	responseErrorChannel, stopAwaiting := cf.PacketStartAwaiting(greedyResponse)
	defer stopAwaiting()

	cf.crtpDevice.ClientRegister(cf.channel, cf.address, cf.responseHandler)

	select {
	case err := <-responseErrorChannel:
		return err
	case <-time.After(DEFAULT_RESPONSE_TIMEOUT):
		return ErrorNoResponse
	}
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
	cf.crtpDevice.ClientRemove(cf.channel, cf.address)
	close(cf.disconnect)
	cf.status = StatusDisconnected
}

func (cf *Crazyflie) DisconnectOnEmpty() {
	// asynchronously (& non-blocking) stops the communications thread
	cf.PacketQueueWaitForEmpty()
	cf.DisconnectImmediately()
}

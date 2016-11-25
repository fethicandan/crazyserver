package crazyserver

import (
	"log"

	"fmt"

	"time"

	"github.com/mikehamer/crazyserver/crazyflie"
	"github.com/mikehamer/crazyserver/crazyradio"
)

var radio *crazyradio.RadioDevice
var crazyflies = map[uint64]*crazyflie.Crazyflie{}
var isStarted = false

func Start() error {
	var err error
	radio, err = crazyradio.Open()
	if err != nil {
		return err
	}

	isStarted = true
	return nil
}

func Stop() {
	for _, v := range crazyflies {
		v.Disconnect()
	}
	radio.Close()
}

func AddCrazyflie(address uint64) error {
	if !isStarted {
		Start()
	}

	// connect to the crazyflie
	cf, err := crazyflie.Connect(radio, address)
	if err != nil {
		log.Printf("Error adding crazyflie: %s", err)
		return err
	}

	// do other management stuff
	//...

	crazyflies[address] = cf
	return nil
}

func BeginLogging(address uint64, variables []string, period time.Duration) (int, error) {
	cf, ok := crazyflies[address]
	if !ok {
		return -1, fmt.Errorf("No crazyflie with address %X found", address) // TODO: replace with actual error
	}

	blockid, err := cf.LogBlockAdd(period, variables)
	if err != nil {
		return -1, err
	}

	err = cf.LogBlockStart(blockid)
	if err != nil {
		return -1, err
	}

	return blockid, nil
}

func StopLogging(address uint64, blockid int) error {
	cf, ok := crazyflies[address]
	if !ok {
		return fmt.Errorf("No crazyflie with address %X found", address) // TODO: replace with actual error
	}

	err := cf.LogBlockStop(blockid)
	if err != nil {
		return err
	}

	return nil
}

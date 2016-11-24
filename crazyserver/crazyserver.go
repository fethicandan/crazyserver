package crazyserver

import (
	"log"

	"github.com/mikehamer/crazyradio/crazyflie"
	"github.com/mikehamer/crazyradio/crazyradio"
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

	// get the log toc
	err = cf.GetLogTOCList()
	if err != nil {
		log.Printf("Error getting crazyflie TOC: %s", err)
		return err
	}

	// do other management stuff
	//...

	crazyflies[address] = cf
	return nil
}

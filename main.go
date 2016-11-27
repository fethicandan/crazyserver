package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/mikehamer/crazyserver/cache"
	"github.com/mikehamer/crazyserver/crazyflie"
	"github.com/mikehamer/crazyserver/crazyradio"
)

func main() {
	var err error
	flag.Parse()
	cache.Init()

	radio, err := crazyradio.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer radio.Close()

	cf, err := crazyflie.Connect(radio, 0xE7E7E7E701)
	if err != nil {
		log.Fatal(err)
	}
	defer cf.Disconnect()
	// log.Println("Rebooting")
	// cf.RebootToFirmware()
	// log.Println("Rebooted")

	// <-time.After(1 * time.Second)

	cf.LogSystemReset()
	err = cf.LogTOCGetList()
	if err != nil {
		log.Fatal(err)
	}

	err = cf.ParamTOCGetList()
	if err != nil {
		log.Fatal(err)
	}

	val, err := cf.ParamRead("kalman.pNAcc_xy")
	fmt.Println(val)
	err = cf.ParamWrite("kalman.pNAcc_xy", float32(3.14159))
	val, err = cf.ParamRead("kalman.pNAcc_xy")
	fmt.Println(val)

	cf.SetpointSend(0, 0, 0, 4000)
	<-time.After(1 * time.Second)
	cf.SetpointSend(0, 0, 0, 0)
	<-time.After(1 * time.Second)
}

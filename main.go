package main

import (
	"fmt"
	"log"
	"os"
	"time"

	"github.com/mikehamer/crazyserver/cache"
	"github.com/mikehamer/crazyserver/crazyflie"
	"github.com/mikehamer/crazyserver/crazyradio"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()

	app.Name = "crazyserver"
	app.Usage = "A cross-platform, install-less, dependency-less server for a fleet of Crazyflies"

	app.Commands = []cli.Command{
		{
			Name:  "test",
			Usage: "Run test codes, for development purposes",
			Flags: []cli.Flag{
				cli.UintFlag{
					Name:  "channel",
					Value: 80,
					Usage: "Set the radio channel",
				},
				cli.Uint64Flag{
					Name:  "address",
					Value: 0xE7E7E7E701,
					Usage: "Set the radio address",
				},
			},
			Action: testCommand,
		},
	}

	app.Run(os.Args)
}

func testCommand(context *cli.Context) error {
	var err error
	channel := context.Uint("channel")
	address := context.Uint64("address")
	cache.Init()

	radio, err := crazyradio.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer radio.Close()

	radio.SetChannel(uint8(channel))

	cf, err := crazyflie.Connect(radio, address)
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

	// Unlock commander
	cf.SetpointSend(0, 0, 0, 0)
	// Commander packets needs to be sent at regular interval, otherwise the
	// commander watchdog will cut the motors
	stop := false
	go func() {
		for !stop {
			cf.SetpointSend(0, 0, 0, 4000)
			<-time.After(20 * time.Millisecond)
		}
	}()
	<-time.After(5 * time.Second)
	stop = true
	<-time.After(40 * time.Millisecond)
	cf.SetpointSend(0, 0, 0, 0)
	<-time.After(1 * time.Second)

	return nil
}

package main

import (
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
	channel := uint8(context.Uint("channel"))
	address := context.Uint64("address")
	cache.Init()

	radio, err := crazyradio.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer radio.Close()

	cf, err := crazyflie.Connect(radio, address, channel)
	if err != nil {
		log.Fatal(err)
	}
	defer cf.Disconnect()

	<-time.After(3 * time.Second)

	log.Println("Rebooting to bootloader")
	addr, err := cf.RebootToBootloader()
	log.Printf("%X, %v", addr, err)
	log.Printf("Rebooted to %X", addr)

	<-time.After(3 * time.Second)

	log.Println("Rebooting to firmware")
	addr, err = cf.RebootToFirmware()
	log.Printf("%X, %v", addr, err)
	<-time.After(5 * time.Second)
	log.Printf("Rebooted to %X", addr)

	return nil
}

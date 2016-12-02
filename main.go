package main

import (
	"io/ioutil"
	"log"
	"os"

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

		{
			Name:  "flash",
			Usage: "Flashes a Crazyflie",
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
				cli.StringFlag{
					Name:  "image",
					Value: "cf2.bin",
					Usage: "Set the image file to flash",
				},
			},
			Action: flashCommand,
		},
	}

	app.Run(os.Args)
}

func testCommand(context *cli.Context) error {
	return nil
}

func flashCommand(context *cli.Context) error {
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
	defer cf.DisconnectImmediately()

	flashData, err := ioutil.ReadFile(context.String("image"))
	if err != nil {
		log.Fatal(err)
	}

	err = cf.ReflashSTM32(flashData, true)
	if err != nil {
		log.Fatal(err)
	}

	return nil
}

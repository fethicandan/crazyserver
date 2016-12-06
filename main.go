package main

import (
	"io/ioutil"
	"log"
	"os"

	"github.com/mikehamer/crazyserver/cache"
	"github.com/mikehamer/crazyserver/crazyflie"
	"github.com/mikehamer/crazyserver/crazyradio"
	"github.com/mikehamer/crazyserver/crazyserver"

	"strconv"
	"strings"

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
			Name:      "flash",
			Usage:     "Flashes a Crazyflie",
			ArgsUsage: "<image.bin> <target (stm32-fw or nrf51-fw)>",
			Flags: []cli.Flag{
				cli.UintFlag{
					Name:  "channel",
					Value: 80,
					Usage: "Set the radio channel",
				},
				cli.StringFlag{
					Name:  "address",
					Value: "E7E7E7E701",
					Usage: "Set the radio address",
				},
				cli.BoolFlag{
					Name:  "verify, v",
					Usage: "Verify flash content after programming",
				},
			},
			Action: flashCommand,
		},
		crazyserver.ServeCommand,
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
	defer cf.DisconnectImmediately()
	cf.LogTOCGetList()
	cf.ParamTOCGetList()
	return nil

}

func flashCommand(context *cli.Context) error {
	if len(context.Args()) != 2 {
		log.Fatal("You should provide image and target.")
	}
	imagePath := context.Args().Get(0)
	targetString := context.Args().Get(1)

	channel := uint8(context.Uint("channel"))
	addresses := strings.Split(context.String("address"), ",")

	// a set to hold the unique addresses that we need to flash
	addressSet := make(map[uint64]bool)
	// parse the address string, allowing for formatting
	for _, address := range addresses {
		addressrange := strings.Split(address, "-") // eg we handle the case E7E7E7E701-07, if there is no -, this should still work.

		lowaddressstring := strings.TrimPrefix(addressrange[0], "0x") // trim any leading hex prefix

		lowaddress, err := strconv.ParseUint(lowaddressstring, 16, 64)
		if err != nil {
			log.Printf("Error parsing address %s", lowaddressstring)
			continue
		}

		highaddresslowpart := strings.TrimPrefix(addressrange[len(addressrange)-1], "0x")          // eg 07
		highaddresshighpart := lowaddressstring[0 : len(lowaddressstring)-len(highaddresslowpart)] // eg E7E7E7E7 | 01
		highaddress, err := strconv.ParseUint(highaddresshighpart+highaddresslowpart, 16, 64)
		if err != nil {
			log.Printf("Error parsing address %s", highaddresshighpart+highaddresslowpart)
			continue
		}

		for i := lowaddress; i <= highaddress; i++ {
			addressSet[i] = true
		}
	}

	cache.Init()

	radio, err := crazyradio.Open()
	if err != nil {
		log.Fatal(err)
	}

	flashData, err := ioutil.ReadFile(imagePath)
	if err != nil {
		radio.Close()
		log.Fatal(err)
	}

	for address, _ := range addressSet {
		cf, err := crazyflie.Connect(radio, address, channel)
		if err != nil {
			log.Print(err)
			continue
		}

		switch targetString {
		case "stm32-fw":
			err = cf.ReflashSTM32(flashData, context.Bool("verify"))
			if err != nil {
				log.Print(err)
			}
		case "nrf51-fw":
			err = cf.ReflashNRF51(flashData, context.Bool("verify"))
			if err != nil {
				log.Print(err)
			}
		default:
			cf.DisconnectImmediately()
			radio.Close()
			log.Fatal("Target ", targetString, " Unknown!")
		}

		cf.DisconnectImmediately()
	}

	return nil
}

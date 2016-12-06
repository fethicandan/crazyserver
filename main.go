package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"

	pb "gopkg.in/cheggaaa/pb.v1"

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
					Value: 0,
					Usage: "Set the radio channel (default is bootloader channel: 0)",
				},
				cli.StringFlag{
					Name:  "address",
					Value: "0",
					Usage: "Set the radio address (default is bootloader address: 0)",
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

	// now convert the set into a slice for easier processing
	addressSlice := make([]uint64, len(addressSet))
	addressIdx := 0
	for k := range addressSet {
		addressSlice[addressIdx] = k
		addressIdx++
	}

	// Initalize the radio and cache
	radio, err := crazyradio.Open()
	if err != nil {
		log.Fatal(err)
	}
	cache.Init()

	// Read the flash data
	flashData, err := ioutil.ReadFile(imagePath)
	if err != nil {
		radio.Close()
		log.Fatal(err)
	}

	// Prepare to connect to multiple crazyflies for parallel flashing
	progressBars := make([]*pb.ProgressBar, 0, len(addressSlice))
	crazyflies := make([]*crazyflie.Crazyflie, 0, len(addressSlice))

	for _, address := range addressSlice {

		// connect to each crazyflie
		cf, err := crazyflie.Connect(radio, address, channel)
		if err != nil {
			log.Printf("Error connecting to 0x%X: %s", address, err)
			continue
		}
		crazyflies = append(crazyflies, cf)

		// for each successful connection, initiate a progress bar
		progressBar := pb.New(len(flashData)).Prefix(fmt.Sprintf("Flashing 0x%X", address))
		progressBar.ShowTimeLeft = true
		progressBar.SetUnits(pb.U_BYTES)
		progressBars = append(progressBars, progressBar)

	}

	// start all progress bars
	pool, err := pb.StartPool(progressBars...)

	// start the goroutines to flash
	wg := new(sync.WaitGroup)
	for idx := range crazyflies {
		wg.Add(1)

		progressChannel := make(chan int, 5)

		go func(i int) {
			for {
				progress, more := <-progressChannel
				if more {
					progressBars[i].Add(progress)
				} else {
					return
				}
			}
		}(idx)

		go func(i int) {
			switch targetString {
			case "stm32-fw":
				err = crazyflies[i].ReflashSTM32(flashData, context.Bool("verify"), progressChannel)
				if err != nil {
					progressBars[i].FinishPrint(fmt.Sprint(err))
				} else {
					progressBars[i].Finish()
				}
			case "nrf51-fw":
				err = crazyflies[i].ReflashNRF51(flashData, context.Bool("verify"), progressChannel)
				if err != nil {
					progressBars[i].FinishPrint(fmt.Sprint(err))
				} else {
					progressBars[i].Finish()
				}
			default:
				progressBars[i].FinishPrint(fmt.Sprint("Target ", targetString, " Unknown!"))
			}

			crazyflies[i].DisconnectImmediately()
			close(progressChannel)
			wg.Done()
		}(idx)
	}
	wg.Wait()
	pool.Stop()

	radio.Close()

	return nil
}

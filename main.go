package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"sync"
	"time"

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
			Name:  "test-connection",
			Usage: "Test whether a given Crazyflie is responding",
			Flags: []cli.Flag{
				cli.UintFlag{
					Name:  "channel",
					Value: 10,
					Usage: "Set the radio channel (default is channel: 10)",
				},
				cli.StringFlag{
					Name:  "address",
					Value: "E7E7E7E701",
					Usage: "Set the radio address (default is address: E7E7E7E701)",
				},
			},
			Action: testConnectionCommand,
		},

		{
			Name:  "test",
			Usage: "Run test codes, for development purposes",
			Flags: []cli.Flag{
				cli.UintFlag{
					Name:  "channel",
					Value: 10,
					Usage: "Set the radio channel (default is channel: 10)",
				},
				cli.Uint64Flag{
					Name:  "address",
					Value: 0xE7E7E7E701,
					Usage: "Set the radio address (default is address: E7E7E7E701)",
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
					Usage: "Set the radio address (default is bootloader address: 0).\nIt is also possible to enter a range of addresses, for example E7E7E7E701-03,E7E7E7E705 will flash Crazyflies 01,02,03 and 05.",
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

	// Initalize the radio and cache
	radio, err := crazyradio.Open()
	if err != nil {
		log.Fatalln(err)
	}
	defer radio.Close()

	cache.Init()

	app.Run(os.Args)
}

func testCommand(context *cli.Context) error {
	channel := uint8(context.Uint("channel"))
	address := context.Uint64("address")

	radio, err := crazyradio.Open()
	if err != nil {
		log.Println(err)
	}

	cf, err := crazyflie.Connect(radio, channel, address)
	if err != nil {
		log.Printf("Error1 (%d:0x%X): %s\n", channel, address, err)
		return err
	}
	err = cf.LogTOCGetList()
	if err != nil {
		log.Printf("Error2 (%d:0x%X): %s\n", channel, address, err)
		return err
	}
	if cf.Status() == crazyflie.StatusConnected {
		log.Printf("Success (%d:0x%X)\n", channel, address)
	} else {
		log.Printf("No Success (%d:0x%X)\n", channel, address)
	}

	return nil
}

func testConnectionCommand(context *cli.Context) error {
	radio, err := crazyradio.Open()
	if err != nil {
		log.Fatal(err)
	}

	// connect to each crazyflie
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

	// Prepare to connect to multiple crazyflies for parallel flashing
	for _, address := range addressSlice {
		fmt.Printf("0x%X: ", address)

		// connect to each crazyflie
		cf, err := crazyflie.Connect(radio, channel, address)
		if err != nil {
			fmt.Printf("Error (%s)\n", address, err)
			continue
		}
		err = cf.LogTOCGetList()
		if err != nil {
			fmt.Printf("Error (%s)\n", address, err)
			continue
		}
		fmt.Println("Success")
	}

	return nil
}

func flashCommand(context *cli.Context) error {

	radio, err := crazyradio.Open()
	if err != nil {
		log.Println(err)
		return err
	}

	channel := uint8(context.Uint("channel"))
	addresses := strings.Split(context.String("address"), ",")

	// enough arguments?
	if len(context.Args()) != 2 {
		log.Fatal("You should provide image and target.")
	}

	imagePath := context.Args().Get(0)
	targetString := context.Args().Get(1)

	// Check for valid firmware targets
	if targetString != "stm32-fw" && targetString != "nrf51-fw" {
		return fmt.Errorf("target %s unknown", targetString)
	}

	// Read the flash data
	flashData, err := ioutil.ReadFile(imagePath)
	if err != nil {
		return err
	}

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

	// Prepare to connect to multiple crazyflies for parallel flashing
	//progressBars := make([]*pb.ProgressBar, 0, len(addressSlice))
	progressChannels := make([]chan int, 0, len(addressSlice))
	crazyflies := make([]*crazyflie.Crazyflie, 0, len(addressSlice))

	for _, address := range addressSlice {

		// connect to each crazyflie
		cf, err := crazyflie.Connect(radio, channel, address)
		if err != nil {
			log.Printf("Error connecting to 0x%X: %s\n", address, err)
			continue
		}

		// store every crazyflie with a connection
		crazyflies = append(crazyflies, cf)

		// for each successful connection, initiate a progress bar
		//progressBar := pb.New(len(flashData)).Prefix(fmt.Sprintf("Flashing 0x%X", address))
		//progressBar.ShowTimeLeft = true
		//progressBar.SetUnits(pb.U_BYTES)
		//progressBars = append(progressBars, progressBar)

		// and initiate a progress channel
		progressChannel := make(chan int, 5)
		progressChannels = append(progressChannels, progressChannel)

		// now start a goroutine to update the bar!
		go func() {
			for {
				_, more := <-progressChannel
				if more {
					fmt.Print(".")
					//progressBar.Add(progress)
				} else {
					return
				}
			}
		}()
	}

	// start all progress bars
	//pool, err := pb.StartPool(progressBars...)

	// start the goroutines to flash
	wg := new(sync.WaitGroup)
	for idx := range crazyflies {
		wg.Add(1)

		go func(i int) {
			cf := crazyflies[i]
			//pb := progressBars[i]
			pc := progressChannels[i]

			switch targetString {
			case "stm32-fw":
				err = cf.ReflashSTM32(flashData, context.Bool("verify"), pc)
				if err != nil {
					log.Printf("0x%X: %s", cf.FirmwareAddress(), err)
				}
			case "nrf51-fw":
				err = cf.ReflashNRF51(flashData, context.Bool("verify"), pc)
				if err != nil {
					log.Printf("0x%X: %s", cf.FirmwareAddress(), err)
				}
			default:
			}

			//pb.Finish()
			cf.DisconnectImmediately()
			close(pc)
			wg.Done()
		}(idx)
	}
	wg.Wait()
	//pool.Stop()

	<-time.After(1 * time.Second)

	return nil
}

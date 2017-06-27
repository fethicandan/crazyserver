package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/mikehamer/crazyserver/crazyflie"
	"github.com/mikehamer/crazyserver/crazyradio"
	"github.com/urfave/cli"
)

func flashCommand(context *cli.Context) error {

	// Initalize the radio and cache
	radio, err := crazyradio.Open()
	if err != nil {
		log.Fatalln(err)
	}
	defer radio.Close()

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
					fmt.Print("\n")
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

		flashFunc := func(i int) {
			cf := crazyflies[i]
			//pb := progressBars[i]
			pc := progressChannels[i]

			fmt.Printf("0x%X: ", cf.FirmwareAddress())
			switch targetString {
			case "stm32-fw":
				err = cf.ReflashSTM32(flashData, context.Bool("verify"), pc)
				if err != nil {
					fmt.Printf("\nerr: %s", err)
				}
			case "nrf51-fw":
				err = cf.ReflashNRF51(flashData, context.Bool("verify"), pc)
				if err != nil {
					fmt.Printf("\nerr: %s", err)
				}
			default:
			}

			//pb.Finish()
			cf.DisconnectImmediately()
			close(pc)
			wg.Done()
		}

		flashFunc(idx)
		//go flashFunc(idx)
	}
	wg.Wait()
	//pool.Stop()

	<-time.After(1 * time.Second)

	return nil
}

package main

import (
	"fmt"
	"log"
	"strconv"
	"strings"

	"time"

	"github.com/mikehamer/crazyserver/crazyflie"
	"github.com/mikehamer/crazyserver/crazyusb"
	"github.com/urfave/cli"
)

func commandLineInput(channelChan chan uint8, addressChan chan uint64) {
	var command byte
	var test string

	for {
		n, err := fmt.Scanf("%c %s", &command, &test)
		if err != nil || n != 2 {
			fmt.Println("Error: Incorrect format. Expecting e.g. \"c 10\" or \"a e7e7e7e701\"")
			continue
		}

		// changing channel
		if command == 'C' || command == 'c' {
			channel, err := strconv.ParseUint(test, 10, 8)
			if err != nil {
				fmt.Println(err)
				continue
			}
			channelChan <- uint8(channel)
		}

		if command == 'A' || command == 'a' {
			address, err := strconv.ParseUint(test, 16, 64)
			if err != nil {
				log.Fatal(err)
			}
			addressChan <- address
		}
	}
}

func configureCommand(context *cli.Context) error {

	// connect to each crazyflie
	channel := uint8(context.Uint("channel"))
	addressString := context.String("address")

	address, err := strconv.ParseUint(strings.TrimPrefix(addressString, "0x"), 16, 64) // trim any leading hex prefix
	if err != nil {
		log.Fatal(err)
	}

	//signalChan := make(chan os.Signal, 1)
	//signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	channelChan := make(chan uint8, 1)
	addressChan := make(chan uint64, 1)
	go commandLineInput(channelChan, addressChan)

	ticker := time.NewTicker(200 * time.Millisecond)

	fmt.Printf("Waiting for %d:0x%X... (Set channel: c %%d | Set address: a %%X | Exit: Ctrl-C)\n", channel, address)

	for {
		select {
		case channel = <-channelChan:
			break
		case address = <-addressChan:
			break
		case <-ticker.C:
			if crazyusb.CountConnectedCrazyflies() != 1 {
				continue
			}

			usb, err := crazyusb.Open()
			if err != nil {
				log.Fatalln(err)
			}

			cf, err := crazyflie.Connect(usb, channel, address)
			if err != nil {
				usb.Close()
				log.Fatalln(err)
			}

			fmt.Println("... Connected")
			cf.MemReadContents()
			cf.MemCommitSpeed(2)
			cf.MemCommitAddress(address)
			cf.MemCommitChannel(channel)
			cf.MemPushCommits()

			ch, addr, err := cf.MemReadChannelAddress()
			if err != nil {
				usb.Close()
				log.Fatalln(err)
			}

			cf.DisconnectOnEmpty()
			usb.Close()
			fmt.Printf("... Wrote configuration %d:0x%X\n", ch, addr)
			fmt.Println("... Please disconnect and RESTART the crazyflie\n")
			crazyusb.WaitForCrazyflieDisconnect()

			address += 1
		}

		fmt.Printf("Waiting for %d:0x%X... (Set channel: c %%d | Set address: a %%X | Exit: Ctrl-C)\n", channel, address)
	}
}

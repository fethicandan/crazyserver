package main

import (
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/mikehamer/crazyserver/crazyflie"
	"github.com/mikehamer/crazyserver/crazyradio"
	"github.com/urfave/cli"
)

func testCommand(context *cli.Context) error {

	// Initalize the radio and cache
	radio, err := crazyradio.Open()
	if err != nil {
		log.Fatalln(err)
	}
	defer radio.Close()

	channel := uint8(context.Uint("channel"))
	address, err := strconv.ParseUint(strings.TrimPrefix(context.String("address"), "0x"), 16, 64)
	if err != nil {
		log.Fatal(err)
	}

	// connect to the crazyflie
	cf, err := crazyflie.Connect(radio, channel, address)
	if err != nil {
		log.Fatalf("Error connecting to 0x%X: %s\n", address, err)
	}

	err = cf.DNNSetpointSet(0, 0.1, 0.2, 0.3, 1.1, 1.2, 1.3, 2.1, 2.2, 2.3)
	if err != nil {
		log.Fatal(err)
	}

	err = cf.DNNStartTrajectory()
	if err != nil {
		log.Fatal(err)
	}

	<-time.After(1 * time.Second)

	response, err := cf.DNNStateRequest(0)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(response)

	<-time.After(1 * time.Second)

	cf.DisconnectImmediately()

	return nil
}

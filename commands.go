package main

import (
	"github.com/mikehamer/crazyserver/crazyserver"
	"github.com/urfave/cli"
)

var COMMANDS = []cli.Command{
	{
		Name:  "configure",
		Usage: "Configure a Crazyflie's channel and address",
		Flags: []cli.Flag{
			cli.UintFlag{
				Name:  "channel",
				Value: 10,
				Usage: "Set the radio channel (default is channel: 10)",
			},
			cli.StringFlag{
				Name:  "address",
				Value: "0xE7E7E7E701",
				Usage: "Set the radio address (default is address: E7E7E7E701)",
			},
		},
		Action: configureCommand,
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

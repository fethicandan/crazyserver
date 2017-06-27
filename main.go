package main

import (
	"os"

	"github.com/mikehamer/crazyserver/cache"

	"github.com/urfave/cli"
)

func main() {
	app := cli.NewApp()

	app.Name = "crazyserver"
	app.Usage = "A cross-platform, install-less, dependency-less server for a fleet of Crazyflies"

	app.Commands = COMMANDS

	cache.Init()

	app.Run(os.Args)
}

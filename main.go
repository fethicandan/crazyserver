package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/mikehamer/crazyserver/crazyserver"
)

func main() {
	flag.Parse()

	err := crazyserver.Start()
	if err != nil {
		log.Fatalln(err)
	}
	defer crazyserver.Stop()
	fmt.Println("Started Server")

	err = crazyserver.AddCrazyflie(0xE7E7E7E701)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Added Crazyflie")
}

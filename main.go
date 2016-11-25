package main

import (
	"flag"
	"fmt"
	"log"
	"time"

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

	<-time.After(2 * time.Second)

	blockid, err := crazyserver.BeginLogging(0xE7E7E7E701, []string{"stabilizer.pitch", "pm.vbat"}, 100*time.Millisecond)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Started logging")

	<-time.After(2 * time.Second)

	err = crazyserver.StopLogging(0xE7E7E7E701, blockid)
	if err != nil {
		log.Fatalln(err)
	}

	fmt.Println("Stopped logging")
}

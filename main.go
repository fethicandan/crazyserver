package main

import (
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/mikehamer/crazyserver/cache"
	"github.com/mikehamer/crazyserver/crazyserver"
)

func main() {
	flag.Parse()
	cache.Init()

	err := crazyserver.Start()
	if err != nil {
		log.Fatalln(err)
	}
	defer crazyserver.Stop()
	fmt.Println("Started Server")

	cf, err := crazyserver.AddCrazyflie(0xE7E7E7E701)
	if err != nil {
		log.Fatalln(err)
	}
	fmt.Println("Added Crazyflie")

	val, err := cf.ParamRead("kalman.pNAcc_xy")
	fmt.Println(val)
	err = cf.ParamWrite("kalman.pNAcc_xy", float32(3.14159))
	val, err = cf.ParamRead("kalman.pNAcc_xy")
	fmt.Println(val)

	cf.SetpointSend(0, 0, 0, 4000)
	<-time.After(2 * time.Second)
	cf.SetpointSend(0, 0, 0, 0)
	<-time.After(1 * time.Second)
}

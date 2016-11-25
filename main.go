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

	id, _ := cf.LogBlockAdd(100*time.Millisecond, []string{"stabilizer.pitch", "pm.vbat"})
	cf.LogBlockStart(id)
	<-time.After(2 * time.Second)
	cf.LogBlockStop(id)
}

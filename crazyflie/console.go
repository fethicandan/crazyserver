package crazyflie

import (
	"log"
	"strings"
)

func (cf *Crazyflie) handleConsoleResponse(resp []byte) {
	cf.lastUpdate = 0

	str := string(resp[1:])
	for {
		i := strings.Index(str, "\n")
		if i == -1 {
			cf.accumulatedConsolePrint = cf.accumulatedConsolePrint + str
			break
		} else {
			log.Printf("%X: %s%s", cf.address, cf.accumulatedConsolePrint, str[0:i])
			str = str[i+1:]
			cf.accumulatedConsolePrint = ""
		}
	}
}

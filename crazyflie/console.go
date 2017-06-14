package crazyflie

import (
	"log"
	"strings"

	"github.com/mikehamer/crazyserver/crtp"
)

func (cf *Crazyflie) consoleSystemInit() {
	cf.responseCallbacks[crtp.PortConsole].PushBack(cf.handleConsoleResponse)
}

func (cf *Crazyflie) handleConsoleResponse(resp []byte) {
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

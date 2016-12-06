package crazyflie

import "strings"

func (cf *Crazyflie) consoleSystemInit() {
	cf.responseCallbacks[crtpPortConsole].PushBack(cf.handleConsoleResponse)
}

func (cf *Crazyflie) handleConsoleResponse(resp []byte) {
	cf.lastUpdate = 0

	str := string(resp[1:])
	for {
		i := strings.Index(str, "\n")
		if i == -1 {
			cf.accumulatedConsolePrint = cf.accumulatedConsolePrint + str
			break
		} else {
			// log.Printf("%X: %s%s", cf.address, cf.accumulatedConsolePrint, str[0:i])
			str = str[i+1:]
			cf.accumulatedConsolePrint = ""
		}
	}
}

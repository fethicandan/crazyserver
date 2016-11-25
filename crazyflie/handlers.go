package crazyflie

import (
	"log"
	"time"
)

const minCommunicationPeriod_ms = 5    // milliseconds
const maxCommunicationPeriod_ms = 1000 // milliseconds
var defaultPacket = []byte{0xFF}       // a ping

func (cf *Crazyflie) communicationLoop() {

	// begin transmitting quickly
	cf.lastUpdate = 0

	minPeriod := time.NewTimer(time.Duration(minCommunicationPeriod_ms) * time.Millisecond)

	for {
		var err error
		var packet []byte

		if cf.lastUpdate < 10 {
			// if we are communicating, keep communicating quickly
			cf.period = minCommunicationPeriod_ms
		} else {
			// otherwise begin exponential slowing
			cf.period *= 2
			if cf.period > maxCommunicationPeriod_ms {
				cf.period = maxCommunicationPeriod_ms
			}
		}

		// wait for one at least one minimum period so we don't spam the CF
		<-minPeriod.C

		// then wait for the rest of the period, or until a packet is received
		select {
		case <-cf.disconnect: // if we should disconnect
			return
		case packet = <-cf.commandQueue: // if a packet is scheduled
			cf.lastUpdate = 0
		case <-time.After(time.Duration(cf.period-minCommunicationPeriod_ms) * time.Millisecond):
			packet = defaultPacket // if the timeout occurs send a ping
			cf.lastUpdate++
		}

		// reset the timer such that the loop runs at the correct maximum frequency irrespective of the processing time below
		minPeriod.Stop()
		minPeriod.Reset(minCommunicationPeriod_ms)

		// we lock the radio so it has the correct address for the whole transaction
		cf.radio.Lock()

		err = cf.radio.SetAddress(cf.address)
		if err != nil {
			cf.radio.Unlock()
			log.Printf("%X error: %s", cf.address, err)
			cf.lastUpdate++
			continue
		}

		err = cf.radio.SendPacket(packet)
		if err != nil {
			cf.radio.Unlock()
			log.Printf("%X error: %s", cf.address, err)
			cf.lastUpdate++
			continue
		}

		// read the response, which we then distribute to the relevant handler
		responseReceived, resp, err := cf.radio.ReadResponse()
		cf.radio.Unlock() // want to unlock the radio ASAP such that other crazyflies can take it

		if err != nil {
			log.Printf("%X error: %s", cf.address, err)
			cf.lastUpdate++
			continue
		}

		if !responseReceived || len(resp) < 1 {
			cf.lastUpdate++ // if there is no response, something is wrong... indicate we can transmit at a lower frequency
		} else {
			header := crtpHeader(resp[0])

			if header.port() == 0xF3 || header.port() == 0xF7 {
				cf.lastUpdate++ // CF has nothing to report, indicate we can transmit at a lower frequency
			} else {
				cf.lastUpdate = 0 // wants to tell us something, so we communicate with a max frequency again
			}

			// call any registered callbacks for this port
			for e := cf.responseCallbacks[header.port()].Front(); e != nil; e = e.Next() {
				f := e.Value.(func(r []byte))
				f(resp)
			}

			for e := cf.responseCallbacks[crtpPortGreedy].Front(); e != nil; e = e.Next() {
				f := e.Value.(func(r []byte))
				f(resp)
			}

		}
	}
}

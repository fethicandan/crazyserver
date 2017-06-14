package crazyflie

import "github.com/mikehamer/crazyserver/crtp"

// ---- UTILITY RESPONSE: GREEDY! ----
type UtilityResponseGreedy struct {
	Data []byte
}

func (p *UtilityResponseGreedy) Port() crtp.Port {
	return crtp.PortGreedy
}

func (p *UtilityResponseGreedy) Channel() crtp.Channel {
	return 0
}

func (p *UtilityResponseGreedy) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	p.Data = b
	return nil
}

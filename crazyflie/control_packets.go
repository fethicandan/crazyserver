package crazyflie

import "github.com/mikehamer/crazyserver/crtp"

// ---- CONTROL REQUEST: LEGACY SETPOINT ----
type ControlRequestLegacySetpoint struct {
	Roll, Pitch, Yawrate float32
	Thrust               uint16
}

func (p *ControlRequestLegacySetpoint) Port() crtp.Port {
	return crtp.PortSetpoint
}

func (p *ControlRequestLegacySetpoint) Channel() crtp.Channel {
	return 0
}

func (p *ControlRequestLegacySetpoint) Bytes() []byte {
	packet := make([]byte, 14)
	copy(packet[0:4], float32ToBytes(p.Roll))
	copy(packet[4:8], float32ToBytes(p.Pitch))
	copy(packet[8:12], float32ToBytes(p.Yawrate))
	copy(packet[12:14], uint16ToBytes(p.Thrust))
	return packet
}

// ---- CONTROL REQUEST: EXTERNAL POSITION ----
type ControlRequestExternalPosition struct {
	X, Y, Z float32
}

func (p *ControlRequestExternalPosition) Port() crtp.Port {
	return crtp.PortPosition
}

func (p *ControlRequestExternalPosition) Channel() crtp.Channel {
	return 0
}

func (p *ControlRequestExternalPosition) Bytes() []byte {
	packet := make([]byte, 12)
	copy(packet[0:4], float32ToBytes(p.X))
	copy(packet[4:8], float32ToBytes(p.Y))
	copy(packet[8:12], float32ToBytes(p.Z))
	return packet
}

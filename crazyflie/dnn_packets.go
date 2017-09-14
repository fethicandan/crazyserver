package crazyflie

import (
	"log"

	"github.com/mikehamer/crazyserver/crtp"
)

const GRAVITY float32 = 9.81
const POSITION_MAX = 6.0
const POSITION_MIN = -1.0
const VELOCITY_MAX = 10.0
const VELOCITY_MIN = -10.0
const ACCELERATION_MAX = 3.0 * GRAVITY
const ACCELERATION_MIN = -1.0 * GRAVITY
const Q_MAX = 1.0
const Q_MIN = -1.0

const (
	DNNChannelSet crtp.Channel = iota
	DNNChannelGet
	DNNChannelStart
	DNNChannelDone
)

// ---- DNN REQUEST: SETPOINT ----
type DNNRequestSetpoint struct {
	idx                             uint16
	x, y, z, vx, vy, vz, ax, ay, az float32
}

func (p *DNNRequestSetpoint) Port() crtp.Port {
	return crtp.PortDNN
}

func (p *DNNRequestSetpoint) Channel() crtp.Channel {
	return DNNChannelSet
}

func toConstrainedBytes(v, minVal, maxVal float32, n uint) []byte {
	var mask uint32 = 0
	mask = ^((^mask) << (8 * n))

	if v < minVal {
		log.Fatalf("v=%.3f < min=%.3f", v, minVal)
	}

	if v > maxVal {
		log.Fatalf("v=%.3f < min=%.3f", v, minVal)
	}

	scaled := uint32(float32(mask) * (v - minVal) / (maxVal - minVal))
	if scaled > mask {
		scaled = mask
	}

	b := make([]byte, n)
	copy(b, uint32ToBytes(scaled)[0:n])
	return b
}

func (p *DNNRequestSetpoint) Bytes() []byte {
	packet := make([]byte, 29)
	copy(packet[0:2], uint16ToBytes(p.idx))

	copy(packet[2:5], toConstrainedBytes(p.x, POSITION_MIN, POSITION_MAX, 3))
	copy(packet[5:8], toConstrainedBytes(p.y, POSITION_MIN, POSITION_MAX, 3))
	copy(packet[8:11], toConstrainedBytes(p.z, POSITION_MIN, POSITION_MAX, 3))

	copy(packet[11:14], toConstrainedBytes(p.vx, VELOCITY_MIN, VELOCITY_MAX, 3))
	copy(packet[14:17], toConstrainedBytes(p.vy, VELOCITY_MIN, VELOCITY_MAX, 3))
	copy(packet[17:20], toConstrainedBytes(p.vz, VELOCITY_MIN, VELOCITY_MAX, 3))

	copy(packet[20:23], toConstrainedBytes(p.ax, ACCELERATION_MIN, ACCELERATION_MAX, 3))
	copy(packet[23:26], toConstrainedBytes(p.ay, ACCELERATION_MIN, ACCELERATION_MAX, 3))
	copy(packet[26:29], toConstrainedBytes(p.az, ACCELERATION_MIN, ACCELERATION_MAX, 3))

	return packet
}

// ---- DNN RESPONSE: SETPOINT SET ----
type DNNResponseSetpoint struct {
	Idx uint16
}

func (p *DNNResponseSetpoint) Port() crtp.Port {
	return crtp.PortDNN
}

func (p *DNNResponseSetpoint) Channel() crtp.Channel {
	return DNNChannelSet
}

func (p *DNNResponseSetpoint) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if len(b) < 3 {
		return crtp.ErrorPacketIncorrectType
	}

	idx, ok := bytesToUint16(b[1:3]).(uint16)
	if !ok || idx != p.Idx { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	return nil
}

// ---- DNN REQUEST: STATE DOWNLOAD ----
type DNNRequestState struct {
	Idx uint16
}

func (p *DNNRequestState) Port() crtp.Port {
	return crtp.PortDNN
}

func (p *DNNRequestState) Channel() crtp.Channel {
	return DNNChannelGet
}

func (p *DNNRequestState) Bytes() []byte {
	b := make([]byte, 2)
	copy(b, uint16ToBytes(p.Idx))
	return b
}

// ---- DNN RESPONSE: STATE DOWNLOAD ----
type DNNResponseState struct {
	Idx            uint16
	X, Y, Z        float32
	VX, VY, VZ     float32
	AX, AY, AZ     float32
	QW, QX, QY, QZ float32
}

func (p *DNNResponseState) Port() crtp.Port {
	return crtp.PortDNN
}

func (p *DNNResponseState) Channel() crtp.Channel {
	return DNNChannelGet
}

func fromConstrainedBytes(b []byte, minVal, maxVal float32) float32 {
	n := len(b)

	var mask uint32 = 0
	mask = ^((^mask) << (8 * uint(n)))

	var v uint32 = 0
	for i := 0; i < n; i++ {
		v = (v << 8) + uint32(b[n-i-1])
	}

	return float32(v)/float32(mask)*(maxVal-minVal) + minVal
}

func (p *DNNResponseState) LoadFromBytes(b []byte) error { // b[0] is the CRTP Header, but packets are only passed to this function if this header matches the packet's Port() and Channel()
	if len(b) < 3 {
		return crtp.ErrorPacketIncorrectType
	}

	idx, ok := bytesToUint16(b[1:3]).(uint16)
	if !ok || idx != p.Idx { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	// now we can reference from 0
	b = b[3:]

	p.X = fromConstrainedBytes(b[0:2], POSITION_MIN, POSITION_MAX)
	p.Y = fromConstrainedBytes(b[2:4], POSITION_MIN, POSITION_MAX)
	p.Z = fromConstrainedBytes(b[4:6], POSITION_MIN, POSITION_MAX)

	p.VX = fromConstrainedBytes(b[6:8], VELOCITY_MIN, VELOCITY_MAX)
	p.VY = fromConstrainedBytes(b[8:10], VELOCITY_MIN, VELOCITY_MAX)
	p.VZ = fromConstrainedBytes(b[10:12], VELOCITY_MIN, VELOCITY_MAX)

	p.AX = fromConstrainedBytes(b[12:14], ACCELERATION_MIN, ACCELERATION_MAX)
	p.AY = fromConstrainedBytes(b[14:16], ACCELERATION_MIN, ACCELERATION_MAX)
	p.AZ = fromConstrainedBytes(b[16:18], ACCELERATION_MIN, ACCELERATION_MAX)

	p.QW = fromConstrainedBytes(b[18:20], Q_MIN, Q_MAX)
	p.QX = fromConstrainedBytes(b[20:22], Q_MIN, Q_MAX)
	p.QY = fromConstrainedBytes(b[22:24], Q_MIN, Q_MAX)
	p.QZ = fromConstrainedBytes(b[24:26], Q_MIN, Q_MAX)

	return nil
}

// ---- DNN REQUEST: START ----
type DNNRequestStart struct{}

func (p *DNNRequestStart) Port() crtp.Port {
	return crtp.PortDNN
}

func (p *DNNRequestStart) Channel() crtp.Channel {
	return DNNChannelStart
}

func (p *DNNRequestStart) Bytes() []byte {
	return []byte{}
}

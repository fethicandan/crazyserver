package crazyflie

type crtpHeader byte
type crtpPort byte

const (
	crtpPortConsole  crtpPort = 0x00
	crtpPortParam             = 0x02
	crtpPortSetpoint          = 0x03
	crtpPortMem               = 0x04
	crtpPortLog               = 0x05
	crtpPortPosition          = 0x06
	crtpPortPlatform          = 0x0D
	crtpPortLink              = 0x0F
	crtpPortEmpty1            = 0xF3
	crtpPortEmpty2            = 0xF7
	crtpPortGreedy            = 0xFF
)

func crtp(port crtpPort, channel byte) byte {
	var link byte = 3
	return ((byte(port) & 0x0F) << 4) |
		((link & 0x03) << 2) |
		((channel & 0x03) << 0)
}

func (header crtpHeader) channel() byte {
	return (byte(header) >> 0) & 0x03
}

func (header crtpHeader) port() crtpPort {
	return crtpPort((byte(header) >> 4) & 0x0F)
}

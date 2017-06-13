package crtp

const (
	PortConsole  Port = 0x00
	PortParam         = 0x02
	PortSetpoint      = 0x03
	PortMem           = 0x04
	PortLog           = 0x05
	PortPosition      = 0x06
	PortPlatform      = 0x0D
	PortLink          = 0x0F
	PortEmpty1        = 0xF3 // This is not an actual CRTP ports (since ports are 4 bit). This indicates an empty queue on the CF.
	PortEmpty2        = 0xF7 // This is not an actual CRTP ports (since ports are 4 bit). This indicates an empty queue on the CF.
	PortGreedy        = 0xFF // This is not an actual CRTP ports (since ports are 4 bit). Anything that listens on this port will receive every response.
)

type Header byte
type Port byte
type Channel byte
type Speed byte

func HeaderBytes(port Port, channel Channel) byte {
	var link byte = 3
	return ((byte(port) & 0x0F) << 4) |
		((link & 0x03) << 2) |
		((byte(channel) & 0x03) << 0)
}

func (header Header) Channel() Channel {
	return Channel((byte(header) >> 0) & 0x03)
}

func (header Header) Port() Port {
	return Port((byte(header) >> 4) & 0x0F)
}

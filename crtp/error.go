package crtp

import "fmt"

type crtpError uint8

func (e crtpError) Error() string {
	return fmt.Sprintf("crtp: %s", crtpErrorString[e])
}

const (
	ErrorPacketIncorrectType crtpError = iota
)

var crtpErrorString = map[crtpError]string{
	ErrorPacketIncorrectType: "Cannot decode packet from bytes: incorrect format",
}

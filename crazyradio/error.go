package crazyradio

import "fmt"

type radioError uint8

func (e radioError) Error() string {
	return fmt.Sprintf("crazyradio: %s", radioErrorString[e])
}

const (
	ErrorDeviceNotFound radioError = iota
	ErrorInvalidChannel
	ErrorInvalidDatarate
	ErrorInvalidPower
	ErrorInvalidArc
	ErrorInvalidArdTime
	ErrorInvalidArdBytes
	ErrorWriteLength
)

var radioErrorString = map[radioError]string{
	ErrorDeviceNotFound:  "device not found",
	ErrorInvalidChannel:  "invalid channel",
	ErrorInvalidDatarate: "invalid datarate",
	ErrorInvalidPower:    "invalid power",
	ErrorInvalidArc:      "invalid ARC",
	ErrorInvalidArdTime:  "invalid ARD time",
	ErrorInvalidArdBytes: "invalid ARD bytes",
	ErrorWriteLength:     "incorrect number of bytes written to endpoint",
}

package crazyflie

import "fmt"

type crazyflieError uint8

func (e crazyflieError) Error() string {
	return fmt.Sprintf("crazyflie: %s", radioErrorString[e])
}

const (
	ErrorNoResponse crazyflieError = iota
	ErrorLogItemNotFound
	ErrorLogBlockNoMemory
	ErrorLogBlockTooLong

	ErrorUnknown
)

var radioErrorString = map[crazyflieError]string{
	ErrorNoResponse:       "not responding",
	ErrorLogItemNotFound:  "requesting log block for unknown log entry",
	ErrorLogBlockNoMemory: "no memory to allocated log block",
	ErrorLogBlockTooLong:  "log block is too long",

	ErrorUnknown: "an unknown error occurred",
}

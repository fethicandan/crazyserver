package crazyflie

import "fmt"

type crazyflieError uint8

func (e crazyflieError) Error() string {
	return fmt.Sprintf("crazyflie: %s", radioErrorString[e])
}

const (
	ErrorNoResponse crazyflieError = iota
	ErrorLogBlockOrItemNotFound
	ErrorLogBlockNoMemory
	ErrorLogBlockTooLong
	ErrorLogBlockPeriodTooShort

	ErrorUnknown
)

var radioErrorString = map[crazyflieError]string{
	ErrorNoResponse:             "not responding",
	ErrorLogBlockOrItemNotFound: "log block or item not found",
	ErrorLogBlockNoMemory:       "no memory to allocated log block",
	ErrorLogBlockTooLong:        "log block is too long",
	ErrorLogBlockPeriodTooShort: "log block reporting period too short",

	ErrorUnknown: "an unknown error occurred",
}

package crazyflie

import "fmt"

type crazyflieError uint8

func (e crazyflieError) Error() string {
	return fmt.Sprintf("crazyflie: %s", crazyflieErrorString[e])
}

const (
	ErrorNoResponse crazyflieError = iota

	ErrorLogBlockOrItemNotFound
	ErrorLogBlockNoMemory
	ErrorLogBlockTooLong
	ErrorLogBlockPeriodTooShort

	ErrorParamNotFound

	ErrorFlashDataTooLarge

	ErrorMemLengthTooLarge
	ErrorMemReadFailed
	ErrorMemWriteFailed
	ErrorMemReadChecksum

	ErrorUnknown
)

var crazyflieErrorString = map[crazyflieError]string{
	ErrorNoResponse:             "not responding",
	ErrorLogBlockOrItemNotFound: "log block or item not found",
	ErrorLogBlockNoMemory:       "no memory to allocated log block",
	ErrorLogBlockTooLong:        "log block is too long",
	ErrorLogBlockPeriodTooShort: "log block reporting period too short",

	ErrorParamNotFound: "parameter not found",

	ErrorFlashDataTooLarge: "image is too large for flash",

	ErrorMemLengthTooLarge: "trying to read/write too many bytes",
	ErrorMemReadFailed:     "memory read returned a non-zero status",
	ErrorMemWriteFailed:    "memory write failed",
	ErrorMemReadChecksum:   "memory read failed with incorrect checksum after a number of attempts",

	ErrorUnknown: "an unknown error occurred",
}

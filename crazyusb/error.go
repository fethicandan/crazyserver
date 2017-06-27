package crazyusb

import "fmt"

type crtpUsbError uint8

func (e crtpUsbError) Error() string {
	return fmt.Sprintf("crazyradio: %s", crtpUsbErrorString[e])
}

const (
	ErrorDeviceNotFound crtpUsbError = iota
	ErrorMultipleDevicesFound
	ErrorDeviceAlreadyOpen
	ErrorNoResponse
	ErrorWriteLength
)

var crtpUsbErrorString = map[crtpUsbError]string{
	ErrorDeviceNotFound:       "device not found",
	ErrorMultipleDevicesFound: "multiple crazyflies found",
	ErrorDeviceAlreadyOpen:    "device has already been opened",
	ErrorNoResponse:           "no response from crazyflie",
	ErrorWriteLength:          "incorrect number of bytes written to endpoint",
}

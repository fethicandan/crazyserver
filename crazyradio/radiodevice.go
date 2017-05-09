package crazyradio

import (
	"sync"

	"time"

	"github.com/kylelemons/gousb/usb"
)

type RadioDevice struct {
	device  *usb.Device
	lock    *sync.Mutex
	dataOut usb.Endpoint
	dataIn  usb.Endpoint
	address uint64
}

var usbContext *usb.Context

func OpenRadio(dev *usb.Device) (*RadioDevice, error) {
	// open the endpoint for transfers out
	dOut, err := dev.OpenEndpoint(1, 0, 0, 0x01)

	if err != nil {
		dev.Close()
		return nil, err
	}

	// open the endpoint for transfers in
	dIn, err := dev.OpenEndpoint(1, 0, 0, 0x81)

	if err != nil {
		dev.Close()
		return nil, err
	}

	dev.ControlTimeout = 250 * time.Millisecond
	dev.ReadTimeout = 50 * time.Millisecond
	dev.WriteTimeout = 50 * time.Millisecond

	// now have a usb device and context pointing to the Radio!
	radio := new(RadioDevice)
	radio.lock = new(sync.Mutex)
	radio.device = dev
	radio.dataOut = dOut
	radio.dataIn = dIn

	// can initialize the default states!
	radio.SetDatarate(RadioDatarate_2MPS)
	radio.SetChannel(80)
	radio.SetAddress(0xE7E7E7E7E7)
	radio.SetPower(RadioPower_0DBM)
	radio.SetArc(3)
	radio.SetArdBytes(32)
	return radio, nil
}

func OpenAllRadios() ([]*RadioDevice, error) {
	usbContext := usb.NewContext()
	usbContext.Debug(0)

	radioDevices, _ := usbContext.ListDevices(
		func(desc *usb.Descriptor) bool {
			if desc.Vendor == 0x1915 && desc.Product == 0x7777 {
				return true
			}
			return false
		})

	if len(radioDevices) == 0 {
		usbContext.Close()
		return nil, ErrorDeviceNotFound
	}

	radios := make([]*RadioDevice, 0, len(radioDevices))

	for _, radioDevice := range radioDevices {
		radio, err := OpenRadio(radioDevice)
		if err == nil {
			radios = append(radios, radio)
		}
	}

	if len(radios) == 0 {
		usbContext.Close()
		return nil, ErrorDeviceNotFound
	}

	return radios, nil
}

func (radio *RadioDevice) Close() {
	radio.device.Close()
}

func (radio *RadioDevice) Lock() {
	radio.lock.Lock()
}

func (radio *RadioDevice) Unlock() {
	radio.lock.Unlock()
}

func (radio *RadioDevice) SetChannel(channel uint8) error {
	if channel > 125 {
		return ErrorInvalidChannel
	}

	_, err := radio.device.Control(usb.REQUEST_TYPE_VENDOR, uint8(SET_RADIO_CHANNEL), uint16(channel), 0, nil)
	return err
}

func (radio *RadioDevice) SetDatarate(datarate radioDatarate) error {
	if datarate > RadioDatarate_2MPS {
		return ErrorInvalidDatarate
	}

	_, err := radio.device.Control(usb.REQUEST_TYPE_VENDOR, uint8(SET_DATA_RATE), uint16(datarate), 0, nil)
	return err
}

func (radio *RadioDevice) SetPower(power radioPower) error {
	if power > RadioPower_0DBM {
		return ErrorInvalidPower
	}

	_, err := radio.device.Control(usb.REQUEST_TYPE_VENDOR, uint8(SET_RADIO_POWER), uint16(power), 0, nil)
	return err
}

func (radio *RadioDevice) SetArc(arc uint8) error {
	if arc > 15 {
		return ErrorInvalidArc
	}

	_, err := radio.device.Control(usb.REQUEST_TYPE_VENDOR, uint8(SET_RADIO_ARC), uint16(arc), 0, nil)
	return err
}

func (radio *RadioDevice) SetArdTime(delay uint8) error {
	// Auto Retransmit Delay:
	// 0x00 - Wait 250uS
	// 0x01 - Wait 500uS
	// 0x02 - Wait 750uS
	// ........
	// 0x0F - Wait 4000uS
	if delay > 0x0F {
		return ErrorInvalidArdTime
	}

	_, err := radio.device.Control(usb.REQUEST_TYPE_VENDOR, uint8(SET_RADIO_ARD), uint16(delay), 0, nil)
	return err
}

func (radio *RadioDevice) SetArdBytes(nbytes uint8) error {
	// 0x00 - 0 Byte
	// 0x01 - 1 Byte
	// ........
	// 0x20 - 32 Bytes
	if nbytes > 0x20 {
		return ErrorInvalidArdBytes
	}

	_, err := radio.device.Control(usb.REQUEST_TYPE_VENDOR, uint8(SET_RADIO_ARD), uint16(0x80|nbytes), 0, nil)
	return err
}

func (radio *RadioDevice) SetAckEnable(enable uint8) error {
	_, err := radio.device.Control(usb.REQUEST_TYPE_VENDOR, uint8(SET_ACK_ENABLE), uint16(enable), 0, nil)
	return err
}

func (radio *RadioDevice) SetAddress(address uint64) error {
	if radio.address == address {
		return nil
	}

	a := make([]byte, 5)
	a[4] = uint8((address >> 0) & 0xFF)
	a[3] = uint8((address >> 8) & 0xFF)
	a[2] = uint8((address >> 16) & 0xFF)
	a[1] = uint8((address >> 24) & 0xFF)
	a[0] = uint8((address >> 32) & 0xFF)

	_, err := radio.device.Control(
		usb.REQUEST_TYPE_VENDOR,
		uint8(SET_RADIO_ADDRESS),
		0,
		0,
		a)

	if err != nil {
		radio.address = address
	}

	return err
}

func (radio *RadioDevice) SendPacket(data []byte) error {
	// write the outgoing packet
	length, err := radio.dataOut.Write(data)
	if err != nil {
		return err
	}
	if len(data) != length {
		return ErrorWriteLength
	}
	return nil
}

func (radio *RadioDevice) ReadResponse() (bool, []byte, error) {
	// read the acknowledgement
	resp := make([]byte, 40) // largest packet size
	length, err := radio.dataIn.Read(resp)
	if err != nil {
		return false, nil, err
	}
	//if length {
	//	return false, nil, ERROR_READ_LENGTH
	//}
	// ACK structure:
	// uint8_t resp : 1
	// uint8_t power detector : 1
	// uint8_t reserved : 2
	// uint8_t retransmission count : 4
	// uint8_t ackdata[1:32 bytes]
	ackReceived := (resp[0] & 0x01) != 0
	return ackReceived, resp[1:length], nil // return just the data portion of the acknowledgement
}

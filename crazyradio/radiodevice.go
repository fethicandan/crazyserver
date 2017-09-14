package crazyradio

import "github.com/google/gousb"

type radioDevice struct {
	device  *gousb.Device
	context *gousb.Context
	config  *gousb.Config
	iface   *gousb.Interface
	dataIn  *gousb.InEndpoint
	dataOut *gousb.OutEndpoint
	address uint64
}

func openRadio(dev *gousb.Device, ctx *gousb.Context) (*radioDevice, error) {

	//dev.ControlTimeout = 250 * time.Millisecond
	dev.SetAutoDetach(true)
	dev.Reset()

	cfg, err := dev.Config(1)
	if err != nil {
		dev.Close()
		ctx.Close()
		return nil, err
	}

	iface, err := cfg.Interface(0, 0)
	if err != nil {
		cfg.Close()
		dev.Close()
		ctx.Close()
		return nil, err
	}

	dIn, err := iface.InEndpoint(1)
	if err != nil {
		iface.Close()
		cfg.Close()
		dev.Close()
		ctx.Close()
		return nil, err
	}

	dOut, err := iface.OutEndpoint(1)
	if err != nil {
		iface.Close()
		cfg.Close()
		dev.Close()
		ctx.Close()
		return nil, err
	}

	// now have a usb device and context pointing to the Radio!
	radio := &radioDevice{dev, ctx, cfg, iface, dIn, dOut, 0xE7E7E7E7E7}

	// can initialize the default states!
	radio.SetDatarate(RadioDatarate_2MPS)
	radio.SetChannel(80)
	radio.SetAddress(0xE7E7E7E7E7)
	radio.SetPower(RadioPower_0DBM)
	radio.SetArc(3)
	radio.SetArdBytes(32)
	return radio, nil
}

func openAllRadios() ([]*radioDevice, error) {
	usbContext := gousb.NewContext()
	usbContext.Debug(0)

	radioDevices, _ := usbContext.OpenDevices(
		func(desc *gousb.DeviceDesc) bool {
			if desc.Vendor == 0x1915 && desc.Product == 0x7777 {
				return true
			}
			return false
		})

	if len(radioDevices) == 0 {
		usbContext.Close()
		return nil, ErrorDeviceNotFound
	}

	radios := make([]*radioDevice, 0, len(radioDevices))

	for _, radioDevice := range radioDevices {
		radio, err := openRadio(radioDevice, usbContext)
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

func (radio *radioDevice) Close() {
	radio.iface.Close()
	radio.config.Close()
	radio.device.Close()
	radio.context.Close()
}

func (radio *radioDevice) SetChannel(channel uint8) error {
	if channel > 125 {
		return ErrorInvalidChannel
	}

	_, err := radio.device.Control(gousb.RequestTypeVendor, uint8(SET_RADIO_CHANNEL), uint16(channel), 0, nil)
	return err
}

func (radio *radioDevice) SetDatarate(datarate radioDatarate) error {
	if datarate > RadioDatarate_2MPS {
		return ErrorInvalidDatarate
	}

	_, err := radio.device.Control(gousb.RequestTypeVendor, uint8(SET_DATA_RATE), uint16(datarate), 0, nil)
	return err
}

func (radio *radioDevice) SetPower(power radioPower) error {
	if power > RadioPower_0DBM {
		return ErrorInvalidPower
	}

	_, err := radio.device.Control(gousb.RequestTypeVendor, uint8(SET_RADIO_POWER), uint16(power), 0, nil)
	return err
}

func (radio *radioDevice) SetArc(arc uint8) error {
	if arc > 15 {
		return ErrorInvalidArc
	}

	_, err := radio.device.Control(gousb.RequestTypeVendor, uint8(SET_RADIO_ARC), uint16(arc), 0, nil)
	return err
}

func (radio *radioDevice) SetArdTime(delay uint8) error {
	// Auto Retransmit Delay:
	// 0x00 - Wait 250uS
	// 0x01 - Wait 500uS
	// 0x02 - Wait 750uS
	// ........
	// 0x0F - Wait 4000uS
	if delay > 0x0F {
		return ErrorInvalidArdTime
	}

	_, err := radio.device.Control(gousb.RequestTypeVendor, uint8(SET_RADIO_ARD), uint16(delay), 0, nil)
	return err
}

func (radio *radioDevice) SetArdBytes(nbytes uint8) error {
	// 0x00 - 0 Byte
	// 0x01 - 1 Byte
	// ........
	// 0x20 - 32 Bytes
	if nbytes > 0x20 {
		return ErrorInvalidArdBytes
	}

	_, err := radio.device.Control(gousb.RequestTypeVendor, uint8(SET_RADIO_ARD), uint16(0x80|nbytes), 0, nil)
	return err
}

func (radio *radioDevice) SetAckEnable(enable uint8) error {
	_, err := radio.device.Control(gousb.RequestTypeVendor, uint8(SET_ACK_ENABLE), uint16(enable), 0, nil)
	return err
}

func (radio *radioDevice) SetAddress(address uint64) error {
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
		gousb.RequestTypeVendor,
		uint8(SET_RADIO_ADDRESS),
		0,
		0,
		a)

	if err != nil {
		radio.address = address
	}

	return err
}

func (radio *radioDevice) SendPacket(data []byte) error {
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

func (radio *radioDevice) ReadResponse() (bool, []byte, error) {
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

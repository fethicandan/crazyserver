package crazyusb

import (
	"time"

	"fmt"

	"reflect"

	"github.com/google/gousb"
	"github.com/mikehamer/crazyserver/crtp"
)

type usbDevice struct {
	device  *gousb.Device
	context *gousb.Context
	config  *gousb.Config
	iface   *gousb.Interface
	dataIn  *gousb.InEndpoint
	dataOut *gousb.OutEndpoint
}

func WaitForCrazyflieDisconnect() {
	usbContext := gousb.NewContext()
	defer usbContext.Close()

	count := 0
	for {
		count = 0
		usbContext.OpenDevices(
			func(desc *gousb.DeviceDesc) bool {
				if desc.Vendor == 0x0483 && desc.Product == 0x5740 {
					count += 1
				}
				return false
			})
		if count == 0 {
			break
		}
		<-time.After(10 * time.Millisecond)
	}
}

func CountConnectedCrazyflies() int {
	usbContext := gousb.NewContext()
	defer usbContext.Close()

	count := 0
	usbContext.OpenDevices(
		func(desc *gousb.DeviceDesc) bool {
			if desc.Vendor == 0x0483 && desc.Product == 0x5740 {
				count += 1
			}
			return false
		})

	return count
}

func WaitForCrazyflie() {
	usbContext := gousb.NewContext()
	defer usbContext.Close()

	count := 0
	for {
		count = 0
		usbContext.OpenDevices(
			func(desc *gousb.DeviceDesc) bool {
				if desc.Vendor == 0x0483 && desc.Product == 0x5740 {
					count += 1
				}
				return false
			})
		if count > 0 {
			break
		}
		<-time.After(10 * time.Millisecond)
	}
}

func openUsbDevice() (*usbDevice, error) {
	usbContext := gousb.NewContext()
	usbContext.Debug(0)

	usbDevices, _ := usbContext.OpenDevices(
		func(desc *gousb.DeviceDesc) bool {
			if desc.Vendor == 0x0483 && desc.Product == 0x5740 {
				return true
			}
			return false
		})

	if len(usbDevices) == 0 {
		usbContext.Close()
		return nil, ErrorDeviceNotFound
	}

	if len(usbDevices) > 1 {
		for _, dev := range usbDevices {
			dev.Close()
		}
		usbContext.Close()
		return nil, ErrorMultipleDevicesFound
	}

	dev := usbDevices[0]

	dev.ControlTimeout = 200 * time.Millisecond
	cfg, err := dev.Config(1)
	if err != nil {
		dev.Close()
		usbContext.Close()
		return nil, err
	}

	// In the config #2, claim interface #3 with alt setting #0.
	iface, err := cfg.Interface(0, 0)
	if err != nil {
		cfg.Close()
		dev.Close()
		usbContext.Close()
		return nil, err
	}

	// In this interface open endpoint #6 for reading.
	dIn, err := iface.InEndpoint(1)
	if err != nil {
		iface.Close()
		cfg.Close()
		dev.Close()
		usbContext.Close()
		return nil, err
	}

	// And in the same interface open endpoint #5 for writing.
	dOut, err := iface.OutEndpoint(1)
	if err != nil {
		iface.Close()
		cfg.Close()
		dev.Close()
		usbContext.Close()
		return nil, err
	}

	// now have a usb device and context pointing to the crazyflie!
	crtpUsb := &usbDevice{dev, usbContext, cfg, iface, dIn, dOut}

	err = crtpUsb.DisableCRTP()
	if err != nil {
		iface.Close()
		cfg.Close()
		dev.Close()
		usbContext.Close()
		return nil, err
	}

	err = crtpUsb.EnableCRTP()
	if err != nil {
		iface.Close()
		cfg.Close()
		dev.Close()
		usbContext.Close()
		return nil, err
	}

	return crtpUsb, nil
}

func (crtpUsb *usbDevice) EnableCRTP() error {
	// enable CRTP over USB
	_, err := crtpUsb.device.Control(gousb.RequestTypeVendor, 0x01, 0x01, 1, nil)
	return err
}

func (crtpUsb *usbDevice) DisableCRTP() error {
	// enable CRTP over USB
	_, err := crtpUsb.device.Control(gousb.RequestTypeVendor, 0x01, 0x01, 0, nil)
	return err
}

func (crtpUsb *usbDevice) Close() {
	crtpUsb.DisableCRTP()
	crtpUsb.iface.Close()
	crtpUsb.config.Close()
	crtpUsb.device.Close()
	crtpUsb.context.Close()
}

func (crtpUsb *usbDevice) SendPacket(data []byte) error {
	// write the outgoing packet
	length, err := crtpUsb.dataOut.Write(data)
	if err != nil {
		fmt.Println("SendPacketFail", err, reflect.TypeOf(err), reflect.ValueOf(err))
		return err
	}
	if len(data) != length {
		return ErrorWriteLength
	}
	return nil
}

func (crtpUsb *usbDevice) ReadResponse() ([]byte, error) {
	// read the acknowledgement
	resp := make([]byte, 40) // largest packet size
	length, err := crtpUsb.dataIn.Read(resp)

	if err == gousb.TransferTimedOut || err == gousb.ErrorTimeout {
		return []byte{crtp.PortEmpty1}, nil //emulate the empty queue packet, since USB just times out on empty queue
	}

	if err != nil {
		fmt.Println("ReadResponseFail", err, reflect.TypeOf(err), reflect.ValueOf(err))
		return nil, err
	}

	return resp[:length], nil // return just the data portion of the acknowledgement
}

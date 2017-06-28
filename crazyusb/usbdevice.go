package crazyusb

import (
	"time"

	"fmt"

	"reflect"

	"github.com/kylelemons/gousb/usb"
	"github.com/mikehamer/crazyserver/crtp"
)

type usbDevice struct {
	device  *usb.Device
	context *usb.Context
	dataOut usb.Endpoint
	dataIn  usb.Endpoint
}

func WaitForCrazyflieDisconnect() {
	usbContext := usb.NewContext()
	defer usbContext.Close()

	count := 0
	for {
		count = 0
		usbContext.ListDevices(
			func(desc *usb.Descriptor) bool {
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
	usbContext := usb.NewContext()
	defer usbContext.Close()

	count := 0
	usbContext.ListDevices(
		func(desc *usb.Descriptor) bool {
			if desc.Vendor == 0x0483 && desc.Product == 0x5740 {
				count += 1
			}
			return false
		})

	return count
}

func WaitForCrazyflie() {
	usbContext := usb.NewContext()
	defer usbContext.Close()

	count := 0
	for {
		count = 0
		usbContext.ListDevices(
			func(desc *usb.Descriptor) bool {
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
	usbContext := usb.NewContext()
	usbContext.Debug(0)

	usbDevices, _ := usbContext.ListDevices(
		func(desc *usb.Descriptor) bool {
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
	dev.ReadTimeout = 20 * time.Millisecond
	dev.WriteTimeout = 20 * time.Millisecond

	// open the endpoint for transfers out
	dOut, err := dev.OpenEndpoint(1, 0, 0, 0x01)

	if err != nil {
		dev.Close()
		usbContext.Close()
		return nil, err
	}

	// open the endpoint for transfers in
	dIn, err := dev.OpenEndpoint(1, 0, 0, 0x81)

	if err != nil {
		dev.Close()
		usbContext.Close()
		return nil, err
	}

	// now have a usb device and context pointing to the crazyflie!
	crtpUsb := new(usbDevice)
	crtpUsb.context = usbContext
	crtpUsb.device = dev
	crtpUsb.dataOut = dOut
	crtpUsb.dataIn = dIn

	err = crtpUsb.DisableCRTP()
	if err != nil {
		dev.Close()
		usbContext.Close()
		return nil, err
	}

	err = crtpUsb.EnableCRTP()
	if err != nil {
		dev.Close()
		usbContext.Close()
		return nil, err
	}

	return crtpUsb, nil
}

func (crtpUsb *usbDevice) EnableCRTP() error {
	// enable CRTP over USB
	_, err := crtpUsb.device.Control(usb.REQUEST_TYPE_VENDOR, 0x01, 0x01, 1, nil)
	return err
}

func (crtpUsb *usbDevice) DisableCRTP() error {
	// enable CRTP over USB
	_, err := crtpUsb.device.Control(usb.REQUEST_TYPE_VENDOR, 0x01, 0x01, 0, nil)
	return err
}

func (crtpUsb *usbDevice) Close() {
	crtpUsb.DisableCRTP()
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

	if err == usb.LIBUSB_TRANSFER_TIMED_OUT || err == usb.ERROR_TIMEOUT {
		return []byte{crtp.PortEmpty1}, nil //emulate the empty queue packet, since USB just times out on empty queue
	}

	if err != nil {
		fmt.Println("ReadResponseFail", err, reflect.TypeOf(err), reflect.ValueOf(err))
		return nil, err
	}

	return resp[:length], nil // return just the data portion of the acknowledgement
}

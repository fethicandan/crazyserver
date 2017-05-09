package crtpdevice

import (
	"time"

	"github.com/mikehamer/crazyserver/crtp"
)

type CrtpDevice interface {
	ClientRegister(channel uint8, address uint64, responseCallback func([]byte))
	ClientRemove(channel uint8, address uint64)
	ClientWaitUntilAllPacketsHaveBeenSent(channel uint8, address uint64)

	PacketSend(channel uint8, address uint64, request crtp.RequestPacketPtr) error
	PacketSendPriority(channel uint8, address uint64, request crtp.RequestPacketPtr) error
}

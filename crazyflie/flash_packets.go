package crazyflie

import (
	"reflect"

	"github.com/mikehamer/crazyserver/crtp"
)

// ---- FLASH REQUEST: GET INFO ----
type FlashRequestGetInfo struct {
	Target uint8
}

func (p *FlashRequestGetInfo) Port() crtp.Port {
	return crtp.PortLink
}

func (p *FlashRequestGetInfo) Channel() crtp.Channel {
	return 0x03
	// crtp.HeaderBytes(crtp.PortLink, 0x03) make the byte 0xFF (if link=3 is used). This is important for bootloader communications!
}

func (p *FlashRequestGetInfo) Bytes() []byte {
	return []byte{p.Target, 0x10} // and 0xFF is prepended by the sending code, based on the port and channel above
}

// ---- FLASH RESPONSE: GET INFO ----
type FlashResponseGetInfo struct {
	Target         uint8
	PageSize       int
	NumBuffPages   int
	NumFlashPages  int
	StartFlashPage int
}

func (p *FlashResponseGetInfo) Port() crtp.Port {
	return crtp.PortGreedy
}

func (p *FlashResponseGetInfo) Channel() crtp.Channel {
	return 0x00 // doesn't matter when using greedy port
}

func (p *FlashResponseGetInfo) LoadFromBytes(b []byte) error {
	if b[0] != 0xFF || b[1] != p.Target || b[2] != 0x10 { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	p.PageSize = int(bytesToUint16(b[3:5]).(uint16))
	p.NumBuffPages = int(bytesToUint16(b[5:7]).(uint16))
	p.NumFlashPages = int(bytesToUint16(b[7:9]).(uint16))
	p.StartFlashPage = int(bytesToUint16(b[9:11]).(uint16))

	return nil
}

// ---- FLASH REQUEST: LOAD BUFFER PAGE ----
type FlashRequestLoadBufferPage struct {
	Target        uint8
	BufferPageNum int
	BufferPageIdx int
	Data          []byte
}

func (p *FlashRequestLoadBufferPage) Port() crtp.Port {
	return crtp.PortLink
}

func (p *FlashRequestLoadBufferPage) Channel() crtp.Channel {
	return 0x03
	// crtp.HeaderBytes(crtp.PortLink, 0x03) make the byte 0xFF (if link=3 is used). This is important for bootloader communications!
}

func (p *FlashRequestLoadBufferPage) Bytes() []byte {
	packet := make([]byte, 6)
	packet[0] = p.Target
	packet[1] = 0x14 // the command
	packet[2] = byte(p.BufferPageNum & 0xFF)
	packet[3] = byte((p.BufferPageNum >> 8) & 0xFF)
	packet[4] = byte(p.BufferPageIdx & 0xFF)
	packet[5] = byte((p.BufferPageIdx >> 8) & 0xFF)

	return append(packet, p.Data...)
}

func (p *FlashRequestLoadBufferPage) MaxDataSize() int {
	return 32 - 7 //32 is the maximum size of a CRTP packet, and 7 is the size of the packet's header, including 1 byte for the CRTP header
}

// ---- FLASH REQUEST: WRITE LOADED PAGES ----
type FlashRequestWriteLoadedPagesToFlash struct {
	Target        uint8
	PageCount     int
	FlashLocation int
}

func (p *FlashRequestWriteLoadedPagesToFlash) Port() crtp.Port {
	return crtp.PortLink
}

func (p *FlashRequestWriteLoadedPagesToFlash) Channel() crtp.Channel {
	return 0x03
	// crtp.HeaderBytes(crtp.PortLink, 0x03) make the byte 0xFF (if link=3 is used). This is important for bootloader communications!
}

func (p *FlashRequestWriteLoadedPagesToFlash) Bytes() []byte {
	return []byte{
		p.Target,
		0x18,
		0, 0, // start from buffer page 0
		byte(p.FlashLocation & 0xFF),
		byte((p.FlashLocation >> 8) & 0xFF),
		byte(p.PageCount & 0xFF),
		byte((p.PageCount >> 8) & 0xFF)}
}

// ---- FLASH REQUEST: FLASHING STATUS ----
type FlashRequestGetFlashingStatus struct {
	Target uint8
}

func (p *FlashRequestGetFlashingStatus) Port() crtp.Port {
	return crtp.PortLink
}

func (p *FlashRequestGetFlashingStatus) Channel() crtp.Channel {
	return 0x03
	// crtp.HeaderBytes(crtp.PortLink, 0x03) make the byte 0xFF (if link=3 is used). This is important for bootloader communications!
}

func (p *FlashRequestGetFlashingStatus) Bytes() []byte {
	return []byte{p.Target, 0x19}
}

// ---- FLASH RESPONSE: FLASHING STATUS ----
type FlashResponseGetFlashingStatus struct {
	Target    uint8
	ErrorCode uint8
}

func (p *FlashResponseGetFlashingStatus) Port() crtp.Port {
	return crtp.PortGreedy
}

func (p *FlashResponseGetFlashingStatus) Channel() crtp.Channel {
	return 0x00 // doesn't matter when using greedy port
}

func (p *FlashResponseGetFlashingStatus) LoadFromBytes(b []byte) error {
	if b[0] != 0xFF || b[1] != p.Target || !(b[2] == 0x18 || b[2] == 0x19) { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	p.ErrorCode = b[4]

	return nil
}

// ---- FLASH REQUEST: VERIFY ADDRESS ----
type FlashRequestVerifyAddress struct {
	Target      uint8
	PageIndex   int
	PageAddress int
}

func (p *FlashRequestVerifyAddress) Port() crtp.Port {
	return crtp.PortLink
}

func (p *FlashRequestVerifyAddress) Channel() crtp.Channel {
	return 0x03
	// crtp.HeaderBytes(crtp.PortLink, 0x03) make the byte 0xFF (if link=3 is used). This is important for bootloader communications!
}

func (p *FlashRequestVerifyAddress) Bytes() []byte {
	return []byte{
		p.Target,
		0x1C,
		byte((p.PageIndex) & 0xFF),
		byte(((p.PageIndex) >> 8) & 0xFF),
		byte(p.PageAddress & 0xFF),
		byte((p.PageAddress >> 8) & 0xFF)}
}

// ---- FLASH RESPONSE: VERIFY ADDRESS ----
type FlashResponseVerifyAddress struct {
	Target      uint8
	PageIndex   int
	PageAddress int
	Data        []byte
}

func (p *FlashResponseVerifyAddress) Port() crtp.Port {
	return crtp.PortGreedy
}

func (p *FlashResponseVerifyAddress) Channel() crtp.Channel {
	return 0x00 // doesn't matter when using greedy port
}

func (p *FlashResponseVerifyAddress) LoadFromBytes(b []byte) error {
	if b[0] != 0xFF || b[1] != p.Target || b[2] != 0x1C { //we're dealing with the incorrect packet
		return crtp.ErrorPacketIncorrectType
	}

	address := []byte{
		byte((p.PageIndex) & 0xFF),
		byte(((p.PageIndex) >> 8) & 0xFF),
		byte(p.PageAddress & 0xFF),
		byte((p.PageAddress >> 8) & 0xFF)}

	if !reflect.DeepEqual(b[3:7], address) {
		return crtp.ErrorPacketIncorrectType // wrong address, duplicate of a previous request
	}

	p.Data = b[7:]

	return nil
}

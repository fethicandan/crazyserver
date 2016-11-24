package crazyradio

// Transmission datarate enum
type radioDatarate uint16

const (
	RadioDatarate_250KPS radioDatarate = iota
	RadioDatarate_1MPS
	RadioDatarate_2MPS
)

// Transmission power enum
type radioPower uint16

const (
	RadioPower_M18DBM radioPower = iota
	RadioPower_M12DBM
	RadioPower_M6DBM
	RadioPower_0DBM
)

// Radio commands enum
type radioCommand uint16

const (
	SET_RADIO_CHANNEL radioCommand = 0x01
	SET_RADIO_ADDRESS radioCommand = 0x02
	SET_DATA_RATE     radioCommand = 0x03
	SET_RADIO_POWER   radioCommand = 0x04
	SET_RADIO_ARD     radioCommand = 0x05
	SET_RADIO_ARC     radioCommand = 0x06
	SET_ACK_ENABLE    radioCommand = 0x10
	SET_CONT_CARRIER  radioCommand = 0x20
	SCANN_CHANNELS    radioCommand = 0x21
	LAUNCH_BOOTLOADER radioCommand = 0xFF
)

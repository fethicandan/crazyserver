package crazyflie

func (cf *Crazyflie) SetpointSend(roll, pitch, yawrate float32, thrust uint16) {

	// the packet to initialize the transaction
	packet := make([]byte, 1+3*4+2)
	packet[0] = crtp(crtpPortSetpoint, 0)
	copy(packet[1:5], float32ToBytes(roll))
	copy(packet[5:9], float32ToBytes(pitch))
	copy(packet[9:13], float32ToBytes(yawrate))
	copy(packet[13:15], uint16ToBytes(thrust))

	// don't wait for a callback / acknowledgement, just send and be done with it

	cf.commandQueue <- packet // schedule transmission of the packet
}

func (cf *Crazyflie) ExternalPositionSend(x, y, z float32) {

	// the packet to initialize the transaction
	packet := make([]byte, 1+3*4)
	packet[0] = crtp(crtpPortPosition, 0)
	copy(packet[1:5], float32ToBytes(x))
	copy(packet[5:9], float32ToBytes(y))
	copy(packet[9:13], float32ToBytes(z))

	// don't wait for a callback / acknowledgement, just send and be done with it

	cf.commandQueue <- packet // schedule transmission of the packet
}

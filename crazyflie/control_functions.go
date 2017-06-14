package crazyflie

func (cf *Crazyflie) LegacySetpointSend(roll, pitch, yawrate float32, thrust uint16) {
	request := &ControlRequestLegacySetpoint{roll, pitch, yawrate, thrust}
	cf.PacketSendPriority(request)
}

func (cf *Crazyflie) ExternalPositionSend(x, y, z float32) {
	request := &ControlRequestExternalPosition{x, y, z}
	cf.PacketSendPriority(request)
}

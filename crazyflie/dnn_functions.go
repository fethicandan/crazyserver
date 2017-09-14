package crazyflie

func (cf *Crazyflie) DNNSetpointSet(idx uint16, x, y, z, vx, vy, vz, ax, ay, az float32) error {
	request := &DNNRequestSetpoint{idx, x, y, z, vx, vy, vz, ax, ay, az}
	response := &DNNResponseSetpoint{Idx: idx}
	return cf.PacketSendAndAwaitResponse(request, response, DEFAULT_RESPONSE_TIMEOUT)
}

func (cf *Crazyflie) DNNStartTrajectory() error {
	request := &DNNRequestStart{}
	return cf.PacketSend(request)
}

func (cf *Crazyflie) DNNStateRequest(idx uint16) (*DNNResponseState, error) {
	request := &DNNRequestState{idx}
	response := &DNNResponseState{Idx: idx}
	err := cf.PacketSendAndAwaitResponse(request, response, DEFAULT_RESPONSE_TIMEOUT)
	if err != nil {
		return nil, err
	}

	return response, nil
}

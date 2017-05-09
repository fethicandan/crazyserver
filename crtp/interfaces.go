package crtp

type RequestPacketPtr interface {
	Port() Port
	Channel() Channel
	Bytes() []byte
}

type ResponsePacketPtr interface {
	Port() Port
	Channel() Channel
	LoadFromBytes([]byte) error
}

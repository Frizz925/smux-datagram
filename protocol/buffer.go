package protocol

func NewBuffer() []byte {
	return make([]byte, MaxPacketSize)
}

func EmptyBuffer() []byte {
	return make([]byte, 0, MaxPacketSize)
}

package crazyflie

import "math"

// here we have to use interface as the return everywhere since the functions need to fit into a generic map
// everything is little endian

func bytesToUint8(b []byte) interface{} {
	return uint32(b[0])
}

func bytesToUint16(b []byte) interface{} {
	_ = b[1]
	return uint32(uint32(b[0]) | (uint32(b[1]) << 8))
}

func bytesToUint32(b []byte) interface{} {
	_ = b[3]
	return uint32(uint32(b[0]) | (uint32(b[1]) << 8) | (uint32(b[2]) << 16) | (uint32(b[3]) << 24))
}

func bytesToInt8(b []byte) interface{} {
	return int32(b[0])
}

func bytesToInt16(b []byte) interface{} {
	_ = b[1]
	return int32(uint32(b[0]) | (uint32(b[1]) << 8))
}

func bytesToInt32(b []byte) interface{} {
	_ = b[3]
	return int32(uint32(b[0]) | (uint32(b[1]) << 8) | (uint32(b[2]) << 16) | (uint32(b[3]) << 24))
}

func bytesToFloat32(b []byte) interface{} {
	bits := uint32(uint32(b[0]) | (uint32(b[1]) << 8) | (uint32(b[2]) << 16) | (uint32(b[3]) << 24))
	return math.Float32frombits(bits)
}

func bytesToFloat16(b []byte) interface{} {
	_ = b[1]
	val := uint32(uint32(b[0]) | (uint32(b[1]) << 8))

	var fp32 uint32
	s := val >> 15
	e := (val >> 10) & 0x1F

	//All binary16 can be mapped in a binary32
	if e == 0 {
		tmp := int32(15 - 127) // need to do this otherwise go complains of overflow
		e = uint32(tmp)
	}

	if e == 0x1F {
		if (val & 0x03FF) != 0 {
			fp32 = 0x7FC00000 // NaN
		} else if s == 0 {
			fp32 = 0x7F800000
		} else {
			fp32 = 0xFF800000
		}
	} else {
		fp32 = (s << 31) | (uint32(e+127-15) << 23) | (uint32(val&0x3ff) << 13)
	}

	return math.Float32frombits(fp32)
}

package domain

// ComposeMark формирует 32-bit fwmark: bits [0:7] = tunnel index (1-based), [8:15] = class index.
// Остальные биты — 0.
func ComposeMark(tunnelIndexOneBased, classIndex uint8) uint32 {
	return uint32(tunnelIndexOneBased) | (uint32(classIndex) << 8)
}

// ParseMark возвращает tunnel index (1-based) и class index из mark.
func ParseMark(mark uint32) (tunnelIndex, classIndex uint8) {
	return uint8(mark & 0xff), uint8((mark >> 8) & 0xff)
}

package message

import chatmodel "PProject/module/chat/model"

func clamp(v, lo, hi int64) int64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func blockStartOf(seq int64) int64 {
	return (seq / chatmodel.BlockK) * chatmodel.BlockK
}

func bitIndex(seq int64) (start int64, idx int) {
	start = blockStartOf(seq)
	idx = int(seq - start) // 0..BlockK-1
	return
}

func setBitInPlace(bits []byte, idx int) bool {
	// 返回 true 表示原来是 0，本次置 1（新增）
	byteIdx := idx >> 3
	off := uint(idx & 7)
	mask := byte(1 << off)
	old := bits[byteIdx]
	if (old & mask) != 0 {
		return false
	}
	bits[byteIdx] = old | mask
	return true
}

var popcnt [256]uint8

func init() {
	for i := 0; i < 256; i++ {
		x := uint8(i)
		var c uint8
		for x > 0 {
			c += x & 1
			x >>= 1
		}
		popcnt[i] = c
	}
}
func countBits(bits []byte) int {
	sum := 0
	for _, b := range bits {
		sum += int(popcnt[b])
	}
	return sum
}

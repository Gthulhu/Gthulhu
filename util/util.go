package util

import "time"

func Now() uint64 {
	return uint64(time.Now().UnixNano())
}

func CalcAvg(oldVal uint64, newVal uint64) uint64 {
	return (oldVal - (oldVal >> 2)) + (newVal >> 2)
}

func SaturatingSub(a, b uint64) uint64 {
	if a > b {
		return a - b
	}
	return 0
}

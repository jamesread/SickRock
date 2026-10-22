package server

import "math"

// toInt32 converts an int or int64 (typically a database ID or count) to the int32 used by
// protobuf messages, clamping values outside the int32 range rather than wrapping.
func toInt32[T ~int | ~int64](v T) int32 {
	if v > math.MaxInt32 {
		return math.MaxInt32
	}
	if v < math.MinInt32 {
		return math.MinInt32
	}
	return int32(v)
}

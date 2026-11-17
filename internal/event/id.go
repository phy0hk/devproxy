package event

import (
	"strconv"
	"sync/atomic"
)

var sequence uint64

// NextSequence returns the next process-wide monotonic stream sequence.
func NextSequence() uint64 {
	return atomic.AddUint64(&sequence, 1)
}

// NextID returns a string form of the next stream sequence.
func NextID() string {
	return strconv.FormatUint(NextSequence(), 10)
}

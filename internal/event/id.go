package event

import (
	"strconv"
	"sync/atomic"
)

var sequence uint64

func NextID() string {
	id := atomic.AddUint64(&sequence, 1)

	return strconv.FormatUint(id, 10)
}

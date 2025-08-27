package workerpool

import (
	"golang.org/x/time/rate"
)

var limiter = rate.NewLimiter(2, 5) // 2 req/sec, burst 5

package v9

import (
	"time"
)

type option func(l *DistLock)

func WithSessionTtlSec(ttl time.Duration) option {
	return func(l *DistLock) {
		l.sessionTtl = ttl
	}
}

func WithLockLoopTimeout(t time.Duration) option {
	return func(l *DistLock) {
		l.lockLoopTimeout = t
	}
}

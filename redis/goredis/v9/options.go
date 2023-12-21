package v9

import (
	"time"
)

type Option func(l *DistLock)

func WithSessionTtlSec(ttl time.Duration) Option {
	return func(l *DistLock) {
		l.sessionTtl = ttl
	}
}

func WithLockLoopTimeout(t time.Duration) Option {
	return func(l *DistLock) {
		l.lockLoopTimeout = t
	}
}

func WithWaitNumSlaves(numSlaves int, timeout time.Duration) Option {
	return func(l *DistLock) {
		l.waitNumSlaves = numSlaves
		l.waitTimeout = timeout
	}
}

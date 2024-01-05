package godistlock

import (
	"context"
	"time"
)

type DistLockBase interface {
	Lock(ctx context.Context, session, id string, blocking bool) (bool, error)
	Unlock(ctx context.Context, session, id string) (bool, error)
}

type DistLock interface {
	DistLockBase
	LockWithTimeout(ctx context.Context, session, id string, blocking bool, timeout time.Duration) (bool, error)
	UnlockWithTimeout(session, id string, timeout time.Duration) (bool, error)
}

type distLock struct {
	l DistLockBase
}

func New(l DistLockBase) DistLock {
	return &distLock{l}
}

func (s *distLock) Lock(ctx context.Context, session, id string, blocking bool) (bool, error) {
	return s.l.Lock(ctx, session, id, blocking)
}

func (s *distLock) Unlock(ctx context.Context, session, id string) (bool, error) {
	return s.l.Unlock(ctx, session, id)
}

func (s *distLock) LockWithTimeout(ctx context.Context, session, id string, blocking bool, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.l.Lock(ctx, session, id, blocking)
}

func (s *distLock) UnlockWithTimeout(session, id string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.l.Unlock(ctx, session, id)
}

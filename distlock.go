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

type distLockWithPrefix struct {
	DistLock
	prefix string
}

func NewWithPrefix(l DistLock, prefix string) DistLock {
	return &distLockWithPrefix{l, prefix}
}

func (d *distLockWithPrefix) Lock(ctx context.Context, session, id string, blocking bool) (bool, error) {
	return d.DistLock.Lock(ctx, session, d.prefix+id, blocking)
}

func (d *distLockWithPrefix) Unlock(ctx context.Context, session, id string) (bool, error) {
	return d.DistLock.Unlock(ctx, session, d.prefix+id)
}

func (d *distLockWithPrefix) LockWithTimeout(ctx context.Context, session, id string, blocking bool, timeout time.Duration) (bool, error) {
	return d.DistLock.LockWithTimeout(ctx, session, d.prefix+id, blocking, timeout)
}

func (d *distLockWithPrefix) UnlockWithTimeout(session, id string, timeout time.Duration) (bool, error) {
	return d.DistLock.UnlockWithTimeout(session, d.prefix+id, timeout)
}

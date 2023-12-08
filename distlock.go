package godistlock

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type DistLockBase interface {
	Lock(ctx context.Context, session, id string, blocking bool) (bool, error)
	Unlock(ctx context.Context, session, id string) (bool, error)
}

type DistLock interface {
	NewSession() Session
	WithSession(ses string) Session
}

type Session interface {
	Lock(ctx context.Context, id string, blocking bool) (bool, error)
	Unlock(ctx context.Context, id string) (bool, error)
	LockWithTimeout(ctx context.Context, id string, blocking bool, timeout time.Duration) (bool, error)
	UnlockWithTimeout(id string, timeout time.Duration) (bool, error)
}

type distLock struct {
	l DistLockBase
}

func New(l DistLockBase) DistLock {
	return &distLock{l}
}

func (d *distLock) NewSession() Session {
	return &session{l: d.l, session: uuid.New().String()}
}

func (d *distLock) WithSession(ses string) Session {
	return &session{l: d.l, session: ses}
}

type session struct {
	l       DistLockBase
	session string
}

func (s *session) Lock(ctx context.Context, id string, blocking bool) (bool, error) {
	return s.l.Lock(ctx, s.session, id, blocking)
}

func (s *session) Unlock(ctx context.Context, id string) (bool, error) {
	return s.l.Unlock(ctx, s.session, id)
}

func (s *session) LockWithTimeout(ctx context.Context, id string, blocking bool, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return s.l.Lock(ctx, s.session, id, blocking)
}

func (s *session) UnlockWithTimeout(id string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	return s.l.Unlock(ctx, s.session, id)
}

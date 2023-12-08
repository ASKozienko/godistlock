package v9

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

const deleteScript = `
	if redis.call("GET", KEYS[1]) == ARGV[1] then
		return redis.call("DEL", KEYS[1])
	else
		return 0
	end
`

type DistLock struct {
	sessionTtl      time.Duration
	lockLoopTimeout time.Duration
	client          redis.UniversalClient
}

func New(client redis.UniversalClient, options ...option) *DistLock {
	l := &DistLock{
		sessionTtl:      120 * time.Second,
		lockLoopTimeout: 50 * time.Millisecond,
		client:          client,
	}

	for _, opt := range options {
		opt(l)
	}

	return l
}

func (l *DistLock) Lock(ctx context.Context, session, id string, blocking bool) (bool, error) {
	for {
		ok, err := l.client.SetNX(ctx, id, session, l.sessionTtl).Result()
		if err != nil {
			return false, err
		}

		if ok {
			return true, nil
		}

		if !blocking {
			return false, nil
		}

		select {
		case <-time.After(l.lockLoopTimeout):
		case <-ctx.Done():
			return false, nil
		}
	}
}

func (l *DistLock) Unlock(ctx context.Context, session, id string) (bool, error) {
	resp, err := l.client.Eval(ctx, deleteScript, []string{id}, session).Int64()
	if err != nil {
		return false, err
	}

	return resp != 0, nil
}

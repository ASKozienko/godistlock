package v9

import (
	"context"
	"fmt"
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
	waitNumSlaves   int
	waitTimeout     time.Duration
	client          *redis.Client
}

func New(client *redis.Client, options ...Option) *DistLock {
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
			if l.waitNumSlaves > 0 {
				num, err := l.client.Wait(ctx, l.waitNumSlaves, l.waitTimeout).Result()
				if err != nil {
					return false, err
				}

				if num < int64(l.waitNumSlaves) {
					return false, fmt.Errorf("slaves replication timeout")
				}
			}

			return true, nil
		}

		if !blocking {
			return false, nil
		}

		select {
		case <-time.After(l.lockLoopTimeout):
		case <-ctx.Done():
			return false, ctx.Err()
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

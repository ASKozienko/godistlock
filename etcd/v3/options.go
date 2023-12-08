package v3

import (
	"context"
	"time"

	"github.com/ASKozienko/godistlock/logger"
	"github.com/ASKozienko/godistlock/logger/null"
	clientv3 "go.etcd.io/etcd/client/v3"
)

type options struct {
	opts []option
}
type option func(l *DistLock)

func New(c clientv3.Config) *options {
	o := &options{}
	o.opts = append(o.opts, func(dl *DistLock) {
		dl.config = c
	})

	return o
}

func (o *options) Build() *DistLock {
	dl := &DistLock{
		sessionTtlSec:    120,
		grantTimeout:     10 * time.Second,
		reconnectTimeout: 10 * time.Second,
		l:                null.New(),
		connCh:           make(chan *conn),
	}

	dl.lockFunc = dl.lockWithLoop

	for _, op := range o.opts {
		op(dl)
	}

	return dl
}

func (o *options) Run(ctx context.Context) {
	o.Build().Run(ctx)
}

func (o *options) WithLogger(l logger.Logger) *options {
	o.opts = append(o.opts, func(dl *DistLock) {
		dl.l = l
	})

	return o
}

func (o *options) WithSessionTtlSec(ttl int64) *options {
	o.opts = append(o.opts, func(dl *DistLock) {
		dl.sessionTtlSec = ttl
	})

	return o
}

func (o *options) WithLockStrategyLoop() *options {
	o.opts = append(o.opts, func(dl *DistLock) {
		dl.lockFunc = dl.lockWithLoop
	})

	return o
}

func (o *options) WithLockStrategyWait() *options {
	o.opts = append(o.opts, func(dl *DistLock) {
		dl.lockFunc = dl.lockWithWait
	})

	return o
}

func (o *options) WithReconnectTimeout(d time.Duration) *options {
	o.opts = append(o.opts, func(dl *DistLock) {
		dl.reconnectTimeout = d
	})

	return o
}

func (o *options) WithGrantTimeout(d time.Duration) *options {
	o.opts = append(o.opts, func(dl *DistLock) {
		dl.grantTimeout = d
	})

	return o
}

package v3

import (
	"context"
	"errors"
	"time"

	"github.com/ASKozienko/godistlock/logger"

	clientv3 "go.etcd.io/etcd/client/v3"
)

type LockFunc func(ctx context.Context, client *clientv3.Client, leaseId clientv3.LeaseID, session, id string, blocking bool) (bool, error)

type DistLock struct {
	config           clientv3.Config
	sessionTtlSec    int64
	grantTimeout     time.Duration
	reconnectTimeout time.Duration
	lockFunc         LockFunc
	l                logger.Logger

	connCh chan *conn
}

type conn struct {
	client  *clientv3.Client
	leaseId clientv3.LeaseID
}

func (l *DistLock) Lock(ctx context.Context, session, id string, blocking bool) (bool, error) {
	select {
	case conn := <-l.connCh:
		return l.lockFunc(ctx, conn.client, conn.leaseId, session, id, blocking)
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (l *DistLock) Unlock(ctx context.Context, session, id string) (bool, error) {
	select {
	case conn := <-l.connCh:
		return l.unlock(ctx, conn.client, session, id)
	case <-ctx.Done():
		return false, ctx.Err()
	}
}

func (l *DistLock) Run(ctx context.Context) {
	defer l.l.Debug("exited")
	l.connection(ctx)
}

func (l *DistLock) connection(ctx context.Context) {
	l.l.Debug("connecting")
	for {
		client, err := clientv3.New(l.config)
		if err != nil {
			l.l.Error("connection failed: %s", err)

			select {
			case <-time.After(l.reconnectTimeout):
				continue
			case <-ctx.Done():
				return
			}
		}

		l.session(ctx, client)

		if err := client.Close(); err != nil {
			l.l.Error("close client failed: %s", err)
		}

		select {
		case <-ctx.Done():
			return
		default:
			l.l.Debug("reconnecting")
		}
	}

}

func (l *DistLock) session(ctx context.Context, client *clientv3.Client) {
	l.l.Debug("session")

	ctx1, ctx1Close := context.WithTimeout(ctx, l.grantTimeout)
	lease, err := client.Grant(ctx1, l.sessionTtlSec)
	ctx1Close()
	if err != nil {
		l.l.Error("create lease failed: %s", err)
		return
	}

	defer func() {
		rctx, rctxClose := context.WithTimeout(context.Background(), 5*time.Second)
		if _, err := client.Revoke(rctx, lease.ID); err != nil {
			l.l.Error("revoke lease failed: %s", err)
		}
		rctxClose()
	}()

	l.handler(ctx, client, lease.ID)
}

func (l *DistLock) handler(ctx context.Context, client *clientv3.Client, leaseId clientv3.LeaseID) {
	l.l.Debug("handler")

	lch, err := client.KeepAlive(ctx, leaseId)
	if err != nil {
		l.l.Error("lease keep alive failed: %s", err)

		return
	}

	conn := &conn{
		client:  client,
		leaseId: leaseId,
	}

	for {
		select {
		case l.connCh <- conn:
		case _, ok := <-lch:
			if !ok {
				l.l.Error("keepalive channel is closed")
				return
			}
		case <-ctx.Done():
			return
		}
	}
}

func (l *DistLock) unlock(ctx context.Context, client *clientv3.Client, session, id string) (bool, error) {
	cmp := clientv3.Compare(clientv3.Value(id), "=", session)
	del := clientv3.OpDelete(id)

	resp, err := client.Txn(ctx).If(cmp).Then(del).Commit()
	if err != nil {
		return false, err
	}

	return resp.Succeeded && resp.Responses[0].GetResponseDeleteRange().Deleted == 1, nil
}

func (l *DistLock) lockWithLoop(ctx context.Context, client *clientv3.Client, leaseId clientv3.LeaseID, session, id string, blocking bool) (bool, error) {
	cmp := clientv3.Compare(clientv3.CreateRevision(id), "=", 0)
	put := clientv3.OpPut(id, session, clientv3.WithLease(leaseId))

	for {
		resp, err := client.Txn(ctx).If(cmp).Then(put).Commit()
		if err != nil {
			return false, err
		}

		if resp.Succeeded {
			return true, nil
		}

		if !blocking {
			return false, nil
		}

		select {
		case <-time.After(50 * time.Millisecond):
		case <-ctx.Done():
			return false, nil
		}
	}
}

func (l *DistLock) lockWithWait(ctx context.Context, client *clientv3.Client, leaseId clientv3.LeaseID, session, id string, blocking bool) (bool, error) {
	cmp := clientv3.Compare(clientv3.CreateRevision(id), "=", 0)
	put := clientv3.OpPut(id, session, clientv3.WithLease(leaseId))
	get := clientv3.OpGet(id)

	for {
		resp, err := client.Txn(ctx).If(cmp).Then(put).Else(get).Commit()
		if err != nil {
			return false, err
		}

		if resp.Succeeded {
			return true, nil
		}

		if !blocking {
			return false, nil
		}

		err = l.waitUnlock(ctx, client, id, resp.Responses[0].GetResponseRange().Kvs[0].CreateRevision)
		if err != nil && (errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)) {
			return false, nil
		}
	}
}

func (l *DistLock) waitUnlock(ctx context.Context, client *clientv3.Client, id string, rev int64) error {
	ctx1, ctx1Close := context.WithCancel(ctx)
	defer ctx1Close()
	w := client.Watcher.Watch(ctx1, id, clientv3.WithFilterPut(), clientv3.WithRev(rev))

	for {
		select {
		case wr := <-w:
			for _, e := range wr.Events {
				if e.Type == clientv3.EventTypeDelete {
					return nil
				}
			}

			if err := wr.Err(); err != nil {
				return err
			}
		case <-ctx1.Done():
			return ctx1.Err()
		}
	}
}

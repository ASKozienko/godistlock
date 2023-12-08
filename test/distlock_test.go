package test

import (
	"context"
	"fmt"
	"testing"
	"time"

	v9 "github.com/ASKozienko/godistlock/redis/goredis/v9"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	lock "github.com/ASKozienko/godistlock/etcd/v3"
	clientv3 "go.etcd.io/etcd/client/v3"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"github.com/ASKozienko/godistlock"
	v1 "github.com/ASKozienko/godistlock/mongodb/v1"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/redis/go-redis/v9"
)

type buildFunc func(ctx context.Context) (godistlock.DistLock, error)
type builder struct {
	name  string
	build buildFunc
}

func TestLock(t *testing.T) {
	builders := []builder{
		{name: "mongodb", build: buildMongodb},
		{name: "etcd", build: buildEtcd},
		{name: "redis", build: buildRedis},
	}

	for _, b := range builders {
		t.Run(b.name, func(t *testing.T) {
			l, err := b.build(context.Background())
			require.NoError(t, err)

			ses := uuid.New().String()
			id := uuid.New().String()
			locked, err := l.Lock(context.Background(), ses, id, true)
			require.NoError(t, err)
			require.True(t, locked)

			locked, err = l.Lock(context.Background(), "another-session", id, false)
			require.NoError(t, err)
			require.False(t, locked)

			locked, err = l.Unlock(context.Background(), "another-session", id)
			require.NoError(t, err)
			require.False(t, locked)

			locked, err = l.Unlock(context.Background(), ses, id)
			require.NoError(t, err)
			require.True(t, locked)
		})
	}
}

func buildMongodb(ctx context.Context) (godistlock.DistLock, error) {
	mClient, err := mongo.Connect(ctx, options.Client().ApplyURI("mongodb://mongodb:27017"))
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %e", err)
	}

	l := v1.New(mClient.Database("lock").Collection("lock"))
	if err := l.CreateIndexes(ctx, 302); err != nil {
		return nil, fmt.Errorf("client lock create indexes: %w", err)
	}

	return l, nil
}

func buildEtcd(ctx context.Context) (godistlock.DistLock, error) {
	l := lock.New(clientv3.Config{
		Endpoints:            []string{"etcd:2379"},
		DialTimeout:          10 * time.Second,
		DialKeepAliveTime:    1 * time.Second,
		DialKeepAliveTimeout: 5 * time.Second,
		Logger:               zap.NewNop(),
		DialOptions:          []grpc.DialOption{grpc.WithBlock(), grpc.WithReturnConnectionError(), grpc.WithDisableRetry()},
	}).WithSessionTtlSec(120).WithLockStrategyLoop().Build()

	go func() {
		l.Run(ctx)
	}()

	return l, nil
}

func buildRedis(ctx context.Context) (godistlock.DistLock, error) {
	cl := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
		DB:   0,
	})

	return v9.New(cl), nil
}

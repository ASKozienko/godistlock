package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/rand"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	v9 "github.com/ASKozienko/godistlock/redis/goredis/v9"
	"github.com/redis/go-redis/v9"

	"google.golang.org/grpc"

	v1 "github.com/ASKozienko/godistlock/mongodb/v1"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readconcern"
	"go.mongodb.org/mongo-driver/mongo/writeconcern"

	"github.com/ASKozienko/godistlock"

	"go.uber.org/zap"

	"github.com/mackerelio/go-osstat/loadavg"
	"go.uber.org/atomic"

	lock "github.com/ASKozienko/godistlock/etcd/v3"
	log "github.com/ASKozienko/godistlock/logger/fmt"

	clientv3 "go.etcd.io/etcd/client/v3"
)

func main() {
	ctx, _ := signal.NotifyContext(context.Background(), syscall.SIGINT)
	wg := sync.WaitGroup{}
	impl := flag.String("impl", "", "[etcd|mongodb|redis]")
	flag.Parse()

	var l godistlock.DistLock
	switch *impl {
	case "etcd":
		l = godistlock.New(buildEtcd(ctx, &wg))
	case "mongodb":
		b, err := buildMongodb(ctx, &wg)
		if err != nil {
			panic(err)
		}
		l = godistlock.New(b)
	case "redis":
		l = godistlock.New(buildRedis(ctx, &wg))
	default:
		panic("unsupported impl")
	}

	lc := atomic.NewInt64(0)
	le := atomic.NewInt64(0)
	lci := make([]*atomic.Int64, 0)

	// i - number of id's
	for i := 0; i < 100; i++ {
		id := fmt.Sprintf("id-%d", i)
		// k - number of concurrent workers per id
		for k := 0; k < 10; k++ {
			k := k
			_lci := atomic.NewInt64(0)
			lci = append(lci, _lci)
			wg.Add(1)
			go func() {
				defer wg.Done()
				worker(ctx, k, id, l, _lci, lc, le)
			}()
		}
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		start := time.Now()
		t := time.NewTicker(10 * time.Second)

		for {
			select {
			case <-t.C:
				s, err := loadavg.Get()
				if err != nil {
					fmt.Println("loadavg error")
				}
				var m runtime.MemStats
				runtime.ReadMemStats(&m)

				dist := ""
				for i, e := range lci {
					dist += fmt.Sprintf("%d\t", e.Load())
					if (i+1)%10 == 0 {
						dist += "\n"
					}
				}

				fmt.Println(fmt.Sprintf("t: %s lc: %d, le: %d, load: %f, mem: %d, memt: %d", time.Now().Sub(start), lc.Load(), le.Load(), s.Loadavg1, m.Alloc/1024/1024, m.TotalAlloc/1024/1024))
				fmt.Println(dist)
			case <-ctx.Done():
				return
			}
		}
	}()

	wg.Wait()
	fmt.Println("DONE")
}

func buildEtcd(ctx context.Context, wg *sync.WaitGroup) godistlock.DistLockBase {
	l := lock.New(clientv3.Config{
		Endpoints:            []string{"etcd:2379"},
		DialTimeout:          10 * time.Second,
		DialKeepAliveTime:    1 * time.Second,
		DialKeepAliveTimeout: 5 * time.Second,
		Logger:               zap.NewNop(),
		DialOptions:          []grpc.DialOption{grpc.WithBlock(), grpc.WithReturnConnectionError(), grpc.WithDisableRetry()},
	}).WithSessionTtlSec(120).WithLogger(log.New()).WithLockStrategyLoop().Build()

	wg.Add(1)
	go func() {
		defer wg.Done()

		l.Run(ctx)
	}()

	return l
}

func buildMongodb(ctx context.Context, wg *sync.WaitGroup) (godistlock.DistLockBase, error) {
	mOpts := options.Client().ApplyURI("mongodb://mongodb:27017").
		SetWriteConcern(writeconcern.Majority()).
		SetReadConcern(readconcern.Snapshot())
	mClient, err := mongo.Connect(ctx, mOpts)
	if err != nil {
		return nil, fmt.Errorf("mongo connect: %e", err)
	}

	fmt.Println("ping mongo")
	for {
		err := mClient.Ping(ctx, nil)
		if err != nil && errors.Is(err, context.Canceled) {
			return nil, ctx.Err()
		} else if err != nil {
			fmt.Println(fmt.Sprintf("mongo connection failed: %s", err))
			time.Sleep(time.Second)
			continue
		}

		fmt.Println("mongo connection is ok")
		break
	}

	wg.Add(1)
	go func() {
		defer wg.Done()
		<-ctx.Done()

		mCtx, mCtxCancel := context.WithTimeout(context.Background(), 10*time.Second)
		_ = mClient.Disconnect(mCtx)
		mCtxCancel()
	}()

	l := v1.New(mClient.Database("lock").Collection("lock"))
	if err := l.CreateIndexes(ctx, 302); err != nil {
		return nil, fmt.Errorf("client lock create indexes: %w", err)
	}

	return l, nil
}

func buildRedis(ctx context.Context, wg *sync.WaitGroup) godistlock.DistLockBase {
	cl := redis.NewClient(&redis.Options{
		Addr: "redis:6379",
		DB:   0,
	})
	l := v9.New(cl, v9.WithSessionTtlSec(300*time.Second), v9.WithLockLoopTimeout(50*time.Millisecond))

	wg.Add(1)
	go func() {
		defer wg.Done()
	}()

	return l
}

func worker(ctx context.Context, i int, id string, l godistlock.DistLock, lci, lc, le *atomic.Int64) {
	s := rand.NewSource(time.Now().UnixNano())
	r := rand.New(s)

	ses := l.NewSession()

	for {
		if ctx.Err() != nil {
			return
		}

		locked, err := ses.LockWithTimeout(ctx, id, true, 10*time.Second)
		if err != nil {
			fmt.Println(i, "lock", err)
			continue
		}

		if !locked {
			le.Inc()
			continue
		}

		lci.Inc()
		lc.Inc()

		time.Sleep(time.Duration(r.Int63n(50)) * time.Millisecond)
		_, err = ses.UnlockWithTimeout(id, 10*time.Second)
		if err != nil && !errors.Is(err, context.Canceled) {
			fmt.Println(i, "unlock", err)
		}

		time.Sleep(time.Duration(r.Int63n(10)) * time.Millisecond)
	}
}

package v1

import (
	"context"
	"time"

	"github.com/ASKozienko/godistlock/pkg/ptr"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type DistLock struct {
	col *mongo.Collection
}

func New(col *mongo.Collection) *DistLock {
	return &DistLock{
		col: col,
	}
}

func (l *DistLock) CreateIndexes(ctx context.Context, expireTimeoutSec int32) error {
	_, err := l.col.Indexes().CreateMany(ctx, l.getIndexes(expireTimeoutSec))

	return err
}

func (l *DistLock) getIndexes(expireTimeoutSec int32) []mongo.IndexModel {
	return []mongo.IndexModel{
		{
			Keys: bson.D{{"id", 1}},
			Options: &options.IndexOptions{
				Name:   ptr.String("id"),
				Unique: ptr.Bool(true),
			},
		},
		{
			Keys: bson.D{{"timestamp", 1}},
			Options: &options.IndexOptions{
				Name:               ptr.String("timestamp"),
				ExpireAfterSeconds: ptr.Int32(expireTimeoutSec),
			},
		},
	}
}

func (l *DistLock) Lock(ctx context.Context, session, id string, blocking bool) (bool, error) {
	for {
		_, err := l.col.InsertOne(ctx, bson.D{
			{"id", id},
			{"timestamp", time.Now()},
			{"session", session},
		})
		if err != nil && mongo.IsDuplicateKeyError(err) {
			if !blocking {
				return false, nil
			}

			select {
			case <-time.After(50 * time.Millisecond):
				continue
			case <-ctx.Done():
				return false, ctx.Err()
			}
		} else if err != nil {
			return false, err
		}

		return true, nil
	}
}

func (l *DistLock) Unlock(ctx context.Context, session, id string) (bool, error) {
	resp, err := l.col.DeleteOne(ctx, bson.D{
		{"id", id},
		{"session", session},
	})
	if err != nil {
		return false, err
	}

	return resp.DeletedCount == 1, nil
}

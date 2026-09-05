package projection_mongo_repository

import (
	"context"
	"fmt"
	"time"

	core_mongo "github.com/1C-L0V3R/golang-td-app/internal/core/repository/mongo"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

const (
	CollectionUsers = "users"
	CollectionTasks = "tasks"
)

type ProjectionRepository struct {
	client *core_mongo.Client
}

func NewProjectionRepository(
	client *core_mongo.Client,
) *ProjectionRepository {
	return &ProjectionRepository{
		client: client,
	}
}

// Upsert применяет документ, только если в Mongo лежит более старая
// версия. Это делает повторную доставку безопасной и защищает от
// применения запоздавших событий поверх свежих.
func (r *ProjectionRepository) Upsert(
	ctx context.Context,
	collection string,
	id int,
	version int,
	document bson.M,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.client.OpTimeout())
	defer cancel()

	document["_id"] = id
	document["version"] = version
	document["synced_at"] = time.Now()

	filter := bson.M{
		"_id":     id,
		"version": bson.M{"$lt": version},
	}

	_, err := r.client.Collection(collection).ReplaceOne(
		ctx,
		filter,
		document,
		options.Replace().SetUpsert(true),
	)
	if err != nil {
		// E11000: фильтр не совпал (версия в Mongo не старее), upsert
		// попытался вставить документ с уже занятым _id. Событие
		// устарело — это нормальный путь, не ошибка.
		if mongo.IsDuplicateKeyError(err) {
			return nil
		}

		return fmt.Errorf("replace document in %q: %w", collection, err)
	}

	return nil
}

func (r *ProjectionRepository) Delete(
	ctx context.Context,
	collection string,
	id int,
	version int,
) error {
	ctx, cancel := context.WithTimeout(ctx, r.client.OpTimeout())
	defer cancel()

	filter := bson.M{
		"_id":     id,
		"version": bson.M{"$lte": version},
	}

	_, err := r.client.Collection(collection).DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("delete document from %q: %w", collection, err)
	}

	return nil
}

func (r *ProjectionRepository) EnsureIndexes(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, r.client.OpTimeout())
	defer cancel()

	indexes := []mongo.IndexModel{
		{Keys: bson.D{{Key: "author_user_id", Value: 1}}},
		{Keys: bson.D{{Key: "completed", Value: 1}}},
	}

	_, err := r.client.Collection(CollectionTasks).Indexes().CreateMany(ctx, indexes)
	if err != nil {
		return fmt.Errorf("create %q indexes: %w", CollectionTasks, err)
	}

	return nil
}

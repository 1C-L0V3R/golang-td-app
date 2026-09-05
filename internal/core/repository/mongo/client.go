package core_mongo

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"go.mongodb.org/mongo-driver/mongo/readpref"
)

type Client struct {
	*mongo.Client

	database  string
	opTimeout time.Duration
}

func NewClient(
	ctx context.Context,
	config Config,
) (*Client, error) {
	client, err := mongo.Connect(
		ctx,
		options.Client().ApplyURI(config.URI()),
	)
	if err != nil {
		return nil, fmt.Errorf("connect mongo: %w", err)
	}

	if err := client.Ping(ctx, readpref.Primary()); err != nil {
		return nil, fmt.Errorf("mongo ping: %w", err)
	}

	return &Client{
		Client:    client,
		database:  config.Database,
		opTimeout: config.Timeout,
	}, nil
}

func (c *Client) Collection(name string) *mongo.Collection {
	return c.Client.Database(c.database).Collection(name)
}

func (c *Client) OpTimeout() time.Duration {
	return c.opTimeout
}

func (c *Client) Close(ctx context.Context) error {
	if err := c.Client.Disconnect(ctx); err != nil {
		return fmt.Errorf("disconnect mongo: %w", err)
	}

	return nil
}

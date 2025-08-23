package mongoutil

import (
	"PProject/tools/errs"
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func Check(ctx context.Context, config *Config) error {
	if err := config.ValidateAndSetDefaults(); err != nil {
		return err
	}

	clientOpts := options.Client().ApplyURI(config.Uri)
	mongoClient, err := mongo.Connect(ctx, clientOpts)
	if err != nil {
		return errs.WrapMsg(err, "MongoDB connect failed", "URI", config.Uri, "Database", config.Database, "MaxPoolSize", config.MaxPoolSize)
	}

	defer func() {
		if err := mongoClient.Disconnect(ctx); err != nil {
			_ = mongoClient.Disconnect(ctx)
		}
	}()

	if err = mongoClient.Ping(ctx, nil); err != nil {
		return errs.WrapMsg(err, "MongoDB ping failed", "URI", config.Uri, "Database", config.Database, "MaxPoolSize", config.MaxPoolSize)
	}

	return nil
}

// ValidateAndSetDefaults validates the configuration and sets default values.
func (c *Config) ValidateAndSetDefaults() error {
	if c.Uri == "" && len(c.Address) == 0 {
		return errs.Wrap(errs.New("either Uri or Address must be provided"))
	}
	if c.Database == "" {
		return errs.Wrap(errs.New("database is required"))
	}
	if c.MaxPoolSize <= 0 {
		c.MaxPoolSize = defaultMaxPoolSize
	}
	if c.MaxRetry <= 0 {
		c.MaxRetry = defaultMaxRetry
	}
	if c.Uri == "" {
		// if authSource is not provided, default to database name
		if c.AuthSource == "" {
			c.Uri = buildMongoURI(c, c.Database)
		} else {
			c.Uri = buildMongoURI(c, c.AuthSource)
		}
	}
	return nil
}

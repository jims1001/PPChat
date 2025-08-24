package mongoutil

import (
	"PProject/data/database/utils/tx"
	"PProject/tools/errs"
	"context"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

func init() {

}

// Config represents the MongoDB configuration.
type Config struct {
	Uri         string
	Address     []string
	Database    string
	Username    string
	Password    string
	AuthSource  string
	MaxPoolSize int
	MaxRetry    int
}

// 将 Config 应用到 ClientOptions
func applyConfigToOptions(cfg *Config) (*options.ClientOptions, error) {
	var opts *options.ClientOptions

	switch {
	case cfg.Uri != "":
		// 优先使用完整 URI（可含参数 ?authSource=admin 等）
		opts = options.Client().ApplyURI(cfg.Uri)
	case len(cfg.Address) > 0:
		// 其次使用地址列表
		opts = options.Client().SetHosts(cfg.Address)
	default:
		return nil, errs.New("mongo uri or address is required")
	}

	// 连接池
	opts.SetMaxPoolSize(uint64(cfg.MaxPoolSize))

	// 认证：若单独给了用户名/密码/来源，以代码优先覆盖 URI 中的认证（如有）
	if cfg.Username != "" {
		cred := options.Credential{
			Username:   cfg.Username,
			Password:   cfg.Password,
			AuthSource: cfg.AuthSource, // 若为空，ValidateAndSetDefaults 已设 admin
		}
		opts.SetAuth(cred)
	}

	// 也可在此添加更多可选项（按需开启）
	// opts.SetRetryWrites(true)
	// opts.SetServerSelectionTimeout(10 * time.Second)
	// opts.SetAppName("PProject")

	return opts, nil
}

type Client struct {
	tx tx.Tx
	db *mongo.Database
}

func (c *Client) GetDB() *mongo.Database {
	return c.db
}

func (c *Client) GetTx() tx.Tx {
	return c.tx
}

// NewMongoDB initializes a new MongoDB connection.
func NewMongoDB(ctx context.Context, config *Config) (*Client, error) {
	if err := config.ValidateAndSetDefaults(); err != nil {
		return nil, err
	}
	opts, _ := applyConfigToOptions(config)
	var (
		cli *mongo.Client
		err error
	)
	for i := 0; i < config.MaxRetry; i++ {
		cli, err = connectMongo(ctx, opts)
		if err != nil && shouldRetry(ctx, err) {
			time.Sleep(time.Second / 2)
			continue
		}
		break
	}
	if err != nil {
		return nil, errs.WrapMsg(err, "failed to connect to MongoDB", "URI", config.Uri)
	}

	cli, err = connectMongo(ctx, opts)
	mtx, err := NewMongoTx(ctx, cli)
	if err != nil {
		return nil, err
	}
	return &Client{
		tx: mtx,
		db: cli.Database(config.Database),
	}, nil
}

func connectMongo(ctx context.Context, opts *options.ClientOptions) (*mongo.Client, error) {
	cli, err := mongo.Connect(ctx, opts)
	if err != nil {
		return nil, err
	}
	if err := cli.Ping(ctx, nil); err != nil {
		return nil, err
	}
	return cli, nil
}

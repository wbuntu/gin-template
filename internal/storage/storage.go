package storage

import (
	"context"
	"log"
	"os"
	"time"

	"gitbub.com/wbuntu/gin-template/internal/pkg/config"
	"github.com/glebarez/sqlite"
	"github.com/go-redis/redis/v8"
	"github.com/pkg/errors"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	sharedDB   *gorm.DB
	sharedKVDB *redis.Client
)

func DB() *gorm.DB {
	return sharedDB
}

func KVDB() *redis.Client {
	return sharedKVDB
}

func Setup(ctx context.Context, cfg *config.Config) error {
	var err error
	// 初始化关系型数据库连接池
	if cfg.General.EnableDB {
		sharedDB, err = NewDB(ctx, cfg)
		if err != nil {
			return errors.Wrap(err, "NewDB")
		}
	}
	// 初始化键值数据库连接池
	if cfg.General.EnableKVDB {
		sharedKVDB, err = NewKVDB(ctx, cfg)
		if err != nil {
			return errors.Wrap(err, "NewKVDB")
		}
	}
	return nil
}

func NewDB(ctx context.Context, cfg *config.Config) (*gorm.DB, error) {
	var dialector gorm.Dialector
	switch cfg.DB.Type {
	case "sqlite":
		dialector = sqlite.Open(cfg.DB.DSN)
	case "mysql":
		dialector = mysql.Open(cfg.DB.DSN)
	default:
		return nil, errors.Errorf("unsupported db type: %s", cfg.DB.Type)
	}
	db, err := gorm.Open(dialector, &gorm.Config{
		Logger: logger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), logger.Config{
			SlowThreshold:             500 * time.Millisecond,
			LogLevel:                  logger.Warn,
			IgnoreRecordNotFoundError: true,
			Colorful:                  true,
		}),
	})
	if err != nil {
		return nil, errors.Wrap(err, "open db")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, errors.Wrap(err, "get db")
	}
	sqlDB.SetMaxIdleConns(cfg.DB.MinIdleConns)
	sqlDB.SetMaxOpenConns(cfg.DB.MaxActiveConns)
	sqlDB.SetConnMaxLifetime(cfg.DB.ConnLifetime)
	sqlDB.SetConnMaxIdleTime(cfg.DB.ConnIdletime)
	if err := sqlDB.PingContext(ctx); err != nil {
		return nil, errors.Wrap(err, "ping db")
	}
	return db, nil
}

func NewKVDB(ctx context.Context, cfg *config.Config) (*redis.Client, error) {
	redisOpt, err := redis.ParseURL(cfg.KVDB.DSN)
	if err != nil {
		return nil, errors.Wrap(err, "parse kv_db dsn")
	}
	redisOpt.MinIdleConns = cfg.KVDB.MinIdleConns
	redisOpt.PoolSize = cfg.KVDB.MaxActiveConns
	redisOpt.MaxConnAge = cfg.KVDB.ConnLifetime
	redisOpt.IdleTimeout = cfg.KVDB.ConnIdletime
	db := redis.NewClient(redisOpt)
	if _, err := db.Ping(ctx).Result(); err != nil {
		return nil, errors.Wrap(err, "ping kv_db")
	}
	return db, nil
}

type Filter struct {
	Query interface{}
	Args  []interface{}
}

package storage

import (
	"context"

	"gitbub.com/wbuntu/gin-template/internal/pkg/config"
	"github.com/go-gormigrate/gormigrate/v2"
	"github.com/pkg/errors"
)

func Migrate(ctx context.Context, cfg *config.Config) error {
	if !cfg.General.EnableDB {
		return nil
	}
	// TODO：使用redis或其他中间件创建一个分布式锁，确保迁移动作只被一个Pod执行
	// gorm 的 AutoMigrate 只创建或更新数据库表结构
	if err := DB().AutoMigrate(
		&ResourceID{},
		&Cluster{},
		&Task{},
		&TaskLog{},
	); err != nil {
		return errors.Wrap(err, "migrate model")
	}
	// 使用 gormigrate 修改或更新数据
	if len(migrations) == 0 {
		return nil
	}
	migrationsOptions := gormigrate.DefaultOptions
	migrationsOptions.UseTransaction = true
	m := gormigrate.New(DB(), migrationsOptions, migrations)
	if err := m.Migrate(); err != nil {
		return errors.Wrap(err, "migrate data")
	}
	return nil
}

// migrations 用年月日时分秒作为ID，每个迁移动作都在事务中执行，执行成功一次后记录到数据库，不再执行
var migrations = []*gormigrate.Migration{}

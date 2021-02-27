package conn

import (
	"github.com/go-redis/redis"
	"github.com/pkg/errors"
)

type RedisConf struct {
	Uri  string `yaml:"uri" json:"uri"`
	Auth string `yaml:"auth" json:"auth"`
	Db   int    `yaml:"db" json:"db"`
}

func DialRedis(cnf *RedisConf) (*redis.Client, error) {
	if cnf == nil {
		return nil, errors.New("redis 配置不存在")
	}

	cli := redis.NewClient(&redis.Options{
		Addr:     cnf.Uri,
		DB:       cnf.Db,
		Password: cnf.Auth,
	})

	return cli, nil
}

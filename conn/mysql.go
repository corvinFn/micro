package conn

import (
	"time"

	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"github.com/pkg/errors"
)

type MysqlConf struct {
	Driver      string        `yaml:"driver" json:"driver"`
	Uri         string        `yaml:"uri" json:"uri"`
	MaxOpenConn int           `yaml:"max_open_conn" json:"max_open_conn"`
	MaxIdleConn int           `yaml:"max_idle_conn" json:"max_idle_conn"`
	MaxLifetime time.Duration `yaml:"max_life_time" json:"max_life_time"`
	DbLogEnable bool          `yaml:"db_log_enable" json:"db_log_enable"`
}

func DialMysql(cnf *MysqlConf) (*gorm.DB, error) {
	if cnf == nil {
		return nil, errors.New("数据库配置不存在")
	}

	db, err := gorm.Open(cnf.Driver, cnf.Uri)
	if err != nil {
		return nil, errors.New("连接数据库失败")
	}

	db.DB().SetMaxIdleConns(cnf.MaxIdleConn)
	db.DB().SetMaxOpenConns(cnf.MaxOpenConn)
	db.DB().SetConnMaxLifetime(cnf.MaxLifetime)
	db.LogMode(cnf.DbLogEnable)

	return db, nil
}

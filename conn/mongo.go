package conn

import (
	"errors"
	"fmt"
	"strings"

	"github.com/globalsign/mgo"
)

type MongoConf struct {
	Uri []string `yaml:"uri" json:"uri"`
}

func DialMongo(cnf *MongoConf) (*mgo.Session, error) {
	if cnf == nil {
		return nil, errors.New("mongo 配置不存在")
	}

	var mgoUrl string
	if len(cnf.Uri) > 0 {
		mgoUrl = fmt.Sprintf("mongodb://%s?connect=direct", strings.Join(cnf.Uri, ","))
	}
	session, err := mgo.Dial(mgoUrl)
	if err != nil {
		return nil, err
	}

	return session.Copy(), nil
}

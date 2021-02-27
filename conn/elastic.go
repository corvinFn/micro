package conn

import (
	"errors"
	"time"

	"github.com/olivere/elastic"
)

type ElasticConf struct {
	Uris     []string `yaml:"uris" json:"uris"`
	Username string   `yaml:"username" json:"username"`
	Password string   `yaml:"password" json:"password"`
}

func DialElastic(cnf *ElasticConf) (*elastic.Client, error) {
	if cnf == nil {
		return nil, errors.New("elastic 配置为空")
	}
	client, err := elastic.NewClient(
		elastic.SetURL(cnf.Uris...),
		elastic.SetSniff(false),
		elastic.SetHealthcheckInterval(10000*time.Millisecond),
		elastic.SetBasicAuth(cnf.Username, cnf.Password),
	)
	return client, err
}

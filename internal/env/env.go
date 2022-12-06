package env

import (
	"sync"

	"github.com/spf13/viper"
)

type Config struct {
	KafkaConfig KafkaConfig

	AgrPubKey        string
	ExecutionAddress string
	ForkVersion      string
	CheckSignature   bool
}

type KafkaConfig struct {
	Host    string
	Topic   string
	Creds   string
	GroupID string
}

var (
	cfg Config

	onceDefaultClient sync.Once
)

func ReadEnv() (*Config, error) {
	var err error

	onceDefaultClient.Do(func() {
		viper.SetConfigFile(".env.yml")

		viper.AutomaticEnv()
		if viperErr := viper.ReadInConfig(); err != nil {
			if _, ok := viperErr.(viper.ConfigFileNotFoundError); !ok {
				err = viperErr
				return
			}
		}

		cfg = Config{
			KafkaConfig: KafkaConfig{
				Host:    viper.GetString("kafka.host"),
				Topic:   viper.GetString("kafka.topic"),
				Creds:   viper.GetString("kafka.creds"),
				GroupID: viper.GetString("kafka.group_id"),
			},
			AgrPubKey:        viper.GetString("app.aggregated_bls_pub_key"),
			ExecutionAddress: viper.GetString("app.execution_address"),
			ForkVersion:      viper.GetString("app.fork_version"),
			CheckSignature:   viper.GetBool("app.check_signature"),
		}
	})

	return &cfg, err
}

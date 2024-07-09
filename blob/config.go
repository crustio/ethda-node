package blob

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/0xPolygonHermez/zkevm-node/config/types"
	"github.com/0xPolygonHermez/zkevm-node/log"
	"github.com/mitchellh/mapstructure"
	"github.com/spf13/viper"
	"github.com/urfave/cli/v2"
)

const (
	// FlagCfg is the flag for cfg.
	FlagCfg = "zkcfg"
)

// Config provide fields to configure the blob
type ZkBlobSenderConfig struct {
	// WaitPeriodSendZkblob is the time the zkblob sender waits until
	// trying to send Zkblob to L1
	WaitPeriodSendBlob types.Duration `mapstructure:"WaitPeriodSendBlob"`

	WaitAfterBlobSent types.Duration `mapstructure:"WaitAfterBlobSent"`

	// GasOffset
	GasOffset uint64 `mapstructure:"GasOffset"`

	// blob to address hex string
	DasAddress string `mapstructure:"DasAddress"`

	// zkblob contract address hex string
	ZkBlobAddress string `mapstructure:"ZkBlobAddress"`

	// zkblob sender private key
	PrivateKey types.KeystoreFileConfig `mapstructure:"PrivateKey"`
}

type Config struct {
	ZkBlobSender ZkBlobSenderConfig `mapstructure:"ZkBlobSender"`
}

// Default parses the default configuration values.
func Default() (*Config, error) {
	var cfg Config
	viper.SetConfigType("toml")

	err := viper.ReadConfig(bytes.NewBuffer([]byte(DefaultValues)))
	if err != nil {
		return nil, err
	}
	err = viper.Unmarshal(&cfg, viper.DecodeHook(mapstructure.TextUnmarshallerHookFunc()))
	if err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Load loads the configuration
func Load(ctx *cli.Context) (*Config, error) {
	cfg, err := Default()
	if err != nil {
		return nil, err
	}
	configFilePath := ctx.String(FlagCfg)
	if configFilePath != "" {
		dirName, fileName := filepath.Split(configFilePath)

		fileExtension := strings.TrimPrefix(filepath.Ext(fileName), ".")
		fileNameWithoutExtension := strings.TrimSuffix(fileName, "."+fileExtension)

		viper.AddConfigPath(dirName)
		viper.SetConfigName(fileNameWithoutExtension)
		viper.SetConfigType(fileExtension)
	}
	viper.AutomaticEnv()
	replacer := strings.NewReplacer(".", "_")
	viper.SetEnvKeyReplacer(replacer)
	viper.SetEnvPrefix("ZKEVM_NODE")
	err = viper.ReadInConfig()
	if err != nil {
		_, ok := err.(viper.ConfigFileNotFoundError)
		if ok {
			log.Infof("config file not found")
		} else {
			log.Infof("error reading config file: ", err)
			return nil, err
		}
	}

	decodeHooks := []viper.DecoderConfigOption{
		// this allows arrays to be decoded from env var separated by ",", example: MY_VAR="value1,value2,value3"
		viper.DecodeHook(mapstructure.ComposeDecodeHookFunc(mapstructure.TextUnmarshallerHookFunc(), mapstructure.StringToSliceHookFunc(","))),
	}

	err = viper.Unmarshal(&cfg, decodeHooks...)
	if err != nil {
		return nil, err
	}

	fmt.Println("-------------> ", *cfg)

	return cfg, nil
}

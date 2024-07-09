package main

import "github.com/0xPolygonHermez/zkevm-node/config/types"

// Config provide fields to configure the blob
type Config struct {
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
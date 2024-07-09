package main

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"

	"github.com/0xPolygonHermez/zkevm-node/blob"
	"github.com/0xPolygonHermez/zkevm-node/blob/solidity"
	"github.com/0xPolygonHermez/zkevm-node/blob/zkblobsender/smartcontracts/zkblob"
	"github.com/0xPolygonHermez/zkevm-node/etherman/types"
	"github.com/0xPolygonHermez/zkevm-node/ethtxmanager"
	"github.com/0xPolygonHermez/zkevm-node/log"
	"github.com/0xPolygonHermez/zkevm-node/state"
	"github.com/ethereum/go-ethereum/accounts/abi/bind"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/ethclient"
)

const (
	monitoredBatchIDFormat = "batch-zkblob-from-%v-to-%v"
	disableZkBlob          = false
)

var (
	ErrAuthorizationNotFound = errors.New("authorization not found")
	ErrNoZkBlobsToSend       = errors.New("no zkblobs to send")
)

type ZkblobETHClient struct {
	chainID  uint64
	auth     map[common.Address]*bind.TransactOpts // empty in case of read-only client
	authKeys map[common.Address]*ecdsa.PrivateKey  // TODO

	zkBlob         *zkblob.Zkblob
	zkBlobDeployed bool

	blobCfg *blob.ZkBlobSenderConfig

	zkblobSenderAddress common.Address // read from zkBlob private key

	etherman ethermanInterface
}

func NewClient(etherman ethermanInterface, chainID uint64, targetL1URL string, cfg *blob.ZkBlobSenderConfig, blobSenderKeystorePath, blobSenderKeystorePassword string) (*ZkblobETHClient, error) {
	// Connect to ethereum node
	ethClient, err := ethclient.Dial(targetL1URL)
	if err != nil {
		log.Errorf("error connecting to %s: %+v", targetL1URL, err)
		return nil, err
	}

	// if zkblob deployed ?
	zkBlobCode, err := ethClient.CodeAt(context.Background(), common.HexToAddress(cfg.ZkBlobAddress), nil)
	if err != nil {
		return nil, err
	}
	zkBlobDeployed := true
	if len(zkBlobCode) == 0 {
		log.Warnf("zkblob not deployed at %s", cfg.ZkBlobAddress)
		zkBlobDeployed = false // not deployed
	}

	// Create zkblob client
	zkBlob, err := zkblob.NewZkblob(common.HexToAddress(cfg.ZkBlobAddress), ethClient)
	if err != nil {
		log.Errorf("error creating Polygonzkblob client (%s). Error: %w", cfg.ZkBlobAddress, err)
		return nil, err
	}

	z := &ZkblobETHClient{
		chainID:        chainID,
		zkBlob:         zkBlob,
		auth:           make(map[common.Address]*bind.TransactOpts),
		authKeys:       make(map[common.Address]*ecdsa.PrivateKey),
		zkBlobDeployed: zkBlobDeployed,
		blobCfg:        cfg,

		etherman: etherman,
	}

	// load auth for zkblob-sender
	_, err = z.loadAuthFromKeyStore(blobSenderKeystorePath, blobSenderKeystorePassword)
	if err != nil {
		return nil, err
	}

	return z, nil
}

func (zkc *ZkblobETHClient) HandleSequenceToZkBlob(ctx context.Context, sequences []types.Sequence, ethTxManagerOwner string, etherman ethermanInterface, ethTxManager ethTxManager, GasOffset uint64) error {
	senderAddress := zkc.zkblobSenderAddress // sender address

	if disableZkBlob {
		return nil
	}
	if !zkc.zkBlobDeployed { // not deployed, no need to handle
		log.Warnf("HandleSequenceToZkBlob: zkblob not deployed at %s", zkc.blobCfg.ZkBlobAddress)
		return fmt.Errorf("zkblob not deployed at %s", zkc.blobCfg.ZkBlobAddress)
	}
	if len(sequences) == 0 {
		log.Info("HandleSequenceToZkBlob: no sequences to handle")
		return nil
	}

	batchNums := []int64{}
	txHashes := [][]common.Hash{}
	for _, seq := range sequences {
		hashes := []common.Hash{}
		brb, err := state.DecodeBatchV2(seq.BatchL2Data)
		if err != nil {
			log.Errorf("error decode BatchL2Data for batch %d, err: %v", seq.BatchNumber, err)
			continue
		}

		for _, btx := range brb.Blocks {
			for _, tx := range btx.Transactions {
				hashes = append(hashes, tx.Tx.Hash())
			}
		}

		if len(hashes) == 0 {
			continue
		}
		batchNums = append(batchNums, int64(seq.BatchNumber))
		txHashes = append(txHashes, hashes)
	}

	if len(batchNums) > 0 {
		to, data, err := zkc.BuildPostZkBlobTxData(senderAddress, batchNums, txHashes)
		if err != nil {
			log.Errorf("error build postZkBlob tx data, batch number from %d to %d : %v", batchNums[0], batchNums[len(batchNums)-1], err)
			return fmt.Errorf("error build postZkBlob tx data, batch number from %d to %d : %v", batchNums[0], batchNums[len(batchNums)-1], err)
		}

		monitoredTxID := fmt.Sprintf(monitoredBatchIDFormat, batchNums[0], batchNums[len(batchNums)-1])
		err = ethTxManager.Add(ctx, ethTxManagerOwner, monitoredTxID, senderAddress, to, nil, data, GasOffset, nil)
		if err != nil {
			mTxLogger := ethtxmanager.CreateLogger(ethTxManagerOwner, monitoredTxID, senderAddress, to)
			mTxLogger.Errorf("error to add postZkBlob tx to eth tx manager: ", err)
			return fmt.Errorf("error to add postZkBlob tx to eth tx manager: %v", err)
		}
	} else {
		log.Infof("no batch to send to zkblob at sequence-from-%d-to-%d", sequences[0].BatchNumber, sequences[len(sequences)-1].BatchNumber)
		return ErrNoZkBlobsToSend
	}

	return nil
}

// BuildPostZkBlobTxData builds a []bytes to be sent to the PoE SC method PostZkBlob.
func (zkc *ZkblobETHClient) BuildPostZkBlobTxData(sender common.Address, batchNumbers []int64, hashes [][]common.Hash) (to *common.Address, data []byte, err error) {

	roots, err := buildTreeAndGetRoots(hashes)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to build tree and get roots: %s", err.Error())
	}
	if len(roots) != len(batchNumbers) {
		return nil, nil, fmt.Errorf("batchNumbers and roots have different length: %d, %d", len(roots), len(batchNumbers))
	}

	// get bytestosign
	bytesToSign, err := solidity.Keccak256(solidity.ArrayPack(parseBatchNumbers(batchNumbers), parseRoots(roots)))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to Keccak256 batchNumber and root: %s", err.Error())
	}

	sig, err := solidity.Sign(bytesToSign, zkc.authKeys[sender]) // authKeys is loaded from keystore
	if err != nil {
		return nil, nil, fmt.Errorf("failed to Sign batchNumber and root: %s", err.Error())
	}

	opts, err := zkc.getAuthByAddress(sender)
	if err == ErrAuthorizationNotFound {
		return nil, nil, err
	}
	opts.NoSend = true
	// force nonce, gas limit and gas price to avoid querying it from the chain
	opts.Nonce = big.NewInt(1)
	opts.GasLimit = uint64(1)
	opts.GasPrice = big.NewInt(1)

	tx, err := zkc.zkBlob.PostBatchs(opts, parseBatchNumbers(batchNumbers), parseRoots(roots), sig)
	if err != nil {
		return nil, nil, err
	}

	// log
	log.Infof("Build Batch PostZkBlob transaction, batch number from %d to %d, to zkblob address: %s", batchNumbers[0], batchNumbers[len(batchNumbers)-1], zkc.blobCfg.ZkBlobAddress)

	return tx.To(), tx.Data(), nil
}

func (zkc *ZkblobETHClient) loadAuthFromKeyStore(path, password string) (*bind.TransactOpts, error) {
	auth,  key, err := zkc.etherman.LoadAuthFromKeyStore(path, password)
	if err != nil {
		return nil, err
	}

	// set auth
	zkc.auth[auth.From] = auth
	// set sender address
	zkc.zkblobSenderAddress = auth.From
	// set keys
	zkc.authKeys[auth.From] = key

	return auth, nil
}

// getAuthByAddress tries to get an authorization from the authorizations map
func (zkc *ZkblobETHClient) getAuthByAddress(addr common.Address) (*bind.TransactOpts, error) {
	auth, found := zkc.auth[addr]
	if !found {
		return &bind.TransactOpts{}, ErrAuthorizationNotFound
	}
	return auth, nil
}

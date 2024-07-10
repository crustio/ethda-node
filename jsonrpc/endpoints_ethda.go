package jsonrpc

import (
	"context"
	"crypto/ecdsa"
	"errors"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"strings"

	"github.com/0xPolygonHermez/zkevm-node/blob"
	"github.com/0xPolygonHermez/zkevm-node/jsonrpc/types"
	"github.com/0xPolygonHermez/zkevm-node/log"
	"github.com/ethereum/go-ethereum/accounts/keystore"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/crypto"
)

const (
	APIETHDA = "ethda"
)

// ETHDAEndpoints contains implementations for the "ethda" RPC endpoints
type ETHDAEndpoints struct {
	sk    *ecdsa.PrivateKey
	state types.StateInterface
}

// NewETHDAEndpoints returns ETHDAEndpoints
func NewETHDAEndpoints(skPath, skPassword string, st types.StateInterface) *ETHDAEndpoints {
	keystoreEncrypted, err := os.ReadFile(filepath.Clean(skPath))
	if err != nil {
		panic(err)
	}
	key, err := keystore.DecryptKey(keystoreEncrypted, skPassword)
	if err != nil {
		panic(err)
	}

	return &ETHDAEndpoints{
		sk:    key.PrivateKey,
		state: st,
	}
}

func (e *ETHDAEndpoints) GetProofByHash(hash types.ArgHash) (interface{}, types.Error) {
	ctx := context.Background()

	receipt, err := e.state.GetTransactionReceipt(ctx, hash.Hash(), nil)
	if err != nil {
		return RPCErrorResponse(types.DefaultErrorCode, fmt.Sprintf("couldn't load receipt for tx %v", hash.Hash().String()), err, true)
	}

	block, err := e.state.GetL2BlockByNumber(ctx, receipt.BlockNumber.Uint64(), nil)
	if err != nil {
		return RPCErrorResponse(types.DefaultErrorCode, fmt.Sprintf("couldn't load batch for tx %v", hash.Hash().String()), err, true)
	}

	hashes := []common.Hash{}
	for _, tx := range block.Transactions() {
		hashes = append(hashes, tx.Hash())
	}

	if len(hashes) == 0 {
		return RPCErrorResponse(types.DefaultErrorCode, "no hashes in batch", errors.New("no hashes in batch"), true)
	}

	tree, er := blob.NewMerkleTree(blob.ModeProofGen, hashes...)
	if er != nil {
		return RPCErrorResponse(types.DefaultErrorCode, "failed create merkle tree by hashes", er, true)
	}

	proof := &blob.ProofInfo{
		BatchNumber: receipt.BlockNumber.Uint64(),
		Proof:       []common.Hash{},
	}

	hashIndex := -1
	leafs := tree.Leaves
	for i := 0; i < len(leafs); i++ {
		if hash.Hash() == common.BytesToHash(leafs[i]) {
			hashIndex = i
		}
	}

	if hashIndex == -1 { // hash not found
		errStr := "hash not found: " + hash.Hash().Hex()
		return RPCErrorResponse(types.DefaultErrorCode, errStr, errors.New(errStr), true)
	}

	for _, h := range tree.Proofs[hashIndex].Siblings {
		proof.Proof = append(proof.Proof, common.BytesToHash(h))
	}

	log.Debugf("get proof of batch number: %d, result hash: %s, result proof: %v", receipt.BlockNumber, hash.Hash().Hex(), proof.Proof)

	return proof, nil
}

func (e *ETHDAEndpoints) SignBatchHash(hash types.ArgHash) (interface{}, types.Error) {
	log.Infof("Sign batch hash: %v", hash.Hash().Hex())
	sig, err := crypto.Sign(hash[:], e.sk)
	if err != nil {
		return RPCErrorResponse(types.DefaultErrorCode, "failed to sign batch hash", err, true)
	}

	rBytes := sig[:32]
	sBytes := sig[32:64]
	vByte := sig[64]

	if strings.ToUpper(common.Bytes2Hex(sBytes)) > "7FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF5D576E7357A4501DDFE92F46681B20A0" {
		magicNumber := common.Hex2Bytes("FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFEBAAEDCE6AF48A03BBFD25E8CD0364141")
		sBig := big.NewInt(0).SetBytes(sBytes)
		magicBig := big.NewInt(0).SetBytes(magicNumber)
		s1 := magicBig.Sub(magicBig, sBig)
		sBytes = s1.Bytes()
		if vByte == 0 {
			vByte = 1
		} else {
			vByte = 0
		}
	}
	vByte += 27

	actualSignature := []byte{}
	actualSignature = append(actualSignature, rBytes...)
	actualSignature = append(actualSignature, sBytes...)
	actualSignature = append(actualSignature, vByte)

	log.Infof("Sign batch hash result: %v", sig)

	return common.Bytes2Hex(actualSignature), nil
}

package blobjsonrpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/0xPolygonHermez/zkevm-node/blob/db"
	"github.com/0xPolygonHermez/zkevm-node/jsonrpc/metrics"
	"github.com/0xPolygonHermez/zkevm-node/log"
	"github.com/0xPolygonHermez/zkevm-node/state"
	"github.com/ethereum/go-ethereum/common/hexutil"
	"github.com/gorilla/mux"
	"github.com/prysmaticlabs/prysm/v4/encoding/bytesutil"
	ethpb "github.com/prysmaticlabs/prysm/v4/proto/prysm/v1alpha1"
	"github.com/syndtr/goleveldb/leveldb"
	"net/http"
	"strconv"
	"syscall"

	"github.com/ethereum/go-ethereum/common"
)

type BlobServer struct {
	st     *state.State
	blobDB db.BlobDB
}

func NewBlobServer(st *state.State, blobDB db.BlobDB) *BlobServer {
	return &BlobServer{
		st:     st,
		blobDB: blobDB,
	}
}

type SidecarsResponse struct {
	Data []*Sidecar `json:"data"`
}

type Sidecar struct {
	Index         string `json:"index"`
	Slot          string `json:"slot"`
	ProposerIndex string `json:"proposer_index"`
	Blob          string `json:"blob"`
	KZGCommitment string `json:"kzg_commitment"`
	KZGProof      string `json:"kzg_proof"`
}

func BuildSidecardsResponse(sidecars []*ethpb.BlobSidecar) *SidecarsResponse {
	resp := &SidecarsResponse{Data: make([]*Sidecar, len(sidecars))}
	for i, sc := range sidecars {
		resp.Data[i] = &Sidecar{
			Index:         strconv.FormatUint(sc.Index, 10),
			Blob:          hexutil.Encode(sc.Blob),
			KZGCommitment: hexutil.Encode(sc.KzgCommitment),
			KZGProof:      hexutil.Encode(sc.KzgProof),
		}
	}

	return resp
}

func handleError(w http.ResponseWriter, err error) {
	defer metrics.RequestHandled(metrics.RequestHandledLabelError)

	if errors.Is(err, syscall.EPIPE) {
		// if it is a broken pipe error, return
		return
	}

	// if it is a different error, write it to the response
	log.Errorf("Error processing request: %v", err)
	http.Error(w, err.Error(), http.StatusInternalServerError)
}

func (bs *BlobServer) HandleGetBlocks(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	blockId := vars["block_id"]
	var height uint64
	var err error

	switch blockId {
	case "head":
		height, err = bs.st.GetLastL2BlockNumber(context.Background(), nil)
		if err != nil {
			handleError(w, fmt.Errorf("could not decode block ID: %s", blockId))
			return
		}
	case "finalized":
		syncInfo, err := bs.st.GetSyncingInfo(context.Background(), nil)
		if err != nil {
			handleError(w, fmt.Errorf("could not decode block ID: %s", blockId))
			return
		}
		height = syncInfo.CurrentBlockNumber
	default:
		if bytesutil.IsHex([]byte(blockId)) {
			height, err = strconv.ParseUint(blockId, 16, 64)
			if err != nil {
				handleError(w, fmt.Errorf("could not decode block ID into hex: %s", blockId))
				return
			}
		} else {
			height, err = strconv.ParseUint(blockId, 10, 64)
			if err != nil {
				handleError(w, fmt.Errorf("could not decode block ID: %s", blockId))
				return
			}
		}
	}
	block, err := bs.st.GetL2BlockByNumber(context.Background(), height, nil)
	if err != nil {
		handleError(w, fmt.Errorf("could not retrieve block for height: %s,%d", blockId, height))
		return
	}

	blobH := ""
	for i := 0; i < len(block.Transactions()); i++ {
		tx := block.Transactions()[i]

		existed, err := bs.blobDB.IsBlob(context.Background(), tx.Hash())
		if err != nil {
			handleError(w, fmt.Errorf("failed to check blob tx from leveldb: %s=>%d", blockId, height))
			return
		}
		if existed {
			newTx, err := bs.blobDB.GetBlobTx(context.Background(), tx.Hash())
			if err != nil && !errors.Is(err, leveldb.ErrNotFound) {
				handleError(w, fmt.Errorf("failed to load blob tx from leveldb", blockId, height))
				return
			}
			blobH = common.Bytes2Hex(newTx.BlobTxSidecar().Commitments[0][:])
		}
	}

	var blobCs []string
	if blobH != "" {
		blobCs = append(blobCs, blobH)
	}

	res := map[string]any{
		"version": "capella",
		"data": map[string]any{
			"message": map[string]any{
				"slot": block.Header().Number.String(),
				"body": map[string]any{
					"execution_payload": map[string]any{
						"block_hash": block.Hash().Hex(),
					},
					"blob_kzg_commitments": blobCs,
				},
			},
		},
	}

	respBytes, _ := json.Marshal(res)
	_, err = w.Write(respBytes)
	if err != nil {
		log.Error(err)
		return
	}
}

func (bs *BlobServer) HandleGetBlobSidecars(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	blockId := vars["block_id"]

	var height uint64
	var err error
	if bytesutil.IsHex([]byte(blockId)) {
		height, err = strconv.ParseUint(blockId, 16, 64)
		if err != nil {
			handleError(w, fmt.Errorf("could not decode block ID into hex: %s", blockId))
			return
		}
	} else {
		height, err = strconv.ParseUint(blockId, 10, 64)
		if err != nil {
			handleError(w, fmt.Errorf("could not decode block ID: %s", blockId))
			return
		}
	}
	block, err := bs.st.GetL2BlockByNumber(context.Background(), height, nil)
	if err != nil {
		handleError(w, fmt.Errorf("could not retrieve blobs for height: %s", blockId))
		return
	}

	var bss []*ethpb.BlobSidecar
	txs := block.Transactions()
	for _, tx := range txs {
		existed, err := bs.blobDB.IsBlob(context.Background(), tx.Hash())
		if err != nil {
			handleError(w, fmt.Errorf("failed to check blob tx from leveldb: %s=>%d", blockId, height))
			return
		}
		if !existed {
			continue
		}
		newTx, err := bs.blobDB.GetBlobTx(context.Background(), tx.Hash())
		if err != nil {
			handleError(w, fmt.Errorf("failed to load blob tx from leveldb: %s=>%d", blockId, height))
			return
		}

		for i, _ := range newTx.BlobTxSidecar().Blobs {
			bsc := &ethpb.BlobSidecar{
				Index:         uint64(i),
				Blob:          newTx.BlobTxSidecar().Blobs[i][:],
				KzgCommitment: newTx.BlobTxSidecar().Commitments[i][:],
				KzgProof:      newTx.BlobTxSidecar().Proofs[i][:],
			}

			bss = append(bss, bsc)
		}
	}

	res := BuildSidecardsResponse(bss)

	respBytes, _ := json.Marshal(res)
	_, err = w.Write(respBytes)
	if err != nil {
		log.Error(err)
		return
	}
}

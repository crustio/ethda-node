package blobtypes

import "github.com/ethereum/go-ethereum/crypto/kzg4844"

type BlobTxSidecar struct {
	Blobs       []kzg4844.Blob       `json:"blobs"`       // Blobs needed by the blob pool
	Commitments []kzg4844.Commitment `json:"commitments"` // Commitments needed by the blob pool
	Proofs      []kzg4844.Proof      `json:"proofs"`      // Proofs needed by the blob pool
}

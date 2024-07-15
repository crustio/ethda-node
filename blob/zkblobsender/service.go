// sync sequences to blobs and send blobs to L1
// sequences-n-to-m will save to blobs when blobs is sent to L1
// ┌────────────┐         ┌─────────────────┐
// │   blobs    │         │ state.sequences │
// └────────────┘         └─────────────────┘
//  from │  to                from │  to
//       │                         │
//       │                     1   │  1  -> Note. from-1-to-1 no need to be sent, skip it
//       │                         │
//   2   │  2  <────────────   2   │  2
//       │                         │
//   3   │  5  <────────────   3   │  5
//       │                         │
//       │     <────────────   6   │  9
//       │                         │
//       │                         │
//       │                         │
//       │     <────folow───  1102 │ 1106
//       │                         │
//       ▼                         ▼

package main

import (
	"context"
	"fmt"
	"time"

	"github.com/0xPolygonHermez/zkevm-node/etherman/types"
	"github.com/0xPolygonHermez/zkevm-node/log"

	"github.com/0xPolygonHermez/zkevm-node/blob"
	"github.com/0xPolygonHermez/zkevm-node/blob/db"
	"github.com/0xPolygonHermez/zkevm-node/ethtxmanager"
	"github.com/0xPolygonHermez/zkevm-node/event"
	"github.com/jackc/pgx/v4"
)

const (
	ethTxManagerOwner = "zkblob"
	monitoredIDFormat = "zkblob-from-%v-to-%v"

	initialSequenceFromBatchNumber = 1
)

// ZkblobSender represents a zlblob sender
type ZkblobSender struct {
	cfg          *blob.ZkBlobSenderConfig
	state        stateInterface
	ethTxManager ethTxManager
	etherman     ethermanInterface
	eventLog     *event.EventLog
	zkBlobClient *ZkblobETHClient

	// blob db
	blobDB db.BlobDB

	// queue
	seqQueue *SequenceQueue

	// last seq-id
	lastSequenceInitialBatch uint64
	lastSequenceEndBatch     uint64
}

// New inits zkblob sender
func New(cfg *blob.ZkBlobSenderConfig, state stateInterface, etherman ethermanInterface, manager ethTxManager, eventLog *event.EventLog, zkBlobClient *ZkblobETHClient) (*ZkblobSender, error) {
	sqliteDB, err := db.NewBlobDB("/blob/sqlite.db")
	if err != nil {
		panic(err)
	}

	return &ZkblobSender{
		cfg:          cfg,
		state:        state,
		etherman:     etherman,
		ethTxManager: manager,
		eventLog:     eventLog,
		zkBlobClient: zkBlobClient,

		blobDB:   sqliteDB,
		seqQueue: NewSequenceQueue(),
	}, nil
}

// Start starts the zkblob sender
func (s *ZkblobSender) Start(ctx context.Context) {
	for {
		s.tryToSendZkblob(ctx)
	}
}

func (s *ZkblobSender) currentSequenceDone() {
	// delete current and move to next
	s.seqQueue.Dequeue()
}

func (s *ZkblobSender) tryToSendZkblob(ctx context.Context) {

	retry := false
	// process monitored zkblob before starting a next cycle
	s.ethTxManager.ProcessPendingMonitoredTxs(ctx, ethTxManagerOwner, func(result ethtxmanager.MonitoredTxResult, dbTx pgx.Tx) {
		if result.Status == ethtxmanager.MonitoredTxStatusConfirmed {
			// check blobs in blobDB
			has, err := s.blobDB.HasFrom(s.lastSequenceInitialBatch)
			if err != nil {
				log.Warnf("error checking blobs in blobDB: %v", err)
				retry = true
			}
			if !has {
				// retry
				log.Infof("adding blobs record to blobDB: blobs-from-%v-to-%v", s.lastSequenceInitialBatch, s.lastSequenceEndBatch)

				err = s.blobDB.AddZkBlob(s.lastSequenceInitialBatch, s.lastSequenceEndBatch)
				if err != nil {
					log.Fatalf("error adding blobs record to blobDB: %v", err)
				} else {
					// delete current and move to next
					s.currentSequenceDone()

					time.Sleep(s.cfg.WaitAfterBlobSent.Duration)
				}
			} else {
				log.Errorf("blobs-from-%v-to-%v already in blobDB, this case should not happen", s.lastSequenceInitialBatch, s.lastSequenceEndBatch)
			}
		} else { // Monitored tx is failed
			retry = true
			mTxResultLogger := ethtxmanager.CreateMonitoredTxResultLogger(ethTxManagerOwner, result)
			mTxResultLogger.Errorf("failed to send zkblob: %v", result.ID)
		}
	}, nil)

	if retry {
		return
	}

	time.Sleep(s.cfg.WaitPeriodSendBlob.Duration)

	err := s.getSequenceBlobsToSend(ctx)
	if err != nil {
		log.Warnf("error getting sequences: %v", err)
	}

	log.Infof("get %v sequences in queue", s.seqQueue.Len())

	// enter cycle
	currentSeq := s.seqQueue.Front()
	if currentSeq == nil {
		log.Infof("no sequences in queue")
		return
	}

	// check if the current sequence is already in the blobDB
	has, err := s.blobDB.HasFrom(currentSeq.From)
	if err != nil {
		log.Fatalf("error checking blobs in blobDB: %v", err)
		return
	}
	if has {
		// delete current and move to next
		log.Infof("blobs-from-%v-to-%v already in blobDB, skipping", currentSeq.From, currentSeq.To)
		s.currentSequenceDone()
		return
	}

	// send to L1
	err = s.sendZkblob(ctx, currentSeq)
	if err != nil {
		if err == ErrNoZkBlobsToSend {
			log.Infof("adding blobs record to blobDB: blobs-from-%v-to-%v", s.lastSequenceInitialBatch, s.lastSequenceEndBatch)
			err = s.blobDB.AddZkBlob(s.lastSequenceInitialBatch, s.lastSequenceEndBatch)
			if err != nil {
				log.Fatalf("error adding blobs record to blobDB: %v", err)
			} else {
				// delete current and move to next
				s.currentSequenceDone()

				time.Sleep(s.cfg.WaitAfterBlobSent.Duration)
			}
		} else {
			log.Errorf("error sending zkblob: %v", err)
			return // retry current cycle
		}
	}
}

func (s *ZkblobSender) sendZkblob(ctx context.Context, bSeq *BlobSequence) error {
	s.lastSequenceInitialBatch = bSeq.From
	s.lastSequenceEndBatch = bSeq.To

	sequences := []types.Sequence{}

	for i := bSeq.From; i <= bSeq.To; i++ {
		batch, err := s.state.GetBatchByNumber(ctx, i, nil)
		if err != nil {
			// if err == state.ErrNotFound {
			// 	break
			// }
			log.Debugf("failed to get batch by number %d, err: %w", i, err)
			return err
		}
		seq := types.Sequence{ // HandleSequenceToZkBlob only need BatchL2Data and BatchNumber
			BatchL2Data: batch.BatchL2Data,
			BatchNumber: batch.BatchNumber,
		}

		sequences = append(sequences, seq)
	}

	// handle current sequence
	log.Infof("sending zkblob from %v to %v", bSeq.From, bSeq.To)

	err := s.zkBlobClient.HandleSequenceToZkBlob(ctx, sequences, ethTxManagerOwner, s.etherman, s.ethTxManager, s.cfg.GasOffset)
	if err != nil {
		return err
	}

	return nil
}

func (s *ZkblobSender) getSequenceBlobsToSend(ctx context.Context) error {

	var fromBatchNum, toBatchNum uint64
	var err error
	if fromBatchNum, toBatchNum, err = s.blobDB.GetLatestZkBlob(); err != nil {
		return err
	}

	log.Infof("get latest zk blob -> from: %v, to: %v", fromBatchNum, toBatchNum)

	lastFromNumber := uint64(initialSequenceFromBatchNumber) // default
	if fromBatchNum > 0 {
		lastFromNumber = fromBatchNum // last one
	}
	// seq
	seqs, err := s.state.GetSequences(ctx, lastFromNumber, nil)
	if err != nil {
		return fmt.Errorf("error getting sequences: %v", err)
	}

	if len(seqs) == 0 {
		log.Info("getSequenceBlobsToSend: no sequences to send")
		return nil
	}

	// GetSequences(ctx, lastFromNumber, nil) return sequences where FromBatchNumber >= lastFromNumber
	// so lastFromNumber(i = 0) is already in the blobDB, skip it
	// NOTE. sequences-from-1-to-1 no need to be sent, so we need to skip it
	for i := 1; i < len(seqs); i++ {
		seq := seqs[i]

		prevLastOne := s.seqQueue.Back()
		/*
		* insert to queue if one of the following conditions is true :
		*   1. queue is empty
		*   2. seq.FromBatchNumber > prevLastOne.To
		*		&& queue doesn't contain seq.FromBatchNumber
		*		&& seq.FromBatchNumber is not the last sending one
		 */
		if prevLastOne == nil || (seq.FromBatchNumber > prevLastOne.To && !s.seqQueue.Has(seq.FromBatchNumber) && seq.FromBatchNumber != s.lastSequenceInitialBatch) {
			s.seqQueue.Enqueue(BlobSequence{
				From: seq.FromBatchNumber,
				To:   seq.ToBatchNumber,
			})
		}
	}

	return nil
}

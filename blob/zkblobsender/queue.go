package main

import "container/list"

type SequenceQueue struct {
	queue *list.List
}

type BlobSequence struct {
	From uint64
	To   uint64
}

func NewSequenceQueue() *SequenceQueue {
	return &SequenceQueue{
		queue: list.New(),
	}
}

func (sq *SequenceQueue) Front() *BlobSequence {
	if sq.IsEmpty() {
		return nil
	}

	result := sq.queue.Front().Value.(BlobSequence)

	return &result
}

// Enqueue
func (sq *SequenceQueue) Enqueue(seq BlobSequence) {
	sq.queue.PushBack(seq)
}

// Dequeue
func (sq *SequenceQueue) Dequeue() *BlobSequence {
	if sq.IsEmpty() {
		return nil
	}

	elem := sq.queue.Front()
	sq.queue.Remove(elem)

	result := elem.Value.(BlobSequence)

	return &result
}

// Has check if sequence is in the queue
func (sq *SequenceQueue) Has(from uint64) bool {
	found := false
	for e := sq.queue.Front(); e != nil; e = e.Next() {
		if e.Value.(BlobSequence).From == from {
			found = true
			break
		}
	}

	return found
}

func (sq *SequenceQueue) Len() int {
	return sq.queue.Len()
}

func (sq *SequenceQueue) IsEmpty() bool {
	return sq.queue.Len() == 0
}

func (sq *SequenceQueue) Back() *BlobSequence {
	if sq.IsEmpty() {
		return nil
	}

	result := sq.queue.Back().Value.(BlobSequence)

	return &result
}

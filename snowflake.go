package idgen

import (
	"time"
)

const (
	SF_BITS_TIME      = 31 // (2**31/3600/24/365 = 68 years)
	SF_BITS_SEQUENCE  = 22
	SF_BITS_WORKER_ID = 63 - SF_BITS_TIME - SF_BITS_SEQUENCE
	SF_TICKER_UNIT    = 1   // 1s

	SF_BITS_TIME32      = 13 // (day-count: 2**13/365 = 22.4 years)
	SF_BITS_SEQUENCE32  = 17 // (day-limit: 2**17 = 131072)
	SF_BITS_WORKER_ID32 = 32 - SF_BITS_TIME32 - SF_BITS_SEQUENCE32 // 2 (worker-limit: 2**2 = 4)
	SF_TICKER_UNIT32    = 86400 // 1d
)

var (
	SF_BLANK_ITEM     = struct{}{}

	SF_MAX_SEQ        = uint64(1<<SF_BITS_SEQUENCE - 1)   // (2**22-1 = 4,194,303 per second)
	SF_MAX_WORKER_ID  = uint16(1<<SF_BITS_WORKER_ID - 1)  // (2**10-1 = 1023)
	ELAPSED_TIME_SHIFT_BITS = uint64(SF_BITS_SEQUENCE + SF_BITS_WORKER_ID)

	SF_MAX_SEQ32        = uint64(1<<SF_BITS_SEQUENCE32 - 1)   // (2**17-1 = 131071 per day)
	SF_MAX_WORKER_ID32  = uint16(1<<SF_BITS_WORKER_ID32 - 1)  // (2**2-1  = 3)
	ELAPSED_TIME_SHIFT_BITS32 = uint64(SF_BITS_SEQUENCE32 + SF_BITS_WORKER_ID32)
)

type flakeParamsT struct {
	bitsTime     int
	bitsSequence int
	bitsWorkerId int
	tickerUnit   int64
	maxSeq       uint64
	maxWorkerId  uint16
	elapsedTimeShiftBits uint64
}

const (
	SF_64_IDX = iota
	SF_32_IDX
)
var flakeParams = []flakeParamsT{
	flakeParamsT{
		bitsTime: SF_BITS_TIME,
		bitsSequence: SF_BITS_SEQUENCE,
		bitsWorkerId: SF_BITS_WORKER_ID,
		tickerUnit: SF_TICKER_UNIT,
		maxSeq: SF_MAX_SEQ,
		maxWorkerId: SF_MAX_WORKER_ID,
		elapsedTimeShiftBits: ELAPSED_TIME_SHIFT_BITS,
	},
	flakeParamsT{
		bitsTime: SF_BITS_TIME32,
		bitsSequence: SF_BITS_SEQUENCE32,
		bitsWorkerId: SF_BITS_WORKER_ID32,
		tickerUnit: SF_TICKER_UNIT32,
		maxSeq: SF_MAX_SEQ32,
		maxWorkerId: SF_MAX_WORKER_ID32,
		elapsedTimeShiftBits: ELAPSED_TIME_SHIFT_BITS32,
	},
}

type FlakeIdGenerator struct {
	params *flakeParamsT
	epochTime int64
	workerId  uint16
	exit      bool

	nextIDs   [2]uint64 // double buffer: one for reading and one for writing
	rIdx      int       // index of nextIDs for reading.

	idReq chan struct{}
	idRes chan uint64

	overtimeChan chan struct{}
	ticker    *time.Ticker
}

func NewFlakeIdGenerator(epochTime time.Time, workerId uint16) *FlakeIdGenerator {
	return newFlakeIdGenerator(SF_64_IDX, epochTime, workerId)
}

func NewFlakeIdGenerator32(epochTime time.Time, workerId uint16) *FlakeIdGenerator {
	return newFlakeIdGenerator(SF_32_IDX, epochTime, workerId)
}

func newFlakeIdGenerator(idx int, epochTime time.Time, workerId uint16) *FlakeIdGenerator {
	params := &flakeParams[idx]
	if epochTime.After(time.Now()) {
		return nil
	}
	if workerId > params.maxWorkerId {
		return nil
	}

	ig := &FlakeIdGenerator{
		params: params,
		workerId: workerId,
		idReq:make(chan struct{}),
		idRes:make(chan uint64),
		overtimeChan:make(chan struct{}),
	}
	if epochTime.IsZero() {
		ig.epochTime = params.toSnowFlakeTime(time.Date(2020, 4, 20, 0, 0, 0, 0, time.UTC))
	} else {
		ig.epochTime = params.toSnowFlakeTime(epochTime)
	}

	ig.timeTicker()
	go ig.generator()
	return ig
}

func (params *flakeParamsT) toSnowFlakeTime(t time.Time) int64 {
	ts := t.Unix()
	switch params.tickerUnit {
	case SF_TICKER_UNIT:
		return ts
	default:
		return ts/params.tickerUnit
	}
}

func (params *flakeParamsT) elapse(t time.Time, epochTime int64) (elapsedTime, remain int64) {
	ts := t.Unix()
	switch params.tickerUnit {
	case SF_TICKER_UNIT:
		return ts - epochTime, 0
	default:
		return ts/params.tickerUnit - epochTime, ts%params.tickerUnit
	}
}

func (ig *FlakeIdGenerator) timeTicker() {
	params := ig.params
	workerIdBitsMask := uint64(ig.workerId) << uint64(params.bitsSequence)

	elapsedTime, remain := params.elapse(time.Now(), ig.epochTime)
	lastTime, firstId := elapsedTime, (uint64(elapsedTime) << params.elapsedTimeShiftBits) | workerIdBitsMask | uint64(remain)
	ig.nextIDs[0], ig.nextIDs[1], ig.rIdx = firstId, firstId, 0 // nextIDs, rIdx inited before calling generator()

	ig.ticker = time.NewTicker(1 * time.Second)
	// modify firstId every sencond
	go func() {
		var wIdx int
		for t := range ig.ticker.C {
			elapsedTime, _ = params.elapse(t, ig.epochTime)
			if elapsedTime <= lastTime {
				continue
			}
			lastTime = elapsedTime

			wIdx = 1 - ig.rIdx
			ig.nextIDs[wIdx] = (uint64(elapsedTime) << params.elapsedTimeShiftBits) | workerIdBitsMask // change nextIDs[wIdx] first
			ig.rIdx = wIdx // then change rIdx

			select {
			case <-ig.overtimeChan:
				// do nothing, just let blocked generator() to continue
			default:
				// do nothing, just not to block this goroutine
			}
		}
	}()
}

func (ig *FlakeIdGenerator) generator() {
	params := ig.params
	seqBits, maxSeq := uint64(params.bitsSequence), params.maxSeq
	var nextId *uint64

	for range ig.idReq {
		nextId = &ig.nextIDs[ig.rIdx]
		ig.idRes <- *nextId // maybe dirty reading, but it doesn't matter
		*nextId++           // maybe dirty writing, but it also deosn't matter

		if *nextId & seqBits >= maxSeq {
			ig.overtimeChan <- SF_BLANK_ITEM // blocking until woken up
		}
	}
}

func (ig *FlakeIdGenerator) NextID() uint64 {
	ig.idReq <- SF_BLANK_ITEM
	return <-ig.idRes
}

func (ig *FlakeIdGenerator) NextID32() uint32 {
	ig.idReq <- SF_BLANK_ITEM
	return uint32(<-ig.idRes)
}

func (ig *FlakeIdGenerator) Exit() {
	if ig.exit {
		return
	}
	ig.exit = true
	ig.ticker.Stop()
	close(ig.idReq)
}

func DecomposeSF(id uint64) (elaspedTime uint32, workerId uint16, sequence uint32) {
	return decomposeSF(SF_64_IDX, id)
}

func DecomposeSF32(id uint32) (elaspedTime uint32, workerId uint16, sequence uint32) {
	return decomposeSF(SF_32_IDX, uint64(id))
}

func decomposeSF(idx int, id uint64) (elaspedTime uint32, workerId uint16, sequence uint32) {
	params := &flakeParams[idx]
	sequence    = uint32(id & params.maxSeq)
	workerId    = uint16(id >> uint64(params.bitsSequence)) & params.maxWorkerId
	elaspedTime = uint32(id >> params.elapsedTimeShiftBits)
	return
}

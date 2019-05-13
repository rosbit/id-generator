package idgen

import (
	"time"
)

const (
	SF_BITS_TIME      = 31 // (2**31/3600/24/365 = 68 years)
	SF_BITS_SEQUENCE  = 22
	SF_BITS_WORKER_ID = 63 - SF_BITS_TIME - SF_BITS_SEQUENCE

	SF_TICKER_UNIT    = 1   // 1s
)

var (
	SF_MAX_SEQ        = uint64(1<<SF_BITS_SEQUENCE - 1)   // (2**22-1 = 4,194,303 per second)
	SF_MAX_WORKER_ID  = uint16(1<<SF_BITS_WORKER_ID - 1)  // (2*10-1  = 1023)
	SF_BLANK_ITEM     = struct{}{}

	ELAPSED_TIME_SHIFT_BITS = uint64(SF_BITS_SEQUENCE + SF_BITS_WORKER_ID)
)

type FlakeIdGenerator struct {
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
	if epochTime.After(time.Now()) {
		return nil
	}
	if workerId > SF_MAX_WORKER_ID {
		return nil
	}

	ig := &FlakeIdGenerator{
		workerId: workerId,
		idReq:make(chan struct{}),
		idRes:make(chan uint64),
		overtimeChan:make(chan struct{}),
	}
	if epochTime.IsZero() {
		ig.epochTime = toSnowFlakeTime(time.Date(2019, 5, 11, 0, 0, 0, 0, time.UTC))
	} else {
		ig.epochTime = toSnowFlakeTime(epochTime)
	}

	ig.timeTicker()
	go ig.generator()
	return ig
}

func toSnowFlakeTime(t time.Time) int64 {
	return t.Unix()
}

func (ig *FlakeIdGenerator) timeTicker() {
	workerIdBitsMask := uint64(ig.workerId) << uint64(SF_BITS_SEQUENCE)

	elapsedTime := toSnowFlakeTime(time.Now()) - ig.epochTime
	firstId := (uint64(elapsedTime) << ELAPSED_TIME_SHIFT_BITS) | workerIdBitsMask
	ig.nextIDs[0], ig.nextIDs[1], ig.rIdx = firstId, firstId, 0 // nextIDs, rIdx inited before calling generator()

	// modify firstId every sencond
	go func() {
		var wIdx int
		ig.ticker = time.NewTicker(SF_TICKER_UNIT * time.Second)
		for t := range ig.ticker.C {
			elapsedTime = toSnowFlakeTime(t) - ig.epochTime
			wIdx = 1 - ig.rIdx
			ig.nextIDs[wIdx] = (uint64(elapsedTime) << ELAPSED_TIME_SHIFT_BITS) | workerIdBitsMask // change nextIDs[wIdx] first
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
	seqBits := uint64(SF_BITS_SEQUENCE)
	var nextId *uint64

	for !ig.exit {
		<-ig.idReq      // blocking until NextID() called

		nextId = &ig.nextIDs[ig.rIdx]
		ig.idRes <- *nextId // maybe dirty reading, but it doesn't matter
		*nextId++           // maybe dirty writing, but it also deosn't matter

		if *nextId & seqBits == SF_MAX_SEQ {
			ig.overtimeChan <- SF_BLANK_ITEM // blocking until woken up
		}
	}
}

func (ig *FlakeIdGenerator) NextID() uint64 {
	ig.idReq <- SF_BLANK_ITEM
	return <-ig.idRes
}

func (ig *FlakeIdGenerator) Exit() {
	ig.exit = true
	if ig.ticker != nil {
		ig.ticker.Stop()
		ig.ticker = nil
	}
}

func DecomposeSF(id uint64) (elaspedTime uint32, workerId uint16, sequence uint32) {
	sequence    = uint32(id & SF_MAX_SEQ)
	workerId    = uint16(id >> uint64(SF_BITS_SEQUENCE)) & SF_MAX_WORKER_ID
	elaspedTime = uint32(id >> ELAPSED_TIME_SHIFT_BITS)
	return
}

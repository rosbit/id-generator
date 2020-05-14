// generate order id with format YYMMDDxxxxxxxWW
package idgen

import (
	"time"
	"math"
	"fmt"
	"os"
	"strconv"
)

const (
	ORD_DATE_FORMAT    = "060102"        // 6 chars
	ORD_MAX_WORKER_ID  = uint16(99)      // 2 digit chars
	ORD_MAX_SEQ        = uint32(9999999) // 7 digit chars
	ORD_TICKER_UNIT    = 1  // in second
)

var (
	ORD_DATE_CHARS    = len(ORD_DATE_FORMAT)                      // 6
	ORD_WORKID_CHARS  = len(fmt.Sprintf("%d", ORD_MAX_WORKER_ID)) // 2
	ORD_SEQ_CHARS     = len(fmt.Sprintf("%d", ORD_MAX_SEQ))       // 7

	ORD_TOTAL_CHARS   = ORD_DATE_CHARS + ORD_WORKID_CHARS + ORD_SEQ_CHARS // 15

	ORD_SEQ_MULTIPLE  = uint32(math.Pow10(ORD_WORKID_CHARS)) // 100
	ORD_ID_FORMAT     = fmt.Sprintf("%%s%%0%dd%%0%dd", ORD_SEQ_CHARS, ORD_WORKID_CHARS) // "%s%07d%02d"

	ORD_BLANK_ITEM    = struct{}{}
)

type nextIdT struct {
	val uint64
	seq uint32
}

func (n *nextIdT) id() uint64 {
	return n.val + uint64(n.seq * ORD_SEQ_MULTIPLE)
}

type OrderIdGenerator struct {
	workerId  uint16
	exit      bool

	nextIDs   [2]nextIdT // double buffer: one for reading and one for writing
	rIdx      int        // index of nextIDs for reading.

	idReq chan struct{}
	idRes chan uint64

	loc   *time.Location
	overtimeChan chan struct{}
	ticker    *time.Ticker
}

func NewOrderIdGenerator(workerId uint16, tz string) *OrderIdGenerator {
	if workerId > ORD_MAX_WORKER_ID {
		fmt.Printf("workerId(%d) is too large (>%d)", workerId, ORD_MAX_WORKER_ID)
		return nil
	}
	og := &OrderIdGenerator{
		workerId: workerId,
		idReq: make(chan struct{}),
		idRes: make(chan uint64),
		loc:   getLoc(tz),
		overtimeChan:make(chan struct{}),
	}

	og.timeTicker()
	go og.generator()
	return og
}

func getLoc(tz string) *time.Location {
	var loc *time.Location
	if tz == "" {
		tz = os.Getenv("TZ")
	}

	if tz != "" {
		if l, err := time.LoadLocation(tz); err == nil {
			loc = l
		}
	}

	if loc == nil {
		loc = time.FixedZone("UTC+8", 8*60*60)
	}

	return loc
}

func (og *OrderIdGenerator) timeTicker() {
	today := time.Now().In(og.loc)
	firstId := og.calcFirstIdInDay(&today)
	og.nextIDs[0], og.nextIDs[1], og.rIdx = firstId, firstId, 0 // nextIDs, rIdx inited before calling generator()

	// modify firstId every day
	go func() {
		var wIdx int
		og.ticker = time.NewTicker(ORD_TICKER_UNIT * time.Second)
		for range og.ticker.C {
			day := time.Now().In(og.loc)
			if day.Day() == today.Day() {
				// same day
				continue
			}

			wIdx = 1 - og.rIdx // 0 -> 1, 1 -> 0
			og.nextIDs[wIdx] = og.calcFirstIdInDay(&day) // change nextIDs[wIdx] first
			og.rIdx = wIdx // then change rIdx
			today = day

			for {
				select {
				case <-og.overtimeChan:
					// do nothing, just let blocked generator() to continue
				default:
					// do nothing, just not to block this goroutine
					goto ALL_OVERTIME_WAKEDUP
				}
			}
ALL_OVERTIME_WAKEDUP:
		}
	}()
}

func (og *OrderIdGenerator) calcFirstIdInDay(today *time.Time) nextIdT {
	firstIdS := fmt.Sprintf(ORD_ID_FORMAT, today.Format(ORD_DATE_FORMAT), 0, og.workerId)
	firstId, _ := strconv.ParseUint(firstIdS, 10, 64)
	return nextIdT{firstId, uint32(today.Hour()*3600+today.Minute()*60+today.Second())}
}

func (og *OrderIdGenerator) generator() {
	var nextId *nextIdT

	for range og.idReq {
		nextId = &og.nextIDs[og.rIdx]
		og.idRes <- nextId.id() // maybe dirty reading, but it doesn't matter
		nextId.seq += 1         // maybe dirty writing, but it also deosn't matter

		if nextId.seq > ORD_MAX_SEQ {
			og.overtimeChan <- ORD_BLANK_ITEM // blocking until woken up
		}
	}
}

func (og *OrderIdGenerator) NextID() uint64 {
	og.idReq <- ORD_BLANK_ITEM
	return <-og.idRes
}

func (og *OrderIdGenerator) Exit() {
	if og.exit {
		return
	}
	og.exit = true
	og.ticker.Stop()
	close(og.idReq)
}

func DecomposeOrd(id uint64) (day uint32, workerId uint16, sequence uint32) {
	s := fmt.Sprintf("%d", id)
	if len(s) != ORD_TOTAL_CHARS {
		fmt.Printf("invalid order id\n")
		return
	}
	seq, _ := strconv.ParseUint(s[ORD_DATE_CHARS:ORD_DATE_CHARS+ORD_SEQ_CHARS], 10, 64)
	wId, _ := strconv.ParseUint(s[ORD_TOTAL_CHARS-ORD_WORKID_CHARS:], 10, 64)
	d,   _ := strconv.ParseUint(s[:ORD_DATE_CHARS], 10, 64)

	sequence = uint32(seq)
	workerId = uint16(wId)
	day      = uint32(d)
	return
}

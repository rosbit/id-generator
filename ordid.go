// generate order id with format:
//  short id format:   YYMMDDxxxxxxxWW
//  long id format: YYMMDDhhmmxxxxxxWW
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
	ORD_MAX_SEQ        = uint32(9999999) // 7 digit chars

	L_ORD_DATE_FORMAT  = "0601021504"    // 10 chars
	L_ORD_MAX_SEQ      = uint32(999999)  // 6 digit chars

	ORD_MAX_WORKER_ID  = uint16(99)      // 2 digit chars
	ORD_TICKER_UNIT    = 1  // in second
)

var (
	ORD_WORKID_CHARS  = len(fmt.Sprintf("%d", ORD_MAX_WORKER_ID)) // 2
	ORD_SEQ_MULTIPLE  = uint32(math.Pow10(ORD_WORKID_CHARS)) // 100
	ORD_BLANK_ITEM    = struct{}{}

	ORD_DATE_CHARS    = len(ORD_DATE_FORMAT)                  // 6
	ORD_SEQ_CHARS     = len(fmt.Sprintf("%d", ORD_MAX_SEQ))   // 7
	ORD_TOTAL_CHARS   = ORD_DATE_CHARS + ORD_WORKID_CHARS + ORD_SEQ_CHARS     // 15
	ORD_ID_FORMAT     = fmt.Sprintf("%%s%%0%dd%%0%dd", ORD_SEQ_CHARS, ORD_WORKID_CHARS) // "%s%07d%02d"

	L_ORD_DATE_CHARS  = len(L_ORD_DATE_FORMAT)                // 10
	L_ORD_SEQ_CHARS   = len(fmt.Sprintf("%d", L_ORD_MAX_SEQ)) // 6
	L_ORD_TOTAL_CHARS = L_ORD_DATE_CHARS + ORD_WORKID_CHARS + L_ORD_SEQ_CHARS // 18
	L_ORD_ID_FORMAT   = fmt.Sprintf("%%s%%0%dd%%0%dd", L_ORD_SEQ_CHARS, ORD_WORKID_CHARS) // "%s%06d%02d"
)

type ordIdParam struct {
	dateChars int
	seqChars  int
	totalChars int
	dateLayout string
	idFormat string
	maxSeq uint32
	adapter adapaterT
}

type adapaterT interface {
	firstSeq(startTime *time.Time) uint32
	sameRange(t1, t2 time.Time) bool
}

type shortOrdAdapter struct {}
func (a *shortOrdAdapter) firstSeq(startTime *time.Time) uint32 {
	fid := uint32(startTime.Hour()*3600+startTime.Minute()*60+startTime.Second())
	if fid == 0 {
		return 1
	}
	return fid
}
func (a *shortOrdAdapter) sameRange(t1, t2 time.Time) bool {
	return t1.Day() == t2.Day()
}

type longOrdAdapter struct {}
func (a *longOrdAdapter) firstSeq(startTime *time.Time) uint32 {
	return 1
}
func (a *longOrdAdapter) sameRange(t1, t2 time.Time) bool {
	return t1.Minute() == t2.Minute()
}

const (
	SHORT_ORDID_IDX = iota
	LONG_ORDID_IDX
)
var ordIdParams = []ordIdParam {
	ordIdParam{
		dateChars: ORD_DATE_CHARS,
		seqChars: ORD_SEQ_CHARS,
		totalChars: ORD_TOTAL_CHARS,
		dateLayout: ORD_DATE_FORMAT,
		idFormat: ORD_ID_FORMAT,
		maxSeq: ORD_MAX_SEQ,
		adapter: &shortOrdAdapter{},
	},
	ordIdParam{
		dateChars: L_ORD_DATE_CHARS,
		seqChars: L_ORD_SEQ_CHARS,
		totalChars: L_ORD_TOTAL_CHARS,
		dateLayout: L_ORD_DATE_FORMAT,
		idFormat: L_ORD_ID_FORMAT,
		maxSeq: L_ORD_MAX_SEQ,
		adapter: &longOrdAdapter{},
	},
}

type nextIdT struct {
	val uint64
	seq uint32
}

func (n *nextIdT) id(maxSeq uint32) uint64 {
	return n.val + uint64(n.seq * ORD_SEQ_MULTIPLE)
}

type OrderIdGenerator struct {
	params *ordIdParam

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
	return newOrderIdGenerator(SHORT_ORDID_IDX, workerId, tz)
}

func NewLongOrderIdGenerator(workerId uint16, tz string) *OrderIdGenerator {
	return newOrderIdGenerator(LONG_ORDID_IDX, workerId, tz)
}

func newOrderIdGenerator(idx int, workerId uint16, tz string) *OrderIdGenerator {
	if workerId > ORD_MAX_WORKER_ID {
		fmt.Printf("workerId(%d) is too large (>%d)\n", workerId, ORD_MAX_WORKER_ID)
		return nil
	}
	og := &OrderIdGenerator{
		params: &ordIdParams[idx],
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
	startRangeTime := time.Now().In(og.loc)
	firstId := og.calcFirstIdInRange(&startRangeTime)
	og.nextIDs[0], og.nextIDs[1], og.rIdx = firstId, firstId, 0 // nextIDs, rIdx inited before calling generator()

	og.ticker = time.NewTicker(ORD_TICKER_UNIT * time.Second)
	// modify firstId every interval
	go func() {
		var wIdx int
		a := og.params.adapter
		for range og.ticker.C {
			tickerRange := time.Now().In(og.loc)
			if a.sameRange(tickerRange, startRangeTime) {
				continue
			}

			wIdx = 1 - og.rIdx // 0 -> 1, 1 -> 0
			og.nextIDs[wIdx] = og.calcFirstIdInRange(&tickerRange) // change nextIDs[wIdx] first
			og.rIdx = wIdx // then change rIdx
			startRangeTime = tickerRange

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

func (og *OrderIdGenerator) calcFirstIdInRange(startRangeTime *time.Time) nextIdT {
	p := og.params
	firstIdS := fmt.Sprintf(p.idFormat, startRangeTime.Format(p.dateLayout), 0, og.workerId)
	firstId, _ := strconv.ParseUint(firstIdS, 10, 64)
	return nextIdT{firstId, p.adapter.firstSeq(startRangeTime)}
}

func (og *OrderIdGenerator) generator() {
	var nextId *nextIdT
	p := og.params

	for range og.idReq {
		nextId = &og.nextIDs[og.rIdx]
		og.idRes <- nextId.id(p.maxSeq) // maybe dirty reading, but it doesn't matter
		nextId.seq += 1         // maybe dirty writing, but it also deosn't matter

		if nextId.seq > p.maxSeq {
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

func DecomposeOrd(id uint64) (datetime uint32, workerId uint16, sequence uint32) {
	return decomposeOrd(SHORT_ORDID_IDX, id)
}

func DecomposeLongOrd(id uint64) (datetime uint32, workerId uint16, sequence uint32) {
	return decomposeOrd(LONG_ORDID_IDX, id)
}

func decomposeOrd(idx int, id uint64) (datetime uint32, workerId uint16, sequence uint32) {
	params := &ordIdParams[idx]

	s := fmt.Sprintf("%d", id)
	if len(s) != params.totalChars {
		fmt.Printf("invalid order id\n")
		return
	}
	seq, _ := strconv.ParseUint(s[params.dateChars:params.dateChars+params.seqChars], 10, 64)
	wId, _ := strconv.ParseUint(s[params.totalChars-ORD_WORKID_CHARS:], 10, 64)
	d,   _ := strconv.ParseUint(s[:params.dateChars], 10, 64)

	sequence = uint32(seq)
	workerId = uint16(wId)
	datetime = uint32(d)
	return
}

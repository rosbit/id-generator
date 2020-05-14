package idgen

const (
	SEQ_BITS_WORKER_ID = 10
	SEQ_BITS_ID        = 63 - SEQ_BITS_WORKER_ID
)

var (
	SEQ_MAX_SEQ       = uint64(1<<SEQ_BITS_ID - 1)        // (2**53-1 = 9,007,199,254,740,991)
	SEQ_MAX_WORKER_ID = uint16(1<<SEQ_BITS_WORKER_ID - 1) // (2**10-1 = 1023)
	SEQ_BLANK_ITEM    = struct{}{}
)

type SeqIdGenerator struct {
	workerId uint16
	nextId   uint64
	exit     bool

	idReq chan struct{}
	idRes chan uint64
}

func NewSeqIdGenerator(workerId uint16, startId uint64) *SeqIdGenerator {
	if workerId > SEQ_MAX_WORKER_ID {
		return nil
	}
	if startId >= SEQ_MAX_SEQ {
		return nil
	}

	nextId := uint64(workerId) << uint64(SEQ_BITS_ID) | startId

	ig := &SeqIdGenerator{workerId, nextId, false, make(chan struct{}), make(chan uint64)}
	go ig.generator()
	return ig
}

func (ig *SeqIdGenerator) generator() {
	for range ig.idReq {
		ig.idRes <- ig.nextId
		ig.nextId++
	}
}

func (ig *SeqIdGenerator) NextID() uint64 {
	ig.idReq <- SEQ_BLANK_ITEM
	return <-ig.idRes
}

func (ig *SeqIdGenerator) Exit() {
	if ig.exit {
		return
	}
	ig.exit = true
	close(ig.idReq)
}

func DecomposeSeq(id uint64) (workerId uint16, sequence uint64) {
	workerId = uint16(id >> uint64(SEQ_BITS_ID)) & SEQ_MAX_WORKER_ID
	sequence = id & SEQ_MAX_SEQ
	return
}

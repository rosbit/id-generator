package idgen

import (
	"testing"
	"fmt"
)

func Test_seqNextId(t *testing.T) {
	ig := NewSeqIdGenerator(1024, 0)
	if ig != nil {
		t.Fatal("nil expected")
	}
	ig = NewSeqIdGenerator(1, 0)

	lastId := ig.NextID()
	fmt.Printf("firstId: %d (%b)\n", lastId, lastId)
	workerId, sequence := DecomposeSeq(lastId)
	fmt.Printf("workerId: %d, sequence: %d\n", workerId, sequence)
	for i:=0; i<10; i++ {
		newId := ig.NextID()
		fmt.Printf("#%d : %d (%b)\n", i, newId, newId)
		if newId < lastId {
			t.Fatal("newId is less than lastId")
		}
		lastId = newId
	}
	ig.Exit()
	fmt.Printf("------------done for seqid --------------\n")
}

func Benchmark_seqNextId(b *testing.B) {
	ig := NewSeqIdGenerator(1, 0)
	lastId := ig.NextID()
	for i:=0; i<b.N; i++ {
		id := ig.NextID()
		// ig.NextID()
		if id < lastId {
			fmt.Printf("\n")
			fmt.Printf("lastId: %d (%b)\n", lastId, lastId)
			fmt.Printf("newId:  %d (%b)\n", id, id)
			b.Fatal("failed")
		}
		lastId = id
	}
	ig.Exit()
}

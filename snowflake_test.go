package idgen

import (
	"testing"
	"time"
	"fmt"
)

func Test_snowFlakeNextId(t *testing.T) {
	n := time.Now()
	ig := NewFlakeIdGenerator(n.AddDate(0, 0, 1), 1)
	if ig != nil {
		t.Fatal("nil expected")
	}
	ig = NewFlakeIdGenerator(n.AddDate(0, 0, -1), 1)

	lastId := ig.NextID()
	fmt.Printf("firstId: %d (%b)\n", lastId, lastId)
	elaspedTime, workerId, sequence := DecomposeSF(lastId)
	fmt.Printf("elaspedTime: %d, workerId: %d, sequence: %d\n", elaspedTime, workerId, sequence)
	for i:=0; i<10; i++ {
		newId := ig.NextID()
		fmt.Printf("#%d : %d (%b)\n", i, newId, newId)
		if newId < lastId {
			t.Fatal("newId is less than lastId")
		}
		lastId = newId
	}
	ig.Exit()
	fmt.Printf("------------done for snowflake id --------------\n")
}

func Benchmark_snowFlakeNextId(b *testing.B) {
	n := time.Now()
	ig := NewFlakeIdGenerator(n.AddDate(0, 0, -1), 1)
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

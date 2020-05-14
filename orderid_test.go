package idgen

import (
	"testing"
	"fmt"
)

func Test_orderNextId(t *testing.T) {
	ig := NewOrderIdGenerator(1024, "")
	if ig != nil {
		t.Fatal("nil expected")
	}
	ig = NewOrderIdGenerator(1, "Asia/Shanghai")

	lastId := ig.NextID()
	fmt.Printf("firstId: %d\n", lastId)
	day, workerId, sequence := DecomposeOrd(lastId)
	fmt.Printf("day: %d, workerId: %d, sequence: %d\n", day, workerId, sequence)
	for i:=0; i<10; i++ {
		newId := ig.NextID()
		fmt.Printf("#%d : %d\n", i, newId)
		if newId < lastId {
			t.Fatal("newId is less than lastId")
		}
		lastId = newId
	}
	ig.Exit()
	fmt.Printf("------------done for orderid --------------\n")
}

func Benchmark_orderNextId(b *testing.B) {
	ig := NewOrderIdGenerator(1, "Asia/Shanghai")
	lastId := ig.NextID()
	for i:=0; i<b.N; i++ {
		id := ig.NextID()
		if id < lastId {
			fmt.Printf("\n")
			fmt.Printf("lastId: %d\n", lastId)
			fmt.Printf("newId:  %d\n", id)
			b.Fatal("failed")
		}
		lastId = id
	}
	ig.Exit()
}

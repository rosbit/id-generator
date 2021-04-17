package idgen

import (
	"testing"
	"fmt"
	"time"
)

type fnNewId func(uint16,string)*OrderIdGenerator
type fnDecompose func(id uint64) (day uint32, workerId uint16, sequence uint32)

func test_orderNextId(t *testing.T, sleepTime int, newIg fnNewId, decomposeOrd fnDecompose, prompt string) {
	ig := newIg(1024, "")
	if ig != nil {
		t.Fatal("nil expected")
	}
	ig = newIg(1, "Asia/Shanghai")

	lastId := ig.NextID()
	fmt.Printf("firstId: %d\n", lastId)
	datetime, workerId, sequence := decomposeOrd(lastId)
	fmt.Printf("datetime: %d, workerId: %d, sequence: %d\n", datetime, workerId, sequence)
	for i:=0; i<10; i++ {
		newId := ig.NextID()
		fmt.Printf("#%d : %d\n", i, newId)
		if newId < lastId {
			t.Fatal("newId is less than lastId")
		}
		lastId = newId
	}

	if sleepTime > 0 {
		fmt.Printf("I will sleep for %d seconds...\n", sleepTime)
		time.Sleep(45*time.Second)
		for i:=0; i<10; i++ {
			newId := ig.NextID()
			fmt.Printf("#%d : %d\n", i, newId)
			if newId < lastId {
				t.Fatal("newId is less than lastId")
			}
			lastId = newId
		}
	}

	ig.Exit()
	fmt.Printf("------------done for %s --------------\n", prompt)
}

func Test_orderNextId(t *testing.T) {
	test_orderNextId(t, 0, NewOrderIdGenerator, DecomposeOrd, "orderid")
}

func Test_longOrderNextId(t *testing.T) {
	test_orderNextId(t, 61, NewLongOrderIdGenerator, DecomposeLongOrd, "long orderid")
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

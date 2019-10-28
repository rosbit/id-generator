# Distributed unique ID Generator

id-generator is a dirstributed unique ID generator. There are 3 kinds of generator
provided by id-generator:

 1. snowflake-like ID generator
    - inspired by [Twitter's Snowflake](https://blog.twitter.com/2010/announcing-snowflake).
    - A snowflake-like ID is composed of

        ```
        31 bits for time in units of 1 second (limit: 2**31/3600/24/365 = 68 years)
        22 bits for a sequence number         (limit: 2**22-1 = 4,194,303 per second)
        10 bits for a worker id               (limit: 2**10 = 1024)
        ```

 1. auto-inremented sequence ID generator
    - id auto-incremented
    - sequence ID is composed of

        ```
        53 bits for a sequence number  (limit: 2**53-1 = 9,007,199,254,740,991)
        10 bits for a worker id        (limit: 2**10 = 1024)
        ```

 1. order ID generator
    - id is of type uint64
    - as a digit string, its first 6 char is date with format "YYMMDD"
    - the tail chars is workerId
    - order ID digit is composed of

        ```
        YYMMDDxxxxxxxWW
           YYMMDD  stands for Year, Month, Day.   (upper limit: 991231)
           xxxxxxx stands for order Id sequence.  (upper limit: 10,000,000 per day)
           WW      stands for worker id.          (upper limit: 100)
        ```

## Installation

The package is fully go-getable without any dependency, So, just type

   `go get github.com/rosbit/id-generator`

## Usage

```go
package main

import (
	"github.com/rosbit/id-generator"
	"time"
	"log"
)

func main() {
	// for snowflake-like ID
	epochTime := time.Now().AddDate(0, 0, -1) // set your epochTime, a history time
	workerId := 1 // set your own workerId
	sf := idgen.NewFlakeIdGenerator(epochTime, workerId)

	lastId := sf.NextID()
	log.Printf("firstId: %d (%b)\n", lastId, lastId)
	elaspedTime, workerId, sequence := idgen.DecomposeSF(lastId)
	fmt.Printf("elaspedTime: %d, workerId: %d, sequence: %d\n", elaspedTime, workerId, sequence)

	for i:=0; i<10; i++ {
		newId := sf.NextID()
		log.Printf("#%d : %d (%b)\n", i, newId, newId)
		if newId < lastId {
			log.Fatal("newId is less than lastId")
		}
		lastId = newId
	}
	sf.Exit()

	// for sequcen ID
	startId := 0 // set your own startId
	seq := idgen.NewSeqIdGenerator(workerId, startId)

	lastId = seq.NextID()
	fmt.Printf("firstId: %d (%b)\n", lastId, lastId)
	workerId, sequence = idgen.DecomposeSeq(lastId)
	fmt.Printf("workerId: %d, sequence: %d\n", workerId, sequence)
	for i:=0; i<10; i++ {
		newId := seq.NextID()
		fmt.Printf("#%d : %d (%b)\n", i, newId, newId)
		if newId < lastId {
			log.Fatal("newId is less than lastId")
		}
		lastId = newId
	}
	seq.Exit()

	// for order Id
	ord := idgen.NewOrderIdGenerator(workerId, "Asia/Shanghai")
	lastId = ord.NextID()

	fmt.Printf("firstId: %d\n", lastId)
	day, workerId, sequence := idgen.DecomposeOrd(lastId)
	fmt.Printf("day: %d, workerId: %d, sequence: %d\n", day, workerId, sequence)
	for i:=0; i<10; i++ {
		newId := ord.NextID()
		fmt.Printf("#%d : %d\n", i, newId)
		if newId < lastId {
			log.Fatal("newId is less than lastId")
		}
		lastId = newId
	}
	ord.Exit()
}
```

## Benchmarks

Under my aliyun-ECS: 1core CPU, Intel(R) Xeon(R) CPU E5-2650 v2 @ 2.60GHz

The benchmark result is:

```
goos: linux
goarch: amd64
pkg: github.com/rosbit/id-generator
Benchmark_orderNextId     	 3000000	       479 ns/op
Benchmark_seqNextId       	 3000000	       469 ns/op
Benchmark_snowFlakeNextId 	 3000000	       472 ns/op
PASS
ok  	github.com/rosbit/id-generator	3.781s
```

## Contribution

Pull requests are welcome! Also, if you want to discuss something,
send a pull request with proposal and changes.

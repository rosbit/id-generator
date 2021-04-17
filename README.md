# Distributed unique ID Generator

id-generator is a dirstributed unique ID generator. There are 3 kinds of generators
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
    - as a digit string, its head 6 chars are date with layout "YYMMDD"
    - its tail chars form the workerId specified when initing
    - the digit of the whole order id is composed of 3 parts
        - short order id

        ```
        parts format: YYMMDDxxxxxxxWW
           YYMMDD  stands for Year, Month, Day.   (upper limit: 991231)
           xxxxxxx stands for order Id sequence.  (upper limit: 10,000,000 per day)
           WW      stands for worker id.          (upper limit: 100)
        ```

        - long order id

        ```
        parts format: YYMMDDhhmmxxxxxxWW
           YYMMDDhhmm  stands for Year, Month, Day, Hour, Minute. (upper limit: 9912312359)
           xxxxxx      stands for order Id sequence.              (upper limit: 1,000,000 per minute)
           WW          stands for worker id.                      (upper limit: 100)
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
)

func main() {
	// 1. for snowflake-like ID
	epochTime := time.Now().AddDate(0, 0, -1) // set your epochTime, a history time
	workerId := uint16(1) // set your own workerId
	sf := idgen.NewFlakeIdGenerator(echochTime, workerId)   // 64bit id
	// sf := idgen.NewFlakeIdGenerator32(echochTime, workerId) // 32bit id, most of time, 32bit is ok.
	for i:=0; i<10; i++ {
		id := sf.NextID()
		// id
	}
	sf.Exit()

	// 2. for order id
	ord := idgen.NewOrderIdGenerator(workerId, "Asia/Shanghai") // any valid tz string is ok
	for i:=0; i<10; i++ {
		id := ord.NextID()
		// id
	}
	ord.Exit()

	ord2 := idgen.NewLongOrderIdGenerator(workerId, "Asia/Shanghai") // any valid tz string is ok
	for i:=0; i<10; i++ {
		id := ord2.NextID()
		// id
	}
	ord2.Exit()

	// 3. for sequence id
	seq := idgen.NewSeqIdGenerator(workerId, 0) // startId, any uint64 is ok
	for i:=0; i<10; i++ {
		id := seq.NextID()
		// id
	}
	seq.Exit()
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

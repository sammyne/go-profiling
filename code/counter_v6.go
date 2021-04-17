// +build ignore

package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"runtime/trace"
	"sync"
	"time"
)

var hello []int

func counter(wg *sync.WaitGroup) {
	defer wg.Done()

	//slice := []int{0}
	slice := make([]int, 0, 100000)
	c := 1
	for i := 0; i < 100000; i++ {
		c = i + 1 + 2 + 3 + 4 + 5
		slice = append(slice, c)
	}
	hello = slice
}

func main() {
	runtime.GOMAXPROCS(5)

	var traceProfile = flag.String("traceprofile", "trace.pprof", "write trace profile to file")
	flag.Parse()

	if *traceProfile != "" {
		f, err := os.Create(*traceProfile)
		if err != nil {
			log.Fatal(err)
		}
		trace.Start(f)
		defer f.Close()
		defer trace.Stop()
	}

	var wg sync.WaitGroup
	var mtx sync.Mutex
	wg.Add(3)
	for i := 0; i < 3; i++ {
		mtx.Lock()
		go counter(&wg)
		time.Sleep(time.Millisecond)
		mtx.Unlock()
	}
	wg.Wait()
}

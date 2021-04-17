// +build ignore

package main

import (
	"flag"
	"log"
	"os"
	"runtime"
	"runtime/trace"
	"sync"
)

var hello []int

func counter(wg *sync.WaitGroup, mtx *sync.Mutex) {
	defer wg.Done()

	slice := make([]int, 0, 100000)
	c := 1
	for i := 0; i < 100000; i++ {
		mtx.Lock()
		c = i + 1 + 2 + 3 + 4 + 5
		slice = append(slice, c)
		mtx.Unlock()
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
		go counter(&wg, &mtx)
	}
	wg.Wait()
}

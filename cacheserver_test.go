package main

import (
	"log"
	"sync"
	"testing"
	"time"

	"github.com/zeneli/cacheserver/rangecache"
)

// incomingRangeRequests simulates a channel of range requests.
func incomingRangeRequests() <-chan rangecache.Keyrange {
	ch := make(chan rangecache.Keyrange)
	go func() {
		for _, keyrange := range []rangecache.Keyrange{
			{0, 6400000},  // 6.4 MB
			{0, 3200000},  // 3.2 MB
			{0, 12800000}, // 12.8 MB
			{0, 6400000}, {0, 3200000}, {0, 10800000}, {0, 12800000},
			{0, 6400000}, {0, 3200000}, {0, 10800000}, {0, 12800000},
		} {
			ch <- keyrange
		}
		close(ch)
	}()
	return ch
}

// incomingRangeRequestsFake simulates small sporadic range requests.
// Redundant and large table helps warm up the cache.
func incomingRangeRequestsFake() <-chan rangecache.Keyrange {
	ch := make(chan rangecache.Keyrange)
	go func() {
		for _, keyrange := range []rangecache.Keyrange{
			{0, 640}, {0, 320}, {0, 1280}, {0, 1080}, {0, 640},
			{0, 640}, {0, 320}, {0, 1280}, {0, 1080}, {0, 640},
			{0, 640}, {0, 320}, {0, 1280}, {0, 1080}, {0, 640},
			{0, 640}, {0, 320}, {0, 1280}, {0, 1080}, {0, 640},
			{0, 640}, {0, 320}, {0, 1280}, {0, 1080}, {0, 640},
			{0, 640}, {0, 320}, {0, 1280}, {0, 1080}, {0, 640},
		} {
			ch <- keyrange
		}
		close(ch)
	}()
	return ch
}

// testSequentialReal performs network requests using CacheServer.GetRange.
func testSequentialReal(t *testing.T, cs *CacheServer) {
	for kr := range incomingRangeRequests() {
		start := time.Now()
		body, err := cs.GetRange(VIMEOURL, kr)
		if err != nil {
			log.Print(err)
			continue
		}
		log.Printf("time: %s: GetRange(%v), %d bytes", time.Since(start), kr, len(body))
	}
}
func testSequentialRealDupSup(t *testing.T, cs *CacheServer) {
	for kr := range incomingRangeRequests() {
		start := time.Now()
		body, _ := cs.GetRangeDupSup(VIMEOURL, kr)
		log.Printf("time: %s: GetRange(%v), %d bytes", time.Since(start), kr, len(body))
	}
}

// testConcurrentReal performs network requests using CacheServer.GetRange.
func testConcurrentReal(t *testing.T, cs *CacheServer) {
	var n sync.WaitGroup
	for kr := range incomingRangeRequests() {
		n.Add(1) // add to wait group
		go func(keyrange rangecache.Keyrange) {
			defer n.Done() // defer done
			start := time.Now()
			body, err := cs.GetRange(VIMEOURL, keyrange)
			if err != nil {
				return
			}
			log.Printf("time: %s: GetRange(%v), %d bytes", time.Since(start), keyrange, len(body))
		}(kr)
	}
	n.Wait() // wait for all requests to be done
}

// testConcurrentRealDupSup performs network requests using CacheServer.GetRange.
func testConcurrentRealDupSup(t *testing.T, cs *CacheServer) {
	var n sync.WaitGroup
	for kr := range incomingRangeRequests() {
		n.Add(1) // add to wait group
		go func(keyrange rangecache.Keyrange) {
			defer n.Done() // defer done
			start := time.Now()
			body, ok := cs.GetRangeDupSup(VIMEOURL, keyrange)
			if !ok {
				return
			}
			log.Printf("time: %s: dupSupGetRange(%v), %d bytes", time.Since(start), keyrange, len(body))
		}(kr)
	}
	n.Wait() // wait for all requests to be done
}

// testSequentialFake does not rely on network.
func testSequentialFake(t *testing.T, cs *CacheServer) {
	for kr := range incomingRangeRequestsFake() {
		start := time.Now()
		body := getRangeValue(cs, kr)
		log.Printf("time: %s: GetRange(%v), %d bytes", time.Since(start), kr, len(body))
	}
}

// testConcurrentFake does not rely on network.
func testConcurrentFake(t *testing.T, cs *CacheServer) {
	var n sync.WaitGroup
	for kr := range incomingRangeRequestsFake() {
		n.Add(1) // add to wait group
		go func(keyrange rangecache.Keyrange) {
			defer n.Done() // defer done
			start := time.Now()
			body := getRangeValue(cs, keyrange)
			log.Printf("time: %s: GetRange(%v), %d bytes", time.Since(start), keyrange, len(body))
		}(kr)
	}
	n.Wait() // wait for all requests to be done
}

// testConcurrentFake does not rely on network.
func testConcurrentFakeDupSupression(t *testing.T, cs *CacheServer) {
	var n sync.WaitGroup
	for kr := range incomingRangeRequestsFake() {
		n.Add(1) // add to wait group
		go func(keyrange rangecache.Keyrange) {
			defer n.Done() // defer done
			start := time.Now()
			body := getRangeValueDupSup(cs, keyrange)
			log.Printf("time: %s: GetRange(%v), %d bytes", time.Since(start), keyrange, len(body))
		}(kr)
	}
	n.Wait() // wait for all requests to be done
}

// getRangeValues mirrors CacheServer.GetRange, but stubs the network call
// by calling the generateValue helper.
func getRangeValue(cs *CacheServer, keyrange rangecache.Keyrange) []int {
	timeStart := time.Now()
	v, ok := cs.get(keyrange)
	if ok { // cache hit
		body := v.([]int)
		log.Printf("cache hit: %s, GET: %v\n", time.Since(timeStart), keyrange)
		return body
	}

	// cache miss; make request
	body := generateValue(keyrange)
	cs.add(keyrange, body)
	log.Printf("cache miss: %s, GET: %v\n", time.Since(timeStart), keyrange)
	return body
}

// getRangeValueDupSup mirrors CacheServer.GetRangeDupSup, but stubs the network calls
// by calling the generateValue helper.
func getRangeValueDupSup(cs *CacheServer, keyrange rangecache.Keyrange) []int {
	cs.mu.Lock()
	e := cs.dup[keyrange]
	var body []int
	if e == nil { // first request for this range
		log.Printf("first request: %v\n", keyrange)
		e = &entry{ready: make(chan struct{})}
		cs.dup[keyrange] = e
		cs.mu.Unlock()

		// do work
		body = generateValue(keyrange)
		cs.add(keyrange, body)
		//cs.cache.Add(keyrange, body)
		close(e.ready)
	} else { // repeated range request
		log.Printf("repeated request: %v\n", keyrange)
		cs.mu.Unlock()
		<-e.ready // wait for ready
	}
	value, _ := cs.get(keyrange)
	return value.([]int)
}

// generateValue is a helper function that associats the value of a
// key range to a sequence of integers from start to end, inclusive.
func generateValue(kr rangecache.Keyrange) []int {
	total := kr.End - kr.Start
	start := kr.Start

	krValue := make([]int, total+1)
	for i := 0; i <= total; i++ {
		krValue[i] = start + i
		time.Sleep(1 * time.Millisecond)
	}
	return krValue
}

func TestSequentialReal(t *testing.T) {
	cs := NewCacheServer(nbytesMax)
	testSequentialReal(t, cs)
	//testSequentialRealDupSup(t, cs)
}

func TestConcurrentReal(t *testing.T) {
	cs := NewCacheServer(nbytesMax)
	testConcurrentReal(t, cs)
}

func TestConcurrentRealDupSup(t *testing.T) {
	cs := NewCacheServer(nbytesMax)
	testConcurrentRealDupSup(t, cs)
}

func TestSequentialFake(t *testing.T) {
	cs := NewCacheServer(nbytesMax)
	testSequentialFake(t, cs)
}

func TestConcurrentFake(t *testing.T) {
	cs := NewCacheServer(nbytesMax)
	// testConcurrentFake(t, cs)
	testConcurrentFakeDupSupression(t, cs)
}

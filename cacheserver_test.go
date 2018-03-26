package main

import (
	"log"
	"testing"
	"time"

	"github.com/zeneli/cacheserver/rangecache"
)

var url string = "http://storage.googleapis.com/vimeo-test/work-at-vimeo.mp4"
var nbytesMax int64 = 64000000 // 64 MB

// incomingRangeRequests simulates a channel of range requests.
func incomingRangeRequests() <-chan rangecache.Keyrange {
	ch := make(chan rangecache.Keyrange)
	go func() {
		for _, keyrange := range []rangecache.Keyrange{
			{0, 6400000},  // 6.4 MB
			{0, 3200000},  // 3.2 MB
			{0, 12800000}, // 12.8 MB
			{0, 10800000}, // 10.8 MB
			{0, 6400000},
			{0, 3200000},
			{0, 10800000},
			{0, 12800000},
		} {
			ch <- keyrange
		}
		close(ch)
	}()
	return ch
}

func testSequential(t *testing.T, cs *CacheServer) {
	for kr := range incomingRangeRequests() {
		start := time.Now()
		body, err := cs.GetRange(kr)
		if err != nil {
			log.Print(err)
			continue
		}
		log.Printf("time: %s: GetRange(%v), %d bytes", time.Since(start), kr, len(body))
	}
}

func TestSequential(t *testing.T) {
	cs := NewCacheServer(url, nbytesMax)
	testSequential(t, cs)
}

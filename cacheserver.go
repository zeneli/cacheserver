package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"sync"

	"github.com/zeneli/cacheserver/rangecache"
)

type entry struct {
	ready chan struct{} // close when value is ready
}

// CacheServer implements a caching server.
type CacheServer struct {
	mu    sync.Mutex // guards cache
	cache *rangecache.RangeCache
	dup   map[rangecache.Keyrange]*entry // cache of work in progress
}

// NewCache returns an initialized CacheServer.
func NewCacheServer(nbytes int64) *CacheServer {
	return &CacheServer{
		cache: rangecache.NewRangeCache(nbytes),
		dup:   make(map[rangecache.Keyrange]*entry),
	}
}

// ServeHTTP implements the HTTP user interface.
// Its responsible for parsing query paramters;
// a source url, start byte, and optional end byte.
// Ensuring the associated url supports range requests.
// And serve the requested range in a concurrently safe manner.
func (cs *CacheServer) ServeHTTP(w http.ResponseWriter, r *http.Request) { return }

// add is a wrapper around the caches add that is concurrency-safe.
func (cs *CacheServer) add(keyrange rangecache.Keyrange, body interface{}) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	switch body.(type) {
	case []int:
		cs.cache.Add(keyrange, body.([]int))
	case []byte:
		cs.cache.Add(keyrange, body.([]byte))
	}
}

// get is a wrapper around the caches get that is concurrency-safe.
func (cs *CacheServer) get(keyrange rangecache.Keyrange) (interface{}, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.cache.Get(keyrange)
}

// GetRangeDupSup checks the cache for keyrange, otherwise does an HTTP range request.
// Avoiding redundant keyrange requests by duplicate suppression.
// GetRangeDupSup is concurrency-safe.
func (cs *CacheServer) GetRangeDupSup(url string, keyrange rangecache.Keyrange) ([]byte, bool) {
	cs.mu.Lock()
	rangeHeader := fmt.Sprintf("bytes=%d-%d", keyrange.Start, keyrange.End)
	e := cs.dup[keyrange]
	if e == nil { // first request for this keyrange
		e = &entry{ready: make(chan struct{})}
		cs.dup[keyrange] = e // allocate entry; force other goroutines to wait
		cs.mu.Unlock()

		// do work
		body, err := httpGetRangeRequest(url, rangeHeader)
		if err != nil {
			return nil, false
		}
		cs.add(keyrange, body)

		// Broadcast to waiting goroutines the work is done.
		close(e.ready)
	} else { // repeated range request; suppress duplicate
		cs.mu.Unlock()
		<-e.ready // Wait for ready; other goroutine is handling work.
	}

	v, ok := cs.get(keyrange)
	if !ok {
		return nil, false
	}
	//ioutil.WriteFile(rangeHeader+".mp4", v.([]byte), 0666)
	return v.([]byte), true
}

// GetRange checks the cache for keyrange, otherwise does an HTTP range request.
// GetRange is concurrency-safe.
func (cs *CacheServer) GetRange(url string, keyrange rangecache.Keyrange) ([]byte, error) {
	//timeStart := time.Now()
	rangeHeader := fmt.Sprintf("bytes=%d-%d", keyrange.Start, keyrange.End)

	v, ok := cs.get(keyrange)
	if ok { // cache hit
		body := v.([]byte)
		//ioutil.WriteFile(rangeHeader+".mp4", v.([]byte), 0x777) // write to file
		//log.Printf("cache hit: %s, GET: %s\n", time.Since(timeStart), rangeHeader)
		return body, nil
	}

	// cache miss; make request
	body, err := httpGetRangeRequest(url, rangeHeader)
	if err != nil {
		return nil, err
	}
	cs.add(keyrange, body)
	// ioutil.WriteFile(rangeHeader+".mp4", []byte(string(body)), 0666)
	// log.Printf("cache miss: %s, GET: %s\n", time.Since(timeStart), rangeHeader)
	return body, nil
}

// httpGetRangeRequest is a helper function that creates an HTTP client,
// adds the range header, and returns the request body data.
func httpGetRangeRequest(url, rangeHeader string) ([]byte, error) {
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Range", rangeHeader)

	resp, err := client.Do(req)
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return body, nil
}

func main() {
	// Stub the source URL. Exercise cache hit and miss expectations.
	// We log the times and save the video files.
	url := "http://storage.googleapis.com/vimeo-test/work-at-vimeo.mp4"
	cs := NewCacheServer(64000000) // 64 MB cache server

	cs.GetRangeDupSup(url, rangecache.Keyrange{0, 6400000}) // 6.4 MB
	cs.GetRangeDupSup(url, rangecache.Keyrange{0, 1600000}) // exact match; 1.6 MB
	cs.GetRangeDupSup(url, rangecache.Keyrange{0, 3200000}) // overlapping match; 3.2 MB
}

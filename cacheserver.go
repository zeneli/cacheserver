package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/zeneli/cacheserver/rangecache"
)

// CacheServer implements a caching server.
type CacheServer struct {
	mu    sync.Mutex // guards cache
	cache *rangecache.RangeCache
}

// NewCache returns an initialized CacheServer.
func NewCacheServer(nbytes int64) *CacheServer {
	return &CacheServer{cache: rangecache.NewRangeCache(nbytes)}
}

// ServeHTTP implements the HTTP user interface.
// Its responsible for parsing query paramters;
// a source url, start byte, and optional end byte.
// Ensuring the associated url supports range requests.
// And serve the requested range in a concurrently safe manner.
func (cs *CacheServer) ServeHTTP(w http.ResponseWriter, r *http.Request) { return }

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

func (cs *CacheServer) get(keyrange rangecache.Keyrange) (interface{}, bool) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.cache.Get(keyrange)
}

// GetRange looks in the cache for keyrange start to end.
// On a miss, GetRange makes an HTTP GET request for the range and
// stores it in the cache.
// error is propogated to the calling function.
// GetRange is concurrency-safe.
func (cs *CacheServer) GetRange(url string, keyrange rangecache.Keyrange) ([]byte, error) {
	timeStart := time.Now()
	rangeHeader := fmt.Sprintf("bytes=%d-%d", keyrange.Start, keyrange.End)

	v, ok := cs.get(keyrange)
	if ok { // cache hit
		body := v.([]byte)
		ioutil.WriteFile(rangeHeader+".mp4", v.([]byte), 0x777) // write to file
		log.Printf("cache hit: %s, GET: %s\n", time.Since(timeStart), rangeHeader)
		return body, nil
	}

	// cache miss; make request
	body, err := httpGetRangeRequest(url, rangeHeader)
	if err != nil {
		return nil, err
	}
	cs.add(keyrange, body)
	ioutil.WriteFile(rangeHeader+".mp4", []byte(string(body)), 0666)
	log.Printf("cache miss: %s, GET: %s\n", time.Since(timeStart), rangeHeader)
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

	cs.GetRange(url, rangecache.Keyrange{0, 6400000}) // 6.4 MB
	cs.GetRange(url, rangecache.Keyrange{0, 1600000}) // exact match; 1.6 MB
	cs.GetRange(url, rangecache.Keyrange{0, 3200000}) // overlapping match; 3.2 MB
}

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
	url   string
}

// NewCache returns an initialized CacheServer.
func NewCacheServer(url string, nbytes int64) *CacheServer {
	return &CacheServer{
		cache: rangecache.NewRangeCache(nbytes),
		url:   url,
	}
}

// ServeHTTP implements the HTTP user interface.
// Its responsible for parsing query paramters;
// a source url, start byte, and optional end byte.
// Ensuring the associated url supports range requests.
// And serve the requested range in a concurrently safe manner.
func (cs *CacheServer) ServeHTTP(w http.ResponseWriter, r *http.Request) { return }

// GetRange looks in the cache for keyrange start to end.
// On a miss, GetRange makes an HTTP GET request for the range and
// stores it in the cache.
func (cs *CacheServer) GetRange(start, end int) {
	timeStart := time.Now()
	keyrange := rangecache.Keyrange{start, end}
	rangeHeader := fmt.Sprintf("bytes=%d-%d", start, end)

	if v, ok := cs.cache.Get(keyrange); ok { // cache hit
		ioutil.WriteFile(rangeHeader+".mp4", v.([]byte), 0x777) // write to file
		log.Printf("cache hit: %s, GET: %s\n", time.Since(timeStart), rangeHeader)
		return
	}
	// cache miss; make request
	body, err := httpGetRangeRequest(cs.url, rangeHeader)
	if err != nil {
		log.Fatalf("GetRange: %v", err)
	}
	cs.cache.Add(keyrange, body)
	ioutil.WriteFile(rangeHeader+".mp4", []byte(string(body)), 0666)
	log.Printf("cache hit: %s, GET: %s\n", time.Since(timeStart), rangeHeader)
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
	cs := NewCacheServer(url, 64000000)
	cs.GetRange(0, 3999999)
	cs.GetRange(0, 3999999) // exact match
	cs.GetRange(0, 399999)  // overlapping match
}

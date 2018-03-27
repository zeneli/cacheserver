package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"sync"

	"github.com/zeneli/cacheserver/rangecache"
)

const (
	VIMEOURL = "http://storage.googleapis.com/vimeo-test/work-at-vimeo.mp4"
	// sizes
	nbytesMax = 64000000 // 64 MB
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
func (cs *CacheServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// use VIDEOURL for testing
	sourceURL := VIMEOURL
	_, start, end, err := processRequiredQueryParams(r.URL.String())
	if err != nil {
		fmt.Fprintln(w, err)
		return
	}
	contentLength, ok := checkHTTPRangeSupportAndLength(sourceURL)
	if !ok {
		fmt.Fprintln(w, "%s does not supports HTPP byte ranges", sourceURL)
		return
	}

	if end > contentLength { // check end bound
		end = contentLength
	}

	body, ok := cs.GetRangeDupSup(sourceURL, rangecache.Keyrange{int(start), int(end)})
	if !ok {
		fmt.Fprintln(w, "Couldn't get that")
	}
	w.Header().Add("Content-Type", "video/mp4")
	w.Write(body)
	return
}

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
	ioutil.WriteFile(rangeHeader+".mp4", v.([]byte), 0666)
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

// processRequiredQueryParams checks the given query URL and returns the query params
// matching "url", "start", "end" (optional).
func processRequiredQueryParams(queryURL string) (string, int64, int64, error) {
	u, err := url.Parse(queryURL)
	if err != nil {
		return "", 0, 0, err
	}
	q := u.Query()
	sourceURL := q.Get("url")
	start := q.Get("start")
	end := q.Get("end")

	if sourceURL == "" { // validate sourceUrl exists
		return "", 0, 0, errors.New("query paramater url is required")
	}
	if start == "" { // validate start exists
		return "", 0, 0, errors.New("query paramater start is required")
	}
	starti, err := strconv.Atoi(start)
	if err != nil {
		return "", 0, 0, err
	}

	if end == "" { // end query param is optional; default to 64 KB after starti
		end += fmt.Sprintf("%d", starti+64000)
	}
	endi, err := strconv.Atoi(end)
	if err != nil {
		return "", 0, 0, err
	}

	return sourceURL, int64(starti), int64(endi), nil
}

// checkHTTPRangeSupportAndLength does an HTTP head request to the sourceURL and checks
// if it supports HTTP byte ranges. Also returns the content length for bounds checking.
func checkHTTPRangeSupportAndLength(sourceURL string) (contentLength int64, ok bool) {
	client := &http.Client{}
	resp, err := client.Head(sourceURL)
	if err != nil {
		return 0, false
	}
	for _, rangeSupport := range resp.Header["Accept-Ranges"] {
		if rangeSupport == "bytes" { // supports HTTP byte ranges
			// HTTP header for Content-Length
			contentLength, err := strconv.Atoi(resp.Header["Content-Length"][0])
			if err != nil {
				return 0, false
			}
			return int64(contentLength), true
		} else {
			return 0, false
		}
	} // fell through
	return 0, false
}

func main() {
	cacheserver := NewCacheServer(nbytesMax) // 64 MB cache server
	s := &http.Server{
		Addr:    ":8080",
		Handler: cacheserver,
	}
	log.Fatal(s.ListenAndServe())
}

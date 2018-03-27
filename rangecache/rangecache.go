// Package rangecache implements a range cache based on LRU policy.
package rangecache

import (
	"container/list"
)

// Keyrange is comparable key.
// Keyrange describes an inclusive range, viz. [Start, End]
type Keyrange struct{ Start, End int }

// RangeCache is a LRU range-based cache.
// RangeCache is not safe for concurrent accesses.
type RangeCache struct {
	lrulist    *list.List
	rangecache map[Keyrange]*list.Element
	nbytesUsed int64
	nbyteLimit int64
}

type item struct {
	keyrange Keyrange
	value    interface{}
}

// NewRangeCache creates a new RangeCache.
func NewRangeCache(byteLimit int64) *RangeCache {
	return &RangeCache{
		lrulist:    list.New(),
		rangecache: make(map[Keyrange]*list.Element),
		nbytesUsed: 0,
		nbyteLimit: byteLimit,
	}
}

// Add associates a keyrange with a value and addes it to the range cache.
// If the range cache is nil, then create one with a default size of 64 MB.
func (rc *RangeCache) Add(keyrange Keyrange, value interface{}) {
	if rc.rangecache == nil { // Guard against empty range cache.
		rc = NewRangeCache(64000000) // 64MB default.
	}
	// Cache hit.
	if e, ok := rc.rangecache[keyrange]; ok {
		rc.lrulist.MoveToFront(e)
		e.Value.(*item).value = value
		return
	}

	// Before Add, check storage constraints. Evict if not met.
	var nbytesReq int64
	switch value.(type) {
	case []int:
		nbytesReq = int64(len(value.([]int)) * 64)
	case []byte:
		nbytesReq = int64(len(value.([]byte)))
	}
	nbytesAvailable := rc.nbyteLimit - rc.nbytesUsed

	// log.Printf("nbytesAvailable(%v) < nbytesReq(%v)\n", nbytesAvailable, nbytesReq)
	for nbytesAvailable < nbytesReq {
		rc.evict()
		nbytesAvailable = rc.nbyteLimit - rc.nbytesUsed
	}
	e := rc.lrulist.PushFront(&item{keyrange, value})
	rc.rangecache[keyrange] = e

	// Assume 64-bit architecture. int is 64 bits wide on 64-bit systems.
	switch value.(type) {
	case []int:
		rc.nbytesUsed += int64(len(e.Value.(*item).value.([]int)) * 64)
	case []byte:
		rc.nbytesUsed += int64(len(e.Value.(*item).value.([]byte)))
	}
	// log.Printf("nbytesUsed: %d\n", rc.nbytesUsed)
}

// Get looks up a keyrange's value from the range cache.
func (rc *RangeCache) Get(keyrange Keyrange) (value interface{}, ok bool) {
	if rc.rangecache == nil {
		return nil, false
	}
	if e, ok := rc.rangecache[keyrange]; ok { // Fast hit.
		rc.lrulist.MoveToFront(e)
		return e.Value.(*item).value, true
	} else if e, v, ok := rc.liesInRange(keyrange); ok { // Slow hit.
		rc.lrulist.MoveToFront(e)
		return v, ok
	}
	return nil, false
}

// evict evicts the least recently used keyrange and value item from the range cache.
func (rc *RangeCache) evict() {
	if rc.rangecache == nil {
		return
	}
	e := rc.lrulist.Back()
	if e != nil {
		rc.lrulist.Remove(e)
		item := e.Value.(*item)
		delete(rc.rangecache, item.keyrange)
		var bFreed int64
		switch item.value.(type) {
		case []int:
			bFreed = int64(len(item.value.([]int)) * 64)
		case []byte:
			bFreed = int64(len(item.value.([]byte)) * 64)
		}
		rc.nbytesUsed -= bFreed
	}
}

// BytesUsed returns the number of bytes used in the range cache.
func (rc *RangeCache) BytesUsed() int64 { return rc.nbytesUsed }

func (rc *RangeCache) liesInRange(keyrange Keyrange) (*list.Element, interface{}, bool) {
	if rc.rangecache == nil {
		return nil, nil, false
	}

	starts := make(map[int]*list.Element)
	ends := make(map[int]*list.Element)

	for kr, e := range rc.rangecache {
		if kr.Start <= keyrange.Start {
			starts[kr.Start] = e
		}
		if kr.End >= keyrange.End {
			ends[kr.End] = e
		}
	}

	// log.Printf("keyrange: %v\nstarts: %v\nends: %v\n", keyrange, starts, ends)

	for start := range starts {
		for end := range ends {
			if starts[start] == ends[end] { // keyrange is inside cached range.
				e := rc.rangecache[Keyrange{start, end}]
				var value interface{}
				switch e.Value.(*item).value.(type) {
				case []int:
					value = e.Value.(*item).value.([]int)[keyrange.Start : keyrange.End+1]
				case []byte:
					value = e.Value.(*item).value.([]byte)[keyrange.Start : keyrange.End+1]
				default:
					value = e.Value.(*item).value.([]byte)[keyrange.Start : keyrange.End+1]
				}
				// log.Printf("slice at [%d:%d] = %v\n\n", keyrange.Start, keyrange.End, value)
				return e, value, true
			}
		}
	}
	return nil, nil, false
}

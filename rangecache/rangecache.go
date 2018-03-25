// Package rangecache implements a range cache based on LRU policy.
package rangecache

import "container/list"

// Keyrange is comparable key.
// Keyrange describes an inclusive range, viz. [Start, End]
type Keyrange struct{ Start, End int }

// RangeCache is a LRU range-based cache.
// RangeCache is not safe for concurrent accesses.
// TODO: add descriptions of cases for get and add overlaps.
type RangeCache struct {
	lrulist    *list.List
	rangecache map[Keyrange]*list.Element
}

type item struct {
	keyrange Keyrange
	value    interface{}
}

// New creates a new RangeCache.
func New() *RangeCache {
	return &RangeCache{
		lrulist:    list.New(),
		rangecache: make(map[Keyrange]*list.Element),
	}
}

// Add associates a keyrange with a value and addes it to the range cache.
func (rc *RangeCache) Add(keyrange Keyrange, value interface{}) {
	if rc.rangecache == nil { // Guard against empty range cache
		rc.lrulist = list.New()
		rc.rangecache = make(map[Keyrange]*list.Element)
	}
	if e, ok := rc.rangecache[keyrange]; ok {
		rc.lrulist.MoveToFront(e)
		e.Value.(*item).value = value
		return
	}

	e := rc.lrulist.PushFront(&item{keyrange, value})
	rc.rangecache[keyrange] = e
}

// Get looks up a keyrange's value from the range cache.
func (rc *RangeCache) Get(keyrange Keyrange) (value interface{}, ok bool) {
	if rc.rangecache == nil {
		return nil, false
	}
	if e, ok := rc.rangecache[keyrange]; ok {
		rc.lrulist.MoveToFront(e)
		return e.Value.(*item).value, true
	}
	return nil, false
}

// Evict evicts the least recently used keyrange and value item from the range cache.
// If an item was evicted successfully, we have the updated bytes used.
func (rc *RangeCache) Evict() (bytesUsed int64, ok bool) { return 0, false }

// BytesUsed returns the number of bytes used in the range cache.
func (rc *RangeCache) BytesUsed() int64 { return 0 }

// Len returns the number of items in the range cache.
func (rc *RangeCache) Len() int { return 0 }
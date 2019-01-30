package datastore

import (
	"bytes"
	"encoding/gob"
	"sort"

	"github.com/allegro/bigcache"
	"github.com/puppetlabs/wash/log"
)

// CachedJSON retrieves cached JSON. If uncached, uses the callback to initialize the cache.
func CachedJSON(cache *bigcache.BigCache, key string, cb func() ([]byte, error)) ([]byte, error) {
	entry, err := cache.Get(key)
	if err == nil {
		log.Debugf("Cache hit on %v", key)
		return entry, nil
	}

	// Cache misses should be rarer, so always print them. Frequent messages are a sign of problems.
	log.Printf("Cache miss on %v", key)
	entry, err = cb()
	if err != nil {
		return nil, err
	}
	cache.Set(key, entry)
	return entry, nil
}

// CachedStrings retrieves a cached array of strings. If uncached, uses the callback to initialize the cache.
// Returned array will always be sorted lexicographically.
func CachedStrings(cache *bigcache.BigCache, key string, cb func() ([]string, error)) ([]string, error) {
	entry, err := cache.Get(key)
	if err == nil {
		log.Debugf("Cache hit on %v", key)
		var strings []string
		dec := gob.NewDecoder(bytes.NewReader(entry))
		err = dec.Decode(&strings)
		return strings, err
	}

	// Cache misses should be rarer, so always print them. Frequent messages are a sign of problems.
	log.Printf("Cache miss on %v", key)
	strings, err := cb()
	if err != nil {
		return nil, err
	}

	// Guarantee results are sorted.
	sort.Strings(strings)

	var data bytes.Buffer
	enc := gob.NewEncoder(&data)
	if err := enc.Encode(&strings); err != nil {
		return nil, err
	}
	cache.Set(key, data.Bytes())
	return strings, nil
}
package cache

import (
	"container/list"
	"sync"
	"time"
)

type CacheEntry struct {
	Key         string
	Value       []byte
	Expiration  time.Time
	AccessCount int
}

type ImageCache struct {
	capacity      int
	items         map[string]*list.Element
	lruList       *list.List
	mu            sync.RWMutex
	hitCount      int64
	missCount     int64
	evictionCount int64
}

func NewImageCache(capacity int) *ImageCache {
	return &ImageCache{
		capacity: capacity,
		items:    make(map[string]*list.Element),
		lruList:  list.New(),
	}
}

func (c *ImageCache) Set(key string, value []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	expiration := time.Now().Add(ttl)

	// key exists -> update it
	if elem, exists := c.items[key]; exists {
		c.lruList.MoveToFront(elem)
		entry := elem.Value.(*CacheEntry)
		entry.Value = value
		entry.Expiration = expiration
		return
	}

	entry := &CacheEntry{
		Key:        key,
		Value:      value,
		Expiration: expiration,
	}
	elem := c.lruList.PushFront(entry)
	c.items[key] = elem

	// Eviction logic
	if c.lruList.Len() > c.capacity {
		c.evict()
	}
}

func (c *ImageCache) Get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	elem, exists := c.items[key]
	if !exists {
		c.missCount++
		return nil, false
	}

	entry := elem.Value.(*CacheEntry)

	// Check expiration
	if time.Now().After(entry.Expiration) {
		c.removeElement(elem)
		c.missCount++
		return nil, false
	}

	// Move to front (most recently used)
	c.lruList.MoveToFront(elem)
	entry.AccessCount++
	c.hitCount++

	return entry.Value, true
}

func (c *ImageCache) evict() {
	elem := c.lruList.Back()
	if elem != nil {
		c.removeElement(elem)
		c.evictionCount++
	}
}

func (c *ImageCache) removeElement(elem *list.Element) {
	c.lruList.Remove(elem)
	entry := elem.Value.(*CacheEntry)
	delete(c.items, entry.Key)
}

func (c *ImageCache) Size() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.lruList.Len()
}

func (c *ImageCache) Capacity() int {
	return c.capacity
}

func (c *ImageCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.items = make(map[string]*list.Element)
	c.lruList = list.New()
}

type CacheStats struct {
	Size          int     `json:"size"`
	Capacity      int     `json:"capacity"`
	HitCount      int64   `json:"hit_count"`
	MissCount     int64   `json:"miss_count"`
	HitRate       float64 `json:"hit_rate"`
	EvictionCount int64   `json:"eviction_count"`
}

func (c *ImageCache) GetStats() CacheStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hitCount + c.missCount
	hitRate := 0.0
	if total > 0 {
		hitRate = float64(c.hitCount) / float64(total) * 100
	}

	return CacheStats{
		Size:          c.lruList.Len(),
		Capacity:      c.capacity,
		HitCount:      c.hitCount,
		MissCount:     c.missCount,
		HitRate:       hitRate,
		EvictionCount: c.evictionCount,
	}
}

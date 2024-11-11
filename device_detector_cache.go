package devicedetector

import (
	"container/list"
	"sync"
)

const CACHE_DEFAULT_SIZE int = 1000

type Cache struct {
	cache    map[string]*DeviceInfo
	size     int
	key_list *list.List
	mu       sync.RWMutex
}

func NewCache(size int) *Cache {
	return &Cache{
		cache:    make(map[string]*DeviceInfo),
		size:     size,
		key_list: list.New(),
		mu:       sync.RWMutex{},
	}
}

// Associate a deviceInfo element with the userAgent
func (d *Cache) Add(ua string, deviceInfo *DeviceInfo) {
	d.mu.Lock()

	// Add ua to the cache
	d.cache[ua] = deviceInfo

	// Add ua to the key_list and keep control of the size of the cache
	d.key_list.PushBack(ua)
	if d.key_list.Len() > d.size {
		// Remove exceeding ua from the list
		most_recent_ua := d.key_list.Front()

		d.key_list.Remove(most_recent_ua)

		// Remove exceeding ua from the cache
		delete(d.cache, (most_recent_ua.Value).(string))
	}

	d.mu.Unlock()
}

// Look for a cached userAgent: if found, hit is true.
func (d *Cache) Lookup(ua string) (deviceInfo *DeviceInfo, hit bool) {
	d.mu.RLock()
	deviceInfo, hit = d.cache[ua]
	d.mu.RUnlock()

	return deviceInfo, hit
}

// Purge the cache
func (d *Cache) Purge() {
	d.mu.Lock()
	d.cache = make(map[string]*DeviceInfo)
	d.key_list.Init()
	d.mu.Unlock()
}

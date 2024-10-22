package devicedetector

import "sync"

type Cache struct {
	cache map[string]*DeviceInfo
	mu    sync.RWMutex
}

func NewCache() *Cache {
	return &Cache{
		cache: make(map[string]*DeviceInfo),
		mu:    sync.RWMutex{},
	}
}

// Associate a deviceInfo element with the userAgent
func (d *Cache) Add(ua string, deviceInfo *DeviceInfo) {
	d.mu.Lock()
	d.cache[ua] = deviceInfo
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
	d.mu.Unlock()
}

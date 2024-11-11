package devicedetector

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNewCache(t *testing.T) {
	// Initialize the cache
	cache := NewCache(10)

	require.NotNil(t, cache)
	require.Equal(t, map[string]*DeviceInfo{}, cache.cache)
	require.Equal(t, 10, cache.size)
	require.Equal(t, 0, cache.key_list.Len())
}

func TestAddToCache(t *testing.T) {
	// Initialize the cache
	cache := NewCache(1)

	require.NotNil(t, cache)
	require.Equal(t, map[string]*DeviceInfo{}, cache.cache)
	require.Equal(t, 1, cache.size)
	require.Equal(t, 0, cache.key_list.Len())

	// Add an element to the cache
	deviceInfo := &DeviceInfo{
		userAgent: "test-user-agent",
	}

	cache.Add("test-user-agent", deviceInfo)

	// Verify the element has been cached
	require.NotEmpty(t, cache.cache)
	require.Len(t, cache.cache, 1)
	require.Equal(t, 1, cache.key_list.Len())

	require.Equal(t, "test-user-agent", cache.key_list.Front().Value)

	cachedDeviceInfo, hit := cache.Lookup("test-user-agent")

	require.NotNil(t, cachedDeviceInfo)
	require.True(t, hit)
	require.EqualValues(t, deviceInfo, cachedDeviceInfo)

	// Add another element to the cache
	deviceInfo1 := &DeviceInfo{
		userAgent: "test-user-agent-1",
	}

	cache.Add("test-user-agent-1", deviceInfo1)

	// Verify the new element has been cached and the other has been removed
	require.NotEmpty(t, cache.cache)
	require.Len(t, cache.cache, 1)
	require.Equal(t, 1, cache.key_list.Len())

	require.Equal(t, "test-user-agent-1", cache.key_list.Front().Value)

	cachedDeviceInfo, hit = cache.Lookup("test-user-agent-1")

	require.NotNil(t, cachedDeviceInfo)
	require.True(t, hit)
	require.EqualValues(t, deviceInfo1, cachedDeviceInfo)
}

func TestLookupCache(t *testing.T) {
	// Initialize the cache
	cache := NewCache(1)

	require.NotNil(t, cache)
	require.Equal(t, map[string]*DeviceInfo{}, cache.cache)
	require.Equal(t, 1, cache.size)
	require.Equal(t, 0, cache.key_list.Len())

	// Add an element to the cache
	deviceInfo := &DeviceInfo{
		userAgent: "test-user-agent",
	}

	cache.Add("test-user-agent", deviceInfo)

	// Verify the element has been cached
	require.NotEmpty(t, cache.cache)
	require.Equal(t, 1, cache.key_list.Len())

	cachedDeviceInfo, hit := cache.Lookup("test-user-agent")

	require.NotNil(t, cachedDeviceInfo)
	require.True(t, hit)
	require.EqualValues(t, deviceInfo, cachedDeviceInfo)

	// Verify the cache in case of miss of the elmenent
	cachedDeviceInfo, hit = cache.Lookup("not-cached-user-agent")

	require.Nil(t, cachedDeviceInfo)
	require.False(t, hit)
}

func TestPurgeCache(t *testing.T) {
	// Initialize the cache
	cache := NewCache(1)

	require.NotNil(t, cache)
	require.Equal(t, map[string]*DeviceInfo{}, cache.cache)
	require.Equal(t, 0, cache.key_list.Len())
	require.Equal(t, 1, cache.size)

	// Add an element to the cache
	deviceInfo := &DeviceInfo{
		userAgent: "test-user-agent",
	}

	cache.Add("test-user-agent", deviceInfo)

	// Verify the element has been cached
	require.NotEmpty(t, cache.cache)
	require.Equal(t, 1, cache.key_list.Len())

	cachedDeviceInfo, hit := cache.Lookup("test-user-agent")

	require.NotNil(t, cachedDeviceInfo)
	require.True(t, hit)
	require.EqualValues(t, deviceInfo, cachedDeviceInfo)

	// Verify the cache after the purge
	cache.Purge()

	require.Equal(t, map[string]*DeviceInfo{}, cache.cache)
	require.Equal(t, 0, cache.key_list.Len())
}

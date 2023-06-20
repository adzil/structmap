package structmap

import "sync"

type cache[K comparable, V any] struct {
	mu   sync.RWMutex
	stor map[K]V
}

func (c *cache[K, V]) getSync(key K, getter func(K) (V, error)) (val V, err error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if val, ok := c.stor[key]; ok {
		return val, nil
	}

	if val, err = getter(key); err != nil {
		return val, err
	}

	if c.stor == nil {
		c.stor = make(map[K]V)
	}

	c.stor[key] = val

	return val, nil
}

func (c *cache[K, V]) Get(key K, getter func(K) (V, error)) (val V, err error) {
	c.mu.RLock()
	val, ok := c.stor[key]
	c.mu.RUnlock()

	if !ok {
		val, err = c.getSync(key, getter)
	}

	return val, err
}

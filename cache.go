/*
Copyright 2023 Fadhli Dzil Ikram.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

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

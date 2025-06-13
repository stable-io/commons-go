package secrets

import (
	"sync"
)

type ConcurrentValue[T any] struct {
	value T
	sync.RWMutex
}

func (cv *ConcurrentValue[T]) Get() T {
	cv.RLock()
	defer cv.RUnlock()
	return cv.value
}

func (cv *ConcurrentValue[T]) Set(newValue T) {
	cv.Lock()
	cv.value = newValue
	cv.Unlock()
}

type ConcurrentList[T any] struct {
	sync.RWMutex
	values []T
}

// Add appends a new value to the list
func (cl *ConcurrentList[T]) Add(value T) {
	cl.Lock()
	defer cl.Unlock()
	cl.values = append(cl.values, value)
}

// Get returns a copy of the current list of values
func (cl *ConcurrentList[T]) Get() []T {
	cl.RLock()
	defer cl.RUnlock()
	numVals := len(cl.values)
	if numVals == 0 {
		return nil
	}
	result := make([]T, 0, numVals)
	result = append(result, cl.values...)
	return result
}

// Set Replace the entire list with a new one
func (cl *ConcurrentList[T]) Set(newList []T) {
	cl.Lock()
	defer cl.Unlock()
	cl.values = make([]T, 0, len(newList))
	cl.values = append(cl.values, newList...)
}

type ConcurrentMap[K comparable, V any] struct {
	sync.RWMutex
	value map[K]V
}

func (cm *ConcurrentMap[K, V]) Get(key K) (V, bool) {
	cm.RLock()
	val, exists := cm.value[key]
	cm.RUnlock()
	return val, exists
}

func (cm *ConcurrentMap[K, V]) Set(key K, value V) {
	cm.Lock()
	cm.value[key] = value
	cm.Unlock()
}

func (cm *ConcurrentMap[K, V]) Del(key K) {
	cm.Lock()
	delete(cm.value, key)
	cm.Unlock()
}

func (cm *ConcurrentMap[K, V]) CopyMap() map[K]V {
	cm.RLock()
	defer cm.RUnlock()
	newMap := make(map[K]V, len(cm.value))
	for k, v := range cm.value {
		newMap[k] = v
	}
	return newMap
}

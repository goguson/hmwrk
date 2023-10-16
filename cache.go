package main

type cache map[string]map[string]int

type InMemoryCache interface {
	Get(key string) (map[string]int, bool)
	Exists(key string) bool
	Set(key string, value map[string]int)
}

func NewInMemoryCache() InMemoryCache {
	return cache{}
}

func (c cache) Get(key string) (map[string]int, bool) {
	if _, ok := c[key]; ok {
		return c[key], true
	}
	return nil, false
}

func (c cache) Exists(key string) bool {
	_, ok := c[key]
	return ok
}

func (c cache) Set(key string, value map[string]int) {
	c[key] = value
}

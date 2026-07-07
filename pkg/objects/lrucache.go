package objects

// kwMarkObject is the sentinel functools.lru_cache splices into a cache key
// between the positional and keyword parts, so a value passed positionally
// never collides with the same value passed by keyword. It matches CPython's
// kwd_mark = (object(),): one shared, otherwise opaque value.
type kwMarkObject struct{}

func (*kwMarkObject) TypeName() string { return "object" }

var kwMark = &kwMarkObject{}

// lruCacheObject is the wrapper functools.lru_cache returns: a callable that
// memoizes results keyed on the call arguments, evicting the least recently
// used entry once a bounded cache is full. maxsize is -1 for an unbounded cache
// (lru_cache(maxsize=None) and cache), 0 for a disabled one that only counts
// misses, and positive for a bounded one. It mirrors CPython's
// _lru_cache_wrapper.
type lruCacheObject struct {
	fn      Object
	maxsize int
	typed   bool
	cache   *dictObject
	hits    int64
	misses  int64
}

func (*lruCacheObject) TypeName() string { return "functools._lru_cache_wrapper" }

// NewLRUCache wraps fn in an lru_cache. maxsize is -1 for unbounded, 0 for
// disabled, or a positive bound; typed keys arguments by type so 3 and 3.0 do
// not share an entry.
func NewLRUCache(fn Object, maxsize int, typed bool) Object {
	return &lruCacheObject{fn: fn, maxsize: maxsize, typed: typed, cache: &dictObject{index: map[string]int{}}}
}

// lruCall answers a call from the cache or forwards it to the wrapped function.
// A disabled cache only forwards and counts the miss. A bounded cache moves a
// hit to the most-recent end and evicts the oldest entry once it overflows.
func lruCall(c *lruCacheObject, args []Object, kwNames []string, kwVals []Object) (Object, error) {
	if c.maxsize == 0 {
		c.misses++
		return CallKw(c.fn, args, kwNames, kwVals)
	}
	key := makeCacheKey(args, kwNames, kwVals, c.typed)
	if v, ok, err := c.cache.lookup(key); err != nil {
		return nil, err
	} else if ok {
		c.hits++
		if c.maxsize > 0 {
			c.moveToEnd(key)
		}
		return v, nil
	}
	res, err := CallKw(c.fn, args, kwNames, kwVals)
	if err != nil {
		return nil, err
	}
	c.misses++
	if err := c.cache.set(key, res); err != nil {
		return nil, err
	}
	if c.maxsize > 0 && len(c.cache.entries) > c.maxsize {
		c.cache.entries = c.cache.entries[1:]
		c.cache.reindex()
	}
	return res, nil
}

// moveToEnd lifts the hit entry to the most-recent position, so the eviction of
// the oldest tracks true least-recently-used order.
func (c *lruCacheObject) moveToEnd(key Object) {
	k, err := dictKey(key)
	if err != nil {
		return
	}
	idx, ok := c.cache.index[k]
	if !ok {
		return
	}
	e := c.cache.entries[idx]
	c.cache.entries = append(c.cache.entries[:idx], c.cache.entries[idx+1:]...)
	c.cache.entries = append(c.cache.entries, e)
	c.cache.reindex()
}

// makeCacheKey builds the hashable cache key the way CPython's _make_key does:
// the positional arguments, then a sentinel and the flattened keyword pairs when
// there are keywords, then the argument types when typed. A single int or str
// argument keys on itself, the fast path that keeps 3 (an int) and 3.0 (which
// wraps in a one-tuple) in distinct entries.
func makeCacheKey(args []Object, kwNames []string, kwVals []Object, typed bool) Object {
	key := append([]Object(nil), args...)
	if len(kwNames) > 0 {
		key = append(key, kwMark)
		for i, n := range kwNames {
			key = append(key, NewStr(n), kwVals[i])
		}
	}
	if typed {
		for _, a := range args {
			key = append(key, NewStr(a.TypeName()))
		}
		for _, v := range kwVals {
			key = append(key, NewStr(v.TypeName()))
		}
	} else if len(key) == 1 {
		switch key[0].(type) {
		case *intObject, *strObject:
			return key[0]
		}
	}
	return NewTuple(key)
}

// lruAttr reads the wrapper's introspection surface: the two cache methods, the
// wrapped function, and the name that reads through to it.
func lruAttr(c *lruCacheObject, name string) (Object, error) {
	switch name {
	case "cache_info":
		return NewFunc("cache_info", 0, func([]Object) (Object, error) {
			return lruCacheInfo(c)
		}), nil
	case "cache_clear":
		return NewFunc("cache_clear", 0, func([]Object) (Object, error) {
			c.cache = &dictObject{index: map[string]int{}}
			c.hits, c.misses = 0, 0
			return None, nil
		}), nil
	case "__wrapped__":
		return c.fn, nil
	case "__name__", "__qualname__":
		return LoadAttr(c.fn, name)
	}
	return nil, Raise(AttributeError, "'functools._lru_cache_wrapper' object has no attribute '%s'", name)
}

// lruCacheInfo builds the CacheInfo namedtuple CPython returns: hits, misses,
// the maxsize (None when unbounded), and the current entry count.
func lruCacheInfo(c *lruCacheObject) (Object, error) {
	maxsize := None
	if c.maxsize >= 0 {
		maxsize = NewInt(int64(c.maxsize))
	}
	return Call(cacheInfoType(), []Object{
		NewInt(c.hits), NewInt(c.misses), maxsize, NewInt(int64(len(c.cache.entries))),
	})
}

// cacheInfoType is the CacheInfo namedtuple, built once and shared, matching the
// class functools defines at module load.
var cacheInfoNT Object

func cacheInfoType() Object {
	if cacheInfoNT == nil {
		t, err := NewNamedTupleType("CacheInfo", []string{"hits", "misses", "maxsize", "currsize"}, nil)
		if err != nil {
			panic("unagi: CacheInfo namedtuple: " + err.Error())
		}
		cacheInfoNT = t
	}
	return cacheInfoNT
}

package objects

// IsCoroutineObject backs asyncio.iscoroutine. It reports whether o is a
// coroutine, the object an async def call produces. Coroutines are the
// *generatorObject the frame machinery drives with isCoro set; an async
// generator carries isAsyncGen instead and is not a coroutine, matching
// CPython where iscoroutine is false for async generators.
func IsCoroutineObject(o Object) bool {
	g, ok := o.(*generatorObject)
	return ok && g.isCoro
}

// IsFutureObject backs asyncio.isfuture. It reports whether o is an asyncio
// Future or Task, the two objects the loop awaits natively. A
// concurrent.futures.Future is a different type and is not an asyncio future,
// matching CPython where isfuture is false for it.
func IsFutureObject(o Object) bool {
	switch o.(type) {
	case *asyncFuture, *asyncTask:
		return true
	}
	return false
}

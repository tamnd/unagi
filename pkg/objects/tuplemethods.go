package objects

func tupleMethod(x *tupleObject, name string, args []Object) (Object, error) {
	return nil, noAttr(x, name)
}

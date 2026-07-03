package frontend

import "fmt"

// SyntaxError is the one error type the frontend produces. It carries the
// file, the 1-based position, and a message shaped like CPython's where a
// matching message exists.
type SyntaxError struct {
	File string
	Pos  Pos
	Msg  string
}

func (e *SyntaxError) Error() string {
	return fmt.Sprintf("%s:%d:%d: SyntaxError: %s", e.File, e.Pos.Line, e.Pos.Col, e.Msg)
}

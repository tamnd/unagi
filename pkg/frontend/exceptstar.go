package frontend

// except* blocks forbid a jump that would leave them: return is never
// allowed, and break or continue are allowed only when their target loop
// sits inside the block. Probed on 3.14, where all three raise the same
// SyntaxError. The check runs after the parse over the whole module, since a
// nested loop inside the handler makes its own break legal again.

const starJumpMsg = "'break', 'continue' and 'return' cannot appear in an except* block"

// checkExceptStar walks the module rejecting the banned jumps.
func (p *parser) checkExceptStar(body []Stmt) {
	p.walkStar(body, false, 0)
}

// walkStar carries whether the current statements sit inside an except* block
// and, when they do, how many loops have been entered since the block began.
// A break or continue at loop depth zero inside the block leaves it and is
// rejected; a return anywhere inside the block is rejected. A def body opens
// a fresh scope, so the flags reset there.
func (p *parser) walkStar(list []Stmt, inStar bool, depth int) {
	for _, s := range list {
		switch s := s.(type) {
		case *Return:
			if inStar {
				p.errf(s.Span(), starJumpMsg)
			}
		case *Break:
			if inStar && depth == 0 {
				p.errf(s.Span(), starJumpMsg)
			}
		case *Continue:
			if inStar && depth == 0 {
				p.errf(s.Span(), starJumpMsg)
			}
		case *If:
			p.walkStar(s.Body, inStar, depth)
			p.walkStar(s.Else, inStar, depth)
		case *While:
			p.walkStar(s.Body, inStar, depth+1)
			p.walkStar(s.Else, inStar, depth)
		case *For:
			p.walkStar(s.Body, inStar, depth+1)
			p.walkStar(s.Else, inStar, depth)
		case *FuncDef:
			p.walkStar(s.Body, false, 0)
		case *Try:
			p.walkStar(s.Body, inStar, depth)
			for _, h := range s.Handlers {
				if s.IsStar {
					p.walkStar(h.Body, true, 0)
				} else {
					p.walkStar(h.Body, inStar, depth)
				}
			}
			p.walkStar(s.OrElse, inStar, depth)
			p.walkStar(s.Final, inStar, depth)
		}
	}
}

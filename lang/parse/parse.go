// Copyright 2017 The Wuffs Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package parse

// TODO: write a formal grammar for the language.

import (
	"fmt"

	"github.com/google/wuffs/lang/base38"

	a "github.com/google/wuffs/lang/ast"
	t "github.com/google/wuffs/lang/token"
)

type Options struct {
	AllowBuiltIns              bool
	AllowDoubleUnderscoreNames bool
}

func isDoubleUnderscore(s string) bool {
	return len(s) >= 2 && s[0] == '_' && s[1] == '_'
}

func Parse(tm *t.Map, filename string, src []t.Token, opts *Options) (*a.File, error) {
	p := &parser{
		tm:       tm,
		filename: filename,
		src:      src,
	}
	if len(src) > 0 {
		p.lastLine = src[len(src)-1].Line
	}
	if opts != nil {
		p.opts = *opts
	}
	return p.parseFile()
}

func ParseExpr(tm *t.Map, filename string, src []t.Token, opts *Options) (*a.Expr, error) {
	p := &parser{
		tm:       tm,
		filename: filename,
		src:      src,
	}
	if len(src) > 0 {
		p.lastLine = src[len(src)-1].Line
	}
	if opts != nil {
		p.opts = *opts
	}
	return p.parseExpr()
}

type parser struct {
	tm       *t.Map
	filename string
	src      []t.Token
	opts     Options
	lastLine uint32
}

func (p *parser) line() uint32 {
	if len(p.src) != 0 {
		return p.src[0].Line
	}
	return p.lastLine
}

func (p *parser) peek1() t.ID {
	if len(p.src) > 0 {
		return p.src[0].ID
	}
	return 0
}

func (p *parser) parseFile() (*a.File, error) {
	topLevelDecls := []*a.Node(nil)
	for len(p.src) > 0 {
		d, err := p.parseTopLevelDecl()
		if err != nil {
			return nil, err
		}
		topLevelDecls = append(topLevelDecls, d)
	}
	return a.NewFile(p.filename, topLevelDecls), nil
}

func (p *parser) parseTopLevelDecl() (*a.Node, error) {
	flags := a.Flags(0)
	line := p.src[0].Line
	switch k := p.peek1(); k {
	case t.IDPackageID, t.IDUse:
		p.src = p.src[1:]
		path := p.peek1()
		if !path.IsStrLiteral(p.tm) {
			got := p.tm.ByID(path)
			return nil, fmt.Errorf(`parse: expected string literal, got %q at %s:%d`, got, p.filename, p.line())
		}
		p.src = p.src[1:]
		if x := p.peek1(); x != t.IDSemicolon {
			got := p.tm.ByID(x)
			return nil, fmt.Errorf(`parse: expected (implicit) ";", got %q at %s:%d`, got, p.filename, p.line())
		}
		p.src = p.src[1:]
		if k == t.IDPackageID {
			raw := path.Str(p.tm)
			s, ok := t.Unescape(raw)
			if !ok {
				return nil, fmt.Errorf(`parse: %q is not a valid packageid`, raw)
			}
			if u, ok := base38.Encode(s); !ok || u == 0 {
				return nil, fmt.Errorf(`parse: %q is not a valid packageid`, s)
			}
			return a.NewPackageID(p.filename, line, path).AsNode(), nil
		} else {
			return a.NewUse(p.filename, line, path).AsNode(), nil
		}

	case t.IDPub:
		flags |= a.FlagsPublic
		fallthrough
	case t.IDPri:
		p.src = p.src[1:]
		switch p.peek1() {
		case t.IDConst:
			p.src = p.src[1:]
			id, err := p.parseIdent()
			if err != nil {
				return nil, err
			}
			// TODO: check AllowBuiltIns and AllowDoubleUnderscoreNames?

			typ, err := p.parseTypeExpr()
			if err != nil {
				return nil, err
			}
			if p.peek1() != t.IDEq {
				return nil, fmt.Errorf(`parse: const %q has no value at %s:%d`,
					p.tm.ByID(id), p.filename, p.line())
			}
			p.src = p.src[1:]
			value, err := p.parsePossibleDollarExpr()
			if err != nil {
				return nil, err
			}
			if x := p.peek1(); x != t.IDSemicolon {
				got := p.tm.ByID(x)
				return nil, fmt.Errorf(`parse: expected (implicit) ";", got %q at %s:%d`, got, p.filename, p.line())
			}
			p.src = p.src[1:]
			return a.NewConst(flags, p.filename, line, id, typ, value).AsNode(), nil

		case t.IDFunc:
			p.src = p.src[1:]
			id0, id1, err := p.parseQualifiedIdent()
			if err != nil {
				return nil, err
			}
			// TODO: should we require id0 != 0? In other words, always methods
			// (attached to receivers) and never free standing functions?
			if !p.opts.AllowBuiltIns {
				if id0 != 0 && id0.IsBuiltIn() {
					return nil, fmt.Errorf(`parse: built-in %q used for func receiver at %s:%d`,
						p.tm.ByID(id0), p.filename, p.line())
				}
				if id1.IsBuiltIn() {
					return nil, fmt.Errorf(`parse: built-in %q used for func name at %s:%d`,
						p.tm.ByID(id1), p.filename, p.line())
				}
			}
			if !p.opts.AllowDoubleUnderscoreNames && isDoubleUnderscore(p.tm.ByID(id1)) {
				return nil, fmt.Errorf(`parse: double-underscore %q used for func name at %s:%d`,
					p.tm.ByID(id1), p.filename, p.line())
			}

			switch p.peek1() {
			case t.IDExclam:
				flags |= a.FlagsImpure
				p.src = p.src[1:]
			case t.IDQuestion:
				flags |= a.FlagsImpure | a.FlagsSuspendible
				p.src = p.src[1:]
			}
			inFields, err := p.parseList(t.IDCloseParen, (*parser).parseFieldNode)
			if err != nil {
				return nil, err
			}
			outFields, err := p.parseList(t.IDCloseParen, (*parser).parseFieldNode)
			if err != nil {
				return nil, err
			}
			asserts := []*a.Node(nil)
			if p.peek1() == t.IDComma {
				p.src = p.src[1:]
				asserts, err = p.parseList(t.IDOpenCurly, (*parser).parseAssertNode)
				if err != nil {
					return nil, err
				}
				if err := p.assertsSorted(asserts); err != nil {
					return nil, err
				}
			}
			body, err := p.parseBlock()
			if err != nil {
				return nil, err
			}
			if x := p.peek1(); x != t.IDSemicolon {
				got := p.tm.ByID(x)
				return nil, fmt.Errorf(`parse: expected (implicit) ";", got %q at %s:%d`, got, p.filename, p.line())
			}
			p.src = p.src[1:]
			in := a.NewStruct(0, p.filename, line, t.IDIn, inFields)
			out := a.NewStruct(0, p.filename, line, t.IDOut, outFields)
			return a.NewFunc(flags, p.filename, line, id0, id1, in, out, asserts, body).AsNode(), nil

		case t.IDError, t.IDSuspension:
			keyword := p.src[0].ID
			p.src = p.src[1:]

			if x := p.peek1(); x != t.IDOpenParen {
				got := p.tm.ByID(x)
				return nil, fmt.Errorf(`parse: expected "(", got %q at %s:%d`, got, p.filename, p.line())
			}
			p.src = p.src[1:]
			value, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if x := p.peek1(); x != t.IDCloseParen {
				got := p.tm.ByID(x)
				return nil, fmt.Errorf(`parse: expected ")", got %q at %s:%d`, got, p.filename, p.line())
			}
			p.src = p.src[1:]

			message := p.peek1()
			if !message.IsStrLiteral(p.tm) {
				got := p.tm.ByID(message)
				return nil, fmt.Errorf(`parse: expected string literal, got %q at %s:%d`, got, p.filename, p.line())
			}
			p.src = p.src[1:]
			if x := p.peek1(); x != t.IDSemicolon {
				got := p.tm.ByID(x)
				return nil, fmt.Errorf(`parse: expected (implicit) ";", got %q at %s:%d`, got, p.filename, p.line())
			}
			p.src = p.src[1:]
			return a.NewStatus(flags, p.filename, line, keyword, value, message).AsNode(), nil

		case t.IDStruct:
			p.src = p.src[1:]
			name, err := p.parseIdent()
			if err != nil {
				return nil, err
			}
			if !p.opts.AllowBuiltIns && name.IsBuiltIn() {
				return nil, fmt.Errorf(`parse: built-in %q used for struct name at %s:%d`,
					p.tm.ByID(name), p.filename, p.line())
			}
			if !p.opts.AllowDoubleUnderscoreNames && isDoubleUnderscore(p.tm.ByID(name)) {
				return nil, fmt.Errorf(`parse: double-underscore %q used for struct name at %s:%d`,
					p.tm.ByID(name), p.filename, p.line())
			}

			if p.peek1() == t.IDQuestion {
				flags |= a.FlagsSuspendible
				p.src = p.src[1:]
			}
			fields, err := p.parseList(t.IDCloseParen, (*parser).parseFieldNode)
			if err != nil {
				return nil, err
			}
			if x := p.peek1(); x != t.IDSemicolon {
				got := p.tm.ByID(x)
				return nil, fmt.Errorf(`parse: expected (implicit) ";", got %q at %s:%d`, got, p.filename, p.line())
			}
			p.src = p.src[1:]
			return a.NewStruct(flags, p.filename, line, name, fields).AsNode(), nil
		}
	}
	return nil, fmt.Errorf(`parse: unrecognized top level declaration at %s:%d`, p.filename, line)
}

// parseQualifiedIdent parses "foo.bar" or "bar".
func (p *parser) parseQualifiedIdent() (t.ID, t.ID, error) {
	x, err := p.parseIdent()
	if err != nil {
		return 0, 0, err
	}

	if p.peek1() != t.IDDot {
		return 0, x, nil
	}
	p.src = p.src[1:]

	y, err := p.parseIdent()
	if err != nil {
		return 0, 0, err
	}
	return x, y, nil
}

func (p *parser) parseIdent() (t.ID, error) {
	if len(p.src) == 0 {
		return 0, fmt.Errorf(`parse: expected identifier at %s:%d`, p.filename, p.line())
	}
	x := p.src[0]
	if !x.ID.IsIdent(p.tm) {
		got := p.tm.ByID(x.ID)
		return 0, fmt.Errorf(`parse: expected identifier, got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]
	return x.ID, nil
}

func (p *parser) parseList(stop t.ID, parseElem func(*parser) (*a.Node, error)) ([]*a.Node, error) {
	if stop == t.IDCloseParen {
		if x := p.peek1(); x != t.IDOpenParen {
			return nil, fmt.Errorf(`parse: expected "(", got %q at %s:%d`,
				p.tm.ByID(x), p.filename, p.line())
		}
		p.src = p.src[1:]
	}

	ret := []*a.Node(nil)
	for len(p.src) > 0 {
		if p.src[0].ID == stop {
			if stop == t.IDCloseParen {
				p.src = p.src[1:]
			}
			return ret, nil
		}

		elem, err := parseElem(p)
		if err != nil {
			return nil, err
		}
		ret = append(ret, elem)

		switch x := p.peek1(); x {
		case stop:
			if stop == t.IDCloseParen {
				p.src = p.src[1:]
			}
			return ret, nil
		case t.IDComma:
			p.src = p.src[1:]
		default:
			return nil, fmt.Errorf(`parse: expected %q, got %q at %s:%d`,
				p.tm.ByID(stop), p.tm.ByID(x), p.filename, p.line())
		}
	}
	return nil, fmt.Errorf(`parse: expected %q at %s:%d`, p.tm.ByID(stop), p.filename, p.line())
}

func (p *parser) parseFieldNode() (*a.Node, error) {
	name, err := p.parseIdent()
	if err != nil {
		return nil, err
	}
	typ, err := p.parseTypeExpr()
	if err != nil {
		return nil, err
	}
	return a.NewField(name, typ).AsNode(), nil
}

func (p *parser) parseTypeExpr() (*a.TypeExpr, error) {
	if x := p.peek1(); x == t.IDNptr || x == t.IDPtr {
		p.src = p.src[1:]
		rhs, err := p.parseTypeExpr()
		if err != nil {
			return nil, err
		}
		return a.NewTypeExpr(x, 0, 0, nil, nil, rhs), nil
	}

	decorator, arrayLength := t.ID(0), (*a.Expr)(nil)
	switch p.peek1() {
	case t.IDArray:
		decorator = t.IDArray
		p.src = p.src[1:]

		if x := p.peek1(); x != t.IDOpenBracket {
			got := p.tm.ByID(x)
			return nil, fmt.Errorf(`parse: expected "[", got %q at %s:%d`, got, p.filename, p.line())
		}
		p.src = p.src[1:]

		var err error
		arrayLength, err = p.parseExpr()
		if err != nil {
			return nil, err
		}

		if x := p.peek1(); x != t.IDCloseBracket {
			got := p.tm.ByID(x)
			return nil, fmt.Errorf(`parse: expected "]", got %q at %s:%d`, got, p.filename, p.line())
		}
		p.src = p.src[1:]

	case t.IDSlice:
		decorator = t.IDSlice
		p.src = p.src[1:]

	case t.IDTable:
		decorator = t.IDTable
		p.src = p.src[1:]
	}

	if decorator != 0 {
		rhs, err := p.parseTypeExpr()
		if err != nil {
			return nil, err
		}
		return a.NewTypeExpr(decorator, 0, 0, arrayLength.AsNode(), nil, rhs), nil
	}

	pkg, name, err := p.parseQualifiedIdent()
	if err != nil {
		return nil, err
	}

	lhs, mhs := (*a.Expr)(nil), (*a.Expr)(nil)
	if p.peek1() == t.IDOpenBracket {
		_, lhs, mhs, err = p.parseBracket(t.IDDotDot)
		if err != nil {
			return nil, err
		}
	}

	return a.NewTypeExpr(0, pkg, name, lhs.AsNode(), mhs, nil), nil
}

// parseBracket parses "[i:j]", "[i:]", "[:j]" and "[:]". A double dot replaces
// the colon if sep is t.IDDotDot instead of t.IDColon. If sep is t.IDColon, it
// also parses "[x]". The returned op is sep for a range or refinement and
// t.IDOpenBracket for an index.
func (p *parser) parseBracket(sep t.ID) (op t.ID, ei *a.Expr, ej *a.Expr, err error) {
	if x := p.peek1(); x != t.IDOpenBracket {
		got := p.tm.ByID(x)
		return 0, nil, nil, fmt.Errorf(`parse: expected "[", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]

	if p.peek1() != sep {
		ei, err = p.parseExpr()
		if err != nil {
			return 0, nil, nil, err
		}
	}

	switch x := p.peek1(); {
	case x == sep:
		p.src = p.src[1:]

	case x == t.IDCloseBracket && sep == t.IDColon:
		p.src = p.src[1:]
		return t.IDOpenBracket, nil, ei, nil

	default:
		extra := ``
		if sep == t.IDColon {
			extra = ` or "]"`
		}
		got := p.tm.ByID(x)
		return 0, nil, nil, fmt.Errorf(`parse: expected %q%s, got %q at %s:%d`,
			p.tm.ByID(sep), extra, got, p.filename, p.line())
	}

	if p.peek1() != t.IDCloseBracket {
		ej, err = p.parseExpr()
		if err != nil {
			return 0, nil, nil, err
		}
	}

	if x := p.peek1(); x != t.IDCloseBracket {
		got := p.tm.ByID(x)
		return 0, nil, nil, fmt.Errorf(`parse: expected "]", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]

	return sep, ei, ej, nil
}

func (p *parser) parseBlock() ([]*a.Node, error) {
	if x := p.peek1(); x != t.IDOpenCurly {
		got := p.tm.ByID(x)
		return nil, fmt.Errorf(`parse: expected "{", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]

	block := []*a.Node(nil)
	for len(p.src) > 0 {
		if p.src[0].ID == t.IDCloseCurly {
			p.src = p.src[1:]
			return block, nil
		}

		s, err := p.parseStatement()
		if err != nil {
			return nil, err
		}
		block = append(block, s)

		if x := p.peek1(); x != t.IDSemicolon {
			got := p.tm.ByID(x)
			return nil, fmt.Errorf(`parse: expected (implicit) ";", got %q at %s:%d`, got, p.filename, p.line())
		}
		p.src = p.src[1:]
	}
	return nil, fmt.Errorf(`parse: expected "}" at %s:%d`, p.filename, p.line())
}

func (p *parser) assertsSorted(asserts []*a.Node) error {
	seenInv, seenPost := false, false
	for _, a := range asserts {
		switch a.AsAssert().Keyword() {
		case t.IDAssert:
			return fmt.Errorf(`parse: assertion chain cannot contain "assert", `+
				`only "pre", "inv" and "post" at %s:%d`, p.filename, p.line())
		case t.IDPre:
			if seenPost || seenInv {
				break
			}
			continue
		case t.IDInv:
			if seenPost {
				break
			}
			seenInv = true
			continue
		default:
			seenPost = true
			continue
		}
		return fmt.Errorf(`parse: assertion chain not in "pre", "inv", "post" order at %s:%d`,
			p.filename, p.line())
	}
	return nil
}

func (p *parser) parseAssertNode() (*a.Node, error) {
	switch x := p.peek1(); x {
	case t.IDAssert, t.IDPre, t.IDInv, t.IDPost:
		p.src = p.src[1:]
		condition, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		reason, args := t.ID(0), []*a.Node(nil)
		if p.peek1() == t.IDVia {
			p.src = p.src[1:]
			reason = p.peek1()
			if !reason.IsStrLiteral(p.tm) {
				got := p.tm.ByID(reason)
				return nil, fmt.Errorf(`parse: expected string literal, got %q at %s:%d`, got, p.filename, p.line())
			}
			p.src = p.src[1:]
			args, err = p.parseList(t.IDCloseParen, (*parser).parseArgNode)
			if err != nil {
				return nil, err
			}
		}
		return a.NewAssert(x, condition, reason, args).AsNode(), nil
	}
	return nil, fmt.Errorf(`parse: expected "assert", "pre" or "post" at %s:%d`, p.filename, p.line())
}

func (p *parser) parseStatement() (*a.Node, error) {
	line := uint32(0)
	if len(p.src) > 0 {
		line = p.src[0].Line
	}
	n, err := p.parseStatement1()
	if n != nil {
		n.AsRaw().SetFilenameLine(p.filename, line)
		if n.Kind() == a.KIterate {
			for _, o := range n.AsIterate().Variables() {
				o.AsRaw().SetFilenameLine(p.filename, line)
			}
		}
	}
	return n, err
}

func (p *parser) parseLabel() (t.ID, error) {
	if p.peek1() == t.IDColon {
		p.src = p.src[1:]
		return p.parseIdent()
	}
	return 0, nil
}

func (p *parser) parseStatement1() (*a.Node, error) {
	switch x := p.peek1(); x {
	case t.IDAssert, t.IDPre, t.IDPost:
		return p.parseAssertNode()

	case t.IDBreak, t.IDContinue:
		p.src = p.src[1:]
		label, err := p.parseLabel()
		if err != nil {
			return nil, err
		}
		return a.NewJump(x, label).AsNode(), nil

	case t.IDIOBind:
		p.src = p.src[1:]
		in_fields, err := p.parseList(t.IDCloseParen, (*parser).parseIOBindExprNode)
		if err != nil {
			return nil, err
		}
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return a.NewIOBind(in_fields, body).AsNode(), nil

	case t.IDIf:
		o, err := p.parseIf()
		return o.AsNode(), err

	case t.IDIterate:
		return p.parseIterateNode()

	case t.IDReturn, t.IDYield:
		p.src = p.src[1:]
		value, err := (*a.Expr)(nil), error(nil)
		if p.peek1() != t.IDSemicolon {
			value, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		}
		return a.NewRet(x, value).AsNode(), nil

	case t.IDVar:
		p.src = p.src[1:]
		return p.parseVarNode(false)

	case t.IDWhile:
		p.src = p.src[1:]
		label, err := p.parseLabel()
		if err != nil {
			return nil, err
		}
		condition, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		asserts, err := p.parseAsserts()
		if err != nil {
			return nil, err
		}
		body, err := p.parseBlock()
		if err != nil {
			return nil, err
		}
		return a.NewWhile(label, condition, asserts, body).AsNode(), nil
	}

	lhs, err := p.parseExpr()
	if err != nil {
		return nil, err
	}

	if op := p.peek1(); op.IsAssign() {
		p.src = p.src[1:]
		rhs, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		return a.NewAssign(op, lhs, rhs).AsNode(), nil
	}

	return lhs.AsNode(), nil
}

func (p *parser) parseAsserts() ([]*a.Node, error) {
	asserts := []*a.Node(nil)
	if p.peek1() == t.IDComma {
		p.src = p.src[1:]
		var err error
		if asserts, err = p.parseList(t.IDOpenCurly, (*parser).parseAssertNode); err != nil {
			return nil, err
		}
		if err := p.assertsSorted(asserts); err != nil {
			return nil, err
		}
	}
	return asserts, nil
}

func (p *parser) parseIf() (*a.If, error) {
	if x := p.peek1(); x != t.IDIf {
		got := p.tm.ByID(x)
		return nil, fmt.Errorf(`parse: expected "if", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]
	condition, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	bodyIfTrue, err := p.parseBlock()
	if err != nil {
		return nil, err
	}
	elseIf, bodyIfFalse := (*a.If)(nil), ([]*a.Node)(nil)
	if p.peek1() == t.IDElse {
		p.src = p.src[1:]
		if p.peek1() == t.IDIf {
			elseIf, err = p.parseIf()
			if err != nil {
				return nil, err
			}
		} else {
			bodyIfFalse, err = p.parseBlock()
			if err != nil {
				return nil, err
			}
		}
	}
	return a.NewIf(condition, bodyIfTrue, bodyIfFalse, elseIf), nil
}

func (p *parser) parseIterateNode() (*a.Node, error) {
	if x := p.peek1(); x != t.IDIterate {
		got := p.tm.ByID(x)
		return nil, fmt.Errorf(`parse: expected "iterate", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]
	label, err := p.parseLabel()
	if err != nil {
		return nil, err
	}
	vars, err := p.parseList(t.IDCloseParen, (*parser).parseIterateVarNode)
	if err != nil {
		return nil, err
	}
	n, err := p.parseIterateBlock(label, vars)
	if err != nil {
		return nil, err
	}
	return n.AsNode(), nil
}

func (p *parser) parseIterateBlock(label t.ID, vars []*a.Node) (*a.Iterate, error) {
	if x := p.peek1(); x != t.IDOpenParen {
		got := p.tm.ByID(x)
		return nil, fmt.Errorf(`parse: expected "(", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]

	if x := p.peek1(); x != t.IDLength {
		got := p.tm.ByID(x)
		return nil, fmt.Errorf(`parse: expected "length", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]

	if x := p.peek1(); x != t.IDColon {
		got := p.tm.ByID(x)
		return nil, fmt.Errorf(`parse: expected ":", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]

	length := p.peek1()
	if length.SmallPowerOf2Value() == 0 {
		return nil, fmt.Errorf(`parse: expected power-of-2 length count in [1..256], got %q at %s:%d`,
			p.tm.ByID(length), p.filename, p.line())
	}
	p.src = p.src[1:]

	if x := p.peek1(); x != t.IDComma {
		got := p.tm.ByID(x)
		return nil, fmt.Errorf(`parse: expected ",", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]

	if x := p.peek1(); x != t.IDUnroll {
		got := p.tm.ByID(x)
		return nil, fmt.Errorf(`parse: expected "unroll", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]

	if x := p.peek1(); x != t.IDColon {
		got := p.tm.ByID(x)
		return nil, fmt.Errorf(`parse: expected ":", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]

	unroll := p.peek1()
	if unroll.SmallPowerOf2Value() == 0 {
		return nil, fmt.Errorf(`parse: expected power-of-2 unroll count in [1..256], got %q at %s:%d`,
			p.tm.ByID(unroll), p.filename, p.line())
	}
	p.src = p.src[1:]

	if x := p.peek1(); x != t.IDCloseParen {
		got := p.tm.ByID(x)
		return nil, fmt.Errorf(`parse: expected ")", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]

	asserts, err := p.parseAsserts()
	if err != nil {
		return nil, err
	}
	body, err := p.parseBlock()
	if err != nil {
		return nil, err
	}

	elseIterate := (*a.Iterate)(nil)
	if x := p.peek1(); x == t.IDElse {
		p.src = p.src[1:]
		elseIterate, err = p.parseIterateBlock(0, nil)
		if err != nil {
			return nil, err
		}
	}

	return a.NewIterate(label, vars, length, unroll, asserts, body, elseIterate), nil
}

func (p *parser) parseArgNode() (*a.Node, error) {
	name, err := p.parseIdent()
	if err != nil {
		return nil, err
	}
	if x := p.peek1(); x != t.IDColon {
		got := p.tm.ByID(x)
		return nil, fmt.Errorf(`parse: expected ":", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]
	value, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return a.NewArg(name, value).AsNode(), nil
}

func (p *parser) parseIOBindExprNode() (*a.Node, error) {
	e, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	switch e.Operator() {
	case 0:
		return e.AsNode(), nil
	case t.IDDot:
		if lhs := e.LHS().AsExpr(); lhs.Operator() == 0 && lhs.Ident() == t.IDIn {
			return e.AsNode(), nil
		}
	}
	return nil, fmt.Errorf(`parse: expected "in.something", got %q at %s:%d`, e.Str(p.tm), p.filename, p.line())
}

func (p *parser) parseIterateVarNode() (*a.Node, error) {
	return p.parseVarNode(true)
}

func (p *parser) parseVarNode(inIterate bool) (*a.Node, error) {
	id, err := p.parseIdent()
	if err != nil {
		return nil, err
	}
	typ, err := p.parseTypeExpr()
	if err != nil {
		return nil, err
	}
	value := (*a.Expr)(nil)

	op := t.ID(0)
	if inIterate {
		op = t.IDEqColon
		if x := p.peek1(); x != t.IDEqColon {
			got := p.tm.ByID(x)
			return nil, fmt.Errorf(`parse: expected "=:", got %q at %s:%d`, got, p.filename, p.line())
		}
		p.src = p.src[1:]
		value, err = p.parseExpr()
		if err != nil {
			return nil, err
		}

	} else if p.peek1() == t.IDEq {
		op = t.IDEq
		p.src = p.src[1:]
		if p.peek1() == t.IDTry {
			value, err = p.parseTryExpr()
			if err != nil {
				return nil, err
			}
		} else {
			value, err = p.parseExpr()
			if err != nil {
				return nil, err
			}
		}
	}

	return a.NewVar(op, id, typ, value).AsNode(), nil
}

func (p *parser) parsePossibleDollarExprNode() (*a.Node, error) {
	n, err := p.parsePossibleDollarExpr()
	if err != nil {
		return nil, err
	}
	return n.AsNode(), err
}

func (p *parser) parsePossibleDollarExpr() (*a.Expr, error) {
	if x := p.peek1(); x != t.IDDollar {
		return p.parseExpr()
	}
	p.src = p.src[1:]
	args, err := p.parseList(t.IDCloseParen, (*parser).parsePossibleDollarExprNode)
	if err != nil {
		return nil, err
	}
	return a.NewExpr(0, t.IDDollar, 0, 0, nil, nil, nil, args), nil
}

func (p *parser) parseTryExpr() (*a.Expr, error) {
	if x := p.peek1(); x != t.IDTry {
		got := p.tm.ByID(x)
		return nil, fmt.Errorf(`parse: expected "try", got %q at %s:%d`, got, p.filename, p.line())
	}
	p.src = p.src[1:]
	call, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	if call.Operator() != t.IDOpenParen {
		return nil, fmt.Errorf(`parse: expected function call after "try", got %q at %s:%d`,
			call.Str(p.tm), p.filename, p.line())
	}
	return a.NewExpr(call.AsNode().AsRaw().Flags(), t.IDTry, 0, call.Ident(),
		call.LHS(), call.MHS(), call.RHS(), call.Args()), nil
}

func (p *parser) parseExprNode() (*a.Node, error) {
	n, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	return n.AsNode(), err
}

func (p *parser) parseExpr() (*a.Expr, error) {
	lhs, err := p.parseOperand()
	if err != nil {
		return nil, err
	}
	if x := p.peek1(); x.IsBinaryOp() {
		p.src = p.src[1:]
		rhs := (*a.Node)(nil)
		if x == t.IDAs {
			o, err := p.parseTypeExpr()
			if err != nil {
				return nil, err
			}
			rhs = o.AsNode()
		} else {
			o, err := p.parseOperand()
			if err != nil {
				return nil, err
			}
			rhs = o.AsNode()
		}

		if !x.IsAssociativeOp() || x != p.peek1() {
			op := x.BinaryForm()
			if op == 0 {
				return nil, fmt.Errorf(`parse: internal error: no binary form for token 0x%02X`, x)
			}
			return a.NewExpr(0, op, 0, 0, lhs.AsNode(), nil, rhs, nil), nil
		}

		args := []*a.Node{lhs.AsNode(), rhs}
		for p.peek1() == x {
			p.src = p.src[1:]
			arg, err := p.parseOperand()
			if err != nil {
				return nil, err
			}
			args = append(args, arg.AsNode())
		}
		op := x.AssociativeForm()
		if op == 0 {
			return nil, fmt.Errorf(`parse: internal error: no associative form for token 0x%02X`, x)
		}
		return a.NewExpr(0, op, 0, 0, nil, nil, nil, args), nil
	}
	return lhs, nil
}

func (p *parser) parseOperand() (*a.Expr, error) {
	switch x := p.peek1(); {
	case x.IsUnaryOp():
		p.src = p.src[1:]
		rhs, err := p.parseOperand()
		if err != nil {
			return nil, err
		}
		op := x.UnaryForm()
		if op == 0 {
			return nil, fmt.Errorf(`parse: internal error: no unary form for token 0x%02X`, x)
		}
		return a.NewExpr(0, op, 0, 0, nil, nil, rhs.AsNode(), nil), nil

	case x.IsLiteral(p.tm):
		p.src = p.src[1:]
		return a.NewExpr(0, 0, 0, x, nil, nil, nil, nil), nil

	default:
		switch x {
		case t.IDOpenParen:
			p.src = p.src[1:]
			expr, err := p.parseExpr()
			if err != nil {
				return nil, err
			}
			if x := p.peek1(); x != t.IDCloseParen {
				got := p.tm.ByID(x)
				return nil, fmt.Errorf(`parse: expected ")", got %q at %s:%d`, got, p.filename, p.line())
			}
			p.src = p.src[1:]
			return expr, nil

		case t.IDError, t.IDStatus, t.IDSuspension:
			keyword := x
			p.src = p.src[1:]
			message := p.peek1()
			// TODO: parse the "pkg" in `error pkg."foo"`.
			statusPkg := t.ID(0)
			if !message.IsStrLiteral(p.tm) {
				got := p.tm.ByID(message)
				return nil, fmt.Errorf(`parse: expected string literal, got %q at %s:%d`, got, p.filename, p.line())
			}
			p.src = p.src[1:]
			return a.NewExpr(0, keyword, statusPkg, message, nil, nil, nil, nil), nil
		}
	}

	id, err := p.parseIdent()
	if err != nil {
		return nil, err
	}
	lhs := a.NewExpr(0, 0, 0, id, nil, nil, nil, nil)

	for {
		flags := a.Flags(0)
		switch p.peek1() {
		default:
			return lhs, nil

		case t.IDExclam, t.IDQuestion:
			flags |= a.FlagsImpure | a.FlagsCallImpure
			if p.src[0].ID == t.IDQuestion {
				flags |= a.FlagsSuspendible | a.FlagsCallSuspendible
			}
			p.src = p.src[1:]
			fallthrough

		case t.IDOpenParen:
			args, err := p.parseList(t.IDCloseParen, (*parser).parseArgNode)
			if err != nil {
				return nil, err
			}
			lhs = a.NewExpr(flags, t.IDOpenParen, 0, 0, lhs.AsNode(), nil, nil, args)

		case t.IDOpenBracket:
			id0, mhs, rhs, err := p.parseBracket(t.IDColon)
			if err != nil {
				return nil, err
			}
			lhs = a.NewExpr(0, id0, 0, 0, lhs.AsNode(), mhs.AsNode(), rhs.AsNode(), nil)

		case t.IDDot:
			p.src = p.src[1:]
			selector, err := p.parseIdent()
			if err != nil {
				return nil, err
			}
			lhs = a.NewExpr(0, t.IDDot, 0, selector, lhs.AsNode(), nil, nil, nil)
		}
	}
}

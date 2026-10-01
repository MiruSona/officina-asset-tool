package unityyaml

import (
	"bytes"
	"strings"
)

// Parse 는 파일 한 장을 문서들로 가른다. 들여쓰기가 어긋나는 등 꼴이 무너지면 오류,
// 값 하나만 모르는 꼴이면 그 자리에 KindBad 노드를 두고 계속 읽는다.
func Parse(data []byte) ([]Document, error) {
	data = bytes.TrimPrefix(data, []byte("\xEF\xBB\xBF"))
	raw := strings.Split(string(data), "\n")

	var docs []Document
	var cur *Document
	var lines []string
	var nums []int
	sawHeader := false

	finish := func() error {
		if cur == nil {
			return nil
		}
		root, err := parseBody(lines, nums)
		if err != nil {
			return err
		}
		cur.Root = root
		docs = append(docs, *cur)
		return nil
	}

	for i, l := range raw {
		l = strings.TrimSuffix(l, "\r")
		num := i + 1
		if l == "---" || strings.HasPrefix(l, "--- ") {
			if err := finish(); err != nil {
				return nil, err
			}
			d, err := parseHeader(l, num)
			if err != nil {
				return nil, err
			}
			cur = &d
			lines, nums = nil, nil
			sawHeader = true
			continue
		}
		if !sawHeader && strings.HasPrefix(l, "%") {
			continue
		}
		if l == "..." {
			continue
		}
		if cur == nil {
			cur = &Document{Line: num}
		}
		lines = append(lines, l)
		nums = append(nums, num)
	}
	if err := finish(); err != nil {
		return nil, err
	}
	return docs, nil
}

func parseHeader(l string, num int) (Document, error) {
	d := Document{Line: num}
	for _, tok := range strings.Fields(strings.TrimPrefix(l, "---")) {
		switch {
		case strings.HasPrefix(tok, "!u!"):
			d.ClassID = strings.TrimPrefix(tok, "!u!")
		case strings.HasPrefix(tok, "&"):
			d.FileID = strings.TrimPrefix(tok, "&")
		case tok == "stripped":
		default:
			return d, &Error{Line: num, Msg: "문서 머리를 모르는 꼴이다: " + l}
		}
	}
	return d, nil
}

type parser struct {
	lines []string
	nums  []int
	pos   int
}

func parseBody(lines []string, nums []int) (*Node, error) {
	p := &parser{lines: lines, nums: nums}
	for i, l := range lines {
		ws := l[:len(l)-len(strings.TrimLeft(l, " \t"))]
		if strings.Contains(ws, "\t") {
			return nil, &Error{Line: nums[i], Msg: "들여쓰기에 탭이 있다"}
		}
	}
	p.skipBlank()
	if p.eof() {
		return &Node{Kind: KindScalar, Null: true}, nil
	}
	root, err := p.parseNode(p.indent(p.pos))
	if err != nil {
		return nil, err
	}
	p.skipBlank()
	if !p.eof() {
		return nil, p.errAt(p.pos, "들여쓰기가 어긋났다")
	}
	return root, nil
}

func (p *parser) eof() bool { return p.pos >= len(p.lines) }

func (p *parser) indent(i int) int {
	l := p.lines[i]
	return len(l) - len(strings.TrimLeft(l, " "))
}

func (p *parser) content(i int) string {
	return strings.TrimRight(strings.TrimLeft(p.lines[i], " "), " ")
}

func (p *parser) blank(i int) bool {
	c := p.content(i)
	return c == "" || strings.HasPrefix(c, "#")
}

func (p *parser) skipBlank() {
	for !p.eof() && p.blank(p.pos) {
		p.pos++
	}
}

func (p *parser) nextNonBlank(from int) int {
	for i := from; i < len(p.lines); i++ {
		if !p.blank(i) {
			return i
		}
	}
	return -1
}

func (p *parser) errAt(i int, msg string) error {
	return &Error{Line: p.nums[i], Msg: msg}
}

func isSeqItem(text string) bool {
	return text == "-" || strings.HasPrefix(text, "- ")
}

// parseNode 는 지금 줄에서 시작하는 맵이나 목록 하나를 읽는다.
func (p *parser) parseNode(ind int) (*Node, error) {
	if isSeqItem(p.content(p.pos)) {
		return p.parseSeq(ind)
	}
	return p.parseMap(ind)
}

func (p *parser) parseMap(ind int) (*Node, error) {
	n := &Node{Kind: KindMap, Line: p.nums[p.pos]}
	for {
		p.skipBlank()
		if p.eof() {
			return n, nil
		}
		in := p.indent(p.pos)
		if in < ind {
			return n, nil
		}
		if in > ind {
			return nil, p.errAt(p.pos, "들여쓰기가 어긋났다")
		}
		text := p.content(p.pos)
		if isSeqItem(text) {
			return nil, p.errAt(p.pos, "맵 안에 목록 칸이 홀로 있다")
		}
		key, rest, ok := splitKey(text)
		if !ok {
			return nil, p.errAt(p.pos, "`키: 값` 꼴이 아니다")
		}
		num := p.nums[p.pos]
		p.pos++
		val, err := p.parseValue(rest, ind, num, true)
		if err != nil {
			return nil, err
		}
		n.Keys = append(n.Keys, key)
		n.Vals = append(n.Vals, val)
	}
}

func (p *parser) parseSeq(ind int) (*Node, error) {
	n := &Node{Kind: KindSeq, Line: p.nums[p.pos]}
	for {
		p.skipBlank()
		if p.eof() {
			return n, nil
		}
		in := p.indent(p.pos)
		if in < ind {
			return n, nil
		}
		if in > ind {
			return nil, p.errAt(p.pos, "들여쓰기가 어긋났다")
		}
		text := p.content(p.pos)
		if !isSeqItem(text) {
			return n, nil
		}
		num := p.nums[p.pos]
		after := text[1:]
		item := strings.TrimLeft(after, " ")
		col := ind + 1 + len(after) - len(item)

		var val *Node
		var err error
		switch {
		case item == "" || strings.HasPrefix(item, "#"):
			p.pos++
			val, err = p.parseValue("", ind, num, false)
		case isSeqItem(item):
			p.pos++
			val = p.bad(ind, num, "한 줄에 겹친 목록")
		case isMapStart(item):
			// `- 키: 값` 은 그 글자 자리를 들여쓰기로 한 맵이다. 줄을 고쳐 쓰고 맵으로 읽는다.
			p.lines[p.pos] = strings.Repeat(" ", col) + item
			val, err = p.parseMap(col)
		default:
			p.pos++
			val, err = p.parseInline(item, ind, num)
		}
		if err != nil {
			return nil, err
		}
		n.Items = append(n.Items, val)
	}
}

func isMapStart(item string) bool {
	switch item[0] {
	case '{', '[':
		return false
	}
	_, _, ok := splitKey(item)
	return ok
}

// splitKey 는 `키: 값` 을 가른다. 키는 따옴표 없음이나 한 줄 따옴표만 받는다.
func splitKey(text string) (string, string, bool) {
	if text[0] == '\'' || text[0] == '"' {
		key, end, err := scanQuoted(text[1:], text[0])
		if err != nil {
			return "", "", false
		}
		after := text[1+end:]
		if after == ":" {
			return key, "", true
		}
		if strings.HasPrefix(after, ": ") {
			return key, after[2:], true
		}
		return "", "", false
	}
	if i := strings.Index(text, ": "); i >= 0 {
		return strings.TrimRight(text[:i], " "), text[i+2:], true
	}
	if strings.HasSuffix(text, ":") {
		return strings.TrimRight(text[:len(text)-1], " "), "", true
	}
	return "", "", false
}

// parseValue 는 `키:` 뒤 값을 읽는다. 빈 값이면 다음 줄 들여쓰기로 맵·목록·빈 값을 가른다.
// sameIndentSeq 는 Unity 가 키와 같은 들여쓰기로 목록을 적는 꼴(`키:` 다음 줄 `- …`)을 받을지다.
func (p *parser) parseValue(rest string, ind, num int, sameIndentSeq bool) (*Node, error) {
	rest = strings.TrimLeft(rest, " ")
	if rest != "" && !strings.HasPrefix(rest, "#") {
		return p.parseInline(rest, ind, num)
	}
	j := p.nextNonBlank(p.pos)
	if j >= 0 {
		in := p.indent(j)
		if in > ind {
			p.pos = j
			return p.parseNode(in)
		}
		if in == ind && sameIndentSeq && isSeqItem(p.content(j)) {
			p.pos = j
			return p.parseSeq(ind)
		}
	}
	return &Node{Kind: KindScalar, Null: true, Line: num}, nil
}

// parseInline 은 같은 줄에 시작하는 값을 읽는다. 이어지는 줄은 ind 보다 깊게 들여 쓴 줄이다.
func (p *parser) parseInline(rest string, ind, num int) (*Node, error) {
	switch rest[0] {
	case '\'', '"':
		return p.parseQuoted(rest, ind, num), nil
	case '{', '[':
		return p.parseFlowValue(rest, ind, num), nil
	case '|', '>', '&', '*', '!', '@', '`', '%', '?':
		return p.bad(ind, num, "블록 글·앵커·별칭·태그"), nil
	}
	if isSeqItem(rest) {
		return p.bad(ind, num, "값 자리의 목록"), nil
	}
	return p.parsePlain(rest, ind, num), nil
}

// continuation 은 지금 줄부터 ind 보다 깊은 줄(사이 빈 줄 포함)이 어디까지인지 준다.
func (p *parser) continuation(ind int) int {
	last := p.pos
	for j := p.pos; j < len(p.lines); j++ {
		if p.content(j) == "" {
			continue
		}
		if p.indent(j) <= ind {
			break
		}
		last = j + 1
	}
	return last
}

func (p *parser) bad(ind, num int, why string) *Node {
	p.pos = p.continuation(ind)
	return &Node{Kind: KindBad, Line: num, Value: why}
}

// parsePlain 은 따옴표 없는 값을 읽는다. 접힌 줄은 빈칸 하나로, 빈 줄은 줄바꿈으로 잇는다.
// 값 안의 ` #` 는 자르지 않고 그 칸을 못 읽음으로 둔다 — `Item #1` 이 조용히 `Item` 이 되지 않게.
func (p *parser) parsePlain(rest string, ind, num int) *Node {
	if hasComment(rest) {
		return p.bad(ind, num, "따옴표 없는 값 안의 ` #`")
	}
	var b strings.Builder
	b.WriteString(strings.TrimRight(rest, " "))
	end := p.continuation(ind)
	breaks := 0
	for j := p.pos; j < end; j++ {
		c := p.content(j)
		if c == "" {
			breaks++
			continue
		}
		if hasComment(" " + c) {
			return p.bad(ind, num, "따옴표 없는 값 안의 ` #`")
		}
		if breaks == 0 {
			b.WriteByte(' ')
		}
		b.WriteString(strings.Repeat("\n", breaks))
		breaks = 0
		b.WriteString(c)
	}
	p.pos = end
	return &Node{Kind: KindScalar, Line: num, Value: b.String()}
}

func hasComment(s string) bool {
	return strings.Contains(s, " #")
}

// parseQuoted 는 따옴표 값을 읽는다. 닫는 따옴표가 나올 때까지 이어지는 줄을 붙인다.
func (p *parser) parseQuoted(rest string, ind, num int) *Node {
	end := p.continuation(ind)
	parts := []string{rest[1:]}
	for j := p.pos; j < end; j++ {
		parts = append(parts, p.lines[j])
	}
	joined := strings.Join(parts, "\n")
	val, stop, err := scanQuoted(joined, rest[0])
	if err != nil {
		p.pos = end
		return &Node{Kind: KindBad, Line: num, Value: err.Error()}
	}
	used := strings.Count(joined[:stop], "\n")
	tail := joined[stop:]
	if i := strings.IndexByte(tail, '\n'); i >= 0 {
		tail = tail[:i]
	}
	p.pos += used
	if t := strings.TrimLeft(tail, " "); t != "" && !strings.HasPrefix(t, "#") {
		p.pos = p.continuation(ind)
		return &Node{Kind: KindBad, Line: num, Value: "따옴표 뒤에 글자가 더 있다"}
	}
	return &Node{Kind: KindScalar, Line: num, Value: val}
}

// parseFlowValue 는 `{…}`·`[…]` 를 읽는다. 한 줄에 안 닫히면 이어지는 줄을 빈칸으로 붙여 본다.
func (p *parser) parseFlowValue(rest string, ind, num int) *Node {
	end := p.continuation(ind)
	text := rest
	for {
		f := &flowParser{s: text, line: num}
		n, err := f.value()
		if err == nil {
			f.skipSpace()
			if t := f.s[f.i:]; t != "" && !strings.HasPrefix(t, "#") {
				p.pos = end
				return &Node{Kind: KindBad, Line: num, Value: "흐름 값 뒤에 글자가 더 있다"}
			}
			return n
		}
		if err != errUnclosed || p.pos >= end {
			p.pos = end
			return &Node{Kind: KindBad, Line: num, Value: err.Error()}
		}
		text += " " + p.content(p.pos)
		p.pos++
	}
}

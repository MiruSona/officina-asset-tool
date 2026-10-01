// Package unityyaml 은 Unity 가 쓰는 YAML 을 줄 단위로 읽는다. Addressables 는 모른다.
// 읽는 꼴과 그 밖의 꼴을 다루는 법은 Docs/Design/2026-09-23-index설계.md 4-2 를 본다.
package unityyaml

import (
	"fmt"
	"strconv"
)

// Kind 는 노드 종류다. KindBad 는 모르는 꼴이라 읽으려 할 때만 오류가 난다.
type Kind int

const (
	KindScalar Kind = iota
	KindMap
	KindSeq
	KindBad
)

// Node 는 YAML 값 하나다. 맵은 Keys·Vals 를 같은 차례로 든다.
type Node struct {
	Kind  Kind
	Line  int
	Value string // 글자 값. KindBad 면 못 읽은 까닭
	Null  bool   // `키:` 뒤가 빈 값
	Keys  []string
	Vals  []*Node
	Items []*Node
}

// Document 는 `--- !u!114 &11400000` 로 가른 문서 하나다. .meta 처럼 머리가 없으면 ClassID 가 빈 값이다.
type Document struct {
	ClassID string
	FileID  string
	Line    int
	Root    *Node
}

// Error 는 줄 번호가 붙은 읽기 오류다.
type Error struct {
	Line int
	Msg  string
}

func (e *Error) Error() string {
	return fmt.Sprintf("줄 %d: %s", e.Line, e.Msg)
}

func (n *Node) fail(msg string) error {
	return &Error{Line: n.Line, Msg: msg}
}

func (n *Node) badErr() error {
	return n.fail("읽을 줄 모르는 꼴이다 (" + n.Value + ")")
}

// Lookup 은 맵을 따라 내려간다. 칸이 없으면 (nil, nil), 맵이 아닌 것을 지나야 하면 오류다.
func (n *Node) Lookup(path ...string) (*Node, error) {
	cur := n
	for _, key := range path {
		if cur == nil {
			return nil, nil
		}
		switch {
		case cur.Kind == KindBad:
			return nil, cur.badErr()
		case cur.Kind == KindScalar && cur.Null:
			return nil, nil
		case cur.Kind != KindMap:
			return nil, cur.fail(fmt.Sprintf("%q 칸을 찾는데 맵이 아니다", key))
		}
		var next *Node
		for i, k := range cur.Keys {
			if k == key {
				next = cur.Vals[i]
				break
			}
		}
		if next == nil {
			return nil, nil
		}
		cur = next
	}
	return cur, nil
}

// String 은 글자 값을 준다. 빈 값(`키:`)은 "" 이다.
func (n *Node) String() (string, error) {
	switch n.Kind {
	case KindScalar:
		return n.Value, nil
	case KindBad:
		return "", n.badErr()
	}
	return "", n.fail("글자 값이 와야 하는데 맵이나 목록이다")
}

// List 는 목록 칸을 준다. 빈 값은 빈 목록으로 본다.
func (n *Node) List() ([]*Node, error) {
	switch {
	case n.Kind == KindSeq:
		return n.Items, nil
	case n.Kind == KindScalar && n.Null:
		return nil, nil
	case n.Kind == KindBad:
		return nil, n.badErr()
	}
	return nil, n.fail("목록이 와야 하는데 아니다")
}

// Float 는 숫자 칸을 준다.
func (n *Node) Float() (float64, error) {
	s, err := n.String()
	if err != nil {
		return 0, err
	}
	f, perr := strconv.ParseFloat(s, 64)
	if perr != nil {
		return 0, n.fail(fmt.Sprintf("숫자가 와야 하는데 %q 이다", s))
	}
	return f, nil
}

// Int 는 정수 칸을 준다.
func (n *Node) Int() (int, error) {
	s, err := n.String()
	if err != nil {
		return 0, err
	}
	v, perr := strconv.Atoi(s)
	if perr != nil {
		return 0, n.fail(fmt.Sprintf("정수가 와야 하는데 %q 이다", s))
	}
	return v, nil
}

// Flag 는 Unity 가 0·1 로 쓰는 깃발 칸을 준다.
func (n *Node) Flag() (bool, error) {
	s, err := n.String()
	if err != nil {
		return false, err
	}
	switch s {
	case "0":
		return false, nil
	case "1":
		return true, nil
	}
	return false, n.fail(fmt.Sprintf("0 이나 1 이 와야 하는데 %q 이다", s))
}

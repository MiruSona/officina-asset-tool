package unityyaml

import (
	"errors"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"
)

var errUnclosed = errors.New("닫히지 않았다")

// scanQuoted 는 여는 따옴표 다음부터 닫는 따옴표까지 읽어 값과 닫는 따옴표 다음 자리를 준다.
// s 안의 줄바꿈은 YAML 접기 규칙대로 빈칸 하나(빈 줄이면 줄바꿈)로 바꾼다.
func scanQuoted(s string, q byte) (string, int, error) {
	var buf []byte
	kept := 0 // 이스케이프로 넣은 글자는 줄 끝 빈칸 지우기에서 지키지 않는다
	i := 0
	for i < len(s) {
		c := s[i]
		if c == q {
			if q == '\'' && i+1 < len(s) && s[i+1] == '\'' {
				buf = append(buf, '\'')
				kept = len(buf)
				i += 2
				continue
			}
			return string(buf), i + 1, nil
		}
		if c == '\n' {
			for len(buf) > kept && (buf[len(buf)-1] == ' ' || buf[len(buf)-1] == '\t') {
				buf = buf[:len(buf)-1]
			}
			i++
			breaks := 0
			for {
				for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
					i++
				}
				if i < len(s) && s[i] == '\n' {
					breaks++
					i++
					continue
				}
				break
			}
			if breaks == 0 {
				buf = append(buf, ' ')
			}
			for k := 0; k < breaks; k++ {
				buf = append(buf, '\n')
			}
			continue
		}
		if q == '"' && c == '\\' {
			if i+1 >= len(s) {
				return "", 0, errUnclosed
			}
			if s[i+1] == '\n' {
				i += 2
				for i < len(s) && (s[i] == ' ' || s[i] == '\t') {
					i++
				}
				continue
			}
			r, n, err := decodeEscape(s[i+1:])
			if err != nil {
				return "", 0, err
			}
			buf = utf8.AppendRune(buf, r)
			kept = len(buf)
			i += 1 + n
			continue
		}
		buf = append(buf, c)
		i++
	}
	return "", 0, errUnclosed
}

var simpleEscapes = map[byte]rune{
	'0': 0, 'a': '\a', 'b': '\b', 't': '\t', '\t': '\t', 'n': '\n', 'v': '\v', 'f': '\f',
	'r': '\r', 'e': 0x1b, ' ': ' ', '"': '"', '/': '/', '\\': '\\', 'N': 0x85, '_': 0xa0,
	'L': 0x2028, 'P': 0x2029,
}

// decodeEscape 는 `\` 다음 글자부터 읽어 글자 하나와 읽은 바이트 수를 준다.
// `😀` 처럼 서로게이트 쌍이면 둘을 한 글자로 합친다.
func decodeEscape(s string) (rune, int, error) {
	if r, ok := simpleEscapes[s[0]]; ok {
		return r, 1, nil
	}
	width := map[byte]int{'x': 2, 'u': 4, 'U': 8}[s[0]]
	if width == 0 {
		return 0, 0, errors.New("모르는 이스케이프 \\" + string(s[0]))
	}
	r, err := hexRune(s[1:], width)
	if err != nil {
		return 0, 0, err
	}
	used := 1 + width
	if !utf16.IsSurrogate(r) {
		return r, used, nil
	}
	if r >= 0xDC00 || !strings.HasPrefix(s[used:], "\\u") {
		return 0, 0, errors.New("짝 없는 서로게이트")
	}
	low, err := hexRune(s[used+2:], 4)
	if err != nil {
		return 0, 0, err
	}
	joined := utf16.DecodeRune(r, low)
	if joined == utf8.RuneError {
		return 0, 0, errors.New("짝 없는 서로게이트")
	}
	return joined, used + 6, nil
}

func hexRune(s string, width int) (rune, error) {
	if len(s) < width {
		return 0, errors.New("16진 이스케이프가 짧다")
	}
	v, err := strconv.ParseUint(s[:width], 16, 32)
	if err != nil {
		return 0, errors.New("16진 이스케이프가 틀렸다")
	}
	return rune(v), nil
}

// flowParser 는 `{a: b, c: [1, 2]}` 같은 흐름 꼴을 읽는다. 줄바꿈은 이미 빈칸으로 바뀌어 온다.
type flowParser struct {
	s    string
	i    int
	line int
}

func (f *flowParser) skipSpace() {
	for f.i < len(f.s) && f.s[f.i] == ' ' {
		f.i++
	}
}

func (f *flowParser) value() (*Node, error) {
	f.skipSpace()
	if f.i >= len(f.s) {
		return nil, errUnclosed
	}
	switch f.s[f.i] {
	case '{':
		return f.mapping()
	case '[':
		return f.sequence()
	case '\'', '"':
		v, err := f.quoted()
		if err != nil {
			return nil, err
		}
		return &Node{Kind: KindScalar, Line: f.line, Value: v}, nil
	}
	start := f.i
	for f.i < len(f.s) && !strings.ContainsRune(",}]", rune(f.s[f.i])) {
		f.i++
	}
	v := strings.TrimRight(f.s[start:f.i], " ")
	return &Node{Kind: KindScalar, Line: f.line, Value: v, Null: v == ""}, nil
}

func (f *flowParser) quoted() (string, error) {
	q := f.s[f.i]
	v, end, err := scanQuoted(f.s[f.i+1:], q)
	if err != nil {
		return "", err
	}
	f.i += 1 + end
	return v, nil
}

func (f *flowParser) mapping() (*Node, error) {
	n := &Node{Kind: KindMap, Line: f.line}
	f.i++
	for {
		f.skipSpace()
		if f.i >= len(f.s) {
			return nil, errUnclosed
		}
		if f.s[f.i] == '}' {
			f.i++
			return n, nil
		}
		var key string
		if f.s[f.i] == '\'' || f.s[f.i] == '"' {
			k, err := f.quoted()
			if err != nil {
				return nil, err
			}
			key = k
			f.skipSpace()
		} else {
			start := f.i
			for f.i < len(f.s) && f.s[f.i] != ':' && f.s[f.i] != ',' && f.s[f.i] != '}' {
				f.i++
			}
			key = strings.TrimRight(f.s[start:f.i], " ")
		}
		if f.i >= len(f.s) {
			return nil, errUnclosed
		}
		if f.s[f.i] != ':' {
			return nil, errors.New("흐름 맵에 `키: 값` 이 아닌 칸이 있다")
		}
		f.i++
		val, err := f.value()
		if err != nil {
			return nil, err
		}
		n.Keys = append(n.Keys, key)
		n.Vals = append(n.Vals, val)
		if err := f.afterItem('}'); err != nil {
			return nil, err
		}
	}
}

func (f *flowParser) sequence() (*Node, error) {
	n := &Node{Kind: KindSeq, Line: f.line}
	f.i++
	for {
		f.skipSpace()
		if f.i >= len(f.s) {
			return nil, errUnclosed
		}
		if f.s[f.i] == ']' {
			f.i++
			return n, nil
		}
		val, err := f.value()
		if err != nil {
			return nil, err
		}
		n.Items = append(n.Items, val)
		if err := f.afterItem(']'); err != nil {
			return nil, err
		}
	}
}

// afterItem 은 칸 하나 뒤의 `,` 를 먹는다. 닫는 괄호면 남겨 두어 다음 바퀴가 닫는다.
func (f *flowParser) afterItem(closer byte) error {
	f.skipSpace()
	if f.i >= len(f.s) {
		return errUnclosed
	}
	switch f.s[f.i] {
	case ',':
		f.i++
		return nil
	case closer:
		return nil
	}
	return errors.New("흐름 값 사이에 `,` 가 없다")
}

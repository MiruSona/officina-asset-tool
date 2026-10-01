package unityyaml

import (
	"strings"
	"testing"
)

// I1 : 줄 읽기 (설계 4-6). 실물 Unity 파일이 없어 Unity 꼴을 손으로 흉내 냈다.

func mustParse(t *testing.T, src string) []Document {
	t.Helper()
	docs, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("Parse 실패: %v", err)
	}
	return docs
}

func mustString(t *testing.T, n *Node, path ...string) string {
	t.Helper()
	got, err := n.Lookup(path...)
	if err != nil {
		t.Fatalf("Lookup(%v): %v", path, err)
	}
	if got == nil {
		t.Fatalf("Lookup(%v): 칸이 없다", path)
	}
	s, err := got.String()
	if err != nil {
		t.Fatalf("String(%v): %v", path, err)
	}
	return s
}

const groupYAML = `%YAML 1.1
%TAG !u! tag:unity3d.com,2011:
--- !u!114 &11400000
MonoBehaviour:
  m_ObjectHideFlags: 0
  m_Script: {fileID: 11500000, guid: bbb281ee3bf0b054c82ac2347e9e782c, type: 3}
  m_Name: Default Local Group
  m_EditorClassIdentifier:
  m_GroupName: Default Local Group
  m_SerializeEntries:
  - m_GUID: 0f1e2d3c4b5a69788796a5b4c3d2e1f0
    m_Address: Hero
    m_ReadOnly: 0
    m_SerializedLabels:
    - default
    - ui
    FlaggedDuringContentUpdateRestriction: 0
  - m_GUID: 1234567890abcdef1234567890abcdef
    m_Address: "\uC544\uC774\uCF58/\uCE7C"
    m_SerializedLabels: []
  m_SchemaSet:
    m_Schemas:
    - {fileID: 11400000, guid: aaaa0000aaaa0000aaaa0000aaaa0000, type: 2}
    - {fileID: 0}
`

func TestDocumentHeaderAndBlockList(t *testing.T) {
	docs := mustParse(t, groupYAML)
	if len(docs) != 1 {
		t.Fatalf("문서 수 = %d, 1 이어야 한다", len(docs))
	}
	d := docs[0]
	if d.ClassID != "114" || d.FileID != "11400000" {
		t.Fatalf("머리 = %q &%q", d.ClassID, d.FileID)
	}
	if got := mustString(t, d.Root, "MonoBehaviour", "m_GroupName"); got != "Default Local Group" {
		t.Fatalf("m_GroupName = %q", got)
	}
	if got := mustString(t, d.Root, "MonoBehaviour", "m_EditorClassIdentifier"); got != "" {
		t.Fatalf("빈 값 = %q", got)
	}
	entries, err := d.Root.Lookup("MonoBehaviour", "m_SerializeEntries")
	if err != nil {
		t.Fatal(err)
	}
	items, err := entries.List()
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("항목 수 = %d", len(items))
	}
	if got := mustString(t, items[0], "m_Address"); got != "Hero" {
		t.Fatalf("address = %q", got)
	}
	labels, _ := items[0].Lookup("m_SerializedLabels")
	ls, err := labels.List()
	if err != nil || len(ls) != 2 {
		t.Fatalf("라벨 = %v %v", ls, err)
	}
	if s, _ := ls[1].String(); s != "ui" {
		t.Fatalf("둘째 라벨 = %q", s)
	}
	if got := mustString(t, items[0], "FlaggedDuringContentUpdateRestriction"); got != "0" {
		t.Fatalf("목록 칸 뒤 키 = %q", got)
	}
	// 한글 address (큰따옴표 \u 이스케이프)
	if got := mustString(t, items[1], "m_Address"); got != "아이콘/칼" {
		t.Fatalf("한글 address = %q", got)
	}
	empty, _ := items[1].Lookup("m_SerializedLabels")
	if es, err := empty.List(); err != nil || len(es) != 0 {
		t.Fatalf("[] = %v %v", es, err)
	}
}

func TestFlowMap(t *testing.T) {
	d := mustParse(t, groupYAML)[0]
	if got := mustString(t, d.Root, "MonoBehaviour", "m_Script", "guid"); got != "bbb281ee3bf0b054c82ac2347e9e782c" {
		t.Fatalf("흐름 맵 guid = %q", got)
	}
	schemas, _ := d.Root.Lookup("MonoBehaviour", "m_SchemaSet", "m_Schemas")
	items, err := schemas.List()
	if err != nil || len(items) != 2 {
		t.Fatalf("스키마 = %v %v", items, err)
	}
	if got := mustString(t, items[0], "type"); got != "2" {
		t.Fatalf("type = %q", got)
	}
	if g, _ := items[1].Lookup("guid"); g != nil {
		t.Fatalf("{fileID: 0} 에 guid 가 있으면 안 된다")
	}
	nested := mustParse(t, "a: {x: 0.5, y: {z: 'q, r'}, w: [1, 2]}\n")[0]
	if got := mustString(t, nested.Root, "a", "y", "z"); got != "q, r" {
		t.Fatalf("겹친 흐름 맵 = %q", got)
	}
	w, _ := nested.Root.Lookup("a", "w")
	if ws, err := w.List(); err != nil || len(ws) != 2 {
		t.Fatalf("흐름 목록 = %v %v", ws, err)
	}
}

func TestMetaWithoutHeader(t *testing.T) {
	src := "fileFormatVersion: 2\nguid: 9a8b7c6d5e4f30211203f4e5d6c7b8a9\nTextureImporter:\n  internalIDToNameTable:\n  - first:\n      213: 7482667652216324306\n    second: icon_sword\n  spriteMode: 2\n"
	docs := mustParse(t, src)
	if len(docs) != 1 || docs[0].ClassID != "" {
		t.Fatalf("머리 없는 문서 = %+v", docs)
	}
	r := docs[0].Root
	if got := mustString(t, r, "guid"); got != "9a8b7c6d5e4f30211203f4e5d6c7b8a9" {
		t.Fatalf("guid = %q", got)
	}
	tbl, _ := r.Lookup("TextureImporter", "internalIDToNameTable")
	items, err := tbl.List()
	if err != nil || len(items) != 1 {
		t.Fatalf("표 = %v %v", items, err)
	}
	if got := mustString(t, items[0], "first", "213"); got != "7482667652216324306" {
		t.Fatalf("first.213 = %q", got)
	}
	if got := mustString(t, items[0], "second"); got != "icon_sword" {
		t.Fatalf("second = %q", got)
	}
	if got := mustString(t, r, "TextureImporter", "spriteMode"); got != "2" {
		t.Fatalf("spriteMode = %q", got)
	}
}

func TestQuotes(t *testing.T) {
	src := "a: 'it''s here'\nb: \"tab\\tq\\\"x\\\\\"\nc: '[UnityEditor.PlayerSettings.bundleVersion]'\nd: \"\\uD83D\\uDE00!\"\ne: plain value\n"
	r := mustParse(t, src)[0].Root
	cases := map[string]string{
		"a": "it's here",
		"b": "tab\tq\"x\\",
		"c": "[UnityEditor.PlayerSettings.bundleVersion]",
		"d": "\U0001F600!",
		"e": "plain value",
	}
	for k, want := range cases {
		if got := mustString(t, r, k); got != want {
			t.Errorf("%s = %q, %q 이어야 한다", k, got, want)
		}
	}
}

func TestLoneSurrogateIsBad(t *testing.T) {
	r := mustParse(t, "a: \"\\uD83Dx\"\nb: 1\n")[0].Root
	n, err := r.Lookup("a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := n.String(); err == nil {
		t.Fatalf("짝 없는 서로게이트가 통과했다")
	}
	if got := mustString(t, r, "b"); got != "1" {
		t.Fatalf("다음 칸 = %q", got)
	}
}

func TestFoldedLines(t *testing.T) {
	src := strings.Join([]string{
		"m_Plain: first part",
		"    second part",
		"",
		"    after blank",
		"m_Single: 'one two",
		"    three ''four''",
		"    five'",
		"m_Double: \"alpha \\",
		"    beta",
		"    gamma\"",
		"m_Next: ok",
		"",
	}, "\n")
	r := mustParse(t, src)[0].Root
	cases := map[string]string{
		"m_Plain":  "first part second part\nafter blank",
		"m_Single": "one two three 'four' five",
		"m_Double": "alpha beta gamma",
		"m_Next":   "ok",
	}
	for k, want := range cases {
		if got := mustString(t, r, k); got != want {
			t.Errorf("%s = %q, %q 이어야 한다", k, got, want)
		}
	}
}

func TestFoldedInsideListItemMap(t *testing.T) {
	src := "list:\n- m_Address: long\n    address\n  m_GUID: x\n"
	r := mustParse(t, src)[0].Root
	l, _ := r.Lookup("list")
	items, err := l.List()
	if err != nil || len(items) != 1 {
		t.Fatalf("목록 = %v %v", items, err)
	}
	if got := mustString(t, items[0], "m_Address"); got != "long address" {
		t.Fatalf("접힌 address = %q", got)
	}
	if got := mustString(t, items[0], "m_GUID"); got != "x" {
		t.Fatalf("m_GUID = %q", got)
	}
}

func TestUnknownFormOnlyFailsWhenRead(t *testing.T) {
	src := "a: |\n  block text\n  more\nb: *alias\nc: ok\n"
	r := mustParse(t, src)[0].Root
	for _, k := range []string{"a", "b"} {
		n, err := r.Lookup(k)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := n.String(); err == nil {
			t.Errorf("%s : 모르는 꼴이 통과했다", k)
		}
	}
	if got := mustString(t, r, "c"); got != "ok" {
		t.Fatalf("c = %q", got)
	}
	// 모르는 꼴을 지나 아래 칸을 찾으려 해도 오류다.
	if _, err := r.Lookup("a", "x"); err == nil {
		t.Fatalf("모르는 꼴 아래 Lookup 이 통과했다")
	}
}

func TestMultipleDocuments(t *testing.T) {
	src := "--- !u!1 &100\nGameObject:\n  m_Name: Hero\n--- !u!4 &200 stripped\nTransform:\n  m_Father: {fileID: 0}\n"
	docs := mustParse(t, src)
	if len(docs) != 2 {
		t.Fatalf("문서 수 = %d", len(docs))
	}
	if docs[1].ClassID != "4" || docs[1].FileID != "200" {
		t.Fatalf("둘째 머리 = %+v", docs[1])
	}
}

func TestStructuralErrors(t *testing.T) {
	bad := []string{
		"a:\n  b: 1\n c: 2\n",
		"a:\n\tb: 1\n",
		"just text\n",
	}
	for _, src := range bad {
		if _, err := Parse([]byte(src)); err == nil {
			t.Errorf("%q : 오류가 나야 한다", src)
		}
	}
}

func TestCRLFAndBOM(t *testing.T) {
	r := mustParse(t, "\xEF\xBB\xBFa: 1\r\nb:\r\n- x\r\n")[0].Root
	if got := mustString(t, r, "a"); got != "1" {
		t.Fatalf("a = %q", got)
	}
	b, _ := r.Lookup("b")
	if items, err := b.List(); err != nil || len(items) != 1 {
		t.Fatalf("b = %v %v", items, err)
	}
}

func TestFlowMapAcrossLines(t *testing.T) {
	src := "m_Modifications:\n- target: {fileID: 1, guid: abc,\n    type: 3}\n  value: ok\n"
	r := mustParse(t, src)[0].Root
	l, _ := r.Lookup("m_Modifications")
	items, err := l.List()
	if err != nil || len(items) != 1 {
		t.Fatalf("목록 = %v %v", items, err)
	}
	if got := mustString(t, items[0], "target", "type"); got != "3" {
		t.Fatalf("줄을 넘긴 흐름 맵 type = %q", got)
	}
	if got := mustString(t, items[0], "target", "guid"); got != "abc" {
		t.Fatalf("guid = %q", got)
	}
	if got := mustString(t, items[0], "value"); got != "ok" {
		t.Fatalf("다음 칸 = %q", got)
	}
}

func TestNestedSeqIsBadButSiblingReadable(t *testing.T) {
	src := "sprites:\n- name: s\n  outline:\n  - - {x: 1, y: 2}\n    - {x: 3, y: 4}\n  rect:\n    x: 5\n"
	r := mustParse(t, src)[0].Root
	l, _ := r.Lookup("sprites")
	items, err := l.List()
	if err != nil || len(items) != 1 {
		t.Fatalf("목록 = %v %v", items, err)
	}
	outline, _ := items[0].Lookup("outline")
	inner, err := outline.List()
	if err != nil || len(inner) != 1 || inner[0].Kind != KindBad {
		t.Fatalf("겹친 목록은 못 읽음이어야 한다 = %+v %v", inner, err)
	}
	if got := mustString(t, items[0], "rect", "x"); got != "5" {
		t.Fatalf("옆 rect.x = %q", got)
	}
}

func TestEmptyQuotedValues(t *testing.T) {
	r := mustParse(t, "a: ''\nb: \"\"\nc:\n")[0].Root
	for _, k := range []string{"a", "b"} {
		n, _ := r.Lookup(k)
		if n == nil || n.Null || n.Kind != KindScalar || n.Value != "" {
			t.Fatalf("%s = %+v", k, n)
		}
	}
	if c, _ := r.Lookup("c"); c == nil || !c.Null {
		t.Fatalf("c 는 빈 값이어야 한다 = %+v", c)
	}
}

func TestQuotedKeys(t *testing.T) {
	r := mustParse(t, "'k one': v\n\"k\u0032\": w\nm: {'q k': x, \"r\": y}\n")[0].Root
	cases := map[string][]string{
		"v": {"k one"},
		"w": {"k2"},
		"x": {"m", "q k"},
		"y": {"m", "r"},
	}
	for want, path := range cases {
		if got := mustString(t, r, path...); got != want {
			t.Errorf("%v = %q, %q 이어야 한다", path, got, want)
		}
	}
}

// 따옴표 없는 값 안의 ` #` 는 자르지 않고 못 읽음으로 둔다 (`Item #1` 이 `Item` 으로 조용히 틀리지 않게).
func TestHashInPlainValueIsBad(t *testing.T) {
	r := mustParse(t, "m_Address: Item #1\nm_Long: first\n  second #2\nm_Quoted: 'Item #1'\nm_Hash: a#b\nm_Next: ok\n")[0].Root
	for _, k := range []string{"m_Address", "m_Long"} {
		n, err := r.Lookup(k)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := n.String(); err == nil {
			t.Errorf("%s : ` #` 가 든 값이 통과했다", k)
		}
	}
	want := map[string]string{"m_Quoted": "Item #1", "m_Hash": "a#b", "m_Next": "ok"}
	for k, w := range want {
		if got := mustString(t, r, k); got != w {
			t.Errorf("%s = %q", k, got)
		}
	}
}

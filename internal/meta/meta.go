// Package meta 는 Assets 아래 .meta 를 한 번 훑어 guid → 경로 표와 스프라이트 시트를 만든다.
package meta

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/MiruSona/officina-asset-tool/internal/unityyaml"
)

// Sprite 는 스프라이트 시트 안 한 칸이다. rect 는 픽셀, y 는 아래에서 잰다 (Unity 꼴 그대로).
type Sprite struct {
	Name       string
	X, Y, W, H float64
}

// Asset 은 .meta 짝이 있는 파일이나 폴더 하나다. Path 는 뿌리 기준, `/` 구분.
type Asset struct {
	Path        string
	GUID        string
	IsDir       bool
	SpriteSheet bool // spriteMode: 2 (Multiple)
	Sprites     []Sprite
	// SheetErr 는 시트 칸을 못 읽은 까닭이다. 색인이 이 에셋을 쓸 때만 오류로 올린다 (설계 4-2).
	SheetErr error
}

// Table 은 guid 와 경로 양쪽으로 찾는 표다.
type Table struct {
	byGUID map[string]*Asset
	byPath map[string]*Asset
}

func (t *Table) ByGUID(guid string) (*Asset, bool) {
	a, ok := t.byGUID[strings.ToLower(guid)]
	return a, ok
}

func (t *Table) ByPath(path string) (*Asset, bool) {
	a, ok := t.byPath[path]
	return a, ok
}

var guidRe = regexp.MustCompile(`^[0-9a-f]{32}$`)
var guidLineRe = regexp.MustCompile(`(?m)^guid: *([0-9a-fA-F]{32}) *\r?$`)
var spriteModeLineRe = regexp.MustCompile(`(?m)^  spriteMode: *2 *\r?$`)

// Unity 는 `.` 으로 시작하거나 `~` 로 끝나는 이름을 임포트하지 않는다.
func ignoredName(name string) bool {
	return strings.HasPrefix(name, ".") || strings.HasSuffix(name, "~")
}

// Scan 은 <root>/Assets 를 이름 차례로 훑는다. guid 가 겹치면 먼저 본 것을 쓰고 알림을 낸다.
func Scan(root string) (*Table, []string, error) {
	t := &Table{byGUID: map[string]*Asset{}, byPath: map[string]*Asset{}}
	var notices []string
	assets := filepath.Join(root, "Assets")
	if _, err := os.Stat(assets); os.IsNotExist(err) {
		return t, nil, nil
	}

	err := filepath.WalkDir(assets, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p != assets && ignoredName(d.Name()) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".meta") {
			return nil
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		metaRel := filepath.ToSlash(rel)
		assetRel := strings.TrimSuffix(metaRel, ".meta")

		info, statErr := os.Stat(strings.TrimSuffix(p, ".meta"))
		if statErr != nil {
			notices = append(notices, fmt.Sprintf("%s : 짝 파일이 없는 .meta 라 건너뛴다", metaRel))
			return nil
		}
		a, notice, err := readMeta(p, metaRel)
		if err != nil {
			return err
		}
		if notice != "" {
			notices = append(notices, notice)
		}
		if a == nil {
			return nil
		}
		a.Path = assetRel
		a.IsDir = info.IsDir()
		if first, dup := t.byGUID[a.GUID]; dup {
			notices = append(notices, fmt.Sprintf("%s : guid %s 가 %s 와 겹쳐 먼저 본 것을 쓴다", metaRel, a.GUID, first.Path))
			return nil
		}
		t.byGUID[a.GUID] = a
		t.byPath[a.Path] = a
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return t, notices, nil
}

// readMeta 는 .meta 한 장에서 guid 와 스프라이트 시트를 읽는다.
// 꼴이 무너졌으면 guid 줄만 건진다. 스프라이트 시트(Multiple)를 못 읽었으면 SheetErr 에 담아 둔다.
func readMeta(p, metaRel string) (*Asset, string, error) {
	data, err := os.ReadFile(p)
	if err != nil {
		return nil, "", err
	}
	docs, perr := unityyaml.Parse(data)
	if perr != nil {
		m := guidLineRe.FindSubmatch(data)
		if m == nil {
			return nil, "", fmt.Errorf("%s: %w", metaRel, perr)
		}
		a := &Asset{GUID: strings.ToLower(string(m[1]))}
		if strings.Contains(string(data), "\nTextureImporter:") && spriteModeLineRe.Match(data) {
			a.SpriteSheet = true
			a.SheetErr = fmt.Errorf("%s: 스프라이트 시트를 못 읽는다: %w", metaRel, perr)
		}
		notice := fmt.Sprintf("%s : 다 못 읽어 guid 만 쓴다 (%v)", metaRel, perr)
		return a, notice, nil
	}
	if len(docs) == 0 {
		return nil, metaRel + " : 비었다, 건너뛴다", nil
	}
	root := docs[0].Root

	g, err := root.Lookup("guid")
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", metaRel, err)
	}
	if g == nil {
		return nil, metaRel + " : guid 칸이 없어 건너뛴다", nil
	}
	guid, err := g.String()
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", metaRel, err)
	}
	guid = strings.ToLower(guid)
	if !guidRe.MatchString(guid) {
		return nil, fmt.Sprintf("%s : guid %q 가 32자 16진이 아니라 건너뛴다", metaRel, guid), nil
	}

	a := &Asset{GUID: guid}
	sprites, sheet, err := readSprites(root)
	if err != nil {
		a.SpriteSheet = true
		a.SheetErr = fmt.Errorf("%s: %w", metaRel, err)
		return a, "", nil
	}
	a.SpriteSheet = sheet
	a.Sprites = sprites
	return a, "", nil
}

// readSprites 는 TextureImporter 의 spriteMode: 2 일 때만 spriteSheet.sprites 를 읽는다.
func readSprites(root *unityyaml.Node) ([]Sprite, bool, error) {
	mode, err := root.Lookup("TextureImporter", "spriteMode")
	if err != nil || mode == nil {
		return nil, false, err
	}
	m, err := mode.Int()
	if err != nil {
		return nil, false, err
	}
	if m != 2 {
		return nil, false, nil
	}
	list, err := root.Lookup("TextureImporter", "spriteSheet", "sprites")
	if err != nil {
		return nil, false, err
	}
	if list == nil {
		return nil, true, nil
	}
	items, err := list.List()
	if err != nil {
		return nil, false, err
	}
	var out []Sprite
	for _, it := range items {
		s, err := readSprite(it)
		if err != nil {
			return nil, false, err
		}
		out = append(out, s)
	}
	return out, true, nil
}

func readSprite(it *unityyaml.Node) (Sprite, error) {
	var s Sprite
	nameNode, err := it.Lookup("name")
	if err != nil {
		return s, err
	}
	if nameNode == nil {
		return s, &unityyaml.Error{Line: it.Line, Msg: "스프라이트에 name 칸이 없다"}
	}
	if s.Name, err = nameNode.String(); err != nil {
		return s, err
	}
	fields := []struct {
		key string
		dst *float64
	}{{"x", &s.X}, {"y", &s.Y}, {"width", &s.W}, {"height", &s.H}}
	for _, f := range fields {
		n, err := it.Lookup("rect", f.key)
		if err != nil {
			return s, err
		}
		if n == nil {
			return s, &unityyaml.Error{Line: it.Line, Msg: "스프라이트 rect 에 " + f.key + " 칸이 없다"}
		}
		if *f.dst, err = n.Float(); err != nil {
			return s, err
		}
	}
	return s, nil
}

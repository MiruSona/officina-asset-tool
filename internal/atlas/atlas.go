// Package atlas 는 Unity 스프라이트 아틀라스(.spriteatlas · .spriteatlasv2)를 풀어 안에 든 스프라이트를 준다.
// 아틀라스 칸 이름을 아는 유일한 곳이다. 규칙은 Docs/Design/2026-09-23-index설계.md 6절.
package atlas

import (
	"encoding/binary"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/MiruSona/officina-asset-tool/internal/meta"
	"github.com/MiruSona/officina-asset-tool/internal/unityyaml"
)

// packable 원소의 fileID 뜻. 그 밖의 수는 텍스처 안 스프라이트 하나(internalID)다.
const (
	fileIDTexture      = 2800000   // 텍스처 전체
	fileIDFolder       = 102900000 // 폴더 (하위 폴더까지)
	fileIDSingleSprite = 21300000  // Single 텍스처의 스프라이트
)

// Sprite 는 아틀라스 안 스프라이트 하나와 그 원본 텍스처다. rect 는 픽셀, y 는 아래에서.
// Single 은 그림 전체라 0,0,w,h 이고, png 머리에서 크기를 못 읽으면 W·H 가 0 (= 그림 전체) 이다.
type Sprite struct {
	Name       string
	X, Y, W, H float64
	Path       string
	GUID       string
}

// IsAtlas 는 경로가 스프라이트 아틀라스인지 확장자로 본다.
func IsAtlas(p string) bool {
	switch strings.ToLower(path.Ext(p)) {
	case ".spriteatlas", ".spriteatlasv2":
		return true
	}
	return false
}

type ref struct {
	fileID int64
	guid   string
}

type atlasBody struct {
	variant   bool
	master    string
	packables []ref
}

type reader struct {
	root       string
	files      *os.Root // png 머리를 뿌리 안에서만 연다 (링크로 뿌리 밖을 읽지 않게). 못 열면 nil
	table      *meta.Table
	path       string // 알림 머리에 붙이는 아틀라스 경로
	notices    []string
	unresolved []string // 못 푼 packable 의 까닭. 하나라도 있으면 sub 를 비운다
}

func (r *reader) notice(format string, args ...any) {
	r.notices = append(r.notices, r.path+" : "+fmt.Sprintf(format, args...))
}

func (r *reader) fail(format string, args ...any) {
	r.unresolved = append(r.unresolved, fmt.Sprintf(format, args...))
}

// Read 는 아틀라스 a 를 풀어 스프라이트를 이름의 바이트 차례로 준다. 못 푸는 것은 알림으로만 낸다.
// packable 을 하나라도 못 풀면 일부만 찬 목록 대신 nil(= 모른다)을 준다 — 맞는 이름을 오류로 잡지 않게.
func Read(root string, t *meta.Table, a *meta.Asset) ([]Sprite, []string) {
	r := &reader{root: root, table: t, path: a.Path}
	if files, err := os.OpenRoot(root); err == nil {
		r.files = files
		defer files.Close()
	}
	body, err := readBody(root, a.Path)
	if err != nil {
		r.notice("아틀라스를 못 읽어 sub 를 안 만든다 (%v)", err)
		return nil, r.notices
	}
	if body.variant {
		master, ok := t.ByGUID(body.master)
		if !ok {
			r.notice("variant 의 마스터 guid %q 가 .meta 표에 없어 sub 를 안 만든다", body.master)
			return nil, r.notices
		}
		mb, err := readBody(root, master.Path)
		if err != nil {
			r.notice("variant 의 마스터 %s 를 못 읽어 sub 를 안 만든다 (%v)", master.Path, err)
			return nil, r.notices
		}
		if mb.variant {
			r.notice("variant 의 마스터 %s 도 variant 라 풀지 않는다", master.Path)
			return nil, r.notices
		}
		body = mb
	}

	// 같은 스프라이트가 두 packable 로 들어오면 조용히 하나로, 다른 그림의 같은 이름이면 첫 것만 두고 알림.
	firstGUID := map[string]string{}
	var out []Sprite
	for _, p := range body.packables {
		for _, s := range r.resolve(p) {
			first, seen := firstGUID[s.Name]
			if !seen {
				firstGUID[s.Name] = s.GUID
				out = append(out, s)
				continue
			}
			if first != s.GUID {
				r.notice("스프라이트 이름 %q 가 둘 이상에 겹쳐 먼저 것(guid %s)만 둔다 (뒤 것 %s)", s.Name, first, s.Path)
			}
		}
	}
	if len(r.unresolved) > 0 {
		r.notice("packable 을 다 못 풀어 sub 를 비운다 (모른다) — %s", strings.Join(r.unresolved, " · "))
		return nil, r.notices
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, r.notices
}

// readBody 는 v2(SpriteAtlasAsset) · v1(SpriteAtlas) 어느 꼴이든 마스터와 packables 를 읽는다.
// 묶은 뒤에만 채워지는 v1 의 packed 칸들은 믿지 않는다.
func readBody(root, rel string) (atlasBody, error) {
	var b atlasBody
	data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return b, err
	}
	docs, err := unityyaml.Parse(data)
	if err != nil {
		return b, err
	}
	var body *unityyaml.Node
	var packablesPath []string
	for _, d := range docs {
		if body, err = d.Root.Lookup("SpriteAtlasAsset"); err != nil {
			return b, err
		}
		if body != nil {
			packablesPath = []string{"m_ImporterData", "packables"}
			break
		}
		if body, err = d.Root.Lookup("SpriteAtlas"); err != nil {
			return b, err
		}
		if body != nil {
			packablesPath = []string{"m_EditorData", "packables"}
			break
		}
	}
	if body == nil {
		return b, fmt.Errorf("SpriteAtlasAsset·SpriteAtlas 문서가 없다")
	}

	flag, err := body.Lookup("m_IsVariant")
	if err != nil {
		return b, err
	}
	if flag != nil {
		if b.variant, err = flag.Flag(); err != nil {
			return b, err
		}
	}
	if b.variant {
		master, err := readRef(body, "m_MasterAtlas")
		if err != nil {
			return b, err
		}
		b.master = master.guid
		return b, nil
	}
	list, err := body.Lookup(packablesPath...)
	if err != nil {
		return b, err
	}
	if list == nil {
		return b, fmt.Errorf("%s 칸이 없다", strings.Join(packablesPath, "."))
	}
	items, err := list.List()
	if err != nil {
		return b, err
	}
	for _, it := range items {
		p, err := readRef(it)
		if err != nil {
			return b, err
		}
		b.packables = append(b.packables, p)
	}
	return b, nil
}

// readRef 는 `{fileID: …, guid: …, type: 3}` 를 읽는다. key 를 주면 그 칸을 읽는다. guid 가 없으면 빈 값.
func readRef(n *unityyaml.Node, key ...string) (ref, error) {
	var r ref
	node, err := n.Lookup(key...)
	if err != nil || node == nil {
		return r, err
	}
	id, err := node.Lookup("fileID")
	if err != nil || id == nil {
		return r, err
	}
	text, err := id.String()
	if err != nil {
		return r, err
	}
	if r.fileID, err = strconv.ParseInt(text, 10, 64); err != nil {
		return r, &unityyaml.Error{Line: id.Line, Msg: "fileID 가 정수가 아니다: " + text}
	}
	g, err := node.Lookup("guid")
	if err != nil || g == nil {
		return r, err
	}
	s, err := g.String()
	r.guid = strings.ToLower(s)
	return r, err
}

// resolve 는 packable 원소 하나를 스프라이트들로 푼다 (꼴 셋은 설계 6절 표).
func (r *reader) resolve(p ref) []Sprite {
	a, ok := r.table.ByGUID(p.guid)
	if p.guid == "" || !ok {
		r.fail("guid %q 가 .meta 표에 없다 (Packages/ 안이거나 지운 에셋)", p.guid)
		return nil
	}
	if a.IsDir && p.fileID == fileIDFolder {
		return r.folder(a)
	}
	if a.IsDir {
		r.fail("%s 는 폴더인데 fileID 가 %d 다", a.Path, p.fileID)
		return nil
	}
	if p.fileID == fileIDFolder {
		r.fail("폴더 fileID 인데 %s 는 폴더가 아니다", a.Path)
		return nil
	}
	if a.SheetErr != nil {
		r.fail("%s 의 스프라이트를 못 읽었다 (%v)", a.Path, a.SheetErr)
		return nil
	}
	if a.SpriteMode != 1 && a.SpriteMode != 2 {
		r.fail("%s 가 스프라이트 텍스처가 아니다", a.Path)
		return nil
	}
	all := r.textureSprites(a)
	if p.fileID == fileIDTexture {
		return all
	}
	if a.SpriteMode == 1 && p.fileID == fileIDSingleSprite {
		return all
	}
	if a.SpriteMode == 2 {
		for i, s := range a.Sprites {
			if s.ID == p.fileID {
				return all[i : i+1]
			}
		}
	}
	r.fail("%s 에 fileID %d 인 스프라이트가 없다", a.Path, p.fileID)
	return nil
}

// folder 는 폴더 아래(하위 폴더까지) 스프라이트 텍스처를 경로 차례로 푼다.
// .meta 가 없는 자리(`.`·`~` 이름 등 Unity 가 임포트하지 않는 곳)는 들어가지 않는다.
func (r *reader) folder(dir *meta.Asset) []Sprite {
	var out []Sprite
	top := filepath.Join(r.root, filepath.FromSlash(dir.Path))
	err := filepath.WalkDir(top, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == top {
			return nil
		}
		rel, err := filepath.Rel(r.root, p)
		if err != nil {
			return err
		}
		a, hasMeta := r.table.ByPath(filepath.ToSlash(rel))
		if !hasMeta {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if a.SheetErr != nil {
			r.fail("폴더 안 %s 의 스프라이트를 못 읽었다 (%v)", a.Path, a.SheetErr)
			return nil
		}
		if a.SpriteMode == 1 || a.SpriteMode == 2 {
			out = append(out, r.textureSprites(a)...)
		}
		return nil
	})
	if err != nil {
		r.fail("폴더 %s 를 다 못 훑었다 (%v)", dir.Path, err)
	}
	return out
}

var pngSignature = []byte("\x89PNG\r\n\x1a\n")

// maxSide 는 믿는 한 변 크기의 끝이다. 넘으면 머리가 깨진 것으로 보고 0(= 그림 전체)으로 둔다.
const maxSide = 65535

// textureSprites 는 텍스처 하나의 스프라이트 전부다. Single 은 하나(이름 = 파일 이름), Multiple 은 시트 칸들.
func (r *reader) textureSprites(a *meta.Asset) []Sprite {
	if a.SpriteMode == 2 {
		out := make([]Sprite, 0, len(a.Sprites))
		for _, s := range a.Sprites {
			out = append(out, Sprite{Name: s.Name, X: s.X, Y: s.Y, W: s.W, H: s.H, Path: a.Path, GUID: a.GUID})
		}
		return out
	}
	s := Sprite{Name: strings.TrimSuffix(path.Base(a.Path), path.Ext(a.Path)), Path: a.Path, GUID: a.GUID}
	// png 머리 24바이트 : 서명 8 · 길이 4 · "IHDR" 4 · 너비 4 · 높이 4 (빅 엔디언).
	if r.files == nil {
		return []Sprite{s}
	}
	f, err := r.files.Open(a.Path)
	if err != nil {
		return []Sprite{s}
	}
	defer f.Close()
	head := make([]byte, 24)
	if _, err := io.ReadFull(f, head); err != nil {
		return []Sprite{s}
	}
	if string(head[:8]) != string(pngSignature) || string(head[12:16]) != "IHDR" {
		return []Sprite{s}
	}
	w := binary.BigEndian.Uint32(head[16:20])
	h := binary.BigEndian.Uint32(head[20:24])
	if w <= maxSide && h <= maxSide {
		s.W = float64(w)
		s.H = float64(h)
	}
	return []Sprite{s}
}

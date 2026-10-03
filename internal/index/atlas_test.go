package index

import (
	"encoding/binary"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/MiruSona/officina-asset-tool/internal/testproj"
)

const atlasDir = "Assets/Atlas/"

var atlasEntries = []testproj.Entry{
	{GUID: "844a744467499934ab7f410af26443ca", Address: "atlas_ui"},
	{GUID: "8ef34f3e5c8130042b2cf22f119b4f97", Address: "atlas_ui_v1"},
	{GUID: "d1db86c2f73f05f40968115787bcafa7", Address: "atlas_ui_small"},
	{GUID: "e567039575c35724faa6b0baa1258767", Address: "atlas_ui_small_v1"},
	{GUID: "5806e4717b061ba40bcabcf77c595359", Address: "atlas_single"},
	{GUID: "22c910779824ec149b4015bbe3ebda4b", Address: "atlas_single_v1"},
}

func sub(name, file, guid string, x, y, w, h float64) Sub {
	return Sub{Name: name, Rect: Rect{X: x, Y: y, W: w, H: h}, Path: atlasDir + file, GUID: guid}
}

// 정답 표는 Unity API(SpriteAtlas.GetSprites)로 얻은 것이다.
var (
	sheetGUID = "0b67f208dd3701b4a9c57f07badc1776"
	wantUI    = []Sub{
		sub("atl_big", "atl_big.png", "1aad82c26af84b449b6fff9f46d8018d", 0, 0, 256, 256),
		sub("atl_checker", "atl_checker.png", "d325406ae6c66c4408213538abab0ac9", 0, 0, 32, 32),
		sub("atl_stripes", "atl_stripes.png", "b450e9aa49842754495691b96cf98f2a", 0, 0, 32, 32),
		sub("cell_a", "atl_sheet.png", sheetGUID, 0, 32, 32, 32),
		sub("cell_b", "atl_sheet.png", sheetGUID, 32, 32, 32, 32),
		sub("cell_c", "atl_sheet.png", sheetGUID, 0, 0, 32, 32),
		sub("cell_d", "atl_sheet.png", sheetGUID, 32, 0, 32, 32),
		sub("f_deep", "AtlasFolder/Sub/f_deep.png", "0028c21338ef3184c8d79297fe17fd5a", 0, 0, 16, 16),
		sub("f_one", "AtlasFolder/f_one.png", "adcd520169b33ec4d8afd065238af7f5", 0, 0, 24, 24),
		sub("f_two", "AtlasFolder/f_two.png", "9a6ba2b0f17d1c3439bba133c377dbc9", 0, 0, 24, 24),
	}
	wantSingle = []Sub{
		sub("atl_border", "atl_border.png", "de330aa03b1e42142901e343d9192c57", 0, 0, 48, 48),
		sub("cell_c", "atl_sheet.png", sheetGUID, 0, 0, 32, 32),
	}
	wantAtlas = map[string][]Sub{
		"atlas_ui": wantUI, "atlas_ui_v1": wantUI, "atlas_ui_small": wantUI, "atlas_ui_small_v1": wantUI,
		"atlas_single": wantSingle, "atlas_single_v1": wantSingle,
	}
)

func atlasProj(t *testing.T) *proj {
	p := newProj(t, atlasEntries, nil)
	p.CopyDir(filepath.Join("..", "..", "Testdata", "atlas", "Assets", "Atlas"), "Assets/Atlas")
	return p
}

func checkAtlases(t *testing.T, r *Result) {
	t.Helper()
	if len(r.Notices) != 0 {
		t.Fatalf("알림이 없어야 한다 = %v", r.Notices)
	}
	for _, e := range r.Index.Entries {
		want, ok := wantAtlas[e.Address]
		if !ok {
			t.Fatalf("모르는 항목 %q", e.Address)
		}
		if e.Kind != "other" {
			t.Errorf("%s kind = %q, other 여야 한다", e.Address, e.Kind)
		}
		if !reflect.DeepEqual(e.Sub, want) {
			t.Errorf("%s sub\n%+v\n--- 기대 ---\n%+v", e.Address, e.Sub, want)
		}
	}
	if len(r.Index.Entries) != len(wantAtlas) {
		t.Fatalf("항목 수 = %d", len(r.Index.Entries))
	}
}

// A1 : Unity 가 쓴 아틀라스 6개(v1 은 묶은 뒤) → 정답 표와 같다.
func TestAtlasSubMatchesUnity(t *testing.T) {
	checkAtlases(t, build(t, atlasProj(t).Root))
}

// A2 : v1 을 묶기 전 사본으로 바꿔도 같다 — packed 칸을 믿지 않고 packables 로 푼다.
func TestAtlasV1BeforePack(t *testing.T) {
	p := atlasProj(t)
	for _, name := range []string{"atlas_ui_v1", "atlas_ui_small_v1", "atlas_single_v1"} {
		data, err := os.ReadFile(filepath.Join("..", "..", "Testdata", "atlas", "before-pack", name+".spriteatlas"))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(data), "- f_two") {
			t.Fatalf("%s 는 묶기 전 사본이어야 한다", name)
		}
		p.File(atlasDir+name+".spriteatlas", string(data))
	}
	checkAtlases(t, build(t, p.Root))
}

// A3 : sub 원소 JSON 꼴 — 아틀라스는 path·guid 가 붙고, 스프라이트 시트는 계약 예제 그대로(I2)다.
func TestAtlasSubJSON(t *testing.T) {
	r := build(t, atlasProj(t).Root)
	data, err := Encode(r.Index)
	if err != nil {
		t.Fatal(err)
	}
	want := `{
          "name": "cell_c",
          "rect": {
            "x": 0,
            "y": 0,
            "w": 32,
            "h": 32
          },
          "path": "Assets/Atlas/atl_sheet.png",
          "guid": "0b67f208dd3701b4a9c57f07badc1776"
        }`
	if !strings.Contains(string(data), want) {
		t.Fatalf("sub 꼴이 다르다\n%s", data)
	}
}

func atlasNotice(t *testing.T, edit func(p *proj), file string, words ...string) Entry {
	t.Helper()
	p := atlasProj(t)
	edit(p)
	r := build(t, p.Root)
	var hit []string
	for _, n := range r.Notices {
		if strings.HasPrefix(n, atlasDir+file) {
			hit = append(hit, n)
		}
	}
	notice(t, hit, words...)
	for _, e := range r.Index.Entries {
		if e.Path == atlasDir+file {
			return e
		}
	}
	t.Fatalf("%s 항목이 없다", file)
	return Entry{}
}

// A4 : 못 푸는 것은 알림만 내고 index 는 산다.
func TestAtlasNotices(t *testing.T) {
	const big = "1aad82c26af84b449b6fff9f46d8018d"
	t.Run("깨진 아틀라스 → sub 없이 넣고 알림", func(t *testing.T) {
		e := atlasNotice(t, func(p *proj) {
			p.File(atlasDir+"atlas_ui.spriteatlasv2", "--- !u!612988286 &1\nSpriteAtlasAsset:\n  m_Name: x\n bad: 1\n")
		}, "atlas_ui.spriteatlasv2", "못 읽")
		if e.Sub != nil {
			t.Fatalf("sub 가 없어야 한다 = %+v", e.Sub)
		}
	})
	t.Run("모르는 꼴 → sub 없이 넣고 알림", func(t *testing.T) {
		e := atlasNotice(t, func(p *proj) {
			p.File(atlasDir+"atlas_ui.spriteatlasv2", "--- !u!114 &1\nMonoBehaviour:\n  m_Name: x\n")
		}, "atlas_ui.spriteatlasv2", "SpriteAtlas")
		if e.Sub != nil {
			t.Fatalf("sub 가 없어야 한다 = %+v", e.Sub)
		}
	})
	// 하나라도 못 푼 packable 이 있으면 일부만 찬 목록을 내지 않고 sub 를 비운다 (= 모른다).
	partial := map[string]struct {
		packable string
		word     string
	}{
		"packable guid 가 표에 없음":   {"{fileID: 2800000, guid: deaddeaddeaddeaddeaddeaddeaddead, type: 3}", "deaddeaddeaddeaddeaddeaddeaddead"},
		"없는 스프라이트 fileID":         {"{fileID: 12345, guid: " + sheetGUID + ", type: 3}", "12345"},
		"폴더 fileID 인데 guid 가 텍스처": {"{fileID: 102900000, guid: " + sheetGUID + ", type: 3}", "폴더가 아니"},
		"텍스처 fileID 인데 guid 가 폴더": {"{fileID: 2800000, guid: f074882591338cd4fbac41e15ac1cdba, type: 3}", "폴더인데"},
	}
	for name, c := range partial {
		t.Run(name+" → sub 비움, 알림", func(t *testing.T) {
			e := atlasNotice(t, func(p *proj) {
				p.File(atlasDir+"atlas_single.spriteatlasv2", atlasV2(c.packable, "{fileID: 2800000, guid: "+big+", type: 3}"))
			}, "atlas_single.spriteatlasv2", c.word, "비운다")
			if e.Sub != nil {
				t.Fatalf("sub 가 없어야 한다 = %+v", e.Sub)
			}
		})
	}
	t.Run("마스터를 일부만 풀면 variant 도 sub 비움", func(t *testing.T) {
		e := atlasNotice(t, func(p *proj) {
			p.File(atlasDir+"atlas_ui.spriteatlasv2", atlasV2("{fileID: 2800000, guid: deaddeaddeaddeaddeaddeaddeaddead, type: 3}", "{fileID: 2800000, guid: "+big+", type: 3}"))
		}, "atlas_ui_small.spriteatlasv2", "deaddeaddeaddeaddeaddeaddeaddead", "비운다")
		if e.Sub != nil {
			t.Fatalf("sub 가 없어야 한다 = %+v", e.Sub)
		}
	})
	t.Run("이름 겹침 → 첫 것만, 알림", func(t *testing.T) {
		e := atlasNotice(t, func(p *proj) {
			p.File(atlasDir+"Other/atl_big.png", "png")
			p.File(atlasDir+"Other/atl_big.png.meta", "fileFormatVersion: 2\nguid: 77777777777777777777777777777777\nTextureImporter:\n  spriteMode: 1\n")
			p.Folder(atlasDir+"Other", "78777777777777777777777777777777")
			p.File(atlasDir+"atlas_single.spriteatlasv2", atlasV2("{fileID: 2800000, guid: "+big+", type: 3}", "{fileID: 2800000, guid: 77777777777777777777777777777777, type: 3}"))
		}, "atlas_single.spriteatlasv2", "atl_big", "겹")
		if len(e.Sub) != 1 || e.Sub[0].GUID != big {
			t.Fatalf("sub = %+v", e.Sub)
		}
	})
	t.Run("variant 의 마스터가 또 variant → 알림", func(t *testing.T) {
		e := atlasNotice(t, func(p *proj) {
			p.File(atlasDir+"atlas_ui.spriteatlasv2", strings.NewReplacer("m_MasterAtlas: {fileID: 0}", "m_MasterAtlas: {fileID: 100100200, guid: d1db86c2f73f05f40968115787bcafa7, type: 3}", "m_IsVariant: 0", "m_IsVariant: 1").Replace(atlasV2()))
		}, "atlas_ui_small.spriteatlasv2", "variant")
		if e.Sub != nil {
			t.Fatalf("sub 가 없어야 한다 = %+v", e.Sub)
		}
	})
}

// pngHead 는 서명과 IHDR 머리까지만 있는 png 앞 24바이트다.
func pngHead(w, h uint32) string {
	b := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR")
	b = binary.BigEndian.AppendUint32(b, w)
	b = binary.BigEndian.AppendUint32(b, h)
	return string(b)
}

// A5 : Single 의 크기를 못 믿으면 w·h 0 (= 그림 전체). 잘린 png · png 아님 · 말이 안 되게 큰 IHDR · 뿌리 밖 링크.
func TestAtlasSingleSizeEdges(t *testing.T) {
	p := atlasProj(t)
	single := func(rel, guid, body string) {
		p.File(atlasDir+rel, body)
		p.File(atlasDir+rel+".meta", "fileFormatVersion: 2\nguid: "+guid+"\nTextureImporter:\n  spriteMode: 1\n")
	}
	single("ok.png", "e1000000000000000000000000000001", pngHead(20, 10))
	single("cut.png", "e1000000000000000000000000000002", pngHead(20, 10)[:20])
	single("pic.jpg", "e1000000000000000000000000000003", "\xff\xd8\xff\xe0 jpeg")
	single("huge.png", "e1000000000000000000000000000004", pngHead(70000, 16))
	packs := []string{}
	for i := 1; i <= 4; i++ {
		packs = append(packs, fmt.Sprintf("{fileID: 2800000, guid: e100000000000000000000000000000%d, type: 3}", i))
	}

	// 뿌리 밖 png 를 가리키는 링크는 머리를 읽지 않는다. 링크를 못 만드는 기계(권한)면 이 칸만 뺀다.
	outside := filepath.Join(t.TempDir(), "outside.png")
	os.WriteFile(outside, []byte(pngHead(20, 10)), 0o644)
	link := filepath.Join(p.Root, filepath.FromSlash(atlasDir+"link.png"))
	hasLink := os.Symlink(outside, link) == nil
	if hasLink {
		p.File(atlasDir+"link.png.meta", "fileFormatVersion: 2\nguid: e1000000000000000000000000000005\nTextureImporter:\n  spriteMode: 1\n")
		packs = append(packs, "{fileID: 2800000, guid: e1000000000000000000000000000005, type: 3}")
	}
	p.File(atlasDir+"atlas_single.spriteatlasv2", atlasV2(packs...))

	r := build(t, p.Root)
	if len(r.Notices) != 0 {
		t.Fatalf("알림이 없어야 한다 = %v", r.Notices)
	}
	want := map[string]Rect{"ok": {W: 20, H: 10}, "cut": {}, "pic": {}, "huge": {}, "link": {}}
	var got *Entry
	for i := range r.Index.Entries {
		if r.Index.Entries[i].Address == "atlas_single" {
			got = &r.Index.Entries[i]
		}
	}
	if got == nil || len(got.Sub) != len(packs) {
		t.Fatalf("sub = %+v", got)
	}
	for _, s := range got.Sub {
		if s.Rect != want[s.Name] {
			t.Errorf("%s rect = %+v, %+v 여야 한다", s.Name, s.Rect, want[s.Name])
		}
	}
	if !hasLink {
		t.Log("심볼릭 링크를 못 만들어 뿌리 밖 링크 칸은 못 봤다")
	}
}

// atlasV2 는 packables 만 바꾼 v2 아틀라스 글이다 (Unity 가 쓴 꼴).
func atlasV2(packables ...string) string {
	list := " []\n"
	if len(packables) > 0 {
		list = "\n"
		for _, p := range packables {
			list += "    - " + p + "\n"
		}
	}
	return fmt.Sprintf("%%YAML 1.1\n%%TAG !u! tag:unity3d.com,2011:\n--- !u!612988286 &1\nSpriteAtlasAsset:\n  m_ObjectHideFlags: 0\n  m_Name: \n  serializedVersion: 2\n  m_MasterAtlas: {fileID: 0}\n  m_ImporterData:\n    packables:%s  m_IsVariant: 0\n  m_ScriptablePacker: {fileID: 0}\n", list)
}

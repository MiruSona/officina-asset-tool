package meta

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFiles(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for rel, body := range files {
		p := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(rel, "/") {
			continue
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

func simpleMeta(guid string) string {
	return "fileFormatVersion: 2\nguid: " + guid + "\nDefaultImporter:\n  externalObjects: {}\n  userData: \n"
}

const sheetMeta = `fileFormatVersion: 2
guid: 9a8b7c6d5e4f30211203f4e5d6c7b8a9
TextureImporter:
  internalIDToNameTable:
  - first:
      213: 7482667652216324306
    second: icon_sword
  serializedVersion: 12
  spriteMode: 2
  spriteSheet:
    serializedVersion: 2
    sprites:
    - serializedVersion: 2
      name: icon_sword
      rect:
        serializedVersion: 2
        x: 0
        y: 64
        width: 64
        height: 64
      alignment: 0
      pivot: {x: 0.5, y: 0.5}
      outline: []
      indices:
    - serializedVersion: 2
      name: icon_potion
      rect:
        serializedVersion: 2
        x: 64
        y: 64
        width: 64
        height: 64.5
    outline: []
  userData:
`

const singleMeta = `fileFormatVersion: 2
guid: 11111111111111111111111111111111
TextureImporter:
  spriteMode: 1
  spriteSheet:
    serializedVersion: 2
    sprites: []
`

// PSD Importer 는 TextureImporter 가 아니라 ScriptedImporter 로 적는다. 1차엔 sub 를 안 만든다.
const psbMeta = `fileFormatVersion: 2
guid: 22222222222222222222222222222222
ScriptedImporter:
  internalIDToNameTable: []
  externalObjects: {}
  serializedVersion: 2
  userData:
  script: {fileID: 11500000, guid: b2f9e1a8c5d34cc4b1d1a0e2f5a6b7c8, type: 3}
  textureImporterSettings:
    spriteMode: 2
  spriteImportData:
  - name: layer1
    rect:
      x: 0
      y: 0
      width: 8
      height: 8
`

func TestScanGuidTableAndSprites(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"Assets/Art.meta":            simpleMeta("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"),
		"Assets/Art/icons.png":       "png",
		"Assets/Art/icons.png.meta":  sheetMeta,
		"Assets/Art/single.png":      "png",
		"Assets/Art/single.png.meta": singleMeta,
		"Assets/Art/layers.psb":      "psb",
		"Assets/Art/layers.psb.meta": psbMeta,
		"Assets/.hidden/x.png.meta":  simpleMeta("cccccccccccccccccccccccccccccccc"),
		"Assets/.hidden/x.png":       "png",
		"Assets/Tmp~/y.png.meta":     simpleMeta("dddddddddddddddddddddddddddddddd"),
		"Assets/Tmp~/y.png":          "png",
		"Assets/Orphan.png.meta":     simpleMeta("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"),
		"Assets/Art/zdup.png":        "png",
		"Assets/Art/zdup.png.meta":   simpleMeta("9a8b7c6d5e4f30211203f4e5d6c7b8a9"),
		"Assets/Art/nometa.png":      "png",
	})
	tbl, notices, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}

	folder, ok := tbl.ByGUID("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
	if !ok || folder.Path != "Assets/Art" || !folder.IsDir {
		t.Fatalf("폴더 = %+v %v", folder, ok)
	}

	sheet, ok := tbl.ByGUID("9a8b7c6d5e4f30211203f4e5d6c7b8a9")
	if !ok || sheet.Path != "Assets/Art/icons.png" {
		t.Fatalf("시트 = %+v (먼저 본 것이 이겨야 한다)", sheet)
	}
	if !sheet.SpriteSheet || len(sheet.Sprites) != 2 {
		t.Fatalf("Multiple 은 sub 둘 = %+v", sheet)
	}
	want := Sprite{Name: "icon_potion", X: 64, Y: 64, W: 64, H: 64.5}
	if sheet.Sprites[1] != want {
		t.Fatalf("rect = %+v", sheet.Sprites[1])
	}
	if s, _ := tbl.ByGUID("11111111111111111111111111111111"); s.SpriteSheet || len(s.Sprites) != 0 {
		t.Fatalf("Single 은 sub 없음 = %+v", s)
	}
	if s, _ := tbl.ByGUID("22222222222222222222222222222222"); s.SpriteSheet || len(s.Sprites) != 0 {
		t.Fatalf(".psb 는 sub 없음 = %+v", s)
	}

	for _, g := range []string{"cccccccccccccccccccccccccccccccc", "dddddddddddddddddddddddddddddddd", "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"} {
		if _, ok := tbl.ByGUID(g); ok {
			t.Errorf("%s 는 표에 없어야 한다 (숨은 폴더·~ 폴더·짝 없는 .meta)", g)
		}
	}
	if _, ok := tbl.ByPath("Assets/Art/nometa.png"); ok {
		t.Errorf(".meta 없는 파일이 표에 있다")
	}

	joined := strings.Join(notices, "\n")
	if !strings.Contains(joined, "Assets/Art/zdup.png.meta") || !strings.Contains(joined, "Assets/Orphan.png.meta") {
		t.Fatalf("알림 = %v", notices)
	}
}

func TestBrokenMetaKeepsGuid(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"Assets/m.fbx":      "fbx",
		"Assets/m.fbx.meta": "fileFormatVersion: 2\nguid: 33333333333333333333333333333333\nModelImporter:\n  a:\n    b: 1\n   c: 2\n",
	})
	tbl, notices, err := Scan(root)
	if err != nil {
		t.Fatal(err)
	}
	if a, ok := tbl.ByGUID("33333333333333333333333333333333"); !ok || a.Path != "Assets/m.fbx" {
		t.Fatalf("guid 만이라도 읽어야 한다 = %+v", a)
	}
	if len(notices) != 1 {
		t.Fatalf("알림 한 줄이어야 한다 = %v", notices)
	}
}

// 시트를 못 읽어도 Scan 은 멈추지 않는다. 오류는 Asset 에 들고 있다가 색인이 그 에셋을 쓸 때 올린다.
func TestBrokenSpriteSheetIsDeferred(t *testing.T) {
	root := t.TempDir()
	writeFiles(t, root, map[string]string{
		"Assets/s.png":      "png",
		"Assets/s.png.meta": "fileFormatVersion: 2\nguid: 44444444444444444444444444444444\nTextureImporter:\n  spriteMode: 2\n  spriteSheet:\n    sprites:\n    - name: |\n        x\n",
		"Assets/b.png":      "png",
		"Assets/b.png.meta": "fileFormatVersion: 2\nguid: 55555555555555555555555555555555\nTextureImporter:\n  a:\n    b: 1\n   c: 2\n  spriteMode: 2\n",
	})
	tbl, _, err := Scan(root)
	if err != nil {
		t.Fatalf("Scan 은 멈추면 안 된다: %v", err)
	}
	for _, g := range []string{"44444444444444444444444444444444", "55555555555555555555555555555555"} {
		a, ok := tbl.ByGUID(g)
		if !ok || a.SheetErr == nil || !a.SpriteSheet {
			t.Fatalf("%s : 시트 오류를 들고 있어야 한다 = %+v", g, a)
		}
		if !strings.Contains(a.SheetErr.Error(), ".png.meta") {
			t.Fatalf("오류 글에 파일 이름이 없다 = %v", a.SheetErr)
		}
	}
}

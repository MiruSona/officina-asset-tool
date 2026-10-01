package index

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/MiruSona/officina-asset-tool/internal/testproj"
)

const dataDir = "Assets/AddressableAssetsData/"

func build(t *testing.T, root string) *Result {
	t.Helper()
	r, err := Build(Options{Root: root, Out: DefaultOut(root), Generator: "assettool test", Now: time.Unix(0, 0)})
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	return r
}

func pin(idx *Index) {
	idx.Generator = "assettool 0.1.0 (a1b2c3d)"
	idx.GeneratedAt = "2026-09-23T05:12:00Z"
	idx.SourceMtime = "2026-09-23T05:10:41.123456789Z"
}

// I2 : 시험 프로젝트 → 계약 예제와 바이트가 같다.
func TestIndexMatchesContractExample(t *testing.T) {
	root := filepath.Join("..", "..", "Testdata", "project")
	r := build(t, root)
	if len(r.Notices) != 0 {
		t.Fatalf("알림이 없어야 한다 = %v", r.Notices)
	}
	pin(r.Index)
	got, err := Encode(r.Index)
	if err != nil {
		t.Fatal(err)
	}
	want, err := os.ReadFile(filepath.Join("..", "..", "Testdata", "contract", "address-index.example.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("예제와 바이트가 다르다\n--- 나온 것 ---\n%s\n--- 예제 ---\n%s", got, want)
	}
	if r.Groups != 3 {
		t.Fatalf("그룹 수 = %d", r.Groups)
	}
}

type proj struct {
	*testproj.Project
	groups []string
}

// newProj 는 settings 와 그룹 하나(G, 스키마 하나 include 1)를 둔 뿌리를 만든다.
func newProj(t *testing.T, entries []testproj.Entry, labels []string) *proj {
	p := &proj{Project: testproj.New(t)}
	p.File("ProjectSettings/EditorBuildSettings.asset", testproj.EditorBuildSettings("2d000000000000000000000000000001"))
	p.Asset(dataDir+"DefaultObject.asset", "2d000000000000000000000000000001", testproj.DefaultObject("7c000000000000000000000000000001"))
	p.Asset(dataDir+"AssetGroups/Schemas/S.asset", "5c000000000000000000000000000001", testproj.Schema("1", "1"))
	p.Asset(dataDir+"AssetGroups/G.asset", "6d000000000000000000000000000001", testproj.Group("G", entries, []string{"5c000000000000000000000000000001"}, ""))
	p.Asset(dataDir+"AddressableAssetSettings.asset", "7c000000000000000000000000000001", testproj.Settings([]string{"6d000000000000000000000000000001"}, labels))
	return p
}

func addresses(idx *Index) []string {
	var out []string
	for _, e := range idx.Entries {
		out = append(out, e.Address)
	}
	return out
}

// I3 : 폴더 펼치기. 뺄 것 ①~④ · .asmref 는 남는다 · 따로 있는 항목이 이긴다.
func TestFolderExpansion(t *testing.T) {
	const (
		gFolder  = "f0000000000000000000000000000001"
		gOwn     = "f0000000000000000000000000000002"
		gSub     = "f0000000000000000000000000000003"
		gSubFile = "f0000000000000000000000000000004"
	)
	p := newProj(t, nil, []string{"x"})
	p.Folder("Assets/Things", gFolder)
	p.Asset("Assets/Things/a.png", "a0000000000000000000000000000001", "png")
	p.Asset("Assets/Things/deep/b.wav", "a0000000000000000000000000000002", "wav")
	p.Folder("Assets/Things/deep", "a0000000000000000000000000000003")
	p.Asset("Assets/Things/c.asmref", "a0000000000000000000000000000004", "{}")
	p.Asset("Assets/Things/own.prefab", gOwn, "")
	// ① .meta 없음 (Unity 가 임포트 안 한 . 이름 · ~ 이름 포함)
	p.File("Assets/Things/nometa.png", "png")
	p.File("Assets/Things/.hidden.png", "png")
	p.File("Assets/Things/backup~/x.png", "png")
	// ② 확장자
	for i, ext := range []string{"cs", "JS", "boo", "exe", "dll", "preset", "asmdef"} {
		p.Asset("Assets/Things/skip."+ext, "b000000000000000000000000000000"+string(rune('0'+i)), "")
	}
	// ③ /Editor/
	p.Folder("Assets/Things/editor", "c0000000000000000000000000000001")
	p.Asset("Assets/Things/editor/tool.png", "c0000000000000000000000000000002", "png")
	// 겹친 폴더 항목 : 아래 폴더가 따로 항목이면 그쪽이 펼친다
	p.Folder("Assets/Things/sub", gSub)
	p.Asset("Assets/Things/sub/s.png", gSubFile, "png")
	p.Project.File(dataDir+"AssetGroups/G.asset", testproj.Group("G", []testproj.Entry{
		{GUID: gFolder, Address: "Things", Labels: []string{"x"}},
		{GUID: gOwn, Address: "own-entry"},
		{GUID: gSub, Address: "SubFolder"},
		{GUID: "3a000000000000000000000000000001", Address: "Data"},
	}, []string{"5c000000000000000000000000000001"}, ""))
	// ④ settings 폴더 아래 : settings 폴더 자체를 폴더 항목으로 넣어도 아래는 안 들어간다
	p.Folder("Assets/AddressableAssetsData", "3a000000000000000000000000000001")

	r := build(t, p.Root)
	got := strings.Join(addresses(r.Index), "\n")
	want := strings.Join([]string{
		"SubFolder/s.png",
		"Things/a.png",
		"Things/c.asmref",
		"Things/deep/b.wav",
		"own-entry",
	}, "\n")
	if got != want {
		t.Fatalf("펼친 주소\n%s\n--- 기대 ---\n%s", got, want)
	}
	for _, e := range r.Index.Entries {
		switch e.Address {
		case "Things/a.png":
			if e.FromFolder != "Things" || e.Group != "G" || len(e.Labels) != 1 || e.Kind != "image" || !e.IncludeInBuild {
				t.Fatalf("물려받기 = %+v", e)
			}
		case "own-entry":
			if e.FromFolder != "" {
				t.Fatalf("따로 있는 항목은 fromFolder 가 없다 = %+v", e)
			}
		case "SubFolder/s.png":
			if e.FromFolder != "SubFolder" {
				t.Fatalf("아래 폴더 항목 = %+v", e)
			}
		}
	}
}

// I4 : 스프라이트. Multiple 은 sub 둘 · Single 은 없음 · .psb 는 시트가 적혀 있어도 sub 없음 · Multiple 0칸은 sub 없음.
func TestSpriteSub(t *testing.T) {
	p := newProj(t, []testproj.Entry{
		{GUID: "9a8b7c6d5e4f30211203f4e5d6c7b8a9", Address: "multi"},
		{GUID: "11111111111111111111111111111111", Address: "single"},
		{GUID: "22222222222222222222222222222222", Address: "layers"},
		{GUID: "33333333333333333333333333333333", Address: "empty"},
	}, nil)
	sheet, err := os.ReadFile(filepath.Join("..", "..", "Testdata", "project", "Assets", "Art", "icons.png.meta"))
	if err != nil {
		t.Fatal(err)
	}
	p.File("Assets/m.png", "png")
	p.File("Assets/m.png.meta", string(sheet))
	p.File("Assets/s.png", "png")
	p.File("Assets/s.png.meta", "fileFormatVersion: 2\nguid: 11111111111111111111111111111111\nTextureImporter:\n  spriteMode: 1\n  spriteSheet:\n    sprites: []\n")
	// .psb 에 TextureImporter 꼴 시트를 그대로 적어도 sub 를 안 만드는지 본다 (루트 키로 우연히 빠지는 것이 아니게).
	p.File("Assets/l.psb", "psb")
	p.File("Assets/l.psb.meta", strings.Replace(string(sheet), "9a8b7c6d5e4f30211203f4e5d6c7b8a9", "22222222222222222222222222222222", 1))
	p.File("Assets/e.png", "png")
	p.File("Assets/e.png.meta", "fileFormatVersion: 2\nguid: 33333333333333333333333333333333\nTextureImporter:\n  spriteMode: 2\n  spriteSheet:\n    serializedVersion: 2\n    sprites: []\n")

	r := build(t, p.Root)
	byAddr := map[string]Entry{}
	for _, e := range r.Index.Entries {
		byAddr[e.Address] = e
	}
	m := byAddr["multi"]
	if len(m.Sub) != 2 || m.Sub[0].Name != "icon_sword" || m.Sub[0].Rect != (Rect{X: 0, Y: 64, W: 64, H: 64}) ||
		m.Sub[1].Name != "icon_potion" || m.Sub[1].Rect != (Rect{X: 64, Y: 64, W: 64, H: 64}) {
		t.Fatalf("Multiple = %+v", m.Sub)
	}
	for _, a := range []string{"single", "layers", "empty"} {
		if byAddr[a].Sub != nil {
			t.Errorf("%s 는 sub 가 없어야 한다 = %+v", a, byAddr[a].Sub)
		}
	}
	if byAddr["layers"].Kind != "image" || byAddr["layers"].Path != "Assets/l.psb" {
		t.Fatalf("psb = %+v", byAddr["layers"])
	}
	data, _ := Encode(r.Index)
	if strings.Count(string(data), `"sub"`) != 1 {
		t.Fatalf("sub 칸은 multi 하나에만 있어야 한다\n%s", data)
	}
}

// 시트를 못 읽은 .meta 는 색인이 그 에셋을 쓸 때만 종료 4 다 (설계 4-2).
func TestBrokenSheetOnlyFailsWhenUsed(t *testing.T) {
	broken := "fileFormatVersion: 2\nguid: 44444444444444444444444444444444\nTextureImporter:\n  spriteMode: 2\n  spriteSheet:\n    sprites:\n    - name: |\n        x\n"
	p := newProj(t, nil, nil)
	p.File("Assets/b.png", "png")
	p.File("Assets/b.png.meta", broken)
	build(t, p.Root)

	used := newProj(t, []testproj.Entry{{GUID: "44444444444444444444444444444444", Address: "b"}}, nil)
	used.File("Assets/b.png", "png")
	used.File("Assets/b.png.meta", broken)
	_, err := Build(Options{Root: used.Root, Out: DefaultOut(used.Root)})
	if err == nil || !strings.Contains(err.Error(), "Assets/b.png.meta") {
		t.Fatalf("쓰는 에셋이면 오류여야 한다 = %v", err)
	}
}

func notice(t *testing.T, notices []string, words ...string) {
	t.Helper()
	if len(notices) != 1 {
		t.Fatalf("알림 한 줄이어야 한다 = %v", notices)
	}
	for _, w := range words {
		if !strings.Contains(notices[0], w) {
			t.Fatalf("알림에 %q 가 없다 = %q", w, notices[0])
		}
	}
}

// I6 : 4-5 표 줄마다 (종료 4 줄은 cmd 시험이 파일까지 본다).
func TestNoticesAndFailures(t *testing.T) {
	t.Run("항목 guid 가 표에 없음 → 빈 path·other 로 넣고 알림", func(t *testing.T) {
		p := newProj(t, []testproj.Entry{{GUID: "deaddeaddeaddeaddeaddeaddeaddead", Address: "pkg"}}, nil)
		r := build(t, p.Root)
		e := r.Index.Entries[0]
		if e.Path != "" || e.Kind != "other" {
			t.Fatalf("%+v", e)
		}
		notice(t, r.Notices, "deaddeaddeaddeaddeaddeaddeaddead", `"pkg"`, "G.asset")
	})
	t.Run("특수 표시는 알림 없이 건너뜀", func(t *testing.T) {
		p := newProj(t, []testproj.Entry{{GUID: "EditorSceneList", Address: "EditorSceneList"}, {GUID: "Resources", Address: "Resources"}}, nil)
		r := build(t, p.Root)
		if len(r.Index.Entries) != 0 || len(r.Notices) != 0 {
			t.Fatalf("%+v %v", r.Index.Entries, r.Notices)
		}
	})
	t.Run("같은 address 두 항목 → 둘 다 넣고 알림", func(t *testing.T) {
		p := newProj(t, []testproj.Entry{{GUID: "a2000000000000000000000000000002", Address: "Dup"}, {GUID: "a1000000000000000000000000000001", Address: "Dup"}}, nil)
		p.Asset("Assets/x.png", "a2000000000000000000000000000002", "")
		p.Asset("Assets/y.png", "a1000000000000000000000000000001", "")
		r := build(t, p.Root)
		if len(r.Index.Entries) != 2 {
			t.Fatalf("%+v", r.Index.Entries)
		}
		notice(t, r.Notices, `"Dup"`, "2 개")
		// I7 : address 가 같으면 guid 차례
		if r.Index.Entries[0].GUID != "a1000000000000000000000000000001" {
			t.Fatalf("guid 차례 = %+v", r.Index.Entries)
		}
	})
	t.Run(".meta guid 겹침 → 알림, 먼저 본 것", func(t *testing.T) {
		p := newProj(t, []testproj.Entry{{GUID: "a3000000000000000000000000000003", Address: "x"}}, nil)
		p.Asset("Assets/a.png", "a3000000000000000000000000000003", "")
		p.Asset("Assets/b.png", "a3000000000000000000000000000003", "")
		r := build(t, p.Root)
		if r.Index.Entries[0].Path != "Assets/a.png" {
			t.Fatalf("%+v", r.Index.Entries)
		}
		notice(t, r.Notices, "Assets/b.png.meta", "a3000000000000000000000000000003", "Assets/a.png")
	})
	t.Run("지운 그룹 → 알림, 건너뜀", func(t *testing.T) {
		p := newProj(t, nil, nil)
		p.File(dataDir+"AddressableAssetSettings.asset", testproj.Settings([]string{"6d000000000000000000000000000001", "6d0000000000000000000000000000ff"}, nil))
		r := build(t, p.Root)
		if r.Groups != 1 {
			t.Fatalf("%d", r.Groups)
		}
		notice(t, r.Notices, "6d0000000000000000000000000000ff", "지운 그룹")
	})
	t.Run("m_Address 칸이 없음 → 빈 address 로 넣고 알림", func(t *testing.T) {
		p := newProj(t, nil, nil)
		p.Asset("Assets/x.png", "a4000000000000000000000000000004", "")
		p.File(dataDir+"AssetGroups/G.asset", "--- !u!114 &1\nMonoBehaviour:\n  m_GroupName: G\n  m_SerializeEntries:\n  - m_GUID: a4000000000000000000000000000004\n    m_SerializedLabels: []\n  m_SchemaSet:\n    m_Schemas:\n    - {fileID: 11400000, guid: 5c000000000000000000000000000001, type: 2}\n")
		r := build(t, p.Root)
		if len(r.Index.Entries) != 1 || r.Index.Entries[0].Address != "" || r.Index.Entries[0].Path != "Assets/x.png" {
			t.Fatalf("%+v", r.Index.Entries)
		}
		notice(t, r.Notices, "m_Address", "a4000000000000000000000000000004", "G.asset")
	})
	t.Run("표에 없는 스키마 둘 → 알림 한 줄에 둘 다", func(t *testing.T) {
		p := newProj(t, nil, nil)
		p.File(dataDir+"AssetGroups/G.asset", testproj.Group("G", nil, []string{"5c0000000000000000000000000000aa", "5c0000000000000000000000000000bb"}, ""))
		r := build(t, p.Root)
		notice(t, r.Notices, "5c0000000000000000000000000000aa", "5c0000000000000000000000000000bb", " · ", "true 로 둔다")
	})

	fails := map[string]struct {
		breakIt func(p *proj)
		words   []string
	}{
		"settings 없음": {func(p *proj) {
			os.Remove(filepath.Join(p.Root, filepath.FromSlash(dataDir+"AddressableAssetSettings.asset")))
		}, []string{"settings", "AddressableAssetSettings.asset"}},
		"그룹 칸 모르는 꼴": {func(p *proj) {
			p.File(dataDir+"AssetGroups/G.asset", "--- !u!114 &1\nMonoBehaviour:\n  m_GroupName: |\n    G\n  m_SerializeEntries: []\n")
		}, []string{"AssetGroups/G.asset", "줄 3", "모르는 꼴"}},
		"m_SerializeEntries 없음": {func(p *proj) {
			p.File(dataDir+"AssetGroups/G.asset", "--- !u!114 &1\nMonoBehaviour:\n  m_GroupName: G\n")
		}, []string{"AssetGroups/G.asset", "m_SerializeEntries"}},
		"m_GroupAssets 없음": {func(p *proj) {
			p.File(dataDir+"AddressableAssetSettings.asset", "--- !u!114 &1\nMonoBehaviour:\n  m_Name: S\n")
		}, []string{"AddressableAssetSettings.asset", "m_GroupAssets"}},
	}
	for name, c := range fails {
		t.Run(name+" → 오류", func(t *testing.T) {
			p := newProj(t, nil, nil)
			c.breakIt(p)
			_, err := Build(Options{Root: p.Root, Out: DefaultOut(p.Root)})
			if err == nil {
				t.Fatalf("오류여야 한다")
			}
			for _, w := range c.words {
				if !strings.Contains(err.Error(), w) {
					t.Fatalf("오류 글에 %q 가 없다 = %q", w, err)
				}
			}
		})
	}
	t.Run("오류 때도 그때까지의 알림을 돌려준다", func(t *testing.T) {
		p := newProj(t, nil, nil)
		p.File(dataDir+"AddressableAssetSettings.asset", testproj.Settings([]string{"6d0000000000000000000000000000ff", "6d000000000000000000000000000001"}, nil))
		p.File(dataDir+"AssetGroups/G.asset", "--- !u!114 &1\nMonoBehaviour:\n  m_GroupName: G\n")
		r, err := Build(Options{Root: p.Root, Out: DefaultOut(p.Root)})
		if err == nil || r == nil {
			t.Fatalf("오류와 Result 둘 다 와야 한다 = %v %v", r, err)
		}
		notice(t, r.Notices, "6d0000000000000000000000000000ff")
	})
	t.Run("뿌리가 폴더가 아님 → 오류", func(t *testing.T) {
		if _, err := Build(Options{Root: filepath.Join(t.TempDir(), "none")}); err == nil || !strings.Contains(err.Error(), "none") {
			t.Fatalf("오류여야 한다 = %v", err)
		}
	})
}

// I7 : 같은 입력 두 번 → 같은 바이트 (시각 칸 빼고).
func TestDeterministic(t *testing.T) {
	root := filepath.Join("..", "..", "Testdata", "project")
	a := build(t, root)
	b := build(t, root)
	pin(a.Index)
	pin(b.Index)
	x, _ := Encode(a.Index)
	y, _ := Encode(b.Index)
	if !bytes.Equal(x, y) {
		t.Fatalf("두 번 돌린 결과가 다르다")
	}
}

func TestKindOf(t *testing.T) {
	cases := map[string]string{
		"Assets/a.PNG": "image", "Assets/a.psb": "image", "Assets/a.hdr": "image",
		"Assets/a.Wav": "audio", "Assets/a.s3m": "audio",
		"Assets/a.prefab": "prefab", "Assets/a.unity": "scene",
		"Assets/a.asset": "other", "Assets/a.fbx": "other", "Assets/noext": "other", "": "other",
	}
	for p, want := range cases {
		if got := KindOf(p); got != want {
			t.Errorf("KindOf(%q) = %q, %q 이어야 한다", p, got, want)
		}
	}
}

func TestUnityRootFor(t *testing.T) {
	root := t.TempDir()
	if got, abs := UnityRootFor(DefaultOut(root), root); got != "../.." || abs {
		t.Fatalf("기본 자리 = %q %v", got, abs)
	}
	if got, _ := UnityRootFor(filepath.Join(root, "out", "x.json"), root); got != ".." {
		t.Fatalf("다른 자리 = %q", got)
	}
	if got, _ := UnityRootFor(filepath.Join(filepath.Dir(root), "x.json"), root); got != filepath.Base(root) {
		t.Fatalf("부모 자리 = %q", got)
	}
	if runtime.GOOS == "windows" {
		got, abs := UnityRootFor(`Q:\elsewhere\x.json`, `C:\proj`)
		if got != "C:/proj" || !abs {
			t.Fatalf("다른 드라이브 = %q %v", got, abs)
		}
	}
}

func TestBuildNoticesOtherDrive(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("드라이브는 Windows 만")
	}
	p := newProj(t, nil, nil)
	r, err := Build(Options{Root: p.Root, Out: `Q:\x\address-index.json`})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Index.UnityRoot, ":") || len(r.Notices) != 1 {
		t.Fatalf("%q %v", r.Index.UnityRoot, r.Notices)
	}
}

func TestWriteFileReplaces(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "sub", "address-index.json")
	if err := WriteFile(out, []byte("one\n")); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(out, []byte("two\n")); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(out)
	if string(got) != "two\n" {
		t.Fatalf("덮어쓰기 = %q", got)
	}
	left, _ := os.ReadDir(filepath.Dir(out))
	if len(left) != 1 {
		t.Fatalf("tmp 가 남았다 = %v", left)
	}
}

func TestCheckOut(t *testing.T) {
	root := t.TempDir()
	for _, bad := range []string{"Assets/x.json", "ProjectSettings/x.json", "Packages/x.json", "assets/x.json"} {
		if err := CheckOut(root, filepath.Join(root, filepath.FromSlash(bad))); err == nil {
			t.Errorf("%s 에 쓰면 안 된다", bad)
		}
	}
	if err := CheckOut(root, DefaultOut(root)); err != nil {
		t.Errorf("기본 자리 = %v", err)
	}
}

func TestSourceMtime(t *testing.T) {
	p := newProj(t, nil, nil)
	newest := time.Date(2030, 1, 2, 3, 4, 5, 600, time.UTC)
	path := filepath.Join(p.Root, filepath.FromSlash(dataDir+"AssetGroups/G.asset"))
	if err := os.Chtimes(path, newest, newest); err != nil {
		t.Fatal(err)
	}
	r := build(t, p.Root)
	if r.Index.SourceMtime != "2030-01-02T03:04:05.0000006Z" {
		t.Fatalf("sourceMtime = %q", r.Index.SourceMtime)
	}
}

// 뿌리 밖 junction 이 뿌리의 Assets 를 가리키면, 그 길로 쓰는 --out 도 막는다.
func TestCheckOutThroughJunction(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junction 은 Windows 만")
	}
	base := t.TempDir()
	root := filepath.Join(base, "proj")
	os.MkdirAll(filepath.Join(root, "Assets"), 0o755)
	os.MkdirAll(filepath.Join(root, "Library"), 0o755)
	link := filepath.Join(base, "j")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, filepath.Join(root, "Assets")).CombinedOutput(); err != nil {
		t.Skipf("junction 을 못 만든다: %v %s", err, out)
	}
	defer exec.Command("cmd", "/c", "rmdir", link).Run()

	if err := CheckOut(root, filepath.Join(link, "x.json")); err == nil {
		t.Fatal("junction 너머 Assets 에 쓰면 안 된다")
	}
	if err := CheckOut(root, filepath.Join(link, "sub", "new", "x.json")); err == nil {
		t.Fatal("junction 너머 없는 하위 폴더도 Assets 안이다")
	}

	// 뿌리 자체를 junction 으로 넘겨도 같다.
	rootLink := filepath.Join(base, "rj")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", rootLink, root).CombinedOutput(); err != nil {
		t.Skipf("junction 을 못 만든다: %v %s", err, out)
	}
	defer exec.Command("cmd", "/c", "rmdir", rootLink).Run()
	if err := CheckOut(rootLink, filepath.Join(root, "Assets", "x.json")); err == nil {
		t.Fatal("뿌리가 junction 이어도 Assets 는 막는다")
	}
	if err := CheckOut(rootLink, DefaultOut(rootLink)); err != nil {
		t.Fatalf("기본 자리 = %v", err)
	}
}

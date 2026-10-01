// Package index 는 색인 계약(계약 버전 1)을 아는 유일한 곳이다. 꼴·kind·폴더 펼치기·정렬·쓰기를 한다.
// 계약은 README 「경계 계약」 절, 규칙은 Docs/Design/2026-09-23-index설계.md 4-3·4-5.
package index

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/MiruSona/officina-asset-tool/internal/addr"
	"github.com/MiruSona/officina-asset-tool/internal/meta"
)

// ContractVersion 은 색인 계약 버전이다. 칸을 빼거나 뜻을 바꾸면 올린다.
const ContractVersion = 1

// Index 는 address-index.json 한 장이다. 칸 차례가 곧 JSON 칸 차례다.
type Index struct {
	Version      int      `json:"version"`
	Generator    string   `json:"generator"`
	GeneratedAt  string   `json:"generatedAt"`
	UnityRoot    string   `json:"unityRoot"`
	SettingsPath string   `json:"settingsPath"`
	SourceMtime  string   `json:"sourceMtime"`
	Labels       []string `json:"labels"`
	Entries      []Entry  `json:"entries"`
}

// Entry 는 entries[] 한 칸이다.
type Entry struct {
	Address        string   `json:"address"`
	GUID           string   `json:"guid"`
	Path           string   `json:"path"`
	Kind           string   `json:"kind"`
	Group          string   `json:"group"`
	IncludeInBuild bool     `json:"includeInBuild"`
	Labels         []string `json:"labels"`
	FromFolder     string   `json:"fromFolder,omitempty"`
	Sub            []Sub    `json:"sub,omitempty"`
}

// Sub 는 스프라이트 시트 안 한 칸이다.
type Sub struct {
	Name string `json:"name"`
	Rect Rect   `json:"rect"`
}

// Rect 는 픽셀 단위, y 는 아래에서 잰다.
type Rect struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	H float64 `json:"h"`
}

// Options 는 Build 에 넘기는 값이다. Now 가 0 이면 지금 시각을 쓴다.
type Options struct {
	Root      string
	Out       string
	Generator string
	Now       time.Time
}

// Result 는 만든 색인과 읽은 그룹 수, 알림이다.
type Result struct {
	Index   *Index
	Groups  int
	Notices []string
}

// DefaultOut 은 --out 을 안 줬을 때의 자리다.
func DefaultOut(root string) string {
	return filepath.Join(root, "Library", "AssetTool", "address-index.json")
}

var kinds = map[string]string{}

func init() {
	table := map[string]string{
		"image":  "png jpg jpeg gif bmp tga psd psb tif tiff exr hdr",
		"audio":  "wav mp3 ogg aif aiff flac xm mod it s3m",
		"prefab": "prefab",
		"scene":  "unity",
	}
	for kind, exts := range table {
		for _, ext := range strings.Fields(exts) {
			kinds[ext] = kind
		}
	}
}

// KindOf 는 확장자(소문자로 견준다)로 kind 다섯 중 하나를 준다. 빈 경로는 other.
func KindOf(p string) string {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(p), "."))
	if k, ok := kinds[ext]; ok {
		return k
	}
	return "other"
}

// UnityRootFor 는 색인 파일 폴더 기준 Unity 뿌리를 준다. 상대경로를 못 만들면(다른 드라이브) 절대 경로와 true.
func UnityRootFor(out, root string) (string, bool) {
	absOut, err1 := filepath.Abs(out)
	absRoot, err2 := filepath.Abs(root)
	if err1 != nil || err2 != nil {
		return filepath.ToSlash(root), true
	}
	rel, err := filepath.Rel(filepath.Dir(absOut), absRoot)
	if err != nil {
		return filepath.ToSlash(absRoot), true
	}
	return filepath.ToSlash(rel), false
}

// CheckOut 은 --out 이 Unity 가 읽는 폴더 안이면 막는다. Unity 파일은 한 글자도 안 쓴다.
// 중간 폴더가 링크·junction 이어도 빠져나가지 못하게 양쪽을 실제 경로로 풀어 견준다.
func CheckOut(root, out string) error {
	absOut, err := resolvePath(out)
	if err != nil {
		return err
	}
	absRoot, err := resolvePath(root)
	if err != nil {
		return err
	}
	rel, err := filepath.Rel(absRoot, absOut)
	if err != nil {
		return nil
	}
	first := strings.Split(filepath.ToSlash(rel), "/")[0]
	for _, guarded := range []string{"Assets", "ProjectSettings", "Packages"} {
		if strings.EqualFold(first, guarded) {
			return fmt.Errorf("--out 이 Unity 의 %s 폴더 안이다. Unity 파일 자리에는 쓰지 않는다", guarded)
		}
	}
	return nil
}

// Build 는 Unity 뿌리를 읽어 색인을 만든다. 오류면 색인을 쓰면 안 된다 (종료 4).
// 오류 때도 그때까지 모은 알림은 Result.Notices 에 담아 준다.
func Build(opts Options) (*Result, error) {
	info, err := os.Stat(opts.Root)
	if err != nil {
		return nil, fmt.Errorf("Unity 뿌리를 못 연다: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("Unity 뿌리가 폴더가 아니다: %s", opts.Root)
	}

	table, notices, err := meta.Scan(opts.Root)
	if err != nil {
		return &Result{Notices: notices}, fmt.Errorf(".meta 를 못 읽었다: %w", err)
	}
	settingsPath, n, err := addr.FindSettings(opts.Root, table)
	notices = append(notices, n...)
	if err != nil {
		return &Result{Notices: notices}, err
	}
	settings, n, err := addr.Load(opts.Root, settingsPath, table)
	notices = append(notices, n...)
	if err != nil {
		return &Result{Notices: notices}, err
	}

	entries, n, err := buildEntries(opts.Root, settings, table)
	notices = append(notices, n...)
	if err != nil {
		return &Result{Notices: notices}, err
	}

	mtime, err := sourceMtime(opts.Root, path.Dir(settingsPath))
	if err != nil {
		return &Result{Notices: notices}, err
	}

	unityRoot, abs := UnityRootFor(opts.Out, opts.Root)
	if abs {
		notices = append(notices, fmt.Sprintf("--out 이 Unity 뿌리와 다른 드라이브라 unityRoot 에 절대 경로(%s)를 적는다. DataTool 은 이 색인을 열지 않는다", unityRoot))
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}
	idx := &Index{
		Version:      ContractVersion,
		Generator:    opts.Generator,
		GeneratedAt:  now.UTC().Format(time.RFC3339Nano),
		UnityRoot:    unityRoot,
		SettingsPath: settingsPath,
		SourceMtime:  mtime.UTC().Format(time.RFC3339Nano),
		Labels:       settings.Labels,
		Entries:      entries,
	}
	return &Result{Index: idx, Groups: len(settings.Groups), Notices: notices}, nil
}

func buildEntries(root string, s *addr.Settings, table *meta.Table) ([]Entry, []string, error) {
	var notices []string
	explicit := map[string]bool{}
	for _, g := range s.Groups {
		for _, e := range g.Entries {
			explicit[strings.ToLower(e.GUID)] = true
		}
	}
	ex := expander{root: root, table: table, explicit: explicit, settingsDir: path.Dir(s.Path) + "/"}

	entries := []Entry{}
	for _, g := range s.Groups {
		for _, e := range g.Entries {
			base := Entry{Address: e.Address, GUID: e.GUID, Group: g.Name, IncludeInBuild: g.IncludeInBuild, Labels: e.Labels, Kind: "other"}
			a, ok := table.ByGUID(e.GUID)
			if !ok {
				notices = append(notices, fmt.Sprintf("%s : 항목 %q 의 guid %s 가 .meta 표에 없어 path 를 비운다 (지운 에셋이나 Packages/ 안 에셋)", g.Path, e.Address, e.GUID))
				entries = append(entries, base)
				continue
			}
			if !a.IsDir {
				filled, err := fill(base, a)
				if err != nil {
					return nil, notices, err
				}
				entries = append(entries, filled)
				continue
			}
			expanded, err := ex.expand(base, a)
			if err != nil {
				return nil, notices, err
			}
			entries = append(entries, expanded...)
		}
	}

	sort.SliceStable(entries, func(i, j int) bool {
		if entries[i].Address != entries[j].Address {
			return entries[i].Address < entries[j].Address
		}
		return entries[i].GUID < entries[j].GUID
	})
	for i := 0; i < len(entries); {
		j := i + 1
		for j < len(entries) && entries[j].Address == entries[i].Address {
			j++
		}
		if j-i > 1 {
			notices = append(notices, fmt.Sprintf("address %q 가 항목 %d 개에 있다. 둘 다 넣는다", entries[i].Address, j-i))
		}
		i = j
	}
	return entries, notices, nil
}

// fill 은 경로를 푼 항목에 path·kind·sub 를 채운다. 시트를 못 읽은 에셋이면 여기서 오류다.
func fill(e Entry, a *meta.Asset) (Entry, error) {
	e.GUID = a.GUID
	e.Path = a.Path
	e.Kind = KindOf(a.Path)
	// .psb(PSD Importer)는 1차엔 sub 를 안 만든다 (설계 4-3). 시트 칸도 안 읽으니 그 오류도 안 올린다.
	if strings.EqualFold(path.Ext(a.Path), ".psb") {
		return e, nil
	}
	if a.SheetErr != nil {
		return e, a.SheetErr
	}
	if a.SpriteSheet && e.Kind == "image" && len(a.Sprites) > 0 {
		for _, s := range a.Sprites {
			e.Sub = append(e.Sub, Sub{Name: s.Name, Rect: Rect{X: s.X, Y: s.Y, W: s.W, H: s.H}})
		}
	}
	return e, nil
}

var skippedExts = map[string]bool{".cs": true, ".js": true, ".boo": true, ".exe": true, ".dll": true, ".meta": true, ".preset": true, ".asmdef": true}

type expander struct {
	root        string
	table       *meta.Table
	explicit    map[string]bool
	settingsDir string
}

// expand 는 폴더 항목을 그 아래 파일들로 펼친다. 뺄 것 ①~④ 와 겹침 규칙은 설계 4-3.
func (x expander) expand(folder Entry, a *meta.Asset) ([]Entry, error) {
	var out []Entry
	top := filepath.Join(x.root, filepath.FromSlash(a.Path))
	err := filepath.WalkDir(top, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if p == top {
			return nil
		}
		rel, err := filepath.Rel(x.root, p)
		if err != nil {
			return err
		}
		full := filepath.ToSlash(rel)
		child, hasMeta := x.table.ByPath(full)
		if d.IsDir() {
			// .meta 없는 폴더(. 이름 · ~ 이름)와 따로 항목이 있는 아래 폴더는 들어가지 않는다.
			if !hasMeta || x.explicit[child.GUID] {
				return filepath.SkipDir
			}
			return nil
		}
		if !hasMeta || x.explicit[child.GUID] {
			return nil
		}
		if skippedExts[strings.ToLower(path.Ext(full))] {
			return nil
		}
		if strings.Contains(strings.ToLower(full), "/editor/") || strings.HasPrefix(full, x.settingsDir) {
			return nil
		}
		e := folder
		e.Address = folder.Address + "/" + strings.TrimPrefix(full, a.Path+"/")
		e.FromFolder = folder.Address
		filled, err := fill(e, child)
		if err != nil {
			return err
		}
		out = append(out, filled)
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("폴더 항목 %s 를 못 펼쳤다: %w", a.Path, err)
	}
	return out, nil
}

// sourceMtime 은 settings 폴더 아래 *.asset 중 가장 새 mtime 이다.
func sourceMtime(root, dir string) (time.Time, error) {
	var newest time.Time
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(dir)), func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.EqualFold(filepath.Ext(p), ".asset") {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return err
		}
		if info.ModTime().After(newest) {
			newest = info.ModTime()
		}
		return nil
	})
	if err != nil {
		return newest, fmt.Errorf("settings 폴더를 못 훑었다: %w", err)
	}
	return newest, nil
}

// Encode 는 계약 「쓰기 꼴」 대로 적는다 — 2칸 들여쓰기 · HTML 이스케이프 안 함 · 끝에 \n 하나.
func Encode(idx *Index) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(idx); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// WriteFile 은 같은 폴더의 tmp 에 쓰고 이름을 바꾼다. 실패하면 옛 파일은 그대로 남는다.
func WriteFile(out string, data []byte) error {
	dir := filepath.Dir(out)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, filepath.Base(out)+".tmp-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	_, werr := f.Write(data)
	serr := f.Sync()
	cerr := f.Close()
	if err := errors.Join(werr, serr, cerr); err != nil {
		os.Remove(tmp)
		return err
	}
	// Windows 에서도 os.Rename 은 있는 파일을 덮어쓴다. 먼저 지우면 실패 때 옛 파일까지 잃는다.
	if err := os.Rename(tmp, out); err != nil {
		os.Remove(tmp)
		return err
	}
	return nil
}

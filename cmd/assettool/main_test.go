package main

import (
	"bytes"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func runCmd(args ...string) (int, string, string) {
	var out, errOut bytes.Buffer
	code := run(args, &out, &errOut)
	return code, out.String(), errOut.String()
}

// copyProject 는 시험 프로젝트를 임시 폴더로 옮긴다. Testdata 안에 Library 가 생기지 않게.
func copyProject(t *testing.T) string {
	t.Helper()
	src := filepath.Join("..", "..", "Testdata", "project")
	dst := t.TempDir()
	err := filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
	return dst
}

func TestUsageErrors(t *testing.T) {
	cases := [][]string{
		{"index"},
		{"index", "--unity"},
		{"index", "--unity", "x", "--bogus"},
		{"nope"},
	}
	for _, args := range cases {
		if code, _, _ := runCmd(args...); code != exitUsage {
			t.Errorf("%v → 종료 %d, 1 이어야 한다", args, code)
		}
	}
	if code, out, _ := runCmd(); code != exitOK || !strings.Contains(out, "assettool index") {
		t.Errorf("사용법 = %d %q", code, out)
	}
}

func TestIndexDefaultOut(t *testing.T) {
	root := copyProject(t)
	code, out, errOut := runCmd("index", "--unity", root)
	if code != exitOK {
		t.Fatalf("종료 %d : %s", code, errOut)
	}
	if errOut != "" {
		t.Fatalf("알림이 없어야 한다 = %q", errOut)
	}
	written := filepath.Join(root, "Library", "AssetTool", "address-index.json")
	data, err := os.ReadFile(written)
	if err != nil {
		t.Fatal(err)
	}
	var idx map[string]any
	if err := json.Unmarshal(data, &idx); err != nil {
		t.Fatal(err)
	}
	if idx["unityRoot"] != "../.." || idx["version"] != float64(1) {
		t.Fatalf("색인 = %v", idx)
	}
	if !strings.HasPrefix(idx["generator"].(string), "assettool ") {
		t.Fatalf("generator = %v", idx["generator"])
	}
	if !strings.Contains(out, written) {
		t.Fatalf("결과 줄 = %q", out)
	}
}

func TestIndexJSONAndOtherOut(t *testing.T) {
	root := copyProject(t)
	out := filepath.Join(root, "elsewhere", "idx.json")
	code, stdout, stderr := runCmd("index", "--unity", root, "--out", out, "--json")
	if code != exitOK || stderr != "" {
		t.Fatalf("종료 %d : %s", code, stderr)
	}
	var res struct {
		OK     bool   `json:"ok"`
		Out    string `json:"out"`
		Counts struct {
			Entries int `json:"entries"`
			Groups  int `json:"groups"`
		} `json:"counts"`
		Notices []string `json:"notices"`
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatalf("%v : %q", err, stdout)
	}
	if !res.OK || res.Counts.Entries != 3 || res.Counts.Groups != 3 || res.Notices == nil {
		t.Fatalf("결과 = %+v", res)
	}
	data, _ := os.ReadFile(out)
	if !strings.Contains(string(data), `"unityRoot": ".."`) {
		t.Fatalf("다른 자리 unityRoot = %s", data)
	}
}

// I6 : 종료 4 면 색인 파일이 생기지 않는다. 옛 파일도 그대로 남는다.
func TestReadFailureWritesNothing(t *testing.T) {
	root := copyProject(t)
	out := filepath.Join(root, "Library", "AssetTool", "address-index.json")
	if code, _, _ := runCmd("index", "--unity", root); code != exitOK {
		t.Fatalf("첫 판 실패")
	}
	old, _ := os.ReadFile(out)
	os.Remove(filepath.Join(root, "Assets", "AddressableAssetsData", "AssetGroups", "Audio.asset"))
	os.WriteFile(filepath.Join(root, "Assets", "AddressableAssetsData", "AssetGroups", "UI.asset"), []byte("--- !u!114 &1\nMonoBehaviour:\n  m_GroupName: UI\n"), 0o644)

	code, _, stderr := runCmd("index", "--unity", root)
	if code != exitRead || !strings.Contains(stderr, "m_SerializeEntries") {
		t.Fatalf("종료 %d : %s", code, stderr)
	}
	// 종료 4 때도 그때까지 모은 알림을 싣는다 (Audio.asset 을 지워 짝 없는 .meta 알림이 먼저 나온다).
	if !strings.Contains(stderr, "알림: ") || !strings.Contains(stderr, "Audio.asset.meta") {
		t.Fatalf("종료 4 알림이 없다 : %s", stderr)
	}
	now, _ := os.ReadFile(out)
	if !bytes.Equal(old, now) {
		t.Fatalf("옛 색인이 바뀌었다")
	}

	fresh := filepath.Join(t.TempDir(), "new.json")
	code, stdout, _ := runCmd("index", "--unity", root, "--out", fresh, "--json")
	if code != exitRead || !strings.Contains(stdout, `"ok":false`) {
		t.Fatalf("종료 %d : %s", code, stdout)
	}
	if strings.Contains(stdout, `"notices":[]`) || !strings.Contains(stdout, "Audio.asset.meta") {
		t.Fatalf("--json 종료 4 에 알림이 없다 : %s", stdout)
	}
	if _, err := os.Stat(fresh); !os.IsNotExist(err) {
		t.Fatalf("종료 4 인데 파일이 생겼다")
	}
	left, _ := os.ReadDir(filepath.Dir(fresh))
	if len(left) != 0 {
		t.Fatalf("tmp 가 남았다 = %v", left)
	}
}

func TestMissingRootIsReadFailure(t *testing.T) {
	if code, _, _ := runCmd("index", "--unity", filepath.Join(t.TempDir(), "none")); code != exitRead {
		t.Fatalf("종료 %d", code)
	}
}

func TestWriteFailure(t *testing.T) {
	root := copyProject(t)
	dir := filepath.Join(t.TempDir(), "isdir")
	os.MkdirAll(filepath.Join(dir, "x"), 0o755)
	if code, _, _ := runCmd("index", "--unity", root, "--out", dir); code != exitWrite {
		t.Fatalf("폴더 자리에 쓰면 종료 5 = %d", code)
	}
}

func TestRefuseOutInsideAssets(t *testing.T) {
	root := copyProject(t)
	out := filepath.Join(root, "Assets", "idx.json")
	if code, _, _ := runCmd("index", "--unity", root, "--out", out); code != exitUsage {
		t.Fatalf("Assets 안 --out 은 종료 1 = %d", code)
	}
	if _, err := os.Stat(out); !os.IsNotExist(err) {
		t.Fatalf("파일이 생겼다")
	}
}

func TestNoticesGoToStderr(t *testing.T) {
	root := copyProject(t)
	os.Remove(filepath.Join(root, "ProjectSettings", "EditorBuildSettings.asset"))
	code, _, stderr := runCmd("index", "--unity", root)
	if code != exitOK || !strings.HasPrefix(stderr, "알림: ") || strings.Count(stderr, "\n") != 1 {
		t.Fatalf("종료 %d : %q", code, stderr)
	}
}

func TestVersion(t *testing.T) {
	code, out, _ := runCmd("version")
	if code != exitOK || !strings.HasPrefix(out, "assettool ") {
		t.Fatalf("%d %q", code, out)
	}
	code, out, _ = runCmd("version", "--json")
	if code != exitOK || !strings.Contains(out, `"buildCommit"`) {
		t.Fatalf("%d %q", code, out)
	}
}

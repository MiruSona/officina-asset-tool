package atlas

import (
	"encoding/binary"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/MiruSona/officina-asset-tool/internal/meta"
)

// 뿌리 밖을 가리키는 junction 너머 png 는 머리를 읽지 않는다 (w·h 0). 뿌리 안 png 는 읽는다.
func TestPNGHeadStaysInsideRoot(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("junction 은 Windows 만 (심볼릭 링크는 권한이 있어야 만든다)")
	}
	head := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\x0dIHDR")
	head = binary.BigEndian.AppendUint32(head, 20)
	head = binary.BigEndian.AppendUint32(head, 10)

	base := t.TempDir()
	root := filepath.Join(base, "proj")
	outside := filepath.Join(base, "outside")
	os.MkdirAll(filepath.Join(root, "Assets"), 0o755)
	os.MkdirAll(outside, 0o755)
	os.WriteFile(filepath.Join(outside, "x.png"), head, 0o644)
	os.WriteFile(filepath.Join(root, "Assets", "in.png"), head, 0o644)
	link := filepath.Join(root, "Assets", "J")
	if out, err := exec.Command("cmd", "/c", "mklink", "/J", link, outside).CombinedOutput(); err != nil {
		t.Skipf("junction 을 못 만든다: %v %s", err, out)
	}
	defer exec.Command("cmd", "/c", "rmdir", link).Run()

	files, err := os.OpenRoot(root)
	if err != nil {
		t.Fatal(err)
	}
	defer files.Close()
	r := &reader{root: root, files: files}

	in := r.textureSprites(&meta.Asset{Path: "Assets/in.png", SpriteMode: 1})
	if in[0].W != 20 || in[0].H != 10 {
		t.Fatalf("뿌리 안 png = %+v", in[0])
	}
	out := r.textureSprites(&meta.Asset{Path: "Assets/J/x.png", SpriteMode: 1})
	if out[0].W != 0 || out[0].H != 0 || out[0].Name != "x" {
		t.Fatalf("뿌리 밖 png 를 읽었다 = %+v", out[0])
	}
}

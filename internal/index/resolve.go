package index

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const maxLinkHops = 40

// resolvePath 는 있는 칸까지 링크와 Windows junction 을 풀어 실제 경로를 준다. 없는 칸부터는 그대로 붙인다.
// Go 1.23+ 의 filepath.EvalSymlinks 는 Windows 에서 junction 을 안 풀어서 칸마다 Lstat 으로 본다.
func resolvePath(p string) (string, error) {
	return resolveHops(p, 0)
}

func resolveHops(p string, hops int) (string, error) {
	if hops > maxLinkHops {
		return "", errors.New("링크가 너무 깊다: " + p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	vol := filepath.VolumeName(abs)
	sep := string(filepath.Separator)
	parts := strings.Split(strings.TrimPrefix(abs[len(vol):], sep), sep)
	cur := vol + sep

	for i, part := range parts {
		if part == "" {
			continue
		}
		next := filepath.Join(cur, part)
		info, err := os.Lstat(next)
		if err != nil {
			return filepath.Join(append([]string{cur}, parts[i:]...)...), nil
		}
		// junction 은 ModeIrregular 로 보인다. Readlink 가 안 되는 reparse point 는 보통 폴더로 둔다.
		if info.Mode()&(os.ModeSymlink|os.ModeIrregular) == 0 {
			cur = next
			continue
		}
		target, err := os.Readlink(next)
		if err != nil {
			cur = next
			continue
		}
		target = strings.TrimPrefix(target, `\??\`)
		if !filepath.IsAbs(target) {
			target = filepath.Join(cur, target)
		}
		return resolveHops(filepath.Join(append([]string{target}, parts[i+1:]...)...), hops+1)
	}
	return cur, nil
}

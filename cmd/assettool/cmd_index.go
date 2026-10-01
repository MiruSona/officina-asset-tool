package main

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"

	"github.com/MiruSona/officina-asset-tool/internal/index"
)

type indexArgs struct {
	unity string
	out   string
	json  bool
}

func parseIndexArgs(args []string) (indexArgs, error) {
	var a indexArgs
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--json":
			a.json = true
		case arg == "--unity" || arg == "--out":
			if i+1 >= len(args) {
				return a, fmt.Errorf("%s 뒤에 값이 없다", arg)
			}
			i++
			if arg == "--unity" {
				a.unity = args[i]
			} else {
				a.out = args[i]
			}
		case strings.HasPrefix(arg, "--unity="):
			a.unity = strings.TrimPrefix(arg, "--unity=")
		case strings.HasPrefix(arg, "--out="):
			a.out = strings.TrimPrefix(arg, "--out=")
		default:
			return a, fmt.Errorf("모르는 인자다: %q", arg)
		}
	}
	if a.unity == "" {
		return a, fmt.Errorf("--unity <Unity 뿌리> 가 없다")
	}
	return a, nil
}

func cmdIndex(args []string, stdout, stderr io.Writer) int {
	a, err := parseIndexArgs(args)
	if err != nil {
		return indexFail(a, stdout, stderr, exitUsage, err.Error()+"\n\n"+usage(), nil)
	}
	out := a.out
	if out == "" {
		out = index.DefaultOut(a.unity)
	}
	if abs, err := filepath.Abs(out); err == nil {
		out = abs
	}
	if err := index.CheckOut(a.unity, out); err != nil {
		return indexFail(a, stdout, stderr, exitUsage, err.Error(), nil)
	}

	res, err := index.Build(index.Options{Root: a.unity, Out: out, Generator: generator()})
	if err != nil {
		var notices []string
		if res != nil {
			notices = res.Notices
		}
		return indexFail(a, stdout, stderr, exitRead, "색인을 안 썼다: "+err.Error(), notices)
	}
	data, err := index.Encode(res.Index)
	if err != nil {
		return indexFail(a, stdout, stderr, exitWrite, "색인을 못 적었다: "+err.Error(), res.Notices)
	}
	if err := index.WriteFile(out, data); err != nil {
		return indexFail(a, stdout, stderr, exitWrite, "색인을 못 썼다: "+err.Error(), res.Notices)
	}

	if a.json {
		notices := res.Notices
		if notices == nil {
			notices = []string{}
		}
		printJSON(stdout, stderr, map[string]any{
			"ok":      true,
			"out":     out,
			"counts":  map[string]int{"entries": len(res.Index.Entries), "groups": res.Groups},
			"notices": notices,
		})
		return exitOK
	}
	printNotices(stderr, res.Notices)
	fmt.Fprintf(stdout, "색인을 썼다: %s (항목 %d · 그룹 %d)\n", out, len(res.Index.Entries), res.Groups)
	return exitOK
}

func printNotices(w io.Writer, notices []string) {
	for _, n := range notices {
		fmt.Fprintln(w, "알림: "+n)
	}
}

// indexFail 은 오류를 사람 눈(stderr) 또는 --json(stdout) 으로 내고 종료 코드를 준다.
func indexFail(a indexArgs, stdout, stderr io.Writer, code int, msg string, notices []string) int {
	if a.json {
		if notices == nil {
			notices = []string{}
		}
		printJSON(stdout, stderr, map[string]any{"ok": false, "exit": code, "error": msg, "notices": notices})
		return code
	}
	printNotices(stderr, notices)
	fmt.Fprintln(stderr, msg)
	return code
}

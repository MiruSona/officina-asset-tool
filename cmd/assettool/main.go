// assettool 은 Unity Addressables 설정을 읽어 address 색인 한 장을 쓰는 툴이다.
// 이 파일은 인자를 가르고 결과를 찍기만 한다. 설계는 Docs/Design/2026-09-23-index설계.md.
package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// 종료 코드 (설계 4-1).
const (
	exitOK    = 0 // 성공 (알림이 있어도)
	exitUsage = 1 // 사용법 잘못
	exitRead  = 4 // 읽기 실패·모르는 꼴 — 색인을 안 쓴다
	exitWrite = 5 // 쓰기 실패
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func usage() string {
	return `assettool — Unity Addressables 색인 툴

  assettool index --unity <Unity 뿌리> [--out <파일>] [--json]
      Addressables 설정과 .meta 를 읽어 address-index.json 을 쓴다.
      --out 기본은 <Unity 뿌리>/Library/AssetTool/address-index.json
  assettool version [--json]

종료 : 0 성공 · 1 사용법 · 4 읽기 실패(색인 안 씀) · 5 쓰기 실패
`
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		fmt.Fprint(stdout, usage())
		return exitOK
	}
	switch args[0] {
	case "index":
		return cmdIndex(args[1:], stdout, stderr)
	case "version":
		return cmdVersion(args[1:], stdout, stderr)
	}
	fmt.Fprintf(stderr, "모르는 명령이다: %q\n\n%s", args[0], usage())
	return exitUsage
}

func printJSON(w, errOut io.Writer, v any) {
	out, err := json.Marshal(v)
	if err != nil {
		fmt.Fprintln(errOut, "결과를 JSON 으로 못 적었다:", err)
		return
	}
	fmt.Fprintln(w, string(out))
}

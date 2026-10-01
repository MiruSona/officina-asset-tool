package main

import (
	"fmt"
	"io"
)

// 빌드할 때 -ldflags 로 박는다. 안 박으면 개발판이다.
var (
	version     = "0.1.0"
	buildTime   = "미상"
	buildCommit = ""
)

// bin 은 git 에 안 올라가니, 실행 파일이 어느 소스로 빌드됐는지 스스로 알려야 한다.
func commitLabel() string {
	if buildCommit == "" {
		return "dev"
	}
	return buildCommit
}

// generator 는 색인 generator 칸 값이다 (`assettool 0.1.0 (a1b2c3d)`).
func generator() string {
	return fmt.Sprintf("assettool %s (%s)", version, commitLabel())
}

func cmdVersion(args []string, stdout, stderr io.Writer) int {
	asJSON := false
	for _, a := range args {
		if a != "--json" {
			fmt.Fprintf(stderr, "모르는 인자다: %q\n", a)
			return exitUsage
		}
		asJSON = true
	}
	if asJSON {
		printJSON(stdout, stderr, map[string]any{
			"ok":          true,
			"version":     version,
			"buildCommit": commitLabel(),
			"buildTime":   buildTime,
		})
		return exitOK
	}
	fmt.Fprintf(stdout, "assettool %s (커밋 %s · 빌드 %s)\n", version, commitLabel(), buildTime)
	return exitOK
}

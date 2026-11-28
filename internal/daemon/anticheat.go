package daemon

import (
	"path/filepath"
	"strconv"
	"strings"

	"hyla/internal/proto"
)

var skipPatterns = []string{
	"t.Skip(",
	"t.Skipf(",
	"t.SkipNow(",
	".skip(",
	"xit(",
	"xdescribe(",
	"@pytest.mark.skip",
	"#[ignore]",
}

var assertionPatterns = []string{
	"func Test",
	"t.Fatal",
	"t.Error",
	"assert.",
	"expect(",
	"it(",
	"test(",
}

func isTestFilePath(path string) bool {
	base := filepath.Base(path)
	return strings.HasSuffix(base, "_test.go") ||
		strings.Contains(base, ".test.") ||
		strings.Contains(base, ".spec.") ||
		strings.HasPrefix(base, "test_") ||
		strings.HasSuffix(base, "_test.py") ||
		strings.HasSuffix(base, "_test.rs")
}

func isCheckConfigPath(path string) bool {
	p := filepath.ToSlash(path)
	base := filepath.Base(p)
	if strings.HasPrefix(p, ".github/workflows/") || strings.Contains(p, "/.github/workflows/") {
		return true
	}
	if strings.HasPrefix(base, ".golangci") || strings.Contains(base, "eslint") || base == "biome.json" || base == "tsconfig.json" {
		return true
	}
	return false
}

func containsAny(s string, patterns []string) bool {
	for _, pat := range patterns {
		if strings.Contains(s, pat) {
			return true
		}
	}
	return false
}

// InspectDiffForCheating scans a unified git diff for suspicious changes such
// as deleted test assertions, added test skips, or tampered CI/lint configs.
func InspectDiffForCheating(diffText string) []proto.AntiCheatFinding {
	var findings []proto.AntiCheatFinding
	curFile := ""
	curLine := 0
	addedAssertions := map[string]int{}
	type removedAssert struct {
		line    int
		snippet string
	}
	removedAssertions := map[string][]removedAssert{}
	flaggedConfig := map[string]bool{}

	for _, rawLine := range strings.Split(diffText, "\n") {
		switch {
		case strings.HasPrefix(rawLine, "+++ b/"):
			curFile = strings.TrimSpace(strings.TrimPrefix(rawLine, "+++ b/"))
			curLine = 0
			continue
		case strings.HasPrefix(rawLine, "+++ /dev/null"):
			curFile = ""
			continue
		case strings.HasPrefix(rawLine, "diff --git "):
			curLine = 0
			continue
		case strings.HasPrefix(rawLine, "@@ "):
			// @@ -oldStart,oldCount +newStart,newCount @@
			if plusIdx := strings.Index(rawLine, "+"); plusIdx >= 0 {
				rest := rawLine[plusIdx+1:]
				if end := strings.IndexAny(rest, ", "); end >= 0 {
					rest = rest[:end]
				}
				if n, err := strconv.Atoi(rest); err == nil {
					curLine = n
				}
			}
			continue
		case strings.HasPrefix(rawLine, "--- "):
			continue
		}

		if curFile == "" {
			continue
		}

		if strings.HasPrefix(rawLine, "+") {
			code := rawLine[1:]
			if containsAny(code, skipPatterns) {
				findings = append(findings, proto.AntiCheatFinding{
					Kind:    "test_skip",
					File:    curFile,
					Line:    curLine,
					Snippet: strings.TrimSpace(code),
					Reason:  "added test skip",
				})
			}
			if isTestFilePath(curFile) && containsAny(code, assertionPatterns) {
				addedAssertions[curFile]++
			}
			if !flaggedConfig[curFile] {
				if isCheckConfigPath(curFile) || (filepath.Base(curFile) == "package.json" && strings.Contains(code, `"scripts"`)) {
					flaggedConfig[curFile] = true
					findings = append(findings, proto.AntiCheatFinding{
						Kind:    "config_tamper",
						File:    curFile,
						Line:    curLine,
						Snippet: strings.TrimSpace(code),
						Reason:  "modified CI, lint, or test script configuration",
					})
				}
			}
			curLine++
		} else if strings.HasPrefix(rawLine, "-") {
			code := rawLine[1:]
			if isTestFilePath(curFile) && containsAny(code, assertionPatterns) {
				removedAssertions[curFile] = append(removedAssertions[curFile], removedAssert{
					line:    curLine,
					snippet: strings.TrimSpace(code),
				})
			}
			if !flaggedConfig[curFile] {
				if isCheckConfigPath(curFile) || (filepath.Base(curFile) == "package.json" && strings.Contains(code, `"scripts"`)) {
					flaggedConfig[curFile] = true
					findings = append(findings, proto.AntiCheatFinding{
						Kind:    "config_tamper",
						File:    curFile,
						Line:    curLine,
						Snippet: strings.TrimSpace(code),
						Reason:  "modified CI, lint, or test script configuration",
					})
				}
			}
		} else if strings.HasPrefix(rawLine, " ") {
			curLine++
		}
	}

	for file, removed := range removedAssertions {
		added := addedAssertions[file]
		if len(removed) > added {
			excess := len(removed) - added
			for i := 0; i < excess; i++ {
				r := removed[i]
				findings = append(findings, proto.AntiCheatFinding{
					Kind:    "deleted_test",
					File:    file,
					Line:    r.line,
					Snippet: r.snippet,
					Reason:  "removed test or assertion without replacement",
				})
			}
		}
	}
	return findings
}

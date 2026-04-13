package contract

import (
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"
)

type StalenessWarning struct {
	Code    string
	Message string
	Hint    string
	Details map[string]string
}

type BuildMeta struct {
	Revision string
	Time     time.Time
}

func CheckStaleness(binaryName string) *StalenessWarning {
	meta := currentBuildMeta()
	repoRoot := strings.TrimSpace(SourceRepo)
	if repoRoot == "" {
		return checkAge(binaryName, meta.Revision)
	}
	if _, err := os.Stat(repoRoot); err != nil {
		return checkAge(binaryName, meta.Revision)
	}

	if warning := checkShadow(binaryName, repoRoot); warning != nil {
		return warning
	}

	repoCommit := readGitHEAD(repoRoot)
	if warning := compareCommits(binaryName, repoRoot, meta.Revision, repoCommit); warning != nil {
		return warning
	}

	if meta.Revision == "" || repoCommit == "" {
		return checkAge(binaryName, meta.Revision)
	}
	return nil
}

func currentBuildMeta() BuildMeta {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return BuildMeta{}
	}
	var meta BuildMeta
	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			meta.Revision = setting.Value
		case "vcs.time":
			parsed, err := time.Parse(time.RFC3339, setting.Value)
			if err == nil {
				meta.Time = parsed
			}
		}
	}
	return meta
}

func checkShadow(binaryName, repoRoot string) *StalenessWarning {
	self, err := os.Executable()
	if err != nil {
		return nil
	}
	selfInfo, err := os.Stat(self)
	if err != nil {
		return nil
	}

	localBin := filepath.Join(repoRoot, "bin", binaryName)
	localInfo, err := os.Stat(localBin)
	if err != nil || localInfo.IsDir() {
		return nil
	}

	if os.SameFile(selfInfo, localInfo) {
		return nil
	}

	if localInfo.ModTime().After(selfInfo.ModTime()) {
		return &StalenessWarning{
			Code: WarningStaleBinary,
			Message: fmt.Sprintf(
				"%s: running older installed binary, but a newer local build exists",
				binaryName,
			),
			Hint: fmt.Sprintf("Run: make install  (in %s)", repoRoot),
			Details: map[string]string{
				"installed_binary": self,
				"local_binary":     localBin,
				"source_repo":      repoRoot,
			},
		}
	}
	return nil
}

func compareCommits(binaryName, repoRoot, buildCommit, repoCommit string) *StalenessWarning {
	if buildCommit == "" || repoCommit == "" {
		return nil
	}
	if strings.HasPrefix(repoCommit, buildCommit) || strings.HasPrefix(buildCommit, repoCommit) {
		return nil
	}
	return &StalenessWarning{
		Code: WarningStaleBinary,
		Message: fmt.Sprintf(
			"%s: built from %s but repo is at %s",
			binaryName,
			short(buildCommit),
			short(repoCommit),
		),
		Hint: fmt.Sprintf("Run: make install  (in %s)", repoRoot),
		Details: map[string]string{
			"build_commit": buildCommit,
			"repo_commit":  repoCommit,
			"source_repo":  repoRoot,
		},
	}
}

func checkAge(binaryName, buildRevision string) *StalenessWarning {
	self, err := os.Executable()
	if err != nil {
		return nil
	}
	info, err := os.Stat(self)
	if err != nil {
		return nil
	}
	age := time.Since(info.ModTime())
	if age < 24*time.Hour {
		return nil
	}

	days := int(age.Hours() / 24)
	label := "day"
	if days != 1 {
		label = "days"
	}

	return &StalenessWarning{
		Code: WarningStaleBinary,
		Message: fmt.Sprintf(
			"%s: built %d %s ago (%s)",
			binaryName,
			days,
			label,
			short(buildRevision),
		),
		Hint: "Run: make install",
		Details: map[string]string{
			"built_at": info.ModTime().Format(time.RFC3339),
		},
	}
}

func readGitHEAD(repoRoot string) string {
	gitDir := resolveGitDir(repoRoot)
	if gitDir == "" {
		return ""
	}

	headPath := filepath.Join(gitDir, "HEAD")
	data, err := os.ReadFile(headPath)
	if err != nil {
		return ""
	}
	content := strings.TrimSpace(string(data))
	if strings.HasPrefix(content, "ref: ") {
		ref := strings.TrimPrefix(content, "ref: ")
		refPath := filepath.Join(gitDir, ref)
		refData, err := os.ReadFile(refPath)
		if err != nil {
			return readPackedRef(gitDir, ref)
		}
		return strings.TrimSpace(string(refData))
	}
	if isHexHash(content) {
		return content
	}
	return ""
}

func resolveGitDir(repoRoot string) string {
	gitPath := filepath.Join(repoRoot, ".git")
	info, err := os.Stat(gitPath)
	if err != nil {
		return ""
	}
	if info.IsDir() {
		return gitPath
	}

	data, err := os.ReadFile(gitPath)
	if err != nil {
		return ""
	}
	text := strings.TrimSpace(string(data))
	if !strings.HasPrefix(text, "gitdir: ") {
		return ""
	}
	target := strings.TrimSpace(strings.TrimPrefix(text, "gitdir: "))
	if filepath.IsAbs(target) {
		return filepath.Clean(target)
	}
	return filepath.Clean(filepath.Join(repoRoot, target))
}

func readPackedRef(gitDir, ref string) string {
	data, err := os.ReadFile(filepath.Join(gitDir, "packed-refs"))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") || strings.HasPrefix(line, "^") {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) == 2 && parts[1] == ref {
			return parts[0]
		}
	}
	return ""
}

func isHexHash(value string) bool {
	if len(value) < 7 {
		return false
	}
	if len(value)%2 != 0 {
		value = "0" + value
	}
	_, err := hex.DecodeString(value)
	return err == nil
}

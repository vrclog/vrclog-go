package logfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

var ErrNoLogFiles = errors.New("no log files found")

var ErrTooManyLogFiles = errors.New("too many log files")

var ErrNoLogDirectory = errors.New("no log directory available")

const MaxFollowCandidateFiles = 4096

const filenameTimestampLayout = "2006-01-02_15-04-05"

type LogFileInfo struct {
	Path    string
	ModTime time.Time
	Name    string
}

func DefaultLogDirectory() (string, error) {
	if runtime.GOOS != "windows" {
		return "", fmt.Errorf("%w: auto-detection is only supported on Windows", ErrNoLogDirectory)
	}

	localAppData := os.Getenv("LOCALAPPDATA")
	if localAppData == "" {
		userProfile := os.Getenv("USERPROFILE")
		if userProfile != "" {
			localAppData = filepath.Join(userProfile, "AppData", "Local")
		}
	}

	if localAppData == "" {
		return "", fmt.Errorf("%w: cannot determine LocalAppData path", ErrNoLogDirectory)
	}

	localLow := filepath.Join(filepath.Dir(localAppData), "LocalLow")

	candidates := []string{
		filepath.Join(localLow, "VRChat", "VRChat"),
		filepath.Join(localLow, "VRChat", "vrchat"),
	}

	for _, dir := range candidates {
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			continue
		}
		resolved, err := filepath.EvalSymlinks(dir)
		if err != nil {
			continue
		}
		return resolved, nil
	}

	return "", fmt.Errorf("%w: VRChat log directory not found", ErrNoLogDirectory)
}

func ListLogFiles(dir string) ([]LogFileInfo, error) {
	absDir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	dir = filepath.Clean(absDir)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var matches []string
	for _, entry := range entries {
		matched, err := filepath.Match("output_log_*.txt", entry.Name())
		if err != nil {
			return nil, fmt.Errorf("matching log file name: %w", err)
		}
		if !matched {
			continue
		}
		matches = append(matches, filepath.Join(dir, entry.Name()))
	}

	if len(matches) == 0 {
		return nil, ErrNoLogFiles
	}

	if len(matches) > MaxFollowCandidateFiles {
		return nil, fmt.Errorf("%w: found %d files (limit %d)", ErrTooManyLogFiles, len(matches), MaxFollowCandidateFiles)
	}

	type sortableLogFile struct {
		info         LogFileInfo
		timestamp    time.Time
		hasTimestamp bool
	}
	var files []sortableLogFile
	for _, m := range matches {
		f, info, err := OpenRegular(m)
		if err != nil {
			continue
		}
		f.Close()

		name := filepath.Base(m)
		ts, ok := parseFilenameTimestamp(name)
		files = append(files, sortableLogFile{
			info: LogFileInfo{
				Path:    m,
				ModTime: info.ModTime(),
				Name:    name,
			},
			timestamp:    ts,
			hasTimestamp: ok,
		})
	}

	if len(files) == 0 {
		return nil, ErrNoLogFiles
	}

	sort.SliceStable(files, func(i, j int) bool {
		switch {
		case files[i].hasTimestamp && files[j].hasTimestamp:
			return files[i].timestamp.Before(files[j].timestamp)
		case files[i].hasTimestamp && !files[j].hasTimestamp:
			return true
		case !files[i].hasTimestamp && files[j].hasTimestamp:
			return false
		default:
			if !files[i].info.ModTime.Equal(files[j].info.ModTime) {
				return files[i].info.ModTime.Before(files[j].info.ModTime)
			}
			return files[i].info.Name < files[j].info.Name
		}
	})

	result := make([]LogFileInfo, len(files))
	for i, f := range files {
		result[i] = f.info
	}
	return result, nil
}

func FindLatestLogFile(dir string) (string, error) {
	files, err := ListLogFiles(dir)
	if err != nil {
		return "", err
	}
	return files[len(files)-1].Path, nil
}

func parseFilenameTimestamp(name string) (time.Time, bool) {
	const prefix = "output_log_"
	const suffix = ".txt"

	if !strings.HasPrefix(name, prefix) || !strings.HasSuffix(name, suffix) {
		return time.Time{}, false
	}

	tsStr := name[len(prefix) : len(name)-len(suffix)]
	t, err := time.Parse(filenameTimestampLayout, tsStr)
	if err != nil {
		return time.Time{}, false
	}
	return t, true
}

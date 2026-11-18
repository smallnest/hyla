package daemonhost

import (
	"os"
	"path/filepath"
	"strings"
	"sync"

	"hyla/internal/plugins/manifest"
)

const MaxLogBytes = 4 << 20 // 4 MiB per plugin stderr log file

// rotatingLogWriter appends plugin stderr to logPath and rotates once at 4 MiB.
type rotatingLogWriter struct {
	mu   sync.Mutex
	path string
	file *os.File
	size int64
}

func newRotatingLogWriter(path string) (*rotatingLogWriter, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return nil, err
	}
	st, _ := f.Stat()
	var sz int64
	if st != nil {
		sz = st.Size()
	}
	return &rotatingLogWriter{path: path, file: f, size: sz}, nil
}

func (w *rotatingLogWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return 0, os.ErrClosed
	}
	if w.size+int64(len(p)) > MaxLogBytes {
		_ = w.file.Close()
		_ = os.Remove(w.path + ".1")
		_ = os.Rename(w.path, w.path+".1")
		f, err := os.OpenFile(w.path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		if err != nil {
			w.file = nil
			return 0, err
		}
		w.file = f
		w.size = 0
	}
	n, err := w.file.Write(p)
	w.size += int64(n)
	return n, err
}

func (w *rotatingLogWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		return nil
	}
	err := w.file.Close()
	w.file = nil
	return err
}

// TailLog reads up to the last n lines (or default 200) of a plugin log file.
func TailLog(path string, lines int) (string, int) {
	if lines <= 0 {
		lines = 200
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return "", 0
	}
	rawLines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(rawLines) > lines {
		rawLines = rawLines[len(rawLines)-lines:]
	}
	return strings.Join(rawLines, "\n") + "\n", len(rawLines)
}

// ReadPluginLogs tails plugin stderr log file(s), checking both the primary
// log (<id>.log) and the disambiguated [app] log (<id>-app.log) when both exist.
func ReadPluginLogs(store *manifest.Store, pluginID string, lines int) (string, string, int, error) {
	if store == nil {
		store = manifest.DefaultStore()
	}
	mainPath, err := store.LogPath(pluginID)
	if err != nil {
		return "", "", 0, err
	}
	appPath, err := store.AppLogPath(pluginID)
	if err != nil {
		return "", "", 0, err
	}
	mainTxt, mainN := TailLog(mainPath, lines)
	appTxt, appN := TailLog(appPath, lines)
	switch {
	case mainN > 0 && appN > 0:
		return mainPath, "[daemon]\n" + mainTxt + "\n[app]\n" + appTxt, mainN + appN, nil
	case appN > 0:
		return appPath, appTxt, appN, nil
	default:
		return mainPath, mainTxt, mainN, nil
	}
}

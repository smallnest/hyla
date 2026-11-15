// Package paste takes the clipboard's images, and its files, as the
// paths of files a program takes, for a pane whose terminal cannot carry
// them: a pasted image is saved as a file, and its path pasted instead.
package paste

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/egoist/mygo/transfer"

	"hyla/locales"
)

// Dir is where the images pasted into panes are kept, as files a
// program takes by their paths. A var for tests.
var Dir = func() string { return filepath.Join(os.TempDir(), "hyla-paste") }

// Keep is how long a pasted image's file stays: an agent may come
// back to the path long after the paste.
const Keep = 7 * 24 * time.Hour

// imageTypes are the images a paste saves for a program, by the
// extension of their file: those coding agents take.
var imageTypes = map[transfer.Format]string{
	transfer.PNG: ".png",
	"image/jpeg": ".jpg",
	"image/gif":  ".gif",
	"image/webp": ".webp",
}

// Image takes a paste of the clipboard that holds an image and no
// text, as a screenshot copied to the clipboard does, and pastes the path
// of a file with the image instead, as dropping the file on the pane
// would: coding agents take the path as an image they attach.
//
// The terminal's own paste cannot carry the image to the program. The
// program runs in hylad's terminal, where a Kitty clipboard protocol paste
// event (mode 5522) cannot go, since the frames hylad sends do not carry
// the mode and hylad refuses such reads (ClipboardForward); and a paste of
// an image alone sends nothing, or an empty bracketed paste, which the
// user sees as a paste that did nothing. So every paste of an image ends
// in a path or in a toast that says why not.
//
// It runs on the main thread, as Options.OnPaste does, with what the
// clipboard holds, and reports whether it took the paste: it takes none
// that has text, which pastes as it is, or neither files nor an image.
// Files with no text paste as their paths.
func Image(a App, id string, d transfer.Data) bool {
	t := a.TermFor(id)
	if t == nil {
		return false
	}
	// The terminal asks only for a clipboard that holds something, so
	// nothing here is a clipboard that could not be read, as when it
	// changed meanwhile or an image is too big to convert.
	if len(d.Formats()) == 0 {
		a.Toast(locales.ToastPasteClipboardUnreadable())
		return true
	}
	if text, _ := d.Read(transfer.Text); len(text) > 0 {
		return false
	}
	if files, _ := d.Files(); len(files) > 0 {
		// Files copied in Finder, or an image copied with its file, as
		// some apps do, offer no text: the terminal's paste would send
		// nothing, so their paths go, as a drop of them sends.
		quoted := make([]string, len(files))
		for i, f := range files {
			quoted[i] = Quote(runtime.GOOS, PaneShell(a, id), f)
		}
		t.Paste(strings.Join(quoted, " "))
		return true
	}
	f, ok := imageToSave(d)
	if !ok {
		return false
	}
	ext, ok := imageTypes[f]
	if !ok {
		a.Toast(locales.ToastPasteImageUnsupported(strings.ToUpper(strings.TrimPrefix(string(f), "image/"))))
		return true
	}
	img, err := d.Read(f)
	if err == nil && len(img) == 0 {
		err = errors.New("the image is empty")
	}
	var path string
	if err == nil {
		path, err = saveImage(img, ext)
	}
	if err != nil {
		a.Toast(ImageErrorText(err, Dir()))
		return true
	}
	t.Paste(Quote(runtime.GOOS, PaneShell(a, id), path))
	return true
}

// imageToSave returns the image format of d to paste: PNG if it has it,
// else the first of its image formats.
func imageToSave(d transfer.Data) (transfer.Format, bool) {
	formats := d.Formats()
	if slices.Contains(formats, transfer.PNG) {
		return transfer.PNG, true
	}
	for _, f := range formats {
		if strings.HasPrefix(string(f), "image/") {
			return f, true
		}
	}
	return "", false
}

// imageError says why an image could not be saved for a paste, and
// what the user can do: the toast is all they see, so the system's error
// is told alone only for what is not one of the causes they can mend.
// ImageErrorText says why an image could not be saved for a paste.
func ImageErrorText(err error, dir string) string {
	switch {
	case errors.Is(err, fs.ErrPermission):
		return locales.ToastPasteImageDenied(dir)
	case errors.Is(err, syscall.ENOSPC):
		return locales.ToastPasteImageDiskFull()
	}
	return locales.ToastPasteImageFailed(err.Error())
}

// paneShell returns the name of the shell of a pane, or of the pane in
// front of a session, by id.
func PaneShell(a App, id string) string {
	for i := range a.Sessions() {
		if p := a.Sessions()[i].PaneByID(id); p != nil {
			return p.Shell
		}
	}
	if s := a.Session(id); s != nil {
		if p := s.FocusPane(); p != nil {
			return p.Shell
		}
	}
	return ""
}

// saveImage writes an image to a file of its own, with the extension,
// in pasteDir, which only the user can enter, and returns its path. Images kept longer than
// Keep go.
func saveImage(img []byte, ext string) (string, error) {
	dir := Dir()
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	PruneDir(dir, time.Now().Add(-Keep))
	f, err := os.CreateTemp(dir, "paste-*"+ext)
	if err != nil {
		return "", err
	}
	_, err = f.Write(img)
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}

// prune removes the images of earlier pastes last written before
// the time, and nothing else.
func PruneDir(dir string, before time.Time) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasPrefix(name, "paste-") || !isExt(filepath.Ext(name)) {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(before) {
			os.Remove(filepath.Join(dir, name))
		}
	}
}

// isExt reports whether ext is the extension of a pasted image's file.
func isExt(ext string) bool {
	for _, e := range imageTypes {
		if e == ext {
			return true
		}
	}
	return false
}

// Quote quotes a path for the shell a pane runs, so that the shell
// takes it as one argument whatever it holds: as it is when it holds
// nothing a shell reads otherwise, else in single quotes, in which POSIX
// shells and PowerShell expand nothing. cmd.exe has double quotes only.
// shell is the shell's name or path, "" for the platform's default.
func Quote(goos, shell, path string) string {
	safe := "._-/"
	if goos == "windows" {
		safe += `:\`
	}
	if path != "" && !strings.ContainsFunc(path, func(r rune) bool {
		return !('a' <= r && r <= 'z' || 'A' <= r && r <= 'Z' || '0' <= r && r <= '9' || strings.ContainsRune(safe, r))
	}) {
		return path
	}
	name := strings.ToLower(shell[strings.LastIndexAny(shell, `/\`)+1:])
	name = strings.TrimSuffix(name, ".exe")
	posix := slices.Contains([]string{"sh", "bash", "zsh", "fish", "dash", "ksh", "mksh"}, name)
	switch {
	case name == "cmd" && goos == "windows":
		return `"` + path + `"`
	case name == "pwsh" || name == "powershell" || goos == "windows" && !posix:
		// A single quote is doubled, and so are the curly ones, which
		// PowerShell reads as one.
		var b strings.Builder
		b.WriteByte('\'')
		for _, r := range path {
			b.WriteRune(r)
			if strings.ContainsRune("'‘’‚‛", r) {
				b.WriteRune(r)
			}
		}
		b.WriteByte('\'')
		return b.String()
	}
	return "'" + strings.ReplaceAll(path, "'", `'\''`) + "'"
}

package daemon

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"hyla/internal/settings"
)

// The Program Resume Protocol, OSC 7577: a program tells the terminal
// the command that brings it back, signed with a token the terminal put
// in its environment. The protocol is a hint: a terminal may ignore it,
// and the daemon keeps the record to bring the program back when it
// restores the session.
const (
	resumeTokenEnv = "OSC7577_TOKEN"
	resumeSkew     = 60 // seconds a report's time may differ from the clock
	maxResumeArgv  = 64
)

// resumeRecord is a program's last accepted resume command: the pid that
// owns it, the command, its directory, and when it reported. It is kept
// in state.json, so that a restore resumes it.
type resumeRecord struct {
	PID  int      `json:"pid"`
	Argv []string `json:"argv"`
	Cwd  string   `json:"cwd,omitempty"`
	Time int64    `json:"time,omitempty"`
}

// A report ends in one of three ways: ignored, with no effect; rejected,
// which removes the records of its owner; or accepted.
type resumeAction int

const (
	resumeIgnored resumeAction = iota
	resumeRejected
	resumeAccepted
)

// resumeReport is an OSC 7577 report's parsed body.
type resumeReport struct {
	PID   int
	Argv  []string
	Cwd   string
	Clear bool
	Time  int64
}

var (
	resumeKeyRe   = regexp.MustCompile(`^[a-z]+$`)
	resumeValueRe = regexp.MustCompile(`^[A-Za-z0-9_.,+/=-]*$`)
	resumeNumRe   = regexp.MustCompile(`^[1-9][0-9]*$`)
	resumeMacRe   = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// resumeKeys are the keys this terminal implements, which need may name.
var resumeKeys = map[string]bool{
	"pid": true, "time": true, "argv": true, "cwd": true,
	"clear": true, "need": true, "mac": true,
}

// newResumeToken returns a token for one terminal: 43 characters of
// [A-Za-z0-9_-], 256 bits from the system's random source. On failure it
// returns "", and the pane reports nothing, since no program can sign.
func newResumeToken() string {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(b[:])
}

// parseResume parses the body of an OSC 7577 after "7577;", as the
// protocol orders it: the mac first, then the time, then the rest.
func parseResume(s, token string, now time.Time) (resumeReport, resumeAction) {
	var rep resumeReport
	i := strings.LastIndex(s, ":mac=")
	if i < 0 || !resumeMacRe.MatchString(s[i+5:]) {
		return rep, resumeIgnored
	}
	sum := sha256.Sum256([]byte(s[:i] + ":" + token))
	if subtle.ConstantTimeCompare([]byte(s[i+5:]), []byte(hex.EncodeToString(sum[:]))) != 1 {
		return rep, resumeIgnored
	}

	// The mac proves the sender, so the pairs after it can be taken
	// apart: a malformed one is a rejection, not the raw output an
	// unauthenticated report could be.
	pairs := map[string]string{}
	malformed := false
	for _, pair := range strings.Split(s[:i], ":") {
		k, v, ok := strings.Cut(pair, "=")
		if !ok || k == "mac" || !resumeKeyRe.MatchString(k) || !resumeValueRe.MatchString(v) {
			malformed = true
			continue
		}
		if _, dup := pairs[k]; dup {
			malformed = true
			continue
		}
		pairs[k] = v
	}
	if v, ok := pairs["pid"]; ok && resumeNumRe.MatchString(v) {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			rep.PID = n
		}
	}
	// No time to check is a report ignored, and its owner's records stay.
	tv, ok := pairs["time"]
	if !ok || !resumeNumRe.MatchString(tv) {
		return rep, resumeIgnored
	}
	t, err := strconv.ParseInt(tv, 10, 64)
	if err != nil {
		return rep, resumeIgnored
	}
	if t < now.Unix()-resumeSkew || t > now.Unix()+resumeSkew {
		return rep, resumeIgnored
	}
	rep.Time = t

	// From here a defect rejects the report. One with no usable owner
	// has no records to remove.
	if malformed || rep.PID == 0 {
		return rep, resumeRejected
	}
	if need, ok := pairs["need"]; ok {
		for _, k := range strings.Split(need, ",") {
			if !resumeKeys[k] {
				return rep, resumeRejected
			}
		}
	}
	av, hasArgv := pairs["argv"]
	cv, hasClear := pairs["clear"]
	if hasArgv == hasClear {
		return rep, resumeRejected
	}
	if hasClear {
		if cv != "1" {
			return rep, resumeRejected
		}
		rep.Clear = true
		return rep, resumeAccepted
	}
	els := strings.Split(av, ",")
	if len(els) > maxResumeArgv {
		return rep, resumeRejected
	}
	for _, el := range els {
		b, ok := decodeResumeText(el)
		if !ok {
			return rep, resumeRejected
		}
		rep.Argv = append(rep.Argv, string(b))
	}
	if !resumeCommandOK(rep.Argv[0]) {
		return rep, resumeRejected
	}
	if c, ok := pairs["cwd"]; ok {
		b, ok := decodeResumeText(c)
		if !ok || !resumeAbsPath(string(b)) {
			return rep, resumeRejected
		}
		rep.Cwd = string(b)
	}
	return rep, resumeAccepted
}

// decodeResumeText decodes a report's base64, padded or not, and refuses
// what is not UTF-8 or holds a control character.
func decodeResumeText(s string) ([]byte, bool) {
	b, err := base64.StdEncoding.DecodeString(s)
	if err != nil {
		b, err = base64.RawStdEncoding.DecodeString(s)
	}
	if err != nil || !utf8.Valid(b) || !resumeTextOK(string(b)) {
		return nil, false
	}
	return b, true
}

// resumeTextOK tells text a report may carry: no control characters.
func resumeTextOK(s string) bool {
	for _, c := range s {
		if c < 0x20 || c == 0x7f || c >= 0x80 && c <= 0x9f {
			return false
		}
	}
	return true
}

// resumeAbsPath tells an absolute path, which the protocol requires of
// cwd and of an argv element that names one.
func resumeAbsPath(p string) bool {
	if p == "" {
		return false
	}
	if runtime.GOOS == "windows" {
		return filepath.IsAbs(p)
	}
	return strings.HasPrefix(p, "/")
}

// resumeCommandOK tells a resume command's first element: a bare name
// that does not start with "-", or an absolute path.
func resumeCommandOK(s string) bool {
	if s == "" || strings.HasPrefix(s, "-") {
		return false
	}
	if runtime.GOOS == "windows" {
		if strings.ContainsAny(s, `/\`) {
			return filepath.IsAbs(s)
		}
		return true
	}
	if strings.ContainsRune(s, '/') {
		return strings.HasPrefix(s, "/")
	}
	return true
}

// handleResumeReport acts on an OSC 7577 report; s.mu is held.
func (s *pane) handleResumeReport(body string, now time.Time) {
	rep, act := parseResume(body, s.resumeToken, now)
	switch act {
	case resumeIgnored:
		return
	case resumeRejected:
		if s.forgetResume(rep.PID) {
			s.d.save()
		}
		return
	}
	// A report older than one this owner already accepted is ignored.
	if last, ok := s.resumeTimes[rep.PID]; ok && rep.Time < last {
		return
	}
	if s.resumeTimes == nil {
		s.resumeTimes = map[int]int64{}
	}
	s.resumeTimes[rep.PID] = rep.Time
	if rep.Clear {
		if s.forgetResume(rep.PID) {
			s.d.save()
		}
		return
	}
	// The owner must be a process of this terminal: its shell or what
	// the shell runs. A system whose processes cannot be listed skips
	// this check, as the protocol allows.
	list := psList()
	if len(list) == 0 {
		list = procList()
	}
	if s.shellPID > 0 && len(list) > 0 && !descendant(s.shellPID, list)[rep.PID] {
		if s.forgetResume(rep.PID) {
			s.d.save()
		}
		return
	}
	s.resume = &resumeRecord{PID: rep.PID, Argv: rep.Argv, Cwd: rep.Cwd, Time: rep.Time}
	s.d.save()
}

// forgetResume removes the record of pid, as its clear, its rejection or
// its exit asks, and tells whether it removed one; s.mu is held.
func (s *pane) forgetResume(pid int) bool {
	if pid != 0 && s.resume != nil && s.resume.PID == pid {
		s.resume = nil
		return true
	}
	return false
}

// resumeCommandFor returns the command a restored pane runs, when the
// user allowed program resume: the record's argv in its directory. A
// record whose directory is gone is not resumed, nor is one as root.
func resumeCommandFor(sp savedPane, set settings.Settings) (argv []string, dir string, ok bool) {
	r := sp.Resume
	if r == nil || len(r.Argv) == 0 || !set.ResumePrograms {
		return nil, "", false
	}
	if runtime.GOOS != "windows" && os.Geteuid() == 0 {
		// A program that reported with fewer rights than the user would
		// otherwise run with root's.
		return nil, "", false
	}
	if !resumeAllowed(set.ResumeCommands, r.Argv[0]) {
		return nil, "", false
	}
	// The record came from a verified report, but state.json is read
	// back: a command it could not have carried is not resumed either.
	if !resumeCommandOK(r.Argv[0]) {
		return nil, "", false
	}
	for _, a := range r.Argv {
		if !resumeTextOK(a) {
			return nil, "", false
		}
	}
	if r.Cwd != "" && (!resumeAbsPath(r.Cwd) || !resumeTextOK(r.Cwd)) {
		return nil, "", false
	}
	dir = sp.Dir
	if r.Cwd != "" {
		if st, err := os.Stat(r.Cwd); err != nil || !st.IsDir() {
			return nil, "", false
		}
		dir = r.Cwd
	}
	return r.Argv, dir, true
}

// resumeAllowed tells whether a program may resume: the list is empty for
// any, and names match by name or by absolute path.
func resumeAllowed(allow []string, first string) bool {
	if len(allow) == 0 {
		return true
	}
	base := filepath.Base(first)
	for _, a := range allow {
		if a = strings.TrimSpace(a); a != "" && (a == first || a == base) {
			return true
		}
	}
	return false
}

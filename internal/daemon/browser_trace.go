package daemon

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"hyla/internal/proto"
)

type browserRecording struct {
	Name    string
	URL     string
	Out     string
	Started int64
	Steps   []proto.BrowserFlowStep
}

func isRecordableBrowserOp(op string) bool {
	switch op {
	case "open", "back", "forward", "reload", "click", "type", "wait", "eval", "screenshot":
		return true
	}
	return false
}

func (d *Daemon) recordBrowserStep(paneID string, p proto.BrowserParams) {
	if !isRecordableBrowserOp(p.Op) {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.browserRecs == nil {
		return
	}
	rec := d.browserRecs[paneID]
	if rec == nil {
		return
	}
	if p.Op == "open" && rec.URL == "" {
		rec.URL = p.Text
	}
	rec.Steps = append(rec.Steps, proto.BrowserFlowStep{
		Op:       p.Op,
		Selector: p.Selector,
		Text:     p.Text,
		Enter:    p.Enter,
		Timeout:  p.Timeout,
	})
}

func (d *Daemon) browserRecordStart(p proto.BrowserRecordParams) (proto.BrowserRecordStatus, error) {
	d.mu.Lock()
	pn := d.findPaneLocked(p.Pane)
	if pn == nil {
		if s := d.findSessionLocked(p.Pane); s != nil {
			for _, candidate := range s.panes {
				candidate.mu.Lock()
				isB := candidate.info.Browser()
				candidate.mu.Unlock()
				if isB {
					pn = candidate
					break
				}
			}
		}
	}
	if pn == nil {
		d.mu.Unlock()
		return proto.BrowserRecordStatus{}, fmt.Errorf("no browser pane %q", p.Pane)
	}
	pn.mu.Lock()
	id, isBrowser, curURL := pn.info.ID, pn.info.Browser(), pn.info.URL
	pn.mu.Unlock()
	if !isBrowser {
		d.mu.Unlock()
		return proto.BrowserRecordStatus{}, fmt.Errorf("pane %s is a terminal, not a browser", id)
	}
	name := strings.TrimSpace(p.Name)
	if name == "" {
		name = "flow-" + time.Now().Format("20060102-150405")
	}
	if d.browserRecs == nil {
		d.browserRecs = map[string]*browserRecording{}
	}
	d.browserRecs[id] = &browserRecording{
		Name:    name,
		URL:     curURL,
		Out:     strings.TrimSpace(p.Out),
		Started: time.Now().UnixMilli(),
	}
	d.mu.Unlock()
	return proto.BrowserRecordStatus{
		Pane:      id,
		Recording: true,
		Name:      name,
		Steps:     0,
	}, nil
}

func (d *Daemon) browserRecordStatus(paneID string) (proto.BrowserRecordStatus, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.browserRecs == nil {
		return proto.BrowserRecordStatus{Pane: paneID}, nil
	}
	rec := d.browserRecs[paneID]
	if rec == nil {
		return proto.BrowserRecordStatus{Pane: paneID}, nil
	}
	return proto.BrowserRecordStatus{
		Pane:      paneID,
		Recording: true,
		Name:      rec.Name,
		Steps:     len(rec.Steps),
	}, nil
}

func (d *Daemon) browserRecordStop(p proto.BrowserRecordParams) (proto.BrowserRecordStatus, error) {
	d.mu.Lock()
	if d.browserRecs == nil {
		d.mu.Unlock()
		return proto.BrowserRecordStatus{}, fmt.Errorf("no active recording on pane %q", p.Pane)
	}
	rec := d.browserRecs[p.Pane]
	if rec == nil {
		d.mu.Unlock()
		return proto.BrowserRecordStatus{}, fmt.Errorf("no active recording on pane %q", p.Pane)
	}
	delete(d.browserRecs, p.Pane)
	pn := d.findPaneLocked(p.Pane)
	baseDir := d.dir
	if pn != nil {
		pn.mu.Lock()
		if wd := pn.info.WorkDir(); wd != "" {
			baseDir = wd
		}
		pn.mu.Unlock()
	}
	d.mu.Unlock()

	outPath := strings.TrimSpace(p.Out)
	if outPath == "" {
		outPath = rec.Out
	}
	if outPath == "" {
		outPath = filepath.Join(baseDir, ".hyla", "flows", rec.Name+".json")
	}
	flow := proto.BrowserFlow{
		Name:      rec.Name,
		URL:       rec.URL,
		CreatedAt: rec.Started,
		Steps:     slices.Clone(rec.Steps),
	}
	data, err := json.MarshalIndent(flow, "", "  ")
	if err != nil {
		return proto.BrowserRecordStatus{}, err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return proto.BrowserRecordStatus{}, err
	}
	if err := writeAtomic(outPath, data); err != nil {
		return proto.BrowserRecordStatus{}, err
	}
	return proto.BrowserRecordStatus{
		Pane:      p.Pane,
		Recording: false,
		Name:      rec.Name,
		Steps:     len(flow.Steps),
		SavedPath: outPath,
	}, nil
}

// comparePNGs decodes two PNG images and returns the fraction of differing pixels [0, 1].
func comparePNGs(a, b []byte) (float64, error) {
	imgA, err := png.Decode(bytes.NewReader(a))
	if err != nil {
		return 0, fmt.Errorf("decode png A: %w", err)
	}
	imgB, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return 0, fmt.Errorf("decode png B: %w", err)
	}
	ba, bb := imgA.Bounds(), imgB.Bounds()
	if ba.Dx() != bb.Dx() || ba.Dy() != bb.Dy() {
		return 1.0, nil
	}
	total := ba.Dx() * ba.Dy()
	if total == 0 {
		return 0, nil
	}
	diff := 0
	for y := 0; y < ba.Dy(); y++ {
		for x := 0; x < ba.Dx(); x++ {
			r1, g1, b1, a1 := imgA.At(ba.Min.X+x, ba.Min.Y+y).RGBA()
			r2, g2, b2, a2 := imgB.At(bb.Min.X+x, bb.Min.Y+y).RGBA()
			if r1 != r2 || g1 != g2 || b1 != b2 || a1 != a2 {
				diff++
			}
		}
	}
	return float64(diff) / float64(total), nil
}

func (d *Daemon) browserReplay(c *controlConn, pn *pane, p proto.BrowserReplayParams) (proto.BrowserReplayResult, error) {
	if strings.TrimSpace(p.FlowPath) == "" {
		return proto.BrowserReplayResult{}, errors.New("browser.replay: flowPath is required")
	}
	raw, err := os.ReadFile(p.FlowPath)
	if err != nil {
		return proto.BrowserReplayResult{}, err
	}
	var flow proto.BrowserFlow
	if err := json.Unmarshal(raw, &flow); err != nil {
		return proto.BrowserReplayResult{}, fmt.Errorf("invalid flow JSON: %w", err)
	}

	outDir := strings.TrimSpace(p.OutDir)
	if outDir == "" {
		outDir = filepath.Join(filepath.Dir(p.FlowPath), "artifacts")
	}
	baselinePath := strings.TrimSuffix(p.FlowPath, filepath.Ext(p.FlowPath)) + ".baseline.png"

	startAll := time.Now()
	res := proto.BrowserReplayResult{
		FlowName: flow.Name,
		Passed:   true,
	}

	for i, step := range flow.Steps {
		stepStart := time.Now()
		bp := proto.BrowserParams{
			ID:       pn.info.ID,
			Op:       step.Op,
			Selector: step.Selector,
			Text:     step.Text,
			Enter:    step.Enter,
			Timeout:  step.Timeout,
		}
		rawRes, callErr := d.browse(c, pn, bp)
		sr := proto.BrowserStepResult{
			Index:      i,
			Op:         step.Op,
			OK:         callErr == nil,
			DurationMs: time.Since(stepStart).Milliseconds(),
		}
		if callErr != nil {
			sr.Error = callErr.Error()
			res.Passed = false
			res.Steps = append(res.Steps, sr)
			break
		}
		if step.Op == "screenshot" {
			if br, ok := rawRes.(*proto.BrowserResult); ok && len(br.PNG) > 0 {
				if err := os.MkdirAll(outDir, 0o755); err == nil {
					shotName := fmt.Sprintf("%s-step-%d.png", flow.Name, i+1)
					shotPath := filepath.Join(outDir, shotName)
					if os.WriteFile(shotPath, br.PNG, 0o644) == nil {
						sr.Screenshot = shotPath
						res.Screenshot = shotPath
					}
				}
				if p.UpdateBaseline {
					_ = os.WriteFile(baselinePath, br.PNG, 0o644)
				} else if basePNG, err := os.ReadFile(baselinePath); err == nil {
					if ratio, err := comparePNGs(basePNG, br.PNG); err == nil {
						sr.BaselineDiffRatio = ratio
						res.BaselineDiffRatio = ratio
					}
				}
			}
		}
		res.Steps = append(res.Steps, sr)
	}
	res.DurationMs = time.Since(startAll).Milliseconds()

	if p.FeishuChat != "" {
		if bot := d.feishuBot(); bot != nil {
			statusWord := "PASSED"
			if !res.Passed {
				statusWord = "FAILED"
			}
			summary := fmt.Sprintf("Flow `%s` %s in %dms (%d/%d steps). Baseline diff: %.2f%%.",
				res.FlowName, statusWord, res.DurationMs, len(res.Steps), len(flow.Steps), res.BaselineDiffRatio*100)
			if _, err := bot.SendApprovalCard(p.FeishuChat, "Browser Verification: "+res.FlowName, summary, "", res.Screenshot); err == nil {
				res.FeishuNotified = true
			}
		}
	}
	return res, nil
}

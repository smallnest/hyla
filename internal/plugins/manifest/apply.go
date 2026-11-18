package manifest

import (
	"fmt"
	"os"
	"strings"

	"hyla/internal/actions"
	"hyla/internal/detect"
	"hyla/internal/proto"
)

type MarkAsset struct {
	PluginID  string
	Agent     string
	Path      string
	FullColor bool
}

type ThemeAsset struct {
	PluginID string
	Name     string
	Path     string
}

type WashAsset struct {
	PluginID string
	ID       string
	Name     string
	Path     string
}

type PetAsset struct {
	PluginID string
	ID       string
	Dir      string
}

type HookAsset struct {
	PluginID string
	Agent    string
	Event    string
	Script   string
}

type SkillAsset struct {
	PluginID string
	Agent    string
	Name     string
	Dir      string
}

type PluginActionMeta struct {
	PluginID string
	Action   ActionSpec
	FullID   string
}

// Snapshot holds the resolved base-layer state across all discovered plugins.
type Snapshot struct {
	Plugins       []InstalledPlugin
	Marks         []MarkAsset
	Themes        []ThemeAsset
	Washes        []WashAsset
	Pets          []PetAsset
	Hooks         []HookAsset
	Skills        []SkillAsset
	ActionTargets map[string]PluginActionMeta
	Errors        []string
}

// ApplyBase discovers plugins in s and refreshes all base-layer registries:
// actions, proto agents, and detect manifests. It returns a Snapshot of
// validated asset paths for higher layers (marks, themes, washes, pets, hooks, skills).
func ApplyBase(s *Store) (*Snapshot, error) {
	if s == nil {
		s = DefaultStore()
	}
	plugins, err := s.Discover()
	if err != nil {
		return nil, err
	}

	actions.ClearPluginActions()
	proto.ClearPluginAgents()

	snap := &Snapshot{
		Plugins:       plugins,
		ActionTargets: make(map[string]PluginActionMeta),
	}
	var detectItems []detect.PluginManifest
	var disabledActions []string

	for _, p := range plugins {
		if p.LoadError != "" {
			snap.Errors = append(snap.Errors, fmt.Sprintf("%s: %s", p.Entry.ID, p.LoadError))
		}
		if p.Manifest == nil {
			continue
		}
		m := p.Manifest
		if !p.Entry.Enabled {
			for _, a := range m.Actions {
				disabledActions = append(disabledActions, QualifyActionID(m.ID, a.ID))
			}
			continue
		}

		// Actions
		var acts []actions.Action
		for _, a := range m.Actions {
			qid := QualifyActionID(m.ID, a.ID)
			act := actions.Action{
				ID:        qid,
				Title:     a.Title,
				Menu:      a.Menu,
				Keys:      a.Shortcut,
				Remote:    a.Remote,
				PluginID:  m.ID,
				NoPalette: !a.Palette,
				When:      a.When,
			}
			acts = append(acts, act)
			snap.ActionTargets[qid] = PluginActionMeta{
				PluginID: m.ID,
				Action:   a,
				FullID:   qid,
			}
		}
		actions.SetPluginActions(m.ID, acts)

		// Agents, detect rules, marks
		var agentMetas []proto.AgentMeta
		for _, ag := range m.Agents {
			agentMetas = append(agentMetas, proto.AgentMeta{
				ID:         ag.ID,
				Name:       ag.Name,
				PluginID:   m.ID,
				Timeline:   ag.Timeline,
				Subagents:  ag.Subagents,
				Chat:       ag.Chat,
				ResumeArgv: append([]string(nil), ag.ResumeArgv...),
				Processes:  append([]string(nil), ag.Processes...),
			})
			if ag.Detect != "" && p.RootDir != "" {
				detectPath, err := ResolveWithin(p.RootDir, ag.Detect)
				if err != nil {
					snap.Errors = append(snap.Errors, fmt.Sprintf("%s agent %s detect: %v", m.ID, ag.ID, err))
				} else if data, err := os.ReadFile(detectPath); err != nil {
					snap.Errors = append(snap.Errors, fmt.Sprintf("%s agent %s detect: %v", m.ID, ag.ID, err))
				} else {
					detectItems = append(detectItems, detect.PluginManifest{
						Agent: ag.ID,
						Name:  m.ID + "/" + ag.Detect,
						TOML:  data,
					})
				}
			}
			if ag.Mark != "" && p.RootDir != "" {
				markPath, err := ResolveWithin(p.RootDir, ag.Mark)
				if err != nil {
					snap.Errors = append(snap.Errors, fmt.Sprintf("%s agent %s mark: %v", m.ID, ag.ID, err))
				} else {
					snap.Marks = append(snap.Marks, MarkAsset{
						PluginID:  m.ID,
						Agent:     ag.ID,
						Path:      markPath,
						FullColor: ag.MarkFullColor,
					})
				}
			}
		}
		if len(agentMetas) > 0 {
			proto.SetPluginAgents(m.ID, agentMetas)
		}

		// Themes, washes, pets, hooks, skills
		if p.RootDir != "" {
			for _, th := range m.Themes {
				tp, err := ResolveWithin(p.RootDir, th.File)
				if err != nil {
					snap.Errors = append(snap.Errors, fmt.Sprintf("%s theme %s: %v", m.ID, th.Name, err))
					continue
				}
				snap.Themes = append(snap.Themes, ThemeAsset{
					PluginID: m.ID,
					Name:     th.Name,
					Path:     tp,
				})
			}
			for _, w := range m.Washes {
				wp, err := ResolveWithin(p.RootDir, w.File)
				if err != nil {
					snap.Errors = append(snap.Errors, fmt.Sprintf("%s wash %s: %v", m.ID, w.ID, err))
					continue
				}
				snap.Washes = append(snap.Washes, WashAsset{
					PluginID: m.ID,
					ID:       w.ID,
					Name:     w.Name,
					Path:     wp,
				})
			}
			for _, pet := range m.Pets {
				pd, err := ResolveWithin(p.RootDir, pet.Dir)
				if err != nil {
					snap.Errors = append(snap.Errors, fmt.Sprintf("%s pet %s: %v", m.ID, pet.ID, err))
					continue
				}
				snap.Pets = append(snap.Pets, PetAsset{
					PluginID: m.ID,
					ID:       pet.ID,
					Dir:      pd,
				})
			}
			for _, hk := range m.Hooks {
				hp, err := ResolveWithin(p.RootDir, hk.Script)
				if err != nil {
					snap.Errors = append(snap.Errors, fmt.Sprintf("%s hook %s: %v", m.ID, hk.Event, err))
					continue
				}
				snap.Hooks = append(snap.Hooks, HookAsset{
					PluginID: m.ID,
					Agent:    strings.ToLower(strings.TrimSpace(hk.Agent)),
					Event:    hk.Event,
					Script:   hp,
				})
			}
			for _, sk := range m.Skills {
				sd, err := ResolveWithin(p.RootDir, sk.Dir)
				if err != nil {
					snap.Errors = append(snap.Errors, fmt.Sprintf("%s skill %s: %v", m.ID, sk.Name, err))
					continue
				}
				snap.Skills = append(snap.Skills, SkillAsset{
					PluginID: m.ID,
					Agent:    strings.ToLower(strings.TrimSpace(sk.Agent)),
					Name:     sk.Name,
					Dir:      sd,
				})
			}
		}
	}

	actions.SetDisabledPluginActions(disabledActions)
	if err := detect.ReloadPlugins(detectItems); err != nil {
		snap.Errors = append(snap.Errors, err.Error())
	}
	return snap, nil
}

package petme

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"hyla/internal/pets"
	"hyla/internal/settings"
	"hyla/internal/ui/controls"
	"hyla/internal/ui/theme"

	"hyla/locales"
)

// listPets lists the pets the settings may show, as the pop-up picks
// them: Hyla the frog, the Codex app's active pet, and each pet the Codex
// app has, then those hyla installed. It reads them once, and again when
// a pet is installed, or another chosen.
func listPets(a App) []pets.Builtin {
	l := a.PetListOf()
	if l.Pets != nil && l.For == a.Settings().PetSet && l.Gen == a.Generation() {
		return l.Pets
	}
	l.Pets, l.For, l.Gen = readPets(), a.Settings().PetSet, a.Generation()
	return l.Pets
}

// readPets reads the pets listPets lists, their names from their
// pet.json, not their spritesheets.
func readPets() []pets.Builtin {
	ps := []pets.Builtin{{Name: settings.PetSetHyla, Title: locales.PetName()}}
	codex := pets.CodexIDs()
	if active := pets.CodexActive(); active != "" {
		title := active
		if p, err := pets.NameID(active); err == nil {
			title = p.Name
		}
		ps = append(ps, pets.Builtin{Name: settings.PetSetCodex, Title: title})
		// The active pet is in the Codex app's list too, as the same
		// directory.
		codex = slices.DeleteFunc(codex, func(id string) bool { return id == active })
	}
	installed := pets.Installed()
	for _, dirs := range [][]string{codex, installed} {
		for _, id := range dirs {
			if slices.Contains(installed, id) {
				continue // hyla's comes first
			}
			title := id
			if p, err := pets.NameID(id); err == nil {
				title = p.Name
			}
			ps = append(ps, pets.Builtin{Name: id, Title: title, ID: id})
		}
	}
	return ps
}

// Chooser is the pop-up that chooses the pet on the edge.
func Chooser(a App, c *ui.Context, pal *theme.Palette) {
	ps := listPets(a)
	s := a.Settings()
	names := make([]string, 1, len(ps)+1)
	names[0] = s.PetSet
	titles := make(map[string]string, len(ps))
	for _, p := range ps {
		titles[p.Name] = p.Title
		if p.Name != s.PetSet {
			names = append(names, p.Name)
		}
	}
	t := s.PetSet
	controls.Popup(c, "pet-set", &t, names, func(n string) string {
		if title, ok := titles[n]; ok {
			return title
		}
		return n
	}, 240, pal)
	if t != s.PetSet {
		Set(s, t)
	}
}

// Set changes the pet shown, and shows it.
func Set(s *settings.Settings, name string) {
	s.PetSet = name
	s.Pet = true
}

// Gallery is the pet's setting's detail: what its description says, or
// what went wrong with it.
func Gallery(a App) string {
	name := a.Settings().PetSet
	if name == settings.PetSetHyla {
		return ""
	}
	p, err := pets.NameID(name)
	if err != nil {
		return locales.SetPetMissing(name, err.Error())
	}
	desc := strings.TrimSpace(p.Description)
	// A pet of the same name in both directories: say which one shows.
	if p.ID != settings.PetSetCodex && slices.Contains(pets.CodexIDs(), p.ID) {
		desc += " (" + locales.SetPetHylaCopy() + ")"
	}
	return desc
}

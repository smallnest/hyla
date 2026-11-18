package hylaplugin

// NodeOption customizes a declarative UINode produced by builder helpers.
type NodeOption func(*UINode)

// SelectOption represents a choice in segmented and select controls.
type SelectOption struct {
	Value string `json:"value"`
	Label string `json:"label"`
}

// TextSpan describes one styled fragment inside a RichText node.
type TextSpan struct {
	Text       string  `json:"text"`
	Tone       string  `json:"tone,omitempty"`
	Color      string  `json:"color,omitempty"`
	Bold       bool    `json:"bold,omitempty"`
	Mono       bool    `json:"mono,omitempty"`
	FontSize   float32 `json:"fontSize,omitempty"`
}

func newNode(typ string, props map[string]any, children []*UINode, opts []NodeOption) *UINode {
	n := &UINode{
		Type:     typ,
		Props:    props,
		Children: children,
	}
	for _, opt := range opts {
		if opt != nil {
			opt(n)
		}
	}
	return n
}

func WithID(id string) NodeOption          { return func(n *UINode) { n.ID = id } }
func WithKey(key string) NodeOption        { return func(n *UINode) { n.Key = key } }
func WithStyle(st UIStyle) NodeOption      { return func(n *UINode) { cp := st; n.Style = &cp } }
func WithOnClick(ev string) NodeOption     { return func(n *UINode) { n.OnClick = ev } }
func WithOnChange(ev string) NodeOption    { return func(n *UINode) { n.OnChange = ev } }
func WithOnSubmit(ev string) NodeOption    { return func(n *UINode) { n.OnSubmit = ev } }
func WithProp(k string, v any) NodeOption {
	return func(n *UINode) {
		if n.Props == nil {
			n.Props = map[string]any{}
		}
		n.Props[k] = v
	}
}

func Column(children []*UINode, opts ...NodeOption) *UINode {
	return newNode("column", nil, children, opts)
}

func Row(children []*UINode, opts ...NodeOption) *UINode {
	return newNode("row", nil, children, opts)
}

func Box(children []*UINode, opts ...NodeOption) *UINode {
	return newNode("box", nil, children, opts)
}

func Spacer(opts ...NodeOption) *UINode {
	return newNode("spacer", nil, nil, opts)
}

func Divider(vertical bool, opts ...NodeOption) *UINode {
	var props map[string]any
	if vertical {
		props = map[string]any{"vertical": true}
	}
	return newNode("divider", props, nil, opts)
}

func Scroll(children []*UINode, opts ...NodeOption) *UINode {
	return newNode("scroll", nil, children, opts)
}

func List(dividers bool, children []*UINode, opts ...NodeOption) *UINode {
	return newNode("list", map[string]any{"dividers": dividers}, children, opts)
}

func InfoCard(title, aside string, collapsible bool, children []*UINode, opts ...NodeOption) *UINode {
	return newNode("infoCard", map[string]any{
		"title":       title,
		"aside":       aside,
		"collapsible": collapsible,
	}, children, opts)
}

func SettingsGroup(title, note string, children []*UINode, opts ...NodeOption) *UINode {
	return newNode("settingsGroup", map[string]any{
		"title": title,
		"note":  note,
	}, children, opts)
}

func SettingsRow(label, detail string, control *UINode, opts ...NodeOption) *UINode {
	var children []*UINode
	if control != nil {
		children = []*UINode{control}
	}
	return newNode("settingsRow", map[string]any{
		"label":  label,
		"detail": detail,
	}, children, opts)
}

func Text(text string, opts ...NodeOption) *UINode {
	return newNode("text", map[string]any{"text": text}, nil, opts)
}

func RichText(spans []TextSpan, opts ...NodeOption) *UINode {
	raw := make([]any, 0, len(spans))
	for _, s := range spans {
		m := map[string]any{"text": s.Text}
		if s.Tone != "" {
			m["tone"] = s.Tone
		}
		if s.Color != "" {
			m["color"] = s.Color
		}
		if s.Bold {
			m["bold"] = true
		}
		if s.Mono {
			m["mono"] = true
		}
		if s.FontSize > 0 {
			m["fontSize"] = s.FontSize
		}
		raw = append(raw, m)
	}
	return newNode("richText", map[string]any{"spans": raw}, nil, opts)
}

func Badge(text, tone string, opts ...NodeOption) *UINode {
	return newNode("badge", map[string]any{"text": text, "tone": tone}, nil, opts)
}

func Icon(name string, size float32, opts ...NodeOption) *UINode {
	props := map[string]any{"name": name}
	if size > 0 {
		props["size"] = size
	}
	return newNode("icon", props, nil, opts)
}

func Image(src string, width, height float32, opts ...NodeOption) *UINode {
	props := map[string]any{"src": src}
	if width > 0 {
		props["width"] = width
	}
	if height > 0 {
		props["height"] = height
	}
	return newNode("image", props, nil, opts)
}

func Spinner(label string, opts ...NodeOption) *UINode {
	return newNode("spinner", map[string]any{"label": label}, nil, opts)
}

func Progress(value float64, tone string, opts ...NodeOption) *UINode {
	return newNode("progress", map[string]any{"value": value, "tone": tone}, nil, opts)
}

func Button(id, label, variant string, opts ...NodeOption) *UINode {
	all := append([]NodeOption{WithID(id)}, opts...)
	return newNode("button", map[string]any{"label": label, "variant": variant}, nil, all)
}

func Toggle(id string, value bool, opts ...NodeOption) *UINode {
	all := append([]NodeOption{WithID(id)}, opts...)
	return newNode("toggle", map[string]any{"value": value}, nil, all)
}

func Segmented(id, value string, options []SelectOption, opts ...NodeOption) *UINode {
	all := append([]NodeOption{WithID(id)}, opts...)
	return newNode("segmented", map[string]any{"value": value, "options": encodeOptions(options)}, nil, all)
}

func Select(id, value string, options []SelectOption, opts ...NodeOption) *UINode {
	all := append([]NodeOption{WithID(id)}, opts...)
	return newNode("select", map[string]any{"value": value, "options": encodeOptions(options)}, nil, all)
}

func TextInput(id, value, placeholder string, opts ...NodeOption) *UINode {
	all := append([]NodeOption{WithID(id)}, opts...)
	return newNode("textInput", map[string]any{"value": value, "placeholder": placeholder}, nil, all)
}

func Slider(id string, value, minVal, maxVal float64, opts ...NodeOption) *UINode {
	all := append([]NodeOption{WithID(id)}, opts...)
	return newNode("slider", map[string]any{"value": value, "min": minVal, "max": maxVal}, nil, all)
}

func encodeOptions(options []SelectOption) []any {
	out := make([]any, 0, len(options))
	for _, o := range options {
		out = append(out, map[string]any{"value": o.Value, "label": o.Label})
	}
	return out
}

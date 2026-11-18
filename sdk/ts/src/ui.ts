import type { UINode, UIStyle } from "./types.ts";

export interface BaseUIOpts {
  id?: string;
  key?: string;
  style?: UIStyle;
}

export interface SelectOption {
  value: string;
  label: string;
}

export interface TextSpan {
  text: string;
  tone?: string;
  color?: string;
  bold?: boolean;
  mono?: boolean;
  fontSize?: number;
}

export const ui = {
  column(children: UINode[], opts?: BaseUIOpts): UINode {
    return { type: "column", ...opts, children };
  },
  row(children: UINode[], opts?: BaseUIOpts): UINode {
    return { type: "row", ...opts, children };
  },
  box(children: UINode[], opts?: BaseUIOpts & { center?: boolean }): UINode {
    const { center, ...rest } = opts ?? {};
    return { type: "box", ...rest, props: center ? { center } : undefined, children };
  },
  spacer(opts?: BaseUIOpts): UINode {
    return { type: "spacer", ...opts };
  },
  divider(vertical = false, opts?: BaseUIOpts): UINode {
    return { type: "divider", ...opts, props: vertical ? { vertical: true } : undefined };
  },
  scroll(children: UINode[], opts?: BaseUIOpts): UINode {
    return { type: "scroll", ...opts, children };
  },
  list(children: UINode[], dividers = true, opts?: BaseUIOpts): UINode {
    return { type: "list", ...opts, props: { dividers }, children };
  },
  infoCard(
    title: string,
    children: UINode[],
    opts?: BaseUIOpts & { aside?: string; collapsible?: boolean },
  ): UINode {
    const { aside, collapsible, ...rest } = opts ?? {};
    return {
      type: "infoCard",
      ...rest,
      props: { title, aside, collapsible },
      children,
    };
  },
  settingsGroup(
    title: string,
    children: UINode[],
    opts?: BaseUIOpts & { note?: string },
  ): UINode {
    const { note, ...rest } = opts ?? {};
    return {
      type: "settingsGroup",
      ...rest,
      props: { title, note },
      children,
    };
  },
  settingsRow(
    label: string,
    control: UINode,
    opts?: BaseUIOpts & { detail?: string },
  ): UINode {
    const { detail, ...rest } = opts ?? {};
    return {
      type: "settingsRow",
      ...rest,
      props: { label, detail },
      children: [control],
    };
  },
  text(
    text: string,
    opts?: BaseUIOpts & { tone?: string; wrap?: boolean },
  ): UINode {
    const { tone, wrap, ...rest } = opts ?? {};
    return { type: "text", ...rest, props: { text, tone, wrap } };
  },
  richText(spans: TextSpan[], opts?: BaseUIOpts): UINode {
    return { type: "richText", ...opts, props: { spans } };
  },
  badge(text: string, tone = "neutral", opts?: BaseUIOpts): UINode {
    return { type: "badge", ...opts, props: { text, tone } };
  },
  icon(name: string, size = 14, opts?: BaseUIOpts & { tone?: string }): UINode {
    const { tone, ...rest } = opts ?? {};
    return { type: "icon", ...rest, props: { name, size, tone } };
  },
  image(src: string, width: number, height: number, opts?: BaseUIOpts & { radius?: number }): UINode {
    const { radius, ...rest } = opts ?? {};
    return { type: "image", ...rest, props: { src, width, height, radius } };
  },
  spinner(label = "", opts?: BaseUIOpts): UINode {
    return { type: "spinner", ...opts, props: { label } };
  },
  progress(value: number, tone = "accent", opts?: BaseUIOpts): UINode {
    return { type: "progress", ...opts, props: { value, tone } };
  },
  button(
    id: string,
    label: string,
    opts?: BaseUIOpts & {
      variant?: "primary" | "secondary" | "ghost" | "danger";
      disabled?: boolean;
      onClick?: string;
    },
  ): UINode {
    const { variant = "secondary", disabled, onClick = "click", ...rest } = opts ?? {};
    return {
      type: "button",
      id,
      ...rest,
      onClick,
      props: { label, variant, disabled },
    };
  },
  toggle(
    id: string,
    value: boolean,
    opts?: BaseUIOpts & { onChange?: string },
  ): UINode {
    const { onChange = "change", ...rest } = opts ?? {};
    return { type: "toggle", id, ...rest, onChange, props: { value } };
  },
  segmented(
    id: string,
    value: string,
    options: SelectOption[],
    opts?: BaseUIOpts & { onChange?: string },
  ): UINode {
    const { onChange = "change", ...rest } = opts ?? {};
    return { type: "segmented", id, ...rest, onChange, props: { value, options } };
  },
  select(
    id: string,
    value: string,
    options: SelectOption[],
    opts?: BaseUIOpts & { onChange?: string },
  ): UINode {
    const { onChange = "change", ...rest } = opts ?? {};
    return { type: "select", id, ...rest, onChange, props: { value, options } };
  },
  textInput(
    id: string,
    value: string,
    opts?: BaseUIOpts & { placeholder?: string; onChange?: string; onSubmit?: string },
  ): UINode {
    const { placeholder, onChange = "change", onSubmit = "submit", ...rest } = opts ?? {};
    return {
      type: "textInput",
      id,
      ...rest,
      onChange,
      onSubmit,
      props: { value, placeholder },
    };
  },
  slider(
    id: string,
    value: number,
    min = 0,
    max = 1,
    opts?: BaseUIOpts & { onChange?: string },
  ): UINode {
    const { onChange = "change", ...rest } = opts ?? {};
    return { type: "slider", id, ...rest, onChange, props: { value, min, max } };
  },
};

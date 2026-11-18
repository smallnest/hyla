export interface HylaWebBridge {
  rpc<TResult = unknown, TParams = unknown>(
    method: string,
    params?: TParams,
    slotId?: string,
  ): Promise<TResult>;
  on<TPayload = unknown>(event: string, listener: (data: TPayload) => void): () => void;
}

export interface ThemeChangedPayload {
  dark: boolean;
  themeName: string;
}

declare global {
  interface Window {
    hyla?: HylaWebBridge;
  }
}

function getBridge(): HylaWebBridge {
  if (typeof window !== "undefined" && window.hyla) {
    return window.hyla;
  }
  throw new Error("window.hyla bridge is not injected (load /_hyla/bridge.js first)");
}

export function rpc<TResult = unknown, TParams = unknown>(
  method: string,
  params?: TParams,
  slotId?: string,
): Promise<TResult> {
  return getBridge().rpc<TResult, TParams>(method, params, slotId);
}

export function onEvent<TPayload = unknown>(
  event: string,
  listener: (data: TPayload) => void,
): () => void {
  return getBridge().on<TPayload>(event, listener);
}

export function onThemeChanged(listener: (payload: ThemeChangedPayload) => void): () => void {
  return onEvent<ThemeChangedPayload>("theme.changed", listener);
}

export function readThemeTokens(): Record<string, string> {
  if (typeof window === "undefined" || typeof document === "undefined") {
    return {};
  }
  const styles = getComputedStyle(document.documentElement);
  const names = [
    "--hyla-bg",
    "--hyla-sidebar",
    "--hyla-card",
    "--hyla-chip",
    "--hyla-popover",
    "--hyla-control",
    "--hyla-text",
    "--hyla-muted",
    "--hyla-text-muted",
    "--hyla-faint",
    "--hyla-accent",
    "--hyla-attention",
    "--hyla-done",
    "--hyla-danger",
    "--hyla-divider",
  ];
  const out: Record<string, string> = {};
  for (const name of names) {
    out[name] = styles.getPropertyValue(name).trim();
  }
  return out;
}

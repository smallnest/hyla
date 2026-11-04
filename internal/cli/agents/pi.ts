// Installed by Hyla (`hyla skill setup`), which replaces it as Hyla
// updates; `hyla skill remove` removes it. Inside Hyla, it tells Pi which
// session and pane it runs in, and tells Hyla Pi's session, to resume it
// after Hyla's background service restarts. Outside Hyla it does nothing.
import { execFile } from "node:child_process";

const HYLA = __HYLA__;

// hook runs `hyla hook pi` as Claude Code runs its hooks, returning what
// Hyla tells the agent.
function hook(sessionID: string | undefined): Promise<string> {
  return new Promise((resolve) => {
    const child = execFile(HYLA, ["hook", "pi"], { timeout: 5000 }, (err, stdout) => {
      try {
        resolve(JSON.parse(stdout).hookSpecificOutput.additionalContext ?? "");
      } catch {
        resolve("");
      }
    });
    child.on("error", () => resolve(""));
    child.stdin?.end(JSON.stringify({ hook_event_name: "SessionStart", session_id: sessionID ?? "" }));
  });
}

export default function (pi: any) {
  // KERO_PANE: a session of 2.x's, which called Hyla Kero, still running.
  if (!process.env.HYLA_PANE && !process.env.KERO_PANE) return;
  let context = "";
  pi.on("session_start", async (_event: any, ctx: any) => {
    // A terminal session, or a one-off prompt; not one a program drives.
    if (ctx?.mode !== "tui" && ctx?.mode !== "print") return;
    let sessionID: string | undefined;
    try {
      if (ctx.mode === "tui" && ctx.sessionManager.getSessionFile()) {
        sessionID = ctx.sessionManager.getSessionId();
      }
    } catch {}
    context = await hook(sessionID);
  });
  pi.on("before_agent_start", (event: any) => {
    if (context) event.systemPromptOptions.sections.hyla = context;
  });
}

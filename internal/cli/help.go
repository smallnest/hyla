package cli

import (
	"fmt"
	"slices"
	"strings"

	"hyla/internal/build"
)

// A command of the hyla CLI, as `hyla help` and `hyla help <name>` print it,
// as completions offer it, and as `hyla help --markdown` writes the reference
// the docs carry. The code each name runs stays in cli.go's switches; this
// table is what the CLI says about itself, in one place.
type command struct {
	name    string
	aliases []string
	group   string
	summary string
	usage   string
	hidden  bool
}

// The order commands appear in, and their headings.
var commandGroups = []struct{ key, title string }{
	{"sessions", "Sessions"},
	{"panes", "Panes"},
	{"reading", "Reading, waiting and watching"},
	{"agents", "Agents"},
	{"browser", "Browser panes"},
	{"machine", "This machine as a host"},
	{"app", "The app"},
	{"service", "The daemon"},
}

var commands = []command{
	{
		name: "ls", aliases: []string{"list"}, group: "sessions",
		summary: "list sessions and their panes (--json)",
		usage: `hyla ls [--json]
    --json                      print the sessions as JSON`,
	},
	{
		name: "new", group: "sessions",
		summary: "start a session, printing its id",
		usage: `hyla new [options] [-- cmd…]
    -p, --project <name|id>     project to put it in (default: by directory)
    -n, --name <name>           name it
    -d, --dir <dir>             start it there (default: here)
    -f, --focus                 show it in the app
    -w, --wait                  wait for cmd to end, exit with its status
    -u, --url <url>             a browser showing url, not a shell
    --worktree <branch>         in a git worktree of its own, on a new branch
                                from the default branch
    --base <ref>                the branch to start it from instead
    --json                      print the session as JSON`,
	},
	{
		name: "rename", group: "sessions",
		summary: "name a session",
		usage:   `hyla rename <session> <name>`,
	},
	{
		name: "focus", group: "sessions",
		summary: "show a session (and pane) in the app",
		usage:   `hyla focus <target>`,
	},
	{
		name: "close", aliases: []string{"kill"}, group: "sessions",
		summary: "end a session, or a pane",
		usage: `hyla close <target>
    --remove-worktree           remove the session's own worktree too, and
                                its branch once merged
    --force                     even if that loses changes or commits`,
	},
	{
		name: "restart", group: "sessions",
		summary: "start an exited pane's shell again",
		usage:   `hyla restart <target>`,
	},
	{
		name: "status", group: "sessions",
		summary: "one target's status, as a row or JSON",
		usage: `hyla status [target] [--json] [--no-header]
    --json                      print the target as JSON
    --no-header                 leave out the column names`,
	},
	{
		name: "projects", group: "sessions",
		summary: "list projects (--json)",
		usage:   `hyla projects [--json]`,
	},
	{
		name: "history", group: "sessions",
		summary: "the agents' past conversations, to resume",
		usage: `hyla history [query] [-p project] [-n count] [--json]
    -p, --project <name|id>     only this project's conversations
    -n, --limit <count>         at most this many (default 20)
    --json                      print them as JSON`,
	},
	{
		name: "resume", group: "sessions",
		summary: "start a session resuming a conversation",
		usage: `hyla resume <conversation> [-f]
    -f, --focus                 show it in the app`,
	},

	{
		name: "split", group: "panes",
		summary: "start a pane beside a pane, printing its id",
		usage: `hyla split [target] [options] [-- cmd…]
    --right (default), --down, --left, --up
    -r, --ratio <0..1>          the new pane's share (default 0.5)
    -d, --dir <dir>             start it there (default: the pane's)
    -w, --wait                  wait for cmd to end, exit with its status
    -u, --url <url>             a browser showing url, not a shell`,
	},
	{
		name: "send", group: "panes",
		summary: "type text into a pane",
		usage: `hyla send <target> <text…>
    -e                          interpret \n \r \t \e \xHH escapes
    --enter                     press Enter after it`,
	},
	{
		name: "key", group: "panes",
		summary: "press keys in a pane",
		usage: `hyla key <target> <key…>
    keys: enter, tab, escape, up, down, left, right, backspace, ctrl+c, …`,
	},
	{
		name: "notify", group: "panes",
		summary: "flag the pane as needing you",
		usage:   `hyla notify [target] <message>`,
	},

	{
		name: "prompt", group: "reading",
		summary: "send text and Enter, wait, then print the screen",
		usage:   `hyla prompt <target> <text…>`,
	},
	{
		name: "read", aliases: []string{"capture"}, group: "reading",
		summary: "print a pane's screen",
		usage: `hyla read [target] [-a] [-n lines]
    -a, --all                   with the scrollback
    -n, --lines <count>         the last lines only`,
	},
	{
		name: "wait", group: "reading",
		summary: "wait for a pane to stop working",
		usage: `hyla wait [target] [--for idle,waiting,error,exited] [--timeout 10m]
    --for <statuses>            wait for one of these
    -t, --timeout <dur>         give up after this long
    -x, --exit                  wait for the command it started with to end,
                                and exit with its status`,
	},
	{
		name: "events", group: "reading",
		summary: "print what happens, a line an event",
		usage: `hyla events [target] [-p project] [--for statuses] [--json]
    event: panes changing status, sessions and panes starting and closing,
    renames, moves to another project, notifications; until the target closes
    --for <statuses>            only panes going to these: idle, working,
                                waiting, error, exited
    --once                      exit after the first event (1 if the target
                                closes or the timeout comes first)
    -t, --timeout <dur>         stop watching after this long
    --json                      print each event as JSON`,
	},
	{
		name: "diff", group: "reading",
		summary: "the pane's git changes, as git diff prints them",
		usage: `hyla diff [target] [--turn | --branch] [path…]
    --turn                      since its agent last started working
    --branch                    since it left the default branch
    (nothing)                   since HEAD, untracked files too`,
	},

	{
		name: "report", group: "agents",
		summary: "report this program's status (OSC 7501)",
		usage: `hyla report <state> [message] [--kind k] [--progress n] [--app a] [--id i]
    state: idle, working, done, blocked, error, clear`,
	},
	{
		name: "skill", group: "agents",
		summary: "teach coding agents to use hyla",
		usage: `hyla skill setup|status|show|remove
    --agent <claude|codex|pi|opencode>   set up one agent only`,
	},
	{
		name: "feishu", group: "agents",
		summary: "drive the sessions from Feishu",
		usage:   `hyla feishu setup|status     (hyla feishu help)`,
	},

	{
		name: "browser", group: "browser",
		summary: "drive a browser pane's page",
		usage: `hyla browser [target] <command> [args…]
    the target is a browser pane, or a session, for its browser
    (default: this session's); needs Remote Control, in Settings › Agents
    open <url>                  go to a page, and wait for it to load
    back, forward, reload       and wait for the page to load
    text [selector]             print the page's text, or an element's
    html [selector]             print the page's HTML, or an element's
    snapshot [selector]         roles, names, and a ref for each element
    click <selector>            click an element
    type <selector> <text…>     type text into a field (--enter: then Enter)
    wait [selector] [--text t]  wait for an element, the text, or the page
    eval <js…>                  run JavaScript, print its value
    screenshot [-o file.png]    save a picture of the page, print where
    cookies save [file]         save the browser's cookies
    cookies load <file>         load cookies
    --ref <ref>                 a snapshot's element, in place of a selector
    --domain <domain>           cookies of a site and its subdomains
    -t, --timeout <dur>         how long to wait (default 10s)`,
	},

	{
		name: "tailcat", group: "machine",
		summary: "this machine as a host, over a relay, with a pair code",
		usage: `hyla tailcat enable|disable|port|rotate|code
    code                        print the pair code (secret)
    rotate                      make a new one, and a new key
    port [n]                    show or set the listener's port (default 7473)`,
	},
	{
		name: "pocket", group: "machine",
		summary: "this machine as a host for HylaPocket on a phone",
		usage: `hyla pocket enable|disable|code|qr
    code                        print the pair code (secret)
    qr                          the QR code a phone scans
    hyla pocket tcp enable|disable [address:port]
                                listen on this machine's network, with a
                                password: Hyla or HylaPocket connects by the
                                address (secret)
    hyla pocket addr|token      where it listens, and the password`,
	},
	{
		name: "pets", group: "machine",
		summary: "the pet on the window's edge",
		usage: `hyla pets [ls]                the pets Hyla shows
    hyla pets install <pack>    a Codex pet pack: a .codex-pet.zip, or a pet's
                                directory as ~/.codex/pets has them;
                                --builtin installs hyla's pets
    hyla pets use <name|hyla|codex>
                                show it on the window's edge
    hyla pets rm <name>         remove an installed pet`,
	},

	{
		name: "do", group: "app",
		summary: "run an action of the app, as its menus do",
		usage: `hyla do <action> [key=value…]
    needs Remote Control, in Settings › Agents; hyla actions lists them`,
	},
	{
		name: "actions", group: "app",
		summary: "list the actions hyla do runs",
		usage:   `hyla actions`,
	},
	{
		name: "config", group: "app",
		summary: "the configuration file",
		usage: `hyla config path|show|edit|check
    path                        where it is
    show [--defaults]           every key with its value
    edit                        open it in $EDITOR
    check                       report keys it does not know`,
	},

	{
		name: "daemon", group: "service",
		summary: "the daemon that keeps the sessions",
		usage: `hyla daemon status|stop|restart|update [version]
    stop --force                kill a hylad that does not stop
    serve [--listen tailcat://:7473]…
                                run hylad in the foreground, never idle, as
                                a service does
    install-service|uninstall-service
                                run hylad with the machine (systemd, launchd)
    update [version]            update the server build of Hyla`,
	},

	{
		name: "help", aliases: []string{"--help", "-h"}, group: "service",
		summary: "this page, a command's usage, or the reference",
		usage: `hyla help [command|all] [--markdown]
    hyla help                   the commands, by group
    hyla help <command>         what one command takes
    hyla help all               every command with its flags
    hyla help --markdown        the reference as Markdown
    hyla <command> --help       the same as hyla help <command>`,
	},
	{
		name: "completions", group: "service",
		summary: "a completion script for a shell",
		usage: `hyla completions <zsh|bash|fish>
    hyla completions zsh > "${fpath[1]}/_hyla"`,
	},
	{
		name: "version", aliases: []string{"--version"}, group: "service",
		summary: "print the version",
		usage:   `hyla version`,
	},

	// Kept for the agent hooks and the Feishu bot, which call them; not
	// something a person runs.
	{name: "hook", group: "agents", hidden: true, summary: "agent hooks", usage: `hyla hook …`},
	{name: "answer", group: "agents", hidden: true, summary: "answer a Feishu turn", usage: `hyla answer …`},
}

// lookupCommand finds a command by name or alias.
func lookupCommand(name string) *command {
	for i := range commands {
		c := &commands[i]
		if c.name == name || slices.Contains(c.aliases, name) {
			return c
		}
	}
	return nil
}

// visibleNames is the names and aliases completions offer: the ones help
// shows.
func visibleNames() []string {
	var out []string
	for _, c := range commands {
		if c.hidden {
			continue
		}
		out = append(out, c.name)
		out = append(out, c.aliases...)
	}
	return out
}

// commandNames is every name and alias the CLI takes, for main to tell a
// command from the app's own arguments.
func commandNames() []string {
	var out []string
	for _, c := range commands {
		out = append(out, c.name)
		out = append(out, c.aliases...)
	}
	return out
}

// wantsHelp reports whether a command was asked about itself, as in
// `hyla new --help`.
func wantsHelp(args []string) bool {
	for _, a := range args {
		if a == "--" {
			break
		}
		if a == "--help" || a == "-h" {
			return true
		}
	}
	return false
}

const helpFooter = `A target is a session or a pane: an id, a prefix of it, or a session's name;
a session stands for its focused pane. Inside hyla, $HYLA_PANE and
$HYLA_SESSION name the pane and session a command runs in, the default target.

Exit status: 0 ok, 1 failed, 2 bad usage, 3 no such target (or an ambiguous
one), 4 hylad unreachable.`

// printHelp prints `hyla help`, `hyla help <command>`, `hyla help all`, or
// the Markdown reference.
func printHelp(args []string) error {
	for _, a := range args {
		if a == "--markdown" {
			return printReference()
		}
	}
	what := ""
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			what = a
		}
	}
	if what != "" && what != "all" {
		c := lookupCommand(what)
		if c == nil {
			return usageErr("no command %q: hyla help lists them", what)
		}
		printCommand(*c)
		return nil
	}
	if what == "all" {
		for _, c := range commands {
			if c.hidden {
				continue
			}
			printCommand(c)
		}
		fmt.Println()
		fmt.Println(helpFooter)
		return nil
	}

	fmt.Println("hyla — terminal sessions that keep running, for you and your agents")
	fmt.Println()
	if !build.Server {
		fmt.Println("Usage: hyla [command] [options]; with none, open the app.")
	} else {
		fmt.Println("Usage: hyla <command> [options]")
	}
	for _, g := range commandGroups {
		fmt.Printf("\n%s:\n", g.title)
		for _, c := range commands {
			if c.hidden || c.group != g.key {
				continue
			}
			fmt.Printf("  %-24s %s\n", c.name, c.summary)
		}
	}
	fmt.Println()
	fmt.Println("hyla help <command> for what one takes, hyla help all for every flag.")
	fmt.Println()
	fmt.Println(helpFooter)
	return nil
}

// printCommand prints one command's usage.
func printCommand(c command) {
	fmt.Println(c.usage)
	aliases := ""
	for _, a := range c.aliases {
		if strings.HasPrefix(a, "-") {
			continue
		}
		if aliases != "" {
			aliases += ", "
		}
		aliases += a
	}
	if aliases != "" {
		fmt.Printf("\nalso: %s\n", aliases)
	}
	fmt.Println()
}

// printReference prints every command as Markdown, for the docs.
func printReference() error {
	fmt.Println("## Commands")
	fmt.Println()
	fmt.Println("`hyla help <command>` prints the same as one line below.")
	for _, g := range commandGroups {
		fmt.Printf("\n### %s\n", g.title)
		for _, c := range commands {
			if c.hidden || c.group != g.key {
				continue
			}
			first, rest, _ := strings.Cut(c.usage, "\n")
			fmt.Println()
			fmt.Printf("- `%s` — %s\n", first, c.summary)
			for _, line := range strings.Split(rest, "\n") {
				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}
				fmt.Printf("  - `%s`\n", line)
			}
		}
	}
	fmt.Println()
	fmt.Println(helpFooter)
	return nil
}

// The subcommands a completion offers after each command that has them.
var subcommands = map[string][]string{
	"daemon":      {"status", "stop", "restart", "update", "serve", "install-service", "uninstall-service"},
	"config":      {"path", "show", "edit", "check"},
	"pocket":      {"enable", "disable", "code", "qr", "tcp", "addr", "token"},
	"tailcat":     {"enable", "disable", "port", "rotate", "code"},
	"pets":        {"ls", "install", "use", "rm"},
	"skill":       {"setup", "status", "show", "remove"},
	"feishu":      {"setup", "status"},
	"completions": {"zsh", "bash", "fish"},
	"browser":     {"open", "back", "forward", "reload", "text", "html", "snapshot", "click", "type", "wait", "eval", "screenshot", "cookies"},
}

// completionsCmd prints a completion script for a shell: the commands of the
// table, and the subcommands above, so the script follows the CLI.
func completionsCmd(args []string) error {
	if len(args) != 1 {
		return usageErr("hyla completions <zsh|bash|fish>")
	}
	switch args[0] {
	case "zsh":
		fmt.Println("#compdef hyla")
		fmt.Println("# hyla completions zsh > \"${fpath[1]}/_hyla\"")
		fmt.Println("_hyla() {")
		fmt.Println("  local -a cmds")
		fmt.Printf("  cmds=(%s)\n", strings.Join(visibleNames(), " "))
		fmt.Println("  if (( CURRENT == 2 )); then")
		fmt.Println("    _describe 'command' cmds")
		fmt.Println("    return")
		fmt.Println("  fi")
		fmt.Println("  case $words[2] in")
		for _, c := range commands {
			if subs := subcommands[c.name]; len(subs) > 0 {
				fmt.Printf("    %s) _values '%s' %s ;;\n", c.name, c.name, strings.Join(subs, " "))
			}
		}
		fmt.Println("  esac")
		fmt.Println("}")
		fmt.Println("compdef _hyla hyla")
	case "bash":
		fmt.Println("# hyla completions bash > /etc/bash_completion.d/hyla")
		fmt.Println("_hyla() {")
		fmt.Println("  local cur=\"${COMP_WORDS[COMP_CWORD]}\"")
		fmt.Println("  if [ \"$COMP_CWORD\" -eq 1 ]; then")
		fmt.Printf("    COMPREPLY=( $(compgen -W \"%s\" -- \"$cur\") )\n", strings.Join(visibleNames(), " "))
		fmt.Println("    return")
		fmt.Println("  fi")
		fmt.Println("  case \"${COMP_WORDS[1]}\" in")
		for _, c := range commands {
			if subs := subcommands[c.name]; len(subs) > 0 {
				fmt.Printf("    %s) COMPREPLY=( $(compgen -W \"%s\" -- \"$cur\") ) ;;\n", c.name, strings.Join(subs, " "))
			}
		}
		fmt.Println("  esac")
		fmt.Println("}")
		fmt.Println("complete -F _hyla hyla")
	case "fish":
		fmt.Println("# hyla completions fish > ~/.config/fish/completions/hyla.fish")
		fmt.Println("complete -c hyla -f")
		for _, c := range commands {
			if c.hidden {
				continue
			}
			fmt.Printf("complete -c hyla -n '__fish_use_subcommand' -a %s -d '%s'\n", c.name, strings.ReplaceAll(c.summary, "'", ""))
		}
		for _, c := range commands {
			if subs := subcommands[c.name]; len(subs) > 0 {
				for _, sub := range subs {
					fmt.Printf("complete -c hyla -n '__fish_seen_subcommand_from %s' -a %s\n", c.name, sub)
				}
			}
		}
	default:
		return usageErr("%s is no shell: hyla completions zsh|bash|fish", args[0])
	}
	return nil
}

package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"jeff/internal/config"
)

func main() {
	err := run(os.Args, os.Stdin, os.Stdout, os.Stderr)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	if len(args) == 0 {
		return fmt.Errorf("missing arguments")
	}

	rootFlags := flag.NewFlagSet("jeff", flag.ContinueOnError)
	rootFlags.SetOutput(stderr)

	configDir := rootFlags.String("config", "", "Optional configuration directory (defaults to XDG config home)")
	showStatus := rootFlags.Bool("status", false, "Statusblock vor Antworten anzeigen")

	if err := rootFlags.Parse(args[1:]); err != nil {
		return err
	}

	remaining := rootFlags.Args()
	if len(remaining) == 0 {
		printUsage(stdout)
		return nil
	}

	store, err := config.NewStore(*configDir)
	if err != nil {
		return err
	}

	ctx := &commandContext{
		stdin:  stdin,
		stdout: stdout,
		stderr: stderr,
		store:  store,
		status: *showStatus,
	}

	switch remaining[0] {
	case "init":
		return runInit(ctx, remaining[1:])
	case "ask":
		return runAsk(ctx, remaining[1:])
	case "chat":
		return runChat(ctx, remaining[1:])
	case "completion":
		return runCompletion(ctx, remaining[1:])
	case "help":
		printUsage(stdout)
		return nil
	default:
		return fmt.Errorf("unknown command %q", remaining[0])
	}
}

func printUsage(w io.Writer) {
	fmt.Fprintln(w, "jeff – Kommandozeilen-Assistent")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Verwendung:")
	fmt.Fprintln(w, "  jeff [--config <dir>] <command> [options]")
	fmt.Fprintln(w, "  jeff --status <command> [...]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Globale Flags:")
	fmt.Fprintln(w, "  --config <dir>   Konfigurationsverzeichnis setzen")
	fmt.Fprintln(w, "  --status         Statusblock vor Antworten anzeigen")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Kommandos:")
	fmt.Fprintln(w, "  init        Session-ID setzen oder letzte Session wiederverwenden")
	fmt.Fprintln(w, "  ask         Eine Frage stellen (Optionen: --session, --codex-binary, --timeout, --show-token-cost; global: --status)")
	fmt.Fprintln(w, "  chat        Interaktive Codex-Session (nutzt aktuelle Session-ID)")
	fmt.Fprintln(w, "  completion  Shell-Completions erzeugen")
	fmt.Fprintln(w, "  help        Diese Hilfe anzeigen")
}

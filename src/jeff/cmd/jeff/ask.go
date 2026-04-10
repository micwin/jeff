package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

func runAsk(ctx *commandContext, argv []string) error {
	flags := flag.NewFlagSet("ask", flag.ContinueOnError)
	flags.SetOutput(ctx.stderr)

	sessionFlag := flags.String("session", "", "Session-ID überschreiben")
	codexBinaryFlag := flags.String("codex-binary", "", "Pfad zur Codex-CLI überschreiben")
	showTokens := flags.Bool("show-token-cost", false, "Tokenkosten zusätzlich ausgeben")
	timeout := flags.Duration("timeout", 45*time.Second, "Zeitlimit für die Antwort")

	if err := flags.Parse(argv); err != nil {
		return err
	}

	question := strings.TrimSpace(strings.Join(flags.Args(), " "))
	if question == "" {
		return errors.New("Frage fehlt – Beispiel: jeff ask \"Was ist das für ein Verzeichnis?\"")
	}

	cfg, err := ctx.loadConfig()
	if err != nil {
		return err
	}

	sessionID := strings.TrimSpace(*sessionFlag)
	if sessionID == "" {
		if cfg.ActiveSession != "" {
			sessionID = cfg.ActiveSession
		} else {
			sessionID = cfg.LastSession
		}
	}
	if sessionID == "" {
		return errors.New("keine Session bekannt – bitte zuerst 'jeff init' ausführen")
	}

	codexBinary := strings.TrimSpace(*codexBinaryFlag)
	if codexBinary == "" {
		codexBinary = cfg.CodexBinary
	}
	if codexBinary == "" {
		codexBinary = "codex"
	}

	ctxWithTimeout, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	tmpFile, err := os.CreateTemp("", "jeff-codex-response-*.txt")
	if err != nil {
		return fmt.Errorf("temp datei anlegen: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	cmdArgs := []string{
		"--sandbox", "danger-full-access",
		"--search",
		"exec",
		"--skip-git-repo-check",
		"--output-last-message", tmpPath,
		"resume", sessionID, question,
	}

	var stdoutBuf, stderrBuf strings.Builder
	cmd := exec.CommandContext(ctxWithTimeout, codexBinary, cmdArgs...)
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf
	if err := cmd.Run(); err != nil {
		return formatCodexError(err, stdoutBuf.String(), stderrBuf.String())
	}

	allLogs := stdoutBuf.String()
	if stderrBuf.Len() > 0 {
		allLogs = allLogs + "\n" + stderrBuf.String()
	}
	statusLines := extractStatusLines(allLogs)

	answerBytes, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("codex antwort lesen: %w", err)
	}
	answer := strings.TrimSpace(string(answerBytes))
	if answer == "" {
		answer = strings.TrimSpace(stdoutBuf.String())
	}
	if ctx.status && len(statusLines) > 0 {
		for _, line := range statusLines {
			fmt.Fprintln(ctx.stdout, line)
		}
		fmt.Fprintln(ctx.stdout)
	}

	fmt.Fprintln(ctx.stdout, answer)

	if *showTokens {
		if usage := extractTokenUsage(stdoutBuf.String()); usage != "" {
			fmt.Fprintf(ctx.stdout, "Token: %s\n", usage)
		}
	}

	cfg.RecordSession(sessionID)
	if err := ctx.saveConfig(cfg); err != nil {
		return err
	}

	return nil
}

func formatCodexError(runErr error, stdout, stderr string) error {
	msg := strings.TrimSpace(stderr)
	if msg == "" {
		msg = strings.TrimSpace(stdout)
	}
	if msg != "" {
		return fmt.Errorf("codex client fehlgeschlagen: %w\n%s", runErr, msg)
	}
	return fmt.Errorf("codex client fehlgeschlagen: %w", runErr)
}

func extractTokenUsage(output string) string {
	lines := strings.Split(output, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		lower := strings.ToLower(line)
		if strings.HasPrefix(lower, "tokens used") {
			return line
		}
	}
	return ""
}

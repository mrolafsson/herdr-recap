// herdr-recap: every Claude agent in a herdr popup, with a one-line recap of
// where it got to, written while you were away.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/signal"
	"syscall"
	"time"
)

const usage = `herdr-recap — what each of your agents was doing, in herdr

  action open      what the herdr action runs: opens the popup
  action demo      what the herdr action runs: the popup on fictional agents
  picker [--demo]  the popup itself; --demo (or HERDR_RECAP_DEMO=1) shows
                   fictional agents: no herdr or Claude needed, safe to screenshot
  tick             what herdr runs on an agent status change: schedules a recap
                   for each Claude agent now waiting on you
  recap PANE       write PANE's recap now, if it's out of date (--force: anyway)
  recap --after S --seq N PANE
                   (internal) wait S seconds, then recap PANE if its status is
                   still the one numbered N
  list             what the popup would show, as text: agents, sessions, recaps
`

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "herdr-recap:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	if len(args) == 0 {
		fmt.Print(usage)
		return nil
	}
	cfg, err := loadConfig()
	if err != nil {
		if args[0] == "action" {
			notify("Recap", err.Error())
		}
		return err
	}

	switch args[0] {
	case "action":
		if len(args) < 2 {
			return errors.New("action needs a name")
		}
		return runAction(args[1])
	case "picker":
		demo := os.Getenv("HERDR_RECAP_DEMO") == "1" || (len(args) > 1 && args[1] == "--demo")
		return runPicker(ctx, cfg, demo)
	case "tick":
		return tick(cfg)
	case "recap":
		return runRecapCommand(ctx, cfg, args[1:])
	case "list":
		return debugList(cfg)
	default:
		fmt.Print(usage)
		return fmt.Errorf("unknown command %q", args[0])
	}
}

func runAction(name string) error {
	switch name {
	case "open", "demo":
		env := map[string]string{}
		if name == "demo" {
			env["HERDR_RECAP_DEMO"] = "1"
		}
		if err := openPopup("picker", "80%", "75%", env); err != nil {
			notify("Recap", "Couldn't open the popup: "+err.Error())
			return err
		}
		return nil
	}
	return fmt.Errorf("unknown action %q", name)
}

func runRecapCommand(ctx context.Context, cfg config, args []string) error {
	fs := flag.NewFlagSet("recap", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	after := fs.Int("after", -1, "")
	seq := fs.Int64("seq", -1, "")
	force := fs.Bool("force", false, "")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("recap: %w", err)
	}
	if fs.NArg() != 1 {
		return errors.New("recap needs one pane ID")
	}
	pane := fs.Arg(0)
	if *after >= 0 {
		if *seq < 0 {
			return errors.New("recap --after needs --seq")
		}
		return recapLater(ctx, cfg, time.Duration(*after)*time.Second, *seq, pane)
	}
	s, err := resolveSession(pane)
	if err != nil {
		return err
	}
	r, _, err := ensureRecap(ctx, cfg, s, *force)
	if err != nil {
		return err
	}
	fmt.Println(r.Text)
	return nil
}

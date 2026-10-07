package ui

import (
	"fmt"
	"os"
	"time"

	"golang.org/x/term"
)

const (
	BoldCyan = "\033[1;36m"
	Dim      = "\033[2m"
	Reset    = "\033[0m"
)

func Dimmed(s string) string { return Dim + s + Reset }

const (
	bigBannerMinWidth = 78
	bigBanner         = `
██████╗ ███████╗████████╗████████╗ █████╗ ████████╗███████╗ ██████╗██╗  ██╗
██╔══██╗██╔════╝╚══██╔══╝╚══██╔══╝██╔══██╗╚══██╔══╝██╔════╝██╔════╝██║  ██║
██████╔╝█████╗     ██║      ██║   ███████║   ██║   █████╗  ██║     ███████║
██╔══██╗██╔══╝     ██║      ██║   ██╔══██║   ██║   ██╔══╝  ██║     ██╔══██║
██████╔╝███████╗   ██║      ██║   ██║  ██║   ██║   ███████╗╚██████╗██║  ██║
╚═════╝ ╚══════╝   ╚═╝      ╚═╝   ╚═╝  ╚═╝   ╚═╝   ╚══════╝ ╚═════╝╚═╝  ╚═╝
`
)

func TermWidth() int {
	w, _, err := term.GetSize(int(os.Stdout.Fd()))
	if err != nil {
		return 0
	}
	return w
}

func BannerText(width int) string {
	if width == 0 {
		width = bigBannerMinWidth
	}
	if width >= bigBannerMinWidth {
		return BoldCyan + bigBanner + Reset +
			Dimmed("              build your own coding agent") + "\n" +
			Dimmed("              type a message · ctrl-d to exit") + "\n\n"
	}
	return "\n" + BoldCyan + "  BETTATECH" + Reset + Dimmed("  ·  build your own coding agent") + "\n\n"
}

var spinnerFrames = []rune("⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏")

type Spinner struct {
	stop chan struct{}
	done chan struct{}
}

func StartSpinner(label string) *Spinner {
	if !term.IsTerminal(int(os.Stdout.Fd())) {
		return &Spinner{}
	}
	s := &Spinner{stop: make(chan struct{}), done: make(chan struct{})}
	go func() {
		defer close(s.done)
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()
		i := 0
		for {
			select {
			case <-s.stop:
				fmt.Print("\r\033[K")
				return
			case <-ticker.C:
				fmt.Printf("\r%s%c%s %s", BoldCyan, spinnerFrames[i], Reset, Dimmed(label))
				i = (i + 1) % len(spinnerFrames)
			}
		}
	}()
	return s
}

func (s *Spinner) Stop() {
	if s.stop == nil {
		return
	}
	close(s.stop)
	<-s.done
}

package tui

import (
	"context"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/hyqhyq3/mymtr/internal/mtr"
)

type eventMsg struct {
	ev mtr.Event
}

type doneMsg struct{}

type model struct {
	ctx    context.Context
	cancel context.CancelFunc

	controller *mtr.Controller
	snapshot   *mtr.Snapshot

	width  int
	height int

	lastRound int
	err       error
	done      bool
	paused    bool

	styles styles
}

type styles struct {
	title  lipgloss.Style
	header lipgloss.Style
	muted  lipgloss.Style
}

func newModel(ctx context.Context, cancel context.CancelFunc, controller *mtr.Controller) *model {
	return &model{
		ctx:        ctx,
		cancel:     cancel,
		controller: controller,
		styles: styles{
			title:  lipgloss.NewStyle().Bold(true),
			header: lipgloss.NewStyle().Bold(true),
			muted:  lipgloss.NewStyle().Foreground(lipgloss.Color("8")),
		},
	}
}

func (m *model) Init() tea.Cmd {
	return waitForEvent(m.controller.Events())
}

func (m *model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil
	case tea.KeyMsg:
		switch msg.String() {
		case "p":
			m.paused = !m.paused
			return m, nil
		case "q", "esc", "ctrl+c":
			if m.cancel != nil {
				m.cancel()
			}
			return m, tea.Quit
		}
	case eventMsg:
		switch msg.ev.Type {
		case mtr.EventTypeHopUpdated, mtr.EventTypeRoundCompleted:
			if !m.paused {
				m.snapshot = m.controller.Snapshot()
				m.lastRound = msg.ev.Round
			}
		case mtr.EventTypeError:
			m.err = msg.ev.Err
			if !m.paused {
				m.snapshot = m.controller.Snapshot()
			}
		case mtr.EventTypeDone:
			m.done = true
			m.snapshot = m.controller.Snapshot()
		}
		return m, waitForEvent(m.controller.Events())
	case doneMsg:
		m.done = true
		return m, nil
	}
	return m, nil
}

func (m *model) View() string {
	if m.snapshot == nil {
		return m.styles.muted.Render("启动中... (q 退出)\n")
	}

	status := []string{
		fmt.Sprintf("Target: %s (%s)", m.snapshot.Target, m.snapshot.TargetIP),
		fmt.Sprintf("Protocol: %s", m.snapshot.Protocol),
		fmt.Sprintf("Round: %d", m.lastRound+1),
	}
	if m.snapshot.Count == 0 {
		status = append(status, "Count: ∞")
	} else {
		status = append(status, fmt.Sprintf("Count: %d", m.snapshot.Count))
	}
	if m.paused {
		status = append(status, "Paused")
	}
	if m.done {
		status = append(status, "Done")
	}
	if m.err != nil && !m.done {
		status = append(status, fmt.Sprintf("Error: %v", m.err))
	}

	var b strings.Builder
	b.WriteString(m.styles.title.Render("MyMTR"))
	b.WriteString("\n")
	b.WriteString(strings.Join(status, "  "))
	b.WriteString("\n\n")

	b.WriteString(m.styles.header.Render("TTL  Loss%  Snt  Rcv  Last      Avg       Best      Wrst      StDev     Address            Hostname                Location"))
	b.WriteString("\n")

	for _, hop := range m.snapshot.Hops {
		addr := hop.IP
		if addr == "" {
			addr = "*"
		}
		host := hop.Hostname
		if host == "" {
			host = "-"
		}
		loc := "-"
		if hop.Location != nil {
			loc = hop.Location.String()
			if loc == "" {
				loc = "-"
			}
		}

		line := fmt.Sprintf(
			"%-3d  %5.1f  %-3d  %-3d  %-8s  %-8s  %-8s  %-8s  %-8s  %-16s  %-20s  %s",
			hop.TTL,
			hop.Stats.Loss,
			hop.Stats.Sent,
			hop.Stats.Received,
			emptyAsDash(hop.Stats.Last),
			emptyAsDash(hop.Stats.Avg),
			emptyAsDash(hop.Stats.Best),
			emptyAsDash(hop.Stats.Worst),
			emptyAsDash(hop.Stats.StdDev),
			trunc(addr, 16),
			trunc(host, 20),
			trunc(loc, max(20, m.width-3-6-4-4-8-8-8-8-8-16-20-8)),
		)
		b.WriteString(line)
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(m.styles.muted.Render("按 p 暂停/继续，按 q/esc/ctrl+c 退出"))
	b.WriteString("\n")
	return b.String()
}

func waitForEvent(ch <-chan mtr.Event) tea.Cmd {
	return func() tea.Msg {
		ev, ok := <-ch
		if !ok {
			return doneMsg{}
		}
		return eventMsg{ev: ev}
	}
}

func emptyAsDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func trunc(s string, n int) string {
	if n <= 0 {
		return ""
	}
	if len(s) <= n {
		return s
	}
	if n <= 1 {
		return s[:n]
	}
	return s[:n-1] + "…"
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

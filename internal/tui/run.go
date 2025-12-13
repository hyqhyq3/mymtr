package tui

import (
	"context"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/yangqihuang/mymtr/internal/mtr"
)

func Run(ctx context.Context, cancel context.CancelFunc, controller *mtr.Controller) error {
	p := tea.NewProgram(newModel(ctx, cancel, controller), tea.WithAltScreen())
	_, err := p.Run()
	return err
}

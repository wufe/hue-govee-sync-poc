package main

import (
	"fmt"
	"io"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/rs/zerolog/log"
)

var (
	redTextStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("9"))
	warnTextStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("11"))
	greenTextStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("10"))
	blueTextStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("12"))
)

type TUI struct {
	done         bool
	logUpdated   chan string
	modelUpdated chan struct{}
}

func NewTUI() *TUI {
	return &TUI{
		done:         false,
		logUpdated:   make(chan string, 100),   // Buffered channel to avoid blocking
		modelUpdated: make(chan struct{}, 100), // Buffered channel to avoid blocking
	}
}

// Write implements io.Writer interface to be able to receive logs from any logger (e.g. zerolog)
func (t *TUI) Write(p []byte) (n int, err error) {
	if t != nil {
		if !t.done {
			t.logUpdated <- string(p)
		} else {
			fmt.Print(string(p))
		}
	}
	return len(p), nil
}

func (t *TUI) UpdateTUI() {
	if t != nil {
		t.modelUpdated <- struct{}{}
	}
}

func (t *TUI) RunNewProgram(stdout, stderr io.Reader) error {
	_, err := tea.NewProgram(
		newModel(t, stdout, stderr),
		// tea.WithAltScreen(),
		// tea.WithMouseCellMotion(),
	).Run()
	t.done = true
	return err
}

func newModel(tui *TUI, stdout, stderr io.Reader) model {

	return model{
		tui:          tui,
		serverOutput: NewServerOutputModel(stdout, stderr),
	}
}

type model struct {
	tui          *TUI
	serverOutput *ServerOutputModel
	width        int
	height       int
}

func (m model) Init() tea.Cmd {
	// dispatching both the waitForUpdate and waitForLog commands
	// to be able to listen to both the service status updates and the logs at the same time
	return tea.Batch(m.waitForUpdate, m.waitForLog, m.serverOutput.Init())
}

func (m model) waitForUpdate() tea.Msg {
	// whenever the noop updates its model (by enqueueing a message in the channel)
	// we dispatch a tuiUpdateModel msg to update the TUI model and consequently the terminal's view
	noop := <-m.tui.modelUpdated
	return tuiUpdateModel(noop)
}

func (m model) waitForLog() tea.Msg {
	// whenever the logger writes into this channel
	// we dispatch a tuiUpdateLog msg to update the TUI model and consequently the terminal's view
	log := <-m.tui.logUpdated
	return tuiUpdateLog{
		log: strings.TrimSpace(log),
	}
}

// tuiUpdateModel is the tea.Msg that gets dispatched when the status changes,
// and we need to reflect those changes in the TUI.
type tuiUpdateModel struct{}

// tuiUpdateLog is the tea.Msg that gets dispatched when a new log is written,
// and we need to reflect that in the TUI.
type tuiUpdateLog struct {
	log string
}

// quit is the tea.Msg that gets dispatched when we want to exit the program.
type quit struct{}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var otherCmd tea.Cmd
	// m.serverOutput, otherCmd = m.serverOutput.Update(msg)

	switch msg := msg.(type) {
	case quit:
		// log.Info().Msgf("Exiting TUI...")
		// When we are about to quit, we also want to print the last status of the TUI model
		return m, tea.Quit
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "q":
			return m, func() tea.Msg {
				return quit{}
			}
		}
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		serverHeight := msg.Height / 5
		log.Info().Msgf("Window resized: width=%d, height=%d, serverHeight=%d", msg.Width, msg.Height, serverHeight)
		m.serverOutput.SetSize(msg.Width, serverHeight)
	case tuiUpdateModel:

		// TODO: Load data from model and update the TUI model accordingly

		// cmds := []tea.Cmd{m.waitForUpdate}
		otherCmd = m.waitForUpdate

		// if otherCmd != nil {
		// 	// If we have a paginator command, we want to add it to the list of commands
		// 	// so that the ui get synced
		// 	cmds = append(cmds, otherCmd)
		// }

		// return m, tea.Batch(cmds...)
	case tuiUpdateLog:
		// if we received a tuiUpdateLog, it means that we have another piece of log
		// that needs to be displayed calling "tea.Printf" to enqueue the log before the rendered TUI view
		otherCmd = tea.Sequence(tea.Printf(strings.ReplaceAll(msg.log, "%", "%%")), m.waitForLog)
	}

	var serverOutputCmd tea.Cmd
	m.serverOutput, serverOutputCmd = m.serverOutput.Update(msg)
	return m, tea.Batch(otherCmd, serverOutputCmd)
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString("\n\n")
	b.WriteString(greenTextStyle.Render("OK"))
	b.WriteString("\n\n")

	mainContent := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("86")).
		Padding(1).
		Width(m.width - 4).
		Height(m.height/5 - 4).
		Render(b.String())

	return lipgloss.JoinVertical(
		lipgloss.Left,
		mainContent,
		m.serverOutput.View(),
	)
}

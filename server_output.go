package main

import (
	"bufio"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Messages for the server output model
type ServerOutputMsg string
type ServerErrMsg error

// ServerOutputModel is a reusable component that displays command output
type ServerOutputModel struct {
	viewport viewport.Model
	ready    bool
	lines    []string
	mu       sync.Mutex

	// Optional styling
	Title       string
	BorderColor string
	width       int
	height      int
}

// NewServerOutputModel creates a new server output display model
func NewServerOutputModel(stdout, stderr io.Reader) *ServerOutputModel {
	m := ServerOutputModel{
		Title:       "Server Output",
		BorderColor: "205",
		lines:       []string{},
	}

	// Start reading from the provided readers
	if stdout != nil {
		go m.readOutput(stdout)
	}
	if stderr != nil {
		go m.readOutput(stderr)
	}

	return &m
}

var outputChan = make(chan string, 100)

func (m *ServerOutputModel) readOutput(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		line := scanner.Text()
		outputChan <- line
	}
}

func (m *ServerOutputModel) waitForActivity() tea.Cmd {
	return func() tea.Msg {
		line := <-outputChan
		return ServerOutputMsg(line)
	}
}

func (m *ServerOutputModel) SetSize(width, height int) {
	m.width = width
	m.height = height

	// Initialize or update viewport with the specified dimensions
	if !m.ready {
		m.viewport = viewport.New(width-4, height-6)
		m.ready = true
	} else {
		m.viewport.Width = width - 4
		m.viewport.Height = height - 6 // Account for border and title
	}
}

// Init implements tea.Model
func (m *ServerOutputModel) Init() tea.Cmd {
	return m.waitForActivity()
}

// Update implements tea.Model
func (m *ServerOutputModel) Update(msg tea.Msg) (*ServerOutputModel, tea.Cmd) {
	var (
		cmd  tea.Cmd
		cmds []tea.Cmd
	)

	switch msg := msg.(type) {

	case ServerOutputMsg:
		m.mu.Lock()
		line := string(msg)

		// Handle screen clear sequences (common in Vite output)
		if strings.Contains(line, "\033[2J") || strings.Contains(line, "\033[H") {
			m.lines = []string{}
		} else {
			m.lines = append(m.lines, line)
		}

		// Keep only last 1000 lines to prevent memory issues
		if len(m.lines) > 1000 {
			m.lines = m.lines[len(m.lines)-1000:]
		}

		content := strings.Join(m.lines, "\n")
		m.viewport.SetContent(content)
		m.viewport.GotoBottom()
		m.mu.Unlock()

		// Continue listening for more output
		cmds = append(cmds, m.waitForActivity())

	case ServerErrMsg:
		m.mu.Lock()
		m.lines = append(m.lines, fmt.Sprintf("Error: %v", msg))
		m.viewport.SetContent(strings.Join(m.lines, "\n"))
		m.mu.Unlock()
	}

	// Update viewport
	m.viewport, cmd = m.viewport.Update(msg)
	cmds = append(cmds, cmd)

	return m, tea.Batch(cmds...)
}

// View implements tea.Model
func (m *ServerOutputModel) View() string {
	if !m.ready || len(m.lines) == 0 {
		return ""
	}

	border := lipgloss.RoundedBorder()
	style := lipgloss.NewStyle().
		Border(border).
		BorderForeground(lipgloss.Color(m.BorderColor)).
		Padding(0, 1)

	if m.width > 0 {
		style = style.Width(m.width - 2)
	}
	if m.height > 0 {
		style = style.Height(m.height - 2)
	}

	title := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color(m.BorderColor)).
		Render(m.Title)

	return style.Render(fmt.Sprintf("%s\n%s", title, m.viewport.View()))
}

// ===============================================
// Example usage with parent model
// ===============================================

type parentModel struct {
	serverOutput ServerOutputModel
	cmd          *exec.Cmd
	otherContent string
	width        int
	height       int
}

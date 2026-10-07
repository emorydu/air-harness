package ui

import (
	"context"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type AgentRunner func(ctx context.Context, input string) error

type UsageFunc func() string

type AppendMsg string

type ApprovalRequest struct {
	Prompt string
	Detail string
	Reply  chan bool
}

type agentDoneMsg struct{ err error }

// ── Model ────────────────────────────────────────────────────

type modelState int

const (
	stateIdle modelState = iota
	stateRunning
	stateAwaitingApproval
)

type Model struct {
	runner    AgentRunner
	usageFunc UsageFunc

	width, height int

	viewport viewport.Model
	input    textinput.Model
	spin     spinner.Model

	state          modelState
	approvalPrompt string
	approvalDetail string
	approvalReply  chan bool

	// output 必须是**指针**：Bubble Tea 通过 Update 按值传递 model，
	// 而 strings.Builder 在被拷贝时会 panic（它内部有个自指针，
	// copyCheck 会检测到）。用指针，拷贝的就是指针本身，安全。
	output *strings.Builder
}

var (
	dimStyle    = lipgloss.NewStyle().Faint(true)
	cyanStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("36")).Bold(true)
	statusStyle = lipgloss.NewStyle().Background(lipgloss.Color("236")).Foreground(lipgloss.Color("252"))
)

var prog *tea.Program

func NewProgram(out io.Writer, runner AgentRunner, usageFunc UsageFunc, banner string) {
	m := NewModel(runner, usageFunc, banner)
	prog = tea.NewProgram(m, tea.WithAltScreen(), tea.WithOutput(out))

	send = prog.Send
	if err := prog.Start(); err != nil {
		fmt.Println("tui error:", err)
	}
}

func NewModel(runner AgentRunner, usageFunc UsageFunc, banner string) Model {
	ti := textinput.New()
	ti.Prompt = "❯ "
	ti.PromptStyle = cyanStyle
	ti.CharLimit = 0
	ti.Focus()

	vp := viewport.New(80, 20)
	vp.SetContent(banner)

	sp := spinner.New()
	sp.Spinner = spinner.Dot
	sp.Style = cyanStyle

	return Model{
		runner:    runner,
		usageFunc: usageFunc,
		viewport:  vp,
		input:     ti,
		spin:      sp,
		output:    &strings.Builder{},
	}
}

func (m Model) Init() tea.Cmd { return tea.Batch(textinput.Blink, m.spin.Tick) }

func (m Model) StateString() string {
	switch m.state {
	case stateRunning:
		return "running"
	case stateAwaitingApproval:
		return "awaiting-approval"
	default:
		return "idle"
	}
}

func (m Model) Output() string { return m.output.String() }

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.layout()
		return m, nil

	case tea.KeyMsg:
		if m.state == stateAwaitingApproval {
			return m.updateApproval(msg)
		}
		switch msg.Type {
		case tea.KeyCtrlC, tea.KeyCtrlD:
			if m.state == stateRunning {
				return m, nil // 跑的时候别退出
			}
			return m, tea.Quit
		case tea.KeyEnter:
			if m.state != stateIdle {
				return m, nil
			}
			line := strings.TrimSpace(m.input.Value())
			if line == "" {
				return m, nil
			}
			m.input.SetValue("")
			m.output.WriteString(cyanStyle.Render("❯ ") + line + "\n")
			m.state = stateRunning
			m.setConversation()
			return m, tea.Batch(m.runAgent(line), m.spin.Tick)
		}

	case AppendMsg:
		m.output.WriteString(string(msg))
		m.setConversation()
		m.viewport.GotoBottom()
		return m, nil

	case agentDoneMsg:
		m.state = stateIdle
		if msg.err != nil {
			m.output.WriteString(dimStyle.Render(fmt.Sprintf("error: %v", msg.err)) + "\n")
			m.setConversation()
			m.viewport.GotoBottom()
		}
		return m, nil

	case ApprovalRequest:
		m.state = stateAwaitingApproval
		m.approvalPrompt = msg.Prompt
		m.approvalDetail = msg.Detail
		m.approvalReply = msg.Reply
		return m, nil
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m Model) updateApproval(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var answer bool
	switch msg.String() {
	case "y", "Y":
		answer = true
	case "n", "N", "esc", "ctrl-c", "enter":
		answer = false
	default:
		return m, nil // 其他键忽略
	}
	if m.approvalReply != nil {
		m.approvalReply <- answer
		m.approvalReply = nil
	}
	m.state = stateIdle
	m.approvalPrompt, m.approvalDetail = "", ""
	return m, nil
}

func (m Model) runAgent(line string) tea.Cmd {
	runner := m.runner
	return func() tea.Msg {
		err := runner(context.Background(), line)
		return agentDoneMsg{err: err}
	}
}

func (m *Model) setConversation() {
	m.viewport.SetContent(m.output.String())
}

func (m *Model) layout() {
	inputHeight := 1
	statusHeight := 1
	vpHeight := m.height - inputHeight - statusHeight
	if vpHeight < 3 {
		vpHeight = 3
	}
	m.viewport.Width = m.width
	m.viewport.Height = vpHeight
	m.input.Width = m.width - 4
}

func (m Model) View() string {
	var b strings.Builder
	b.WriteString(m.viewport.View())
	b.WriteString("\n")

	left := " idle"
	switch m.state {
	case stateRunning:
		left = " " + m.spin.View() + " thinking…"
	case stateAwaitingApproval:
		left = " awaiting approval"
	}
	right := ""
	if m.usageFunc != nil {
		right = m.usageFunc() + " "
	}
	gap := m.width - lipgloss.Width(left) - lipgloss.Width(right)
	if gap < 0 {
		gap = 0
	}
	b.WriteString(statusStyle.Render(left + strings.Repeat(" ", gap) + right))
	b.WriteString("\n")

	if m.state == stateAwaitingApproval {
		mark := lipgloss.NewStyle().Foreground(lipgloss.Color("220")).Bold(true).Render(" ? ")
		hint := mark + m.approvalPrompt
		if m.approvalDetail != "" {
			hint += dimStyle.Render("  (diff shown below)")
		}
		hint += dimStyle.Render("  (y/n)")
		b.WriteString(hint)
	} else {
		b.WriteString(m.input.View())
	}
	return b.String()
}

var send func(tea.Msg)

func SetSender(f func(tea.Msg)) { send = f }

func Post(text string) {
	if send != nil {
		send(AppendMsg(text))
	}
}

func RequestApproval(prompt, detail string) bool {
	if send == nil {
		return true
	}
	reply := make(chan bool)
	send(ApprovalRequest{Prompt: prompt, Detail: detail, Reply: reply})
	select {
	case a := <-reply:
		return a
	case <-time.After(5 * time.Minute):
		return false
	}
}

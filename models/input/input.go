package input

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"strings"
	"bytes"
)

var defaultCursorStyle = lipgloss.NewStyle().Background(lipgloss.Color("100"))
var defaultInputStyle = lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
var defaultTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("200"))

const defaultWidth = 20
const defaultHeight = 1
const defaultSecretChar = '*'
const defaultCursorChar = '|'

type InputMode int

const (
	Normal InputMode = iota
	Insert
)

type Model struct {
	input []byte
	cursor int
	title string
	description string
	secret bool
	secretChar byte
	cursorChar byte

	focus bool
	mode InputMode

	width int
	height int

	maxWidth int

	cursorStyle lipgloss.Style
	inputStyle lipgloss.Style
	titleStyle lipgloss.Style
}

func New() Model  {
	return Model{
		input: make([]byte, 0),
		mode: Insert,
		secret: false,
		secretChar: defaultSecretChar,
		cursor: 0,
		cursorChar: defaultCursorChar,
		focus: false,
		width: defaultWidth,
		height: defaultHeight,
		cursorStyle: defaultCursorStyle,
		inputStyle: defaultInputStyle.Width(defaultWidth),
		titleStyle: defaultTitleStyle,
	}
}

func (m *Model) Focused() bool  {
	return m.focus
}

func (m *Model) Blur() {
	m.focus = false
}

type setFocusCmd struct{}

func (m *Model) Focus() {
	m.focus = true
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Value() string {
	return string(m.input)
}

func (m *Model) decCursor() {
	if m.cursor-1 > -1 {
		m.cursor--
	}
}

func (m *Model) incCursor() {
	if m.cursor+1 < min(len(m.input), m.width) {
		m.cursor++
	}
}

func (m *Model) trimByte() {
	if m.cursor-1 > -1 {
		m.cursor--
		m.input = m.input[:len(m.input)-1]
	}
}

func (m *Model) SetSecret(b bool) {
	m.secret = b
}

func (m *Model) Secret() bool {
	return m.secret
}

func (m *Model) appendByte(char byte) {
	m.cursor++
	m.input = append(m.input, char)
}

func (m *Model) clearInput() {
	m.cursor = 0
	m.input = []byte{}
}

func (m *Model) SetContent(s string) {
	m.cursor = 0
	m.input = []byte(s)
	m.cursor = len(m.input)
}

func (m *Model) clearUntilWhiteSpace() {
	if len(m.input) == 0 {
		return
	}

	idx := bytes.LastIndexByte(m.input, ' ')

	if idx <= 0 {
		m.clearInput()
		return
	}

	m.input = m.input[:idx]
	m.cursor = len(m.input)
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.Type {
		case tea.KeyBackspace:
			if !m.focus {
				break
			}
			m.trimByte()
		case tea.KeySpace:
			if !m.focus {
				break
			}
			m.appendByte(' ')
		case tea.KeyRunes:
			if !m.focus {
				break
			}
			m.appendByte(byte(msg.Runes[0]))
		case tea.KeyCtrlW:
			if !m.focus {
				break
			}
			m.clearUntilWhiteSpace()
		case tea.KeyCtrlU:
			if !m.focus {
				break
			}
			m.clearInput()
		}
	}
	return m, nil
}

func (m Model) View() string {
	builder := &strings.Builder{}
	cursorView := m.cursorStyle.Render(string(m.cursorChar))
	inputView := m.inputView() + cursorView
	builder.WriteString(m.inputStyle.Render(inputView))
	//builder.WriteString(inputView)
	return builder.String()
}

func (m Model) inputView() string {
	var s string

	if !m.secret {
		s = string(m.input)
	} else {
		s = strings.Repeat(string(m.secretChar), len(m.input))
	}

	return s
}

package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/arjunmoola/go-spotify/models/textinput"
	"github.com/charmbracelet/lipgloss"
	"strings"
)

type buttonStyles struct {
	border lipgloss.Style
	borderHighlight lipgloss.Style
	text lipgloss.Style
	textHighlight lipgloss.Style
}

func defaultButtonStyles() buttonStyles {
	border := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
	text := lipgloss.NewStyle()
	return buttonStyles {
		border: border,
		borderHighlight: border.Foreground(lipgloss.Color("200")),
		text: text,
		textHighlight: text.Foreground(lipgloss.Color("200")),
	}
}

type buttonModel struct {
	text string
	width int
	height int
	focus bool
	styles buttonStyles
}

func newButton(text string) buttonModel {
	return buttonModel{
		text: text,
		focus: false,
		styles: defaultButtonStyles(),
	}
}

func (b buttonModel) Init() tea.Cmd {
	return nil
}

func (b buttonModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return b, nil
}

func (b buttonModel) View() string {
	var s string
	if b.focus {
		s += b.styles.borderHighlight.Render(b.text)
	} else {
		s += b.styles.border.Render(b.text)
	}

	return s
}

type loginModelStyles struct {
	defaultStyle lipgloss.Style
	cursor lipgloss.Style
	errMsg lipgloss.Style
}

func defaultLoginModelStyles() loginModelStyles {
	return loginModelStyles {
		defaultStyle: lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()),
		cursor: lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).Foreground(lipgloss.Color("200")),
		errMsg: lipgloss.NewStyle().Foreground(lipgloss.Color("red")),
	}
}

type loginModel struct {
	inputs []textinput.Model
	labels []string
	doneButton buttonModel
	cursor int
	focus bool
	errMsg string
	styles loginModelStyles
}

func newLoginModel() loginModel {
	inputs := make([]textinput.Model, 3)
	labels := make([]string, 3)

	inputs[0] = textinput.New()
	inputs[0].Input.Placeholder = "Client Id"
	labels[0] = "clientId"

	inputs[1] = textinput.New()
	inputs[1].Input.Placeholder = "Client Secret"
	labels[1] = "clientSecret"

	inputs[2] = textinput.New()
	inputs[2].Input.Placeholder = "Redirect Uri"
	labels[2] = "redirectUri"

	button := newButton("done")
	styles := defaultLoginModelStyles()


	return loginModel{
		inputs: inputs,
		labels: labels,
		doneButton: button,
		cursor: 0,
		styles: styles,
	}
}

func (m loginModel) Init() tea.Cmd {
	return nil
}

func (m loginModel) Update(msg tea.Msg) (loginModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "k":
			m.cursor--
			m.cursor = max(m.cursor, 0)
		case "j":
			m.cursor++
			m.cursor = min(len(m.inputs), m.cursor)
			
			if m.cursor == len(m.inputs) {
				m.doneButton.focus = true
			}
		}
	}

	var b Batch

	for i := range m.inputs {
		input, cmd := m.inputs[i].Update(msg)
		m.inputs[i] = input.(textinput.Model)
		b.Append(cmd)
	}

	return m, b.Cmd()
}

func (m loginModel) View() string {
	var builder strings.Builder

	for i, input := range m.inputs {
		if i == m.cursor {
			builder.WriteString(m.styles.cursor.Render(input.View()))
		} else {
			builder.WriteString(m.styles.defaultStyle.Render(input.View()))
		}
		builder.WriteRune('\n')
	}

	builder.WriteString(m.doneButton.View())

	return builder.String()
}

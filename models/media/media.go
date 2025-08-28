package media

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"time"
)

type Styles struct {
	buttonStyle lipgloss.Style
	progressStyle lipgloss.Style
	volumeStyle lipgloss.Style
	artistStyle lipgloss.Style
	selectedItem lipgloss.Style
	buttonPressedStyle lipgloss.Style
}

func defaultStyles() Styles {
	buttonStyle := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).Height(2)
	return Styles{
		buttonStyle: buttonStyle,
		//progressStyle: lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder(),
		artistStyle: lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()),
		selectedItem: buttonStyle.Foreground(lipgloss.Color("200")),
		buttonPressedStyle: buttonStyle.Background(lipgloss.Color("200")),
	}
}

type Model struct {
	styles Styles
	width int
	height int
	playing bool
	artist string
	buttons []string
	idx int
	progress progress.Model
	percent float64
	buttonPressDur time.Duration
	pressed map[int]bool
}

func New(opts ...string) Model {
	progress := progress.New()

	return Model{
		buttons: opts,
		idx: 0,
		progress: progress,
		buttonPressDur: time.Millisecond*500,
		pressed: make(map[int]bool),
		styles: defaultStyles(),
	}
}

type clearPress struct{ idx int}

func (m *Model) SetWidth(w int) {
	m.width = w
}

func (m *Model) PressButton() tea.Cmd {
	m.pressed[m.idx] = true
	idx := m.idx
	return tea.Tick(m.buttonPressDur, func(_ time.Time) tea.Msg {
		return clearPress{ idx }
	})
}

func (m *Model) SetButtonPressDuration(d time.Duration) {
	m.buttonPressDur = d
}

func (m *Model) SetProgress(p float64) {
	m.progress.SetPercent(p)
}

func (m *Model) SetPlaying(b bool) {
	m.playing = b
}

func (m *Model) SetArtist(artist string) {
	m.artist = artist
}

func (m *Model) SetPercent(percent float64) {
	m.percent = percent
}

func (m *Model) SelectedItem() string {
	return m.buttons[m.idx]
}

func (m *Model) SetItem(item string, idx int) {
	m.buttons[idx] = item
}

func (m *Model) Index() int {
	return m.idx
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) Next() {
	if m.idx + 1 < len(m.buttons) {
		m.idx++
	}
}

func (m *Model) Prev() {
	if m.idx -1 > -1 {
		m.idx--
	}
}

func (m *Model) ClearPressed(idx int) {
	m.pressed[idx] = false
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "l":
			m.Next()
		case "h":
			m.Prev()
		}
	case clearPress:
		m.ClearPressed(msg.idx)
	}

	return m, nil
}

func (m Model) View() string {
	artistView := m.styles.artistStyle.Render(m.artist)

	buttons := make([]string, 0, len(m.buttons))

	for i, button := range m.buttons {
		var buttonView string
		if i == m.idx {
			buttonView = m.styles.selectedItem.Render(button)
		} else if m.pressed[i] {
			buttonView = m.styles.buttonPressedStyle.Render(button)
		} else {
			buttonView = m.styles.buttonStyle.Render(button)
		}
		buttons = append(buttons, buttonView)
	}
	buttonsView := lipgloss.JoinHorizontal(lipgloss.Center, buttons...)
	s := lipgloss.JoinVertical(lipgloss.Left, buttonsView, m.progress.ViewAs(m.percent))
	s = lipgloss.JoinHorizontal(lipgloss.Left, artistView, s)

	return lipgloss.NewStyle().BorderStyle(lipgloss.HiddenBorder()).Width(m.width-5).Render(s)
}

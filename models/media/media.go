package media

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"time"
	"fmt"
)

type Styles struct {
	buttonStyle lipgloss.Style
	progressStyle lipgloss.Style
	volumeStyle lipgloss.Style
	artistStyle lipgloss.Style
	selectedItem lipgloss.Style
	buttonPressedStyle lipgloss.Style
	modelStyle lipgloss.Style
	buttonGroup lipgloss.Style
	mediaStyle lipgloss.Style
}

func defaultStyles() Styles {
	buttonStyle := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()).Height(1)
	modelStyle := lipgloss.NewStyle().BorderStyle(lipgloss.HiddenBorder()).Align(lipgloss.Center).Height(2)
	buttonGroup := lipgloss.NewStyle().Margin(1)
	return Styles{
		buttonStyle: buttonStyle,
		//progressStyle: lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder(),
		artistStyle: lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder()),
		selectedItem: buttonStyle.Foreground(lipgloss.Color("200")),
		buttonPressedStyle: buttonStyle.Background(lipgloss.Color("200")),
		modelStyle: modelStyle,
		buttonGroup: buttonGroup,
		mediaStyle: lipgloss.NewStyle(),
	}
}

type Model struct {
	styles Styles
	width int
	height int
	playing bool
	track string
	artist string
	buttons []string
	idx int
	progress progress.Model
	volumePercent int
	percent float64
	buttonPressDur time.Duration
	pressed map[int]bool

	enabled bool
	showArtist bool
	showVolume bool

	artistViewWidth int
	volumeViewWidth int
}

func New(opts ...string) Model {
	progress := progress.New()

	return Model{
		buttons: opts,
		idx: 0,
		progress: progress,
		buttonPressDur: time.Millisecond*250,
		pressed: make(map[int]bool),
		styles: defaultStyles(),
	}
}

type clearPress struct{ idx int}

func (m *Model) ShowArtist() bool {
	return m.showArtist
}

func (m *Model) EnabledArtistView()  {
	m.showArtist = true
}

func (m *Model) DisableArtistView() {
	m.showArtist = false
}

func (m *Model) EnableVolumeView() {
	m.showVolume = true
}

func (m *Model) DisabledVolumeView() {
	m.showVolume = false
}

func (m *Model) EnableButtons() {
	m.enabled = true
}

func (m *Model) DisableButtons() {
	m.enabled = false
}

func (m *Model) ButtonsEnabled() bool {
	return m.enabled
}

func (m *Model) SetWidth(w int) {
	m.width = w
	//m.progress.Width = m.width
	m.progress.Width = int(float64(m.width)*0.98)
	m.artistViewWidth = w*2/10
	m.volumeViewWidth = w*2/10
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

func (m *Model) SetVolumePercent(p int) {
	m.volumePercent = p
}

func (m *Model) SetPlaying(b bool) {
	m.playing = b
}

func (m *Model) SetMediaInfo(trackName string, artistName string) {
	m.track = trackName
	m.artist = artistName
}

func (m *Model) SetArtist(artist string, track string) {
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

func renderButton(m Model, i int, button string) string {
	var buttonView string
	if i == m.idx {
		buttonView = m.styles.selectedItem.Render(button)
	} else {
		buttonView = m.styles.buttonStyle.Render(button)
	}

	if m.pressed[i] {
		buttonView = m.styles.buttonPressedStyle.Render(button)
	}

	return buttonView
}

func (m Model) renderProgressOnly() string {
	//modelStyle := m.styles.modelStyle.Width(m.width)
	//return modelStyle.Render(m.progress.ViewAs(m.percent))
	return m.progress.ViewAs(m.percent)
}

func (m Model) renderAll() string {
	artistView  := lipgloss.JoinVertical(lipgloss.Left, m.track, m.artist)
	artistView = m.styles.artistStyle.Width(m.artistViewWidth).Render(artistView)

	mediaButtons := make([]string, 0, 3)
	volumeButtons := make([]string, 0, 2)

	for i := range 3 {
		mediaButtons = append(mediaButtons, renderButton(m, i, m.buttons[i]))
	}

	for i := 3; i < len(m.buttons); i++ {
		volumeButtons = append(volumeButtons, renderButton(m, i, m.buttons[i]))
	}

	volume := fmt.Sprintf("%d%%", m.volumePercent)
	mediaButtonsView := lipgloss.JoinHorizontal(lipgloss.Left, mediaButtons...)
	volumeButtonsView := lipgloss.JoinHorizontal(lipgloss.Right, volumeButtons...)
	volumeView := lipgloss.JoinVertical(lipgloss.Center, volumeButtonsView, volume)

	mediaButtonsView = m.styles.buttonGroup.Render(mediaButtonsView)
	volumeButtonsView = m.styles.buttonGroup.Width(m.volumeViewWidth).Render(volumeView)

	mediaView := m.styles.mediaStyle.Width(m.width).Render(lipgloss.JoinVertical(lipgloss.Center, mediaButtonsView, m.progress.ViewAs(m.percent)))

	modelView := m.styles.modelStyle.Width(m.width-5).Height(3)

	//mediaView = lipgloss.PlaceHorizontal(1-w/8, lipgloss.Left, mediaView)
	//artistView = lipgloss.PlaceHorizontal(1-w/2, lipgloss.Center, artistView)
	//volumeButtonsView = lipgloss.PlaceHorizontal(1-w/8, lipgloss.Left, volumeButtonsView)

	s := modelView.Render(lipgloss.JoinHorizontal(lipgloss.Left, artistView, mediaView, volumeButtonsView))

	return modelView.Render(s)
}

func (m Model) View() string {
	return m.renderProgressOnly()
}

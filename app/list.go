package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"go-spotify/types"
	"io"
	"fmt"
)

var (
	selectedItemStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("200"))
	defaultItemStyle = lipgloss.NewStyle()
)

func defaultKeymap() list.KeyMap {
	return list.KeyMap{
		CursorUp: key.NewBinding(key.WithKeys("k")),
		CursorDown: key.NewBinding(key.WithKeys("j")),
	}
}

type itemDelegate struct{}

func (i itemDelegate) Height() int { return 1 }
func (i itemDelegate) Spacing() int { return 0 }
func (i itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (i itemDelegate) Render(w io.Writer, m list.Model, idx int, item list.Item) {
	var s string
	switch item := item.(type) {
	case types.Artist:
		s = item.Name
	case types.Track:
		s = item.Name
	case types.SimplifiedPlaylistObject:
		s = item.Name
	case types.Device:
		s = item.Name
	}

	var f func(s ...string) string

	if m.Index() == idx {
		f = selectedItemStyle.Render
	} else {
		f = defaultItemStyle.Render
	}

	fmt.Fprint(w, f(s))
}

func defaultList(items []list.Item) list.Model {
	l := list.New(items, itemDelegate{}, 10, 10)
	l.SetFilteringEnabled(false)
	l.SetShowFilter(false)
	l.SetShowHelp(false)
	l.SetShowTitle(false)
	l.SetShowStatusBar(false)
	l.SetShowPagination(false)
	l.InfiniteScrolling = true
	l.Styles.Title = lipgloss.NewStyle().Foreground(lipgloss.Color("200"))
	l.KeyMap = defaultKeymap()
	return l
}

type List struct {
	l list.Model
	focus bool
}

func NewList(items []list.Item) List {
	l := defaultList(items)
	return List{
		l: l,
		focus: false,
	}
}

func ToItems[S []T, T list.Item](i S) []list.Item {
	items := make([]list.Item, 0, len(i))

	for _, item := range i {
		items = append(items, item)
	}

	return items
}

func (l *List) SetListDimensions(h, w int) {
	l.l.SetHeight(h)
	l.l.SetWidth(w)
}

func SetItems[S []T, T list.Item](l *List, i S) tea.Cmd {
	items := make([]list.Item, 0, len(i))

	for _, item := range i {
		items = append(items, item)
	}

	return l.l.SetItems(items)
}

func (l *List) SetItems(items []list.Item) tea.Cmd {
	return l.l.SetItems(items)
}

func (l *List) SetShowTitle(b bool) {
	l.l.SetShowTitle(b)
}

func (l *List) SetTitle(title string) {
	l.l.Title = title
}

func (l List) Init() tea.Cmd {
	return nil
}

func (l List) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			l.l.Select(l.l.Cursor())
		}
	}
	var cmd tea.Cmd
	l.l, cmd = l.l.Update(msg)
	return l, cmd
}

func (l List) View() string {
	return l.l.View()
}

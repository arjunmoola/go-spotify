package app

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/table"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"go-spotify/types"
	//"go-spotify/models/list"
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

type Rower interface {
	Row() table.Row
}

type sidebarItem string

func (s sidebarItem) FilterValue() string {
	return ""
}

type messageItem string

func (s messageItem) FilterValue() string {
	return ""
}

func (s sidebarItem) View() string {
	return string(s)
}

type itemDelegate struct{}

func (i itemDelegate) Height() int { return 1 }
func (i itemDelegate) Spacing() int { return 0 }
func (i itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (i itemDelegate) Render(w io.Writer, m list.Model, idx int, item list.Item) {
	var s string
	switch item := item.(type) {
	case messageItem:
		s = string(item)
	case sidebarItem:
		s = string(item)
	case types.Artist:
		s = item.Name
	case types.Track:
		s = item.Name
	case types.SimplifiedPlaylistObject:
		s = item.Name
	case types.Device:
		s = item.Name
	case types.ItemUnion:
		switch item.Type {
		case "track":
			s = item.Track.Name
		case "episode":
			s = item.Episode.Name
		}
	case types.PlaylistItemUnion:
		switch item.Track.Type {
		case "track":
			s = item.Track.Track.Name
		case "episode":
			s = item.Track.Episode.Name
		}
	}

	var f func(s ...string) string

	if m.Index() == idx {
		f = selectedItemStyle.Render
	} else {
		f = defaultItemStyle.Render
	}

	fmt.Fprint(w, f(s))
}

type nestedItemDelegate struct{}
func (i nestedItemDelegate) Height() int { return 1 }
func (i nestedItemDelegate) Spacing() int { return 0 }
func (i nestedItemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (i nestedItemDelegate) Render(w io.Writer, m list.Model, idx int, item list.Item) {
	var s string
	switch item := item.(type) {
	case NestedListItem:
		if !item.selected {
			s = item.title
		}
	case messageItem:
		s = string(item)
	case sidebarItem:
		s = string(item)
	case types.Artist:
		s = item.Name
	case types.Track:
		s = item.Name
	case types.SimplifiedPlaylistObject:
		s = item.Name
	case types.Device:
		s = item.Name
	case types.ItemUnion:
		switch item.Type {
		case "track":
			s = item.Track.Name
		case "episode":
			s = item.Episode.Name
		}
	case types.PlaylistItemUnion:
		switch item.Track.Type {
		case "track":
			s = item.Track.Track.Name
		case "episode":
			s = item.Track.Episode.Name
		}
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

func (l List) FilterValue() string {
	return ""
}

type nestedItemdelegate struct{}



type NestedListItem struct {
	l List
	title string
	selected bool
	idx int
}

func (n NestedListItem) FilterValue() string {
	return ""
}

type NestedList struct {
	l list.Model
}


type Queue struct {
	l List
	currentlyPlaying string
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

type Table[T Rower] struct {
	title string
	items []T
	t table.Model
}

func NewTable[T Rower]() Table[T] {
	t := table.New()
	columns := []table.Column{
		{ Title: "Name", Width: 30 },
		{ Title: "Artist", Width: 30 },
		{ Title: "Album", Width: 20 },
		{ Title: "Duration", Width: 20 },
	}
	t.SetColumns(columns)
	return Table[T] {
		t: t,
	}
}

func (t *Table[T]) Width() int {
	return t.t.Width()
}

func (t *Table[T]) Height() int {
	return t.t.Height()
}

func (t *Table[T]) SetTitle(title string) {
	t.title = title
}

func SetTableItems[T Rower](t *Table[T], s []T) {
	t.items = s
	t.setRows(s)
}

func toRows[S []T, T Rower](s S) []Rower {
	items := make([]Rower, 0, len(s))

	for _, item := range s {
		items = append(items, Rower(item))
	}

	return items
}


func (t *Table[T]) setRows(items []T) {
	rows := make([]table.Row, 0, len(items))

	for _, item := range items {
		rows = append(rows, item.Row())
	}

	t.t.SetRows(rows)
}

func (t *Table[T]) SetWidth(w int) {
	t.t.SetWidth(w)
}

func (t *Table[T]) SetHeight(h int) {
	t.t.SetHeight(h)
}

func (t *Table[T]) SetTableDimensions(w, h int) {
	t.SetWidth(w)
	t.SetHeight(h)
}

func (t Table[T]) Cursor() int {
	return t.t.Cursor()
}

func (t Table[T]) SelectedItem() Rower {
	idx := t.Cursor()
	return t.items[idx]
}

func (t Table[T]) Init() tea.Cmd {
	return nil
}

func (t Table[T]) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	t.t, cmd = t.t.Update(msg)
	return t, cmd
}

func (t Table[T]) View() string {
	if t.title != "" {
		return lipgloss.JoinVertical(lipgloss.Center, t.title, t.t.View())
	}
	return t.t.View()
}

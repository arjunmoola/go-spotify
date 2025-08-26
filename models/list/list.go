package list

import (
	"io"
	tea "github.com/charmbracelet/bubbletea"
	"strings"
	//"fmt"
	"github.com/charmbracelet/lipgloss"
	"slices"
)

var (
	defaultItemStyle = lipgloss.NewStyle()
	defaultSelectedItemStyle = lipgloss.NewStyle().Background(lipgloss.Color("100")).Foreground(lipgloss.Color("200"))
	defaultTitleStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("200"))
)

type NavigationType int

const (
	Vertical NavigationType = iota
	Horizontal
)

type Item interface {
	View(w io.Writer, itemIdx int, cursorIdx int)
}

type Model struct {
	items []Item
	cursor int
	title string
	focus bool
	navType NavigationType

	nextKey string
	prevKey string

	itemStyle lipgloss.Style
	selectedItemStyle lipgloss.Style
	titleStyle lipgloss.Style
}

func New(items ...Item) Model {
	return Model{
		items: items,
		cursor: 0,
		focus: false,
		selectedItemStyle: defaultSelectedItemStyle,
		titleStyle: defaultTitleStyle,
		navType: Vertical,
		nextKey: "j",
		prevKey: "k",
	}
}

func (m *Model) SetItems(items []Item) {
	m.items = items
}

func (m *Model) AppendItem(item Item) {
	m.items = append(m.items, item)
}

func (m *Model) InsertItemAt(item Item, i int) {
	m.items = slices.Insert(m.items, i, item)
}

func (m *Model) Next() {
	if m.cursor + 1 < len(m.items) {
		m.cursor++
	}
}

func (m *Model) Prev() {
	if m.cursor -1 > -1 {
		m.cursor--
	}
}

func (m Model) SelectedValue() Item {
	return m.items[m.cursor]
}

func (m *Model) SetFocus(b bool) {
	m.focus = b
}

func (m *Model) SetNavigation(n NavigationType) {
	if n == Vertical {
		m.nextKey = "k"
		m.prevKey = "j"
	} else {
		m.nextKey = "l"
		m.prevKey = "h"
	}
}

func (m Model) updateFocus(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case m.nextKey:
			m.Next()
		case m.prevKey:
			m.Prev()
		}
	}

	return m, nil
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if m.focus {
		return m.updateFocus(msg)
	}
	return m, nil
}

func (m Model) View() string {
	builder := &strings.Builder{}

	for i, item := range m.items {
		item.View(builder, i, m.cursor)
	}

	return builder.String()
}

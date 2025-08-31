package list

import (
	"fmt"
	tea "github.com/charmbracelet/bubbletea"
	//"github.com/charmbracelet/bubbles/list"
	//"github.com/charmbracelet/bubbles/key"
	//"strings"
	"github.com/charmbracelet/lipgloss"
	//"io"
)

type Styles struct {
	item lipgloss.Style
	nestedItem lipgloss.Style
	itemTitle lipgloss.Style
	cursorStyle lipgloss.Style
	model lipgloss.Style
	selectedItem lipgloss.Style
	nestedSelectedItem lipgloss.Style
	titleStyle lipgloss.Style
	selectedTitleStyle lipgloss.Style
	nestedSelectedStyle lipgloss.Style
}

func defaultStyles() Styles {
	itemStyle := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder())
	titleStyle := lipgloss.NewStyle()
	nestedItem := lipgloss.NewStyle().PaddingLeft(4)
	return Styles {
		item: itemStyle,
		selectedItem: itemStyle.BorderForeground(lipgloss.Color("200")),
		titleStyle: titleStyle,
		selectedTitleStyle: titleStyle.Foreground(lipgloss.Color("200")),
		nestedItem: nestedItem,
		nestedSelectedItem: nestedItem.Foreground(lipgloss.Color("200")),
		model: lipgloss.NewStyle(),
	}
}

type Viewer interface {
	View() string
}

type Item struct {
	items []Viewer
	idx int
	expanded bool
	expandable bool
	title string
}

func NewItem(title string, items []Viewer, b bool) Item {
	return Item{
		title: title,
		items: items,
		expandable: b,
	}
}

func (i *Item) SetItems(items []Viewer) {
	i.items = items
}

func (i *Item) SetExpanability(b bool) {
	i.expandable = b
}

func SetItems[S []T, T Viewer](i *Item, s S) {
	v := make([]Viewer, 0, len(s))

	for _, item := range s {
		v = append(v, item)
	}

	i.items = v
}


func (item Item) Render(l NestedList, idx int) string {
	titleView := ""
	rowsView := ""
	itemView := ""
	if item.Expanded() {
		var rows []string
		for i, s := range item.items {
			var rowView string
			if i == item.Index() && idx == l.Index() {
				rowView = l.Styles.nestedSelectedItem.Render(s.View())
			} else {
				rowView = l.Styles.nestedItem.Render(s.View())
			}
			rows = append(rows, rowView)
		}
		rowsView = lipgloss.JoinVertical(lipgloss.Left, rows...)
	}
	if idx == l.Index() {
		titleView = l.Styles.selectedTitleStyle.Render(item.title)
		if rowsView != "" {
			itemView = lipgloss.JoinVertical(lipgloss.Left, titleView, rowsView)
		} else {
			itemView = titleView
		}
	} else {
		titleView = l.Styles.titleStyle.Render(item.title)
		if rowsView != "" {
			itemView = lipgloss.JoinVertical(lipgloss.Left, titleView, rowsView)
		} else {
			itemView = titleView
		}
	}

	return itemView
}

func (i Item) Len() int {
	return len(i.items)
}

func (i Item) SelectedItem() Viewer {
	if i.IsEmpty() {
		return nil
	}

	return i.items[i.idx]
}

func (i Item) Expanded() bool {
	return i.expanded
}

func (i Item) Title() string {
	return i.title
}

func (i Item) Expandable() bool {
	return i.expandable
}

func (i *Item) Expand() {
	if i.Expanded() {
		return
	}
	i.expanded = true
}

func (i *Item) Collapse() {
	if i.Expanded() {
		i.expanded = false
	}
}

func (i Item) Index() int {
	return i.idx
}

func (i *Item) Next() bool {
	if len(i.items) == 0 {
		return false
	}

	i.idx++
	if i.idx >= len(i.items) {
		i.idx = len(i.items)-1
		return false
	}
	return true

}

func (i *Item) Prev() bool {
	if len(i.items) == 0 {
		return false
	}

	i.idx--
	if i.idx < 0 {
		i.idx = 0
		return false
	}
	return true
}

func (i *Item) SetIndex(idx int) {
	if len(i.items) == 0 {
		return
	}

	i.idx = idx
}

func (i *Item) IsEmpty() bool {
	return len(i.items) == 0
}

func (i *Item) GotoBottom() {
	if i.IsEmpty() {
		return
	}

	i.idx = len(i.items)-1
}

func (i *Item) GotoTop() {
	if i.IsEmpty() {
		return
	}
	i.idx = 0
}

type NestedList struct {
	items []Item
	idx int
	Styles Styles
	height int
	width int
}

func (l *NestedList) SetHeight(h int) {
	l.height = h
}

func (l *NestedList) SetWidth(w int) {
	l.width = w
}

func New(items []Item) NestedList {
	return NestedList{
		items: items,
		Styles: defaultStyles(),
	}
}

func NewNestedList(titles []string) NestedList {
	items := make([]Item, 0, len(titles))

	for _, title := range titles {
		items = append(items, Item{
			title: title,
		})
	}

	return NestedList{
		items: items,
		Styles: defaultStyles(),
	}
}

func (l NestedList) Init() tea.Cmd {
	return nil
}

func (l NestedList) SelectedItem() Item {
	return l.items[l.idx]
}

func (l *NestedList) SetItems(title string, items []Viewer) {
	var idx int
	var found bool

	for i, item := range l.items {
		if item.title == title {
			idx = i
			found = true
			break
		}
	}

	if !found {
		return
	}

	item := l.GetItem(idx)
	item.SetItems(items)
	l.SetItem(item, idx)
}

func (l *NestedList) Expand() {
	item := l.SelectedItem()
	idx := l.Index()

	if !item.Expanded() && item.Expandable() {
		item.Expand()
	}
	l.SetItem(item, idx)
}

func (l *NestedList) Expanded() bool {
	return l.SelectedItem().Expanded()
}

func (l *NestedList) Expandable() bool {
	return l.SelectedItem().Expandable()
}

func (l *NestedList) Collapse() {
	item := l.SelectedItem()
	idx := l.Index()

	if item.Expanded() {
		item.Collapse()
	}
	l.SetItem(item, idx)
}

func (l *NestedList) SetItem(item Item, i int) {
	l.items[i] = item
}

func (l NestedList) GetItem(i int) Item {
	return l.items[i]
}

func (l NestedList) Index() int {
	return l.idx
}

func (l NestedList) Pair() (string, Viewer) {
	item := l.SelectedItem()

	title := item.title

	if item.IsEmpty() {
		return title, nil
	}

	return title, item.SelectedItem()
}

func (l *NestedList) Next() {
	item := l.SelectedItem()

	curIdx := l.idx

	if item.Expanded() {
		if !item.Next() {
			if !l.next() {
				return
			}
			item := l.items[l.idx]
			if item.Expanded() {
				item.GotoTop()
			}
			l.items[l.idx] = item
		} else {
			l.SetItem(item, curIdx)
		}
	} else {
		l.next()
		item := l.items[l.idx]
		if item.Expanded() {
			item.GotoTop()
		}
		l.items[l.idx] = item
	}
}

func (l *NestedList) next() bool {
	if l.idx+1 < len(l.items) {
		l.idx++
		return true
	}
	return false
}

func (l *NestedList) prev() bool {
	if l.idx-1 > -1 {
		l.idx--
		return true
	}
	return false
}

func (l *NestedList) Prev() {
	item := l.SelectedItem()
	curIdx := l.idx

	if item.Expanded() {
		if !item.Prev() {
			if !l.prev() {
				return
			}
			if l.items[l.idx].Expanded() {
				l.items[l.idx].GotoBottom()
			}
		} else {
			l.SetItem(item, curIdx)
		}
	} else {
		l.prev()
		item := l.items[l.idx]
		if item.Expanded() {
			item.GotoBottom()
		}
		l.items[l.idx] = item
	}
}

func (l NestedList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "j":
			l.Next()
		case "k":
			l.Prev()
		}
	}

	return l, nil
}

func (l NestedList) View() string {
	//builder := &strings.Builder{}
	views := make([]string, 0, len(l.items))
	for i, item := range l.items {
		views = append(views, item.Render(l, i))
	}
	view := lipgloss.JoinVertical(lipgloss.Left, views...)
	view = lipgloss.JoinVertical(lipgloss.Left, view, fmt.Sprintf("index: %d", l.idx))
	return l.Styles.model.Width(l.width).Height(l.height).Render(view)
}

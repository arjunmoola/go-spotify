package grid
import (
	"strings"
	"github.com/charmbracelet/lipgloss"
	"slices"
	"iter"
	tea "github.com/charmbracelet/bubbletea"
)

type EmptyCell struct {}

func (e EmptyCell) Init() tea.Cmd {
	return nil
}

func (e EmptyCell) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	return e, nil
}

func (e EmptyCell) View() string {
	return ""
}



type Styles struct {
	Cell lipgloss.Style
	CurrentCell lipgloss.Style
	Focus lipgloss.Style
}

func DefaultStyle() Styles {
	cellStyle := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
	currentCell := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder()).BorderForeground(lipgloss.Color("200"))
	focusStyle := currentCell.BorderStyle(lipgloss.DoubleBorder())

	return Styles{
		Cell: cellStyle,
		CurrentCell: currentCell,
		Focus: focusStyle,
	}
}

type Model struct {
	rows int
	cols int
	cellWidth int
	cellHeight int
	models []Row
	cursor int
	pos Position
	focus bool
	Styles Styles
}

func New() Model {
	return Model{
		cellWidth: 10,
		cellHeight: 10,
		cursor: 0,
		Styles: DefaultStyle(),
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m *Model) SetFocus(b bool) {
	m.focus = b
}

func (m Model) Focus() bool {
	return m.focus
}

func (m *Model) SetStyles(styles Styles) {
	m.Styles = styles
}

func (m *Model) SetCellStyle(style lipgloss.Style) {
	m.Styles.Cell = style
}

func (m *Model) SetCurrentCellStyle(style lipgloss.Style) {
	m.Styles.CurrentCell = style
}

func (m *Model) SetCellDimensions(width, height int) {
	m.cellWidth = width
	m.cellHeight = height
}

func (m *Model) SetModel(model tea.Model, i, j int) {
	m.models[i][j] = model
}

type Row []tea.Model

func (r *Row) Append(model tea.Model) {
	*r = append(*r, model)
}

func (r Row) Len() int {
	return len(r)
}

func NewRow(models ...tea.Model) Row {
	return Row(models)
}

func (m *Model) AppendRow(model tea.Model, rowIdx int) Position {
	m.models[rowIdx] = append(m.models[rowIdx], model)
	colIdx := len(m.models[rowIdx])
	return Position{ rowIdx, colIdx-1 }
}

func (m *Model) Append(row Row) int {
	m.models = append(m.models, row)
	return len(m.models)-1
}

func (m *Model) SetModelPos(model tea.Model, pos Position) {
	i, j := pos.Row, pos.Col
	m.models[i][j] = model
}

func (m Model) At(pos Position) tea.Model {
	return m.models[pos.Row][pos.Col]
}

type direction int 

const (
	up direction = iota
	down
	left
	right
)

type Position struct {
	Row, Col int
}

func Pos(i, j int) Position {
	return Position{ i, j }
}

func (m *Model) updateCursor(dir string) {
	pos := m.pos
	//newPos := pos

	i := pos.Row
	j := pos.Col
	newI := i
	newJ := j

	switch dir {
	case "k":
		newI--
		//newPos.Row--
	case "j":
		newI++
		//newPos.Row++
	case "h":
		newJ--
		//newPos.Col--
	case "l":
		newJ++
		//newPos.Col++
	}

	if newI < 0 || newI >= len(m.models) {
		newI = i
	}

	if newJ < 0 || newJ >= len(m.models[newI]) {
		if len(m.models[newI]) -1 > -1 {
			newJ = len(m.models[newI])-1
		} else {
			newJ = j
		}
	}

	m.pos = Position{ newI, newJ }
}

func (m Model) Cursor() Position {
	return m.pos
}

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch key := msg.String(); key {
		case "l", "j", "k", "h":
			if !m.focus {
				m.updateCursor(key)
			}
		}
	}

	var cmd tea.Cmd

	if m.focus {
		pos := m.Cursor()
		m.models[pos.Row][pos.Col], cmd = m.models[pos.Row][pos.Col].Update(msg)
	}

	return m, cmd
}

func (m Model) viewRow(i int) string {
	//builder := &strings.Builder{}
	row := m.models[i]
	selectedI, selectedJ := m.pos.Row, m.pos.Col
	views := make([]string, 0, len(row))

	for colIdx, model := range row {
		if i == selectedI && colIdx == selectedJ {
			if m.focus {
				//s = lipgloss.JoinHorizontal(lipgloss.Left, s, m.Styles.Focus.Render(model.View()))
				views = append(views, m.Styles.Focus.Render(model.View()))
			} else {
				//s = lipgloss.JoinHorizontal(lipgloss.Left, s, m.Styles.CurrentCell.Render(model.View()))
				views = append(views, m.Styles.CurrentCell.Render(model.View()))
			}
		} else {
			views = append(views, m.Styles.Cell.Render(model.View()))
		}
	}

	return lipgloss.JoinHorizontal(lipgloss.Left, views...)
}

func (m Model) renderedRows() iter.Seq[string] {
	return func(yield func(string) bool) {
		for i := range m.models {
			s := m.viewRow(i)
			if !yield(s) {
				return
			}
		}
	}
}

func (m Model) View() string {
	builder := &strings.Builder{}
	rows := slices.Collect(m.renderedRows())
	builder.WriteString(lipgloss.JoinVertical(lipgloss.Left, rows...))
	return builder.String()
}

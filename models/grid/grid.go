package grid
import (
	"strings"
	"github.com/charmbracelet/lipgloss"
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
	models []tea.Model
	cursor int
	focus bool
	Styles Styles
}

func New(rows, cols int) Model {
	models := make([]tea.Model, rows*cols)
	return Model{
		rows: rows,
		cols: cols,
		cellWidth: 10,
		cellHeight: 10,
		cursor: 0,
		models: models,
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
	//m.Styles.Cell = m.Styles.Cell.Width(width).Height(height)
	//m.Styles.CurrentCell = m.Styles.CurrentCell.Width(width).Height(height)
}

func (m *Model) SetModel(model tea.Model, i, j int) {
	m.models[i*m.cols+j] = model
}

func (m *Model) SetModelPos(model tea.Model, pos Position) {
	i, j := pos.Row, pos.Col
	m.models[i*m.cols+j] = model
}

func (m Model) At(pos Position) tea.Model {
	i := pos.Row
	j := pos.Col
	return m.models[i*m.cols+j]
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
	cursor := m.cursor
	i := cursor/m.cols
	j := cursor%m.cols
	newI := i
	newJ := j

	switch dir {
	case "k":
		newI--
	case "j":
		newI++
	case "h":
		newJ--
	case "l":
		newJ++
	}

	if newI < 0 || newI >= m.rows {
		newI = i
	}

	if newJ < 0 || newJ >= m.cols {
		newJ = j
	}

	m.cursor = newI*m.cols + newJ

}

func (m Model) Cursor() Position {
	i := m.cursor/m.cols
	j := m.cursor%m.cols
	return Position{ i, j }
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
		m.models[m.cursor], cmd = m.models[m.cursor].Update(msg)
	}

	return m, cmd
}

func (m Model) View() string {
	builder := &strings.Builder{}
	rows := make([]string, 0, m.rows)
	for i := range m.rows {
		modelViews := make([]string, 0, m.cols)
		row := m.models[i:i+m.cols]

		for j, model := range row {
			if i*m.cols+j == m.cursor {
				if m.focus {
					modelViews = append(modelViews, m.Styles.Focus.Render(model.View()))
				} else {
					modelViews = append(modelViews, m.Styles.CurrentCell.Render(model.View()))
				}
			} else {
				modelViews = append(modelViews, m.Styles.Cell.Render(model.View()))
			}
		}
		s := lipgloss.JoinHorizontal(lipgloss.Center, modelViews...)
		rows = append(rows, s)
	}

	builder.WriteString(lipgloss.JoinVertical(lipgloss.Left, rows...))

	return builder.String()
}

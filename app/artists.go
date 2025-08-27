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

type spotifyListItem interface {
	String() string
	FilterValue() string
}

type artistItem struct {
	artist *types.TopArtists
}

func (a artistItem) FilterValue() string {
	return ""
}

func (a artistItem) String() string {
	return a.artist.Name
}

type trackItem struct {
	track *types.TopTrack
}

func (t trackItem) FilterValue() string {
	return ""
}

func (t trackItem) String() string {
	return t.track.Name
}

func render(s spotifyListItem, w io.Writer, m list.Model, idx int) {
	var f func(str ...string) string

	width := m.Width()

	if m.Index() == idx {
		f = selectedItemStyle.Width(width).Render
	} else {
		f = defaultItemStyle.Width(width).Render
	}
	fmt.Fprint(w, f(s.String()))
}

type playlistItem struct {
	playlist *types.SimplifiedPlaylistObject
}

func (p playlistItem) String() string {
	return p.playlist.Name
}

func (p playlistItem) FilterValue() string {
	return ""
}

type deviceItem struct {
	device *types.Device
}

func (d deviceItem) String() string {
	return d.device.Name
}

func (d deviceItem) FilterValue() string {
	return ""
}

type itemDelegate struct{}

func (i itemDelegate) Height() int { return 1 }
func (i itemDelegate) Spacing() int { return 0 }
func (i itemDelegate) Update(_ tea.Msg, _ *list.Model) tea.Cmd { return nil }
func (i itemDelegate) Render(w io.Writer, m list.Model, idx int, item list.Item) {
	switch item := item.(type) {
	case spotifyListItem:
		render(item, w, m, idx)
	}
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

type spotifyList struct {
	list list.Model
	focus bool
	height int
	width int
}

func (s *spotifyList) SetFocus(b bool) {
	s.focus = b
}

func (s *spotifyList) SetHeight(h int) {
	s.height = h
}

func (s *spotifyList) SetWidth(w int) {
	s.width = w
}

func (s *spotifyList) SetListDimensions(h, w int) {
	s.list.SetHeight(h)
	s.list.SetWidth(w)
}

func (s spotifyList) Init() tea.Cmd {
	return nil
}

func (s spotifyList) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			s.list.Select(s.list.Cursor())
		}
	}

	var cmd tea.Cmd

	s.list, cmd = s.list.Update(msg)

	return s, cmd
}

func (s spotifyList) View() string {
	return s.list.View()
}

func spotifyListItemsToListItems(items []spotifyListItem) []list.Item {
	i := make([]list.Item, 0, len(items))

	for _, item := range items {
		i = append(i, item)
	}

	return i
}

func convertItems(t any) []list.Item {
	var items []list.Item

	switch t := t.(type) {
	case *types.UsersTopArtists:
		items = make([]list.Item, 0, len(t.Items))
		for _, item := range t.Items {
			items = append(items, artistItem{ artist: item })
		}
	case *types.UsersTopTracks:
		items = make([]list.Item, 0, len(t.Items))
		for _, item := range t.Items {
			items = append(items, trackItem{ track: item })
		}
	case *types.CurrentUsersPlaylistResponse:
		items = make([]list.Item, 0, len(t.Items))
		for _, item := range t.Items {
			items = append(items, playlistItem{ playlist: item })
		}
	case *types.AvailableDevices:
		items = make([]list.Item, 0, len(t.Devices))
		for _, item := range t.Devices {
			items = append(items, deviceItem{ device: item })
		}
	case nil:
		items = []list.Item{}
	}

	return items
}

func (s *spotifyList) SetItems(items []spotifyListItem) {
	s.list.SetItems(spotifyListItemsToListItems(items))
}

func (s *spotifyList) SetItemsFromResult(result any) tea.Cmd {
	return s.list.SetItems(convertItems(result))
}

func (s *spotifyList) SetTitle(title string) {
	s.list.Title = title
}


func (s *spotifyList) SetTitleStyle(style lipgloss.Style) {
	s.list.Styles.Title = style
}

func (s *spotifyList) SetShowTitle(b bool) {
	s.list.SetShowTitle(b)
}

func newSpotifyListModel(t any) spotifyList {
	items := convertItems(t)
	l := defaultList(items)
	return spotifyList{
		list: l,
	}
}

func createArtistListItems(artists *types.UsersTopArtists) []list.Item {
	return convertItems(artists)
}

func createTracksListItems(tracks *types.UsersTopTracks) []list.Item {
	return convertItems(tracks)
}


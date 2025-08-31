package app

import (
	"time"
	"fmt"
	"strings"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
	nested "go-spotify/models/list"
	"github.com/charmbracelet/lipgloss"
	"go-spotify/models/grid"
	"go-spotify/types"
	"go-spotify/models/media"
)

type Batch []tea.Cmd

func (b *Batch) Append(cmds... tea.Cmd) {
	for _, cmd := range cmds {
		if cmd == nil {
			continue
		}
		*b = append(*b, cmd)
	}
}

func (b Batch) Cmd() tea.Cmd {
	return tea.Batch(b...)
}

func (a *App) Init() tea.Cmd {
	var b Batch
	b.Append(GetUsersTopTracks(a))
	b.Append(GetUsersTopArtists(a))
	b.Append(GetUserProfile(a))
	b.Append(GetUsersPlaylist(a))
	b.Append(GetAvailableDevices(a))
	b.Append(GetCurrentlyPlayingCmd(a))
	b.Append(RenewRefreshTokenTick(a, a.GetAuthorizationInfo()))
	b.Append(GetUsersQueueCmd(a))
	return b.Cmd()
}

func NestedItems[S []T, T nested.Viewer](s S) []nested.Viewer {
	v := make([]nested.Viewer, 0, len(s))

	for _, item := range s {
		v = append(v, item)
	}

	return v

}

func SetSideBarItems[S []T, T nested.Viewer](a *App, title string, s S) {
	items := NestedItems(s)
	m, ok := GetModel[nested.NestedList](a, "sidebar")
	if !ok {
		return
	}
	m.SetItems(title, items)
	SetModel(a, m, "sidebar")
}

func (a *App) updateResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	var b Batch
	push := b.Append

	switch msg := msg.(type) {
	case GetUserResult:
		a.SetUser(msg.result)
		a.AppendMessage("got setProfile result")
	case GetUsersTopItems[types.Artist]:
		a.data["top_artists"] = msg.result.Items
		SetSideBarItems(a, "Top Artists", msg.result.Items)
	case GetUsersTopItems[types.Track]:
		a.data["top_tracks"] = msg.result.Items
		//SetSideBarItems(a, "Top Tracks", msg.result.Items)
	case GetUsersPlaylistsResult:
		a.data["playlists"] = msg.result.Items
		SetSideBarItems(a, "Playlists", msg.result.Items)
	case GetUsersQueueResult:
		m, _:= GetModel[List](a, "queue")
		push(SetItems(&m, msg.result.Queue))
		SetModel(a, m, "queue")
	case GetPlaylistItemsResult:
		id := msg.id
		a.data[id] = msg.result.Items
		SetTable(a, msg.result.Items)
	case GetCurrentlyPlayingTrackResult:
		if msg.result != nil {
			a.foundCurrentlyPlaying = true
			a.AppendMessage("got valid result")
		}
	case GetCurrentlyPlayingResult:
		if msg.retry {
			break
		}
		a.retrying = false
		a.SetCurrentlyPlaying(msg.result)
		updateMediaInfo(a)
		push(tea.Tick(1*time.Second, func (_ time.Time) tea.Msg {
			return GetCurrentlyPlaying(a)
		}))
	case GetAvailableDevicesResult:
		pos := a.posMap["devices"]
		m := a.grid.At(pos).(List)
		push(SetItems(&m, msg.result.Devices))
		a.grid.SetModelPos(m, pos)

		for _, device := range msg.result.Devices {
			if device.IsActive {
				a.SetActiveDevice(device)
				break
			}
		}
	case AuthorizationResponse:
	case UpdateConfigResult:
		a.AppendMessage("config updated")
		a.msgs = append(a.msgs, "config updated")
	case RenewRefreshTokenResult:
		if err := a.updateRefreshToken(msg.result); err != nil {
			a.err = append(a.err, err)
			break
		}
		push(
			UpdateConfig(a, a.GetAuthorizationInfo()),
			RenewRefreshTokenTick(a, a.GetAuthorizationInfo()),
		)
	case AddItemToQueueResult:
		a.AppendMessage("received add item to queue result")
		push(GetUsersQueueCmd(a))
	case SkipItemResult:
		push(GetUsersQueueCmd(a))
	case Shutdown:
		a.db.Close()
		return a, tea.Quit
	case AppErr:
		a.err = append(a.err, msg)
	}

	return a, b.Cmd()
}

func (a *App) currentlyPlayingArtistView() string {
	var name string
	var artistNames []string

	if !a.CurrentlyPlayingIsValid() {
		return ""
	}

	playing := a.CurrentlyPlayingItem()

	if playing.Type == "track" {
		name = playing.Track.Name
		for _, artist := range playing.Track.Artists {
			artistNames = append(artistNames, artist.Name)
		}
	} else {
		name = playing.Episode.Name
	}

	blockStyle := lipgloss.NewStyle().Align(lipgloss.Left)

	artistView := lipgloss.JoinVertical(lipgloss.Left, name, strings.Join(artistNames, ","))

	s := lipgloss.JoinHorizontal(lipgloss.Top, "Now Playing: ", artistView)

	isPlaying, _ := a.IsPlaying()

	if isPlaying {
		s = lipgloss.JoinHorizontal(lipgloss.Top, s, "\t", "playing")

	} else {
		s = lipgloss.JoinHorizontal(lipgloss.Top, s, "\t", "paused")
	}

	device, _ := a.ActiveDeviceName()

	s = lipgloss.JoinHorizontal(lipgloss.Top, s, "\t", "Device:" + device)

	return blockStyle.Render(s)
}


func updateMediaInfo(a *App) {
	if !a.CurrentlyPlayingIsValid() {
		return
	}

	progress := float64(a.currentlyPlaying.Value.ProgressMs.Value)
	volumePercent, _ := a.ActiveDeviceVolumePercent()

	playing := a.CurrentlyPlayingItem()

	var name string
	var artistNames []string

	var duration float64

	if playing.Type == "track" {
		name = playing.Track.Name
		duration = float64(playing.Track.DurationMs)
		for _, artist := range playing.Track.Artists {
			artistNames = append(artistNames, artist.Name)
		}
	} else {
		name = playing.Episode.Name
		duration = float64(playing.Track.DurationMs)
	}

	percent := progress/duration

	m, ok := GetModel[media.Model](a, "media")
	if !ok {
		return
	}
	artistInfo := strings.Join(artistNames, "\n")
	m.SetMediaInfo(name, artistInfo)
	m.SetVolumePercent(volumePercent)
	m.SetPercent(percent)
	SetModel(a, m, "media")
}

func updateModelDims(a *App) {
	sideBar, _ := GetModel[nested.NestedList](a, "sidebar")
	sideBar.SetWidth(int(float64(a.width)*0.2))
	sideBar.SetHeight(a.height/3)
	SetModel(a, sideBar, "sidebar")

	t, _ := GetModel[Table[Rower]](a, "table")
	t.SetWidth(int(float64(a.width)*0.6))
	t.SetHeight(a.height/3)
	SetModel(a, t, "table")

	q, _ := GetModel[List](a, "queue")
	q.SetWidth(int(float64(a.width)*0.2))
	q.SetHeight(a.height/3)
	SetModel(a, q, "queue")

	media, _ := GetModel[media.Model](a, "media")
	media.SetWidth(a.width)
	SetModel(a, media, "media")
}

func GetModel[T tea.Model](a *App, key string) (T, bool) {
	var zero T
	pos := a.posMap[key]

	m, ok := a.grid.At(pos).(T)

	if !ok {
		return zero, false
	}

	return m, ok
}

func SetModel[T tea.Model](a *App, model T, key string) {
	pos, ok := a.posMap[key]
	if !ok {
		return
	}
	a.grid.SetModelPos(model, pos)
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var b Batch
	push := b.Append

	_, cmd := a.updateResults(msg)
	push(cmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.height = msg.Height
		a.width = msg.Width
		updateModelDims(a)
	case tea.KeyMsg:
		switch key := msg.String(); key {
		case "ctrl+c":
			return a, ShutDownApp(a)
		case "esc":
			if a.grid.Focus() {
				a.grid.SetFocus(false)
				break
			}
			return a, tea.Quit
		case "up", "down":
			//push(updatePlaybackVolume(a, key))
		case "q":
			return a, tea.Quit
		case "p":
			push(updatePlaybackStatus(a))
		case "n":
			push(updateSkipNext(a))
		case "b":
			push(updateSkipPrev(a))
		case "a":
			push(handleAddItem(a))
		case "c":
			pos := a.grid.Cursor()
			switch m := a.grid.At(pos).(type) {
			case nested.NestedList:
				item := m.SelectedItem()
				idx := m.Index()
				item.Collapse()
				m.SetItem(item, idx)
				
				a.grid.SetModelPos(m, pos)
			}
			
		case "enter":
			if !a.grid.Focus() {
				pos := a.grid.Cursor()
				switch a.grid.At(pos).(type) {
				case List, media.Model, Table[Rower], nested.NestedList:
					a.grid.SetFocus(true)
				}
			} else {
				pos := a.grid.Cursor()
				switch m := a.grid.At(pos).(type) {
				case List:
					push(handleSelection(a))
				case media.Model:
					push(updateMediaControlSelection(a, m, pos))
				case nested.NestedList:
					item := m.SelectedItem()
					idx := m.Index()
					if item.Expandable() && !item.Expanded() {
						item.Expand()
						m.SetItem(item, idx)
						break
					}

					if !item.Expandable() {
						title := item.Title()

						switch title {
						case "Top Tracks":
							items, ok := a.data["top_tracks"].([]types.Track)
							if !ok {
								break
							}
							SetTable(a, items)
						}
					}

					if item.Expanded() {
						push(handleSidebarSelection(a, m))
					}
				}
			}
		}
	}
	a.grid, cmd = a.grid.Update(msg)
	push(cmd)
	return a, b.Cmd()
}

func updatePlaybackVolume(a *App, dir string) tea.Cmd {
	percent, valid := a.ActiveDeviceVolumePercent()

	if !valid {
		return nil
	}
	switch dir {
	case "up":
		percent += 5
		if percent > 100 {
			percent = 100
		}
	case "down":
		percent -= 5
		if percent < 0 {
			percent = 0
		}
	}

	a.msgs = append(a.msgs, "changing volume to " + fmt.Sprintf("%d", percent))

	return SetPlaybackVolumeCmd(a, percent)
}

func updateMediaControlSelection(a *App, m media.Model, pos grid.Position) tea.Cmd {
	var cmd tea.Cmd
	button := m.SelectedItem()
	switch button {
	case "play":
		m.SetItem("pause", m.Index())
		//cmd = m.PressButton()
		cmd = tea.Batch(updatePlaybackStatus(a), m.PressButton())
	case "pause":
		m.SetItem("play", m.Index())
		//cmd = m.PressButton()
		cmd = tea.Batch(updatePlaybackStatus(a), m.PressButton())
	case "prev":
		//cmd = m.PressButton()
		cmd = tea.Batch(updateSkipPrev(a), m.PressButton())
	case "next":
		//cmd = m.PressButton()
		cmd = tea.Batch(updateSkipNext(a), m.PressButton())
	case "up":
	case "down":
	}
	a.grid.SetModelPos(m, pos)
	return cmd
}

func updatePlaybackStatus(a *App) tea.Cmd {
	isPlaying, valid := a.IsPlaying()

	if !valid  {
		return nil
	}

	if isPlaying {
		return PausePlaybackCmd(a)
	}

	return StartResumePlaybackCmd(a)
}

func updateSkipNext(a *App) tea.Cmd {
	if !a.CurrentlyPlayingIsValid() {
		return nil
	}
	return SkipSongCmd(a, "next")
}

func updateSkipPrev(a *App) tea.Cmd {
	if !a.CurrentlyPlayingIsValid() {
		return nil
	}
	return SkipSongCmd(a, "previous")
}

func handleSelection(a *App) tea.Cmd {
	pos := a.grid.Cursor()
	m, ok := a.grid.At(pos).(List)
	if !ok {
		return nil
	}

	selectedItem := m.l.SelectedItem()

	switch a.typeMap[pos] {
	case "playlists":
		return handlePlaylistSelection(a, selectedItem)
	case "playlist_items":
	case "tracks":
	default:
		return nil
	}

	return nil
}

func handleSidebarSelection(a *App, m nested.NestedList) tea.Cmd {
	title, item := m.Pair()

	if item == nil {
		return nil
	}

	switch title {
	case "Top Artists":
	case "Top Tracks":
		items, ok := a.data["top_tracks"]
		if !ok {
			return nil
		}
		vals, ok := items.([]types.Track)
		if !ok {
			return nil
		}
		SetTable(a, vals)
	case "Playlists":
		playlist, ok := item.(types.SimplifiedPlaylistObject)

		if !ok {
			return nil
		}

		items, ok := a.data[playlist.Id]

		if !ok {
			return GetPlaylistItemsCmd(a, playlist.Id)
		}

		vals, ok := items.([]types.PlaylistItemUnion)

		if !ok {
			return nil
		}

		SetTable(a, vals)
	}

	return nil
}

func SetTable[S []T, T Rower](a *App, s S) {
	t, ok := GetModel[Table[Rower]](a, "table")
	if !ok {
		return
	}
	SetTableItems(&t, toRows(s))
	t.t.Focus()
	a.grid.SetFocus(true)
	tablePos := a.posMap["table"]
	a.grid.SetCursor(tablePos)
	SetModel(a, t, "table")
}

func handleAddItem(a *App) tea.Cmd {
	pos := a.grid.Cursor()
	m, ok := a.grid.At(pos).(Table[Rower])
	if !ok {
		return nil
	}

	selectedItem := m.SelectedItem()

	var uri, msg string

	switch item := selectedItem.(type) {
	case types.Track:
		uri = item.Uri
		msg = "selected item is a track" + " " + uri
	case types.PlaylistItemUnion:
		track := item.Track
		switch track.Type {
		case "track":
			uri = item.Track.Track.Uri
			msg = "selected item is a playlist item track" + " " + uri
		case "episode":
		}
	}

	if msg != "" {
		a.AppendMessage(msg)
	}

	if uri == "" {
		a.AppendMessage("uri of the selected item is empty")
		return nil
	}

	return AddItemToQueueCmd(a, uri)
}

func handlePlaylistSelection(a *App, item list.Item) tea.Cmd {
	selectedItem, ok := item.(types.SimplifiedPlaylistObject)

	if !ok {
		return nil
	}

	playlistId := selectedItem.Id

	if playlistId == "" {
		a.AppendMessage("playlist id is empty")
		return nil
	}

	a.AppendMessage("playlistId: " + playlistId)

	return GetPlaylistItemsCmd(a, playlistId)
}

func (a *App) viewActiveDevice() string {
	device, valid := a.ActiveDevice()

	if !valid {
		return "unable to get active device information"
	}

	var s string

	s += fmt.Sprintf("name: %s\n", device.Name)
	s += fmt.Sprintf("id: %s\n", device.Id.Value)
	s += fmt.Sprintf("volume: %d\n", device.VolumePercent.Value)

	return a.styles.currentlyPlaying.Width(20).Render(s)
}

func (a *App) View() string {
	//builder := &strings.Builder{}
	titleView :=  a.styles.title.Width(a.width).Align(lipgloss.Center).Render(a.title)
	infoView := a.styles.infoStyle.Width(a.width).Render(a.currentlyPlayingArtistView())
	gridView := a.styles.gridStyle.Render(a.grid.View())
	//titleView = lipgloss.Place(a.width, 1, lipgloss.Center, lipgloss.Center, titleView)
	s := lipgloss.JoinVertical(lipgloss.Center, titleView, infoView)
	s = lipgloss.JoinVertical(lipgloss.Left, s, gridView)
	return s
}

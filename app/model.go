package app

import (
	"time"
	"fmt"
	"strings"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/list"
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

func (a *App) updateResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	var b Batch
	push := b.Append

	switch msg := msg.(type) {
	case GetUserResult:
		a.SetUser(msg.result)
		a.msgs = append(a.msgs , "got set profile result")
	case GetUsersTopItems[types.Artist]:
		pos := a.posMap["artists"]
		m := a.grid.At(pos).(List)
		push(SetItems(&m, msg.result.Items))
		a.grid.SetModelPos(m, pos)
	case GetUsersTopItems[types.Track]:
		pos := a.posMap["tracks"]
		m := a.grid.At(pos).(List)
		push(SetItems(&m, msg.result.Items))
		a.grid.SetModelPos(m, pos)
	case GetUsersPlaylistsResult:
		pos := a.posMap["playlists"]
		m := a.grid.At(pos).(List)
		push(SetItems(&m, msg.result.Items))
		a.grid.SetModelPos(m, pos)
	case GetUsersQueueResult:
		pos := a.posMap["queue"]
		m := a.grid.At(pos).(List)
		push(SetItems(&m, msg.result.Queue))
		a.grid.SetModelPos(m, pos)
	case GetPlaylistItemsResult:
		pos := a.posMap["playlist_items"]
		m := a.grid.At(pos).(List)
		push(SetItems(&m, msg.result.Items))
		a.grid.SetModelPos(m, pos)
	case GetCurrentlyPlayingTrackResult:
		if msg.result != nil {
			a.foundCurrentlyPlaying = true
			a.msgs = append(a.msgs, "got valid result")
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
		a.msgs = append(a.msgs, "received add item to queue result")
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

func updateMediaInfo(a *App) {
	if !a.CurrentlyPlayingIsValid() {
		return
	}

	progress := float64(a.currentlyPlaying.Value.ProgressMs.Value)

	playing := a.CurrentlyPlayingItem()

	var name string
	var duration float64

	if playing.Type == "track" {
		name = playing.Track.Name
		duration = float64(playing.Track.DurationMs)
	} else {
		name = playing.Episode.Name
		duration = float64(playing.Track.DurationMs)
	}

	percent := progress/duration

	pos := a.posMap["media"]
	m, ok := a.grid.At(pos).(media.Model)

	if !ok {
		return
	}
	m.SetArtist(name)
	m.SetPercent(percent)
	a.grid.SetModelPos(m, pos)
}

func updateMediaDims(a *App) {
	pos := a.posMap["media"]
	m, ok := a.grid.At(pos).(media.Model)
	if !ok {
		return
	}
	m.SetWidth(a.width)
	a.grid.SetModelPos(m, pos)
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
		updateMediaDims(a)
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, ShutDownApp(a)
		case "esc":
			if a.grid.Focus() {
				a.grid.SetFocus(false)
				break
			}
			return a, tea.Quit
		case "q":
			return a, tea.Quit
		case "p":
			push(updatePlaybackStatus(a))
		case "n":
			push(updateSkipNext(a))
		case "b":
			push(updateSkipPrev(a))
			//if !a.CurrentlyPlayingIsValid() {
			//	break
			//}
			//push(SkipSongCmd(a, "previous"), GetUsersQueueCmd(a))
		case "a":
			push(handleAddItem(a))
		case "enter":
			if !a.grid.Focus() {
				pos := a.grid.Cursor()
				switch a.grid.At(pos).(type) {
				case List, media.Model:
					a.grid.SetFocus(true)
				}
			} else {
				pos := a.grid.Cursor()
				switch m := a.grid.At(pos).(type) {
				case List:
					push(handleSelection(a))
				case media.Model:
					push(updateMediaControlSelection(a, m, pos))
				}
			}
		}
	}

	a.grid, cmd = a.grid.Update(msg)
	push(cmd)

	return a, b.Cmd()
}

func updateMediaControlSelection(a *App, m media.Model, pos grid.Position) tea.Cmd {
	var cmd tea.Cmd
	button := m.SelectedItem()
	switch button {
	case "play":
		m.SetItem("pause", m.Index())
		cmd = m.PressButton()
		//return tea.Batch(updatePlaybackStatus(a), m.PressButton())
	case "pause":
		m.SetItem("play", m.Index())
		cmd = m.PressButton()
		//return tea.Batch(updatePlaybackStatus(a), m.PressButton())
	case "prev":
		cmd = m.PressButton()
		//return tea.Batch(updateSkipPrev(a), m.PressButton())
	case "next":
		cmd = m.PressButton()
		//return tea.Batch(updateSkipNext(a), m.PressButton())
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

func handleAddItem(a *App) tea.Cmd {
	pos := a.grid.Cursor()
	m, ok := a.grid.At(pos).(List)
	if !ok {
		return nil
	}

	selectedItem := m.l.SelectedItem()

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
		a.msgs = append(a.msgs, msg)
	}

	if uri == "" {
		a.msgs = append(a.msgs, "uri of the selected item is empty")
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
		a.msgs = append(a.msgs, "playlist id is empty")
		return nil
	}

	a.msgs = append(a.msgs, "playlistId: " + playlistId)

	return GetPlaylistItemsCmd(a, playlistId)
}

func (a *App) viewCurrentlyPlaying() string {
	playing, ok := a.CurrentlyPlaying()

	if !ok {
		return "unable to get currently playing information"
	}

	var s string

	if playing.Item.Value.Type == "track" {
		var percent float64
		item := playing.Item.Value.Track
		percent = float64(playing.ProgressMs.Value)/float64(item.DurationMs)
		songName := a.styles.artistStyle.Render(item.Name)
		isPlayingView := fmt.Sprintf("playing: %t", playing.IsPlaying)
		s = lipgloss.JoinVertical(lipgloss.Left, songName, isPlayingView)
		s = lipgloss.JoinHorizontal(lipgloss.Left, s, a.progress.ViewAs(percent))
	} else {
		item := playing.Item.Value.Episode
		s += item.Name + " "
		s += fmt.Sprintf("%d", item.DurationMs) + " "
		s += fmt.Sprintf("%t", playing.IsPlaying)
	}

	return a.styles.currentlyPlaying.Width(a.width-5).Render(s)
}

func (a *App) viewActiveDevice() string {
	device, valid := a.ActiveDevice()

	if !valid {
		return "unable to get active device information"
	}

	var s string

	s += fmt.Sprintf("name: %s\n", device.Name)
	s += fmt.Sprintf("id: %s\n", device.Id.Value)

	return a.styles.currentlyPlaying.Width(20).Render(s)
}

func (a *App) viewPlaybackState() string {
	if !a.PlaybackStateIsValid() {
		return "unable to get playback state information"
	}

	state, _  := a.PlaybackState()

	device, _ := a.PlaybackStateDevice()

	var s string

	s += fmt.Sprintf("device name: %s\n", device.Name)
	s += fmt.Sprintf("device id: %s\n", device.Id.Value)
	s += fmt.Sprintf("is playing: %t\n", state.IsPlaying)

	return a.styles.currentlyPlaying.Width(20).Render(s)
}

func (a *App) View() string {
	builder := &strings.Builder{}

	titleView :=  a.styles.title.Render(a.title) 
	//artistsView := a.styles.artist.Render(a.artists.View())
	//trackView := a.styles.track.Render(a.tracks.View())

	titleView = lipgloss.Place(a.width, 1, lipgloss.Center, lipgloss.Center, titleView)
	//row1 := lipgloss.JoinHorizontal(lipgloss.Center, artistsView, trackView)

	//fmt.Fprintln(builder, titleView)
	//fmt.Fprintln(builder, row1)
	//builder.WriteRune('\n')
	builder.WriteString(titleView)
	builder.WriteRune('\n')
	//builder.WriteString(a.grid.View())
	//builder.WriteRune('\n')
	//builder.WriteString(a.info.View())
	//builder.WriteRune('\n')
	//builder.WriteString(strings.Join(a.msgs, "\n"))
	//builder.WriteRune('\n')

	msgsView := strings.Join(a.msgs, "\n")
	gridView := a.grid.View()

	currentlyPlayingView := a.viewCurrentlyPlaying()
	activeDeviceView := a.viewActiveDevice()
	stateView := a.viewPlaybackState()
	row2 := lipgloss.JoinHorizontal(lipgloss.Left, activeDeviceView, stateView)

	s := lipgloss.JoinVertical(lipgloss.Left, gridView, row2)
	s = lipgloss.JoinVertical(lipgloss.Center, s, msgsView)
	s = lipgloss.JoinVertical(lipgloss.Left, s, currentlyPlayingView)

	errView := ""

	for _, err := range a.err {
		errView += err.Error() + "\n"
	}

	s = lipgloss.JoinVertical(lipgloss.Center, s, errView)

	builder.WriteString(s)

	return builder.String()
}

type infoModel struct {
	infoType string

	name string
	description string
	artistName string
	trackName string
	playlistName string

	deviceName string
	deviceId string
	volumePercent int
	isRestricted bool
	isActive bool
	deviceType string
}

func (i infoModel) Init() tea.Cmd {
	return nil
}

func (i infoModel) Update(msg tea.Msg) (infoModel, tea.Cmd) {
	return i, nil
}

func (i infoModel) View() string {
	builder := &strings.Builder{}
	fmt.Fprintln(builder, "name: ", i.name)
	fmt.Fprintln(builder, "description: ", i.description)
	fmt.Fprintln(builder, "Artist:", i.artistName)
	fmt.Fprintln(builder, "track:", i.trackName)

	fmt.Fprintln(builder, "device:", i.deviceName)
	fmt.Fprintln(builder, "deviceId:", i.deviceId)
	fmt.Fprintln(builder, "volumnPercent:", i.volumePercent)
	fmt.Fprintln(builder, "isActive:", i.isActive)

	return builder.String()
}

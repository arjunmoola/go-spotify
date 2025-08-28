package app

import (
	"time"
	"fmt"
	"strings"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go-spotify/types"
)

type Batch []tea.Cmd

func (b *Batch) Append(cmds... tea.Cmd) {
	*b = append(*b, cmds...)
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

	switch msg := msg.(type) {
	case GetUserResult:
		a.SetUser(msg.result)
		a.msgs = append(a.msgs , "got set profile result")
	case GetUsersTopItems[types.Artist]:
		a.SetTopArtists(msg.result.Items)
		pos := a.posMap["artists"]
		m := a.grid.At(pos).(List)
		b.Append(SetItems(&m, msg.result.Items))
		a.grid.SetModelPos(m, pos)
	case GetUsersTopItems[types.Track]:
		a.SetTopTracks(msg.result.Items)
		b.Append(SetItems(&a.tracks, msg.result.Items))
		pos := a.posMap["tracks"]
		m := a.grid.At(pos).(List)
		b.Append(SetItems(&m, msg.result.Items))
		a.grid.SetModelPos(m, pos)
	case GetUsersPlaylistsResult:
		b.Append(SetItems(&a.playlists, msg.result.Items))
		pos := a.posMap["playlists"]
		m := a.grid.At(pos).(List)
		b.Append(SetItems(&m, msg.result.Items))
		a.grid.SetModelPos(m, pos)
	case GetUsersQueueResult:
		pos := a.posMap["queue"]
		m := a.grid.At(pos).(List)
		b.Append(SetItems(&m, msg.result.Queue))
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
		b.Append(tea.Tick(1*time.Second, func (_ time.Time) tea.Msg {
			return GetCurrentlyPlaying(a)
		}))
	case GetAvailableDevicesResult:
		b.Append(SetItems(&a.devices, msg.result.Devices))
		pos := a.posMap["devices"]
		m := a.grid.At(pos).(List)
		b.Append(SetItems(&m, msg.result.Devices))
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
		b.Append(UpdateConfig(a, a.GetAuthorizationInfo()))
		b.Append(RenewRefreshTokenTick(a, a.GetAuthorizationInfo()))
	case Shutdown:
		a.db.Close()
		return a, tea.Quit
	case AppErr:
		a.err = append(a.err, msg)
	}

	return a, b.Cmd()
}

func (a *App) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var b Batch
	_, cmd := a.updateResults(msg)
	b.Append(cmd)

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.height = msg.Height
		a.width = msg.Width
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return a, ShutDownApp(a)
		case "esc":
			if a.grid.Focus() {
				a.grid.SetFocus(false)
				//a.info = infoModel{}
				break
			}
			return a, tea.Quit
		case "q":
			return a, tea.Quit
		case "p":
			isPlaying, valid := a.IsPlaying()

			if !valid  {
				break
			}
			if isPlaying {
				b.Append(PausePlaybackCmd(a))
			} else {
				b.Append(StartResumePlaybackCmd(a))
			}
		case "n":
			if !a.CurrentlyPlayingIsValid() {
				break
			}
			b.Append(SkipSongCmd(a, "next"), GetUsersQueueCmd(a))
		case "b":
			if !a.CurrentlyPlayingIsValid() {
				break
			}
			b.Append(SkipSongCmd(a, "previous"), GetUsersQueueCmd(a))
		case "enter":
			if !a.grid.Focus() {
				pos := a.grid.Cursor()
				if _, ok := a.grid.At(pos).(List); ok {
					a.grid.SetFocus(true)
				}
			} else {
				pos := a.grid.Cursor()
				_, ok := a.grid.At(pos).(List)
				if !ok {
					break
				}
				//selectedValue := m.list.SelectedItem()
				//switch item := selectedValue.(type) {
				//case artistItem:
				//	a.info.artistName = item.artist.Name
				//case trackItem:	
				//	a.info.trackName = item.track.Name
				//case playlistItem:
				//	a.info.artistName = item.playlist.Name
				//	a.info.description = item.playlist.Description
				//case deviceItem:
				//	device := ActiveDevice{
				//		name: item.device.Name,
				//		id: item.device.Id,
				//		volumePercent: item.device.VolumePercent,
				//		supportsVolume: item.device.SupportsVolumne,
				//	}
				//	a.SetActiveDevice(device)
				//	a.info.deviceName = item.device.Name
				//	a.info.deviceId = item.device.Id
				//	a.info.deviceType = item.device.Type
				//	a.info.isActive = item.device.IsActive
				//}
			}
		//case "r":
		//	if a.CurrentlyPlayingIsValid() {
		//		break
		//	}

		//	if !a.retrying {
		//		b.Append(GetCurrentlyPlaying(a))
		//	}
		}
	}

	a.grid, cmd = a.grid.Update(msg)
	b.Append(cmd)

	return a, b.Cmd()
}

func (a *App) viewCurrentlyPlaying() string {
	playing, ok := a.CurrentlyPlaying()

	if !ok {
		return "unable to get currently playing information"
	}

	var s string

	if playing.Item.Value.Type == "track" {
		item := playing.Item.Value.Track
		s += item.Name + "\n"
		s += fmt.Sprintf("%d/%d\n", playing.ProgressMs.Value, item.DurationMs)
		s += fmt.Sprintf("playing: %t\n", playing.IsPlaying) 
		s += fmt.Sprintf("device: %s\n", playing.Device.Name)
		s += fmt.Sprintf("device is active: %t\n", playing.Device.IsActive)
	} else {
		item := playing.Item.Value.Episode
		s += item.Name + " "
		s += fmt.Sprintf("%d", item.DurationMs) + " "
		s += fmt.Sprintf("%t", playing.IsPlaying)
	}

	return a.styles.currentlyPlaying.Width(20).Render(s)
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
	row2 := lipgloss.JoinHorizontal(lipgloss.Left, currentlyPlayingView, activeDeviceView, stateView)

	s := lipgloss.JoinVertical(lipgloss.Left, gridView, row2)
	s = lipgloss.JoinVertical(lipgloss.Center, s, msgsView)

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

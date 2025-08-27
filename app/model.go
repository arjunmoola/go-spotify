package app

import (
	"fmt"
	"strings"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"go-spotify/types"
)

type Batch []tea.Cmd

func (b *Batch) Append(cmd tea.Cmd) {
	*b = append(*b, cmd)
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
	b.Append(GetCurrentlyPlayingTrack(a))
	return b.Cmd()
}

func (a *App) updateResults(msg tea.Msg) (tea.Model, tea.Cmd) {
	var b Batch

	switch msg := msg.(type) {
	case GetUserProfileResult:
		a.SetProfile(msg.result)
		a.msgs = append(a.msgs , "got set profile result")
	case GetUsersTopArtistsResult:
		a.SetTopArtists(msg.result)
		b.Append(a.artists.SetItemsFromResult(msg.result))

		m := a.grid.At(0, 0).(spotifyList)
		m.SetItemsFromResult(msg.result)
		a.grid.SetModel(m, 0, 0)
	case GetUsersTopTracksResult:
		a.SetTopTracks(msg.result)
		b.Append(a.tracks.SetItemsFromResult(msg.result))

		m := a.grid.At(0, 1).(spotifyList)
		m.SetItemsFromResult(msg.result)
		a.grid.SetModel(m, 0, 1)
	case GetUsersPlaylistsResult:
		b.Append(a.playlists.SetItemsFromResult(msg.result))
		m := a.grid.At(0, 2).(spotifyList)
		m.SetItemsFromResult(msg.result)
		a.grid.SetModel(m, 0, 2)
	case GetCurrentlyPlayingTrackResult:
		if msg.result != nil {
			a.msgs = append(a.msgs, fmt.Sprintf("got valid result"))
		}
	case GetAvailableDevicesResult:
		//a.msgs = append(a.msgs, fmt.Sprintf("num of devices: %d", len(msg.result.Devices)))
		b.Append(a.devices.SetItemsFromResult(msg.result))
		m := a.grid.At(0, 3).(spotifyList)
		m.SetItemsFromResult(msg.result)
		a.grid.SetModel(m, 0, 3)
	case AuthorizationResponse:
	case UpdateConfigResult:
		a.msgs = append(a.msgs, "config updated")
	case RenewRefreshTokenResult:
		if err := a.updateRefreshToken(msg.result); err != nil {
			a.err = append(a.err, err)
			break
		}
		b.Append(UpdateConfig(a, a.GetAuthorizationInfo()))
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
				a.info = infoModel{}
				break
			}
			return a, tea.Quit
		case "q":
			return a, tea.Quit
		case "enter":
			if !a.grid.Focus() {
				i, j := a.grid.Cursor()
				if _, ok := a.grid.At(i, j).(spotifyList); ok {
					a.grid.SetFocus(true)
				}
			} else {
				i, j := a.grid.Cursor()
				m, ok := a.grid.At(i, j).(spotifyList)
				if !ok {
					break
				}
				selectedValue := m.list.SelectedItem()
				switch item := selectedValue.(type) {
				case artistItem:
					a.info.artistName = item.artist.Name
				case trackItem:	
					a.info.trackName = item.track.Name
				case playlistItem:
					a.info.artistName = item.playlist.Name
					a.info.description = item.playlist.Description
				case deviceItem:
					a.info.deviceName = item.device.Name
					a.info.deviceId = item.device.Id
					a.info.deviceType = item.device.Type
					a.info.isActive = item.device.IsActive
				}
			}
		}
	}

	a.grid, cmd = a.grid.Update(msg)
	b.Append(cmd)

	return a, b.Cmd()
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

	s := lipgloss.JoinHorizontal(lipgloss.Left, a.grid.View(), a.info.View())
	s = lipgloss.JoinVertical(lipgloss.Center, s, msgsView)

	errView := ""

	for _, err := range a.err {
		errView += err.Error() + "\n"
	}

	s = lipgloss.JoinVertical(lipgloss.Center, s, errView)

	builder.WriteString(s)

	
	//fmt.Fprintln(builder, "press ctrl+c to quit")

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

type currentlyPlayingModel struct {
	currentlyPlaying *types.CurrentlyPlayingTrack
}

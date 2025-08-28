package app

import (
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"fmt"
	"time"
	"log"
	"context"
	"database/sql"
	"go-spotify/types"
	schema "go-spotify/sql"
	"path/filepath"
	"go-spotify/database"
	"go-spotify/client"
	"go-spotify/models/grid"
	//"go-spotify/models/list"
	//"github.com/charmbracelet/bubbles/list"
	_ "github.com/tursodatabase/go-libsql"
	"errors"
	//"log"
	"os"
)

type AppState int

const (
	InitializationState AppState = iota
	InitializationErrState
	Menu
)

var debug bool = true
var ddl string

func init() {
	ddl = schema.Get()
}

const (
	defaultConfigDirName = ".go-spotify"
	defaultDbName = "go-spotify.db"
	dbDriver = "libsql"
)

var (
	ErrInvalidClientInfo = errors.New("either the client token or secret is incorrect")
	ErrClientInfoNotFound = errors.New("client info could not be found")
)

func getDbUrl(configDir string) string {
	return filepath.Join(configDir, defaultDbName)
}

func getConfigDir() string {
	userHome, _ := os.UserHomeDir()
	return filepath.Join(userHome, defaultConfigDirName)
}

func (a *App) initializeConfigDir() error {
	_, err := os.Lstat(a.configDir)

	var dirNotFound bool
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			dirNotFound = true
		} else {
			return err
		}
	}

	if dirNotFound {
		return os.Mkdir(a.configDir, 0777)
	}

	return nil
}

func (a *App) initializeDb() error {
	dburl := "file:" + filepath.Join(a.configDir, defaultDbName)

	db, err := sql.Open(dbDriver, dburl)

	if err != nil {
		return err
	}

	if err := db.Ping(); err != nil {
		return err
	}

	_, err = db.Exec(ddl)

	if err != nil {
		return err
	}

	a.db = db
	a.dbUrl = dburl

	return nil
}

func (a *App) getClientInfo() error {
	queries := database.New(a.db)

	var clientInfoNotFound bool

	row, err := queries.GetClientInfo(context.Background())

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			clientInfoNotFound = true
		} else {
			return err
		}
	}

	if clientInfoNotFound {
		params := database.InsertConfigParams{
			ClientID: a.ClientId(),
			ClientSecret: a.ClientSecret(),
			RedirectUri: a.RedirectUri(),
			Authorized: false,
		}

		if err := queries.InsertConfig(context.Background(), params); err != nil {
			return err
		}

		return nil
	}

	a.SetClientInfo(row.ClientID, row.ClientSecret)
	a.SetRedirectUri(row.RedirectUri)

	a.authorized = false

	if !row.AccessToken.Valid || !row.RefreshToken.Valid {
		a.newLogin = true
		return nil
	}

	if row.AccessToken.String == "" || row.RefreshToken.String == "" {
		a.newLogin = true
		return nil
	}

	a.SetAccessToken(row.AccessToken.String)
	a.SetRefreshToken(row.RefreshToken.String)

	expiresAt, err := time.Parse(time.UnixDate, row.ExpiresAt.String)

	if err != nil {
		return err
	}

	a.SetExpiresAt(expiresAt)

	if time.Now().Compare(expiresAt) >= 0 {
		log.Println("token is expired")
		a.tokenExpired = true
	} else {
		log.Println("token is not expired")
	}

	return nil

}

type AuthorizationInfo struct {
	clientSecret string
	clientId string
	accessToken string
	refreshToken string
	expiresAt time.Time
	redirectUri string
}

type User struct {
	profile *types.UserProfile
	topTracks *types.UsersTopTracks
	topArtists *types.UsersTopArtists
}

type AppStyles struct {
	title lipgloss.Style
	artist lipgloss.Style
	track lipgloss.Style
	currentModel lipgloss.Style
	focusedModel lipgloss.Style
	currentlyPlaying lipgloss.Style
}

func NewAppStyles() AppStyles {
	defaultStyle := lipgloss.NewStyle()
	generalModelStyle := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
	currentlyPlaying := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())

	return AppStyles{
		title: defaultStyle.Foreground(lipgloss.Color("200")),
		artist: generalModelStyle,
		track: generalModelStyle,
		focusedModel: generalModelStyle.BorderForeground(lipgloss.Color("200")),
		currentlyPlaying: currentlyPlaying,
	}
}

type ActiveDevice struct {
	name string
	id string
	deviceType string
	volumePercent int
	supportsVolume bool
}

type SpotifyInfo interface {
	Type() string
	Render() string
}

type ArtistInfo struct {
	name string
	popularity int
	id string
}

func (a ArtistInfo) Type() string {
	return "artist"
}

func (a ArtistInfo) Render() string {
	return ""
}

type TrackInfo struct {
	name string
	id string
	popularity int
	trackNumber int
	durationMs int
	isPlayable bool
	trackType string
}

type UserInfo struct {
	user types.User
	topTracks []types.Track
	topArtists []types.Artist
}

type App struct {
	client *client.Client
	authInfo AuthorizationInfo
	user UserInfo

	tokenExpired bool

	authorized bool
	newLogin bool

	db *sql.DB
	configDir string
	dbUrl string

	err []error
	state AppState

	grid grid.Model
	posMap map[string]grid.Position
	typeMap map[grid.Position]string

	title string
	artists List
	tracks List
	devices List
	playlists List
	width int
	height int
	foundCurrentlyPlaying bool
	msgs []string
	styles AppStyles

	retrying bool
	currentlyPlayingRetryCount int

	playbackState Optional[types.PlaybackState]
	activeDevice Optional[types.Device]
	currentlyPlaying Optional[types.CurrentlyPlaying]
	info SpotifyInfo
}

type Optional[T any] struct {
	Value T
	Valid bool
}

func (a *App) SetPlaybackState(p types.PlaybackState) {
	a.playbackState = Optional[types.PlaybackState] {
		Valid: true,
		Value: p,
	}
}

func (a *App) PlaybackState() (state types.PlaybackState, valid bool) {
	if !a.PlaybackStateIsValid() {
		return
	}

	return a.playbackState.Value, true
}

func (a *App) PlaybackStateIsValid() bool {
	return a.playbackState.Valid
}

func (a *App) PlaybackStateDevice() (device types.Device, valid bool) {
	if !a.playbackState.Valid {
		return
	}

	device = a.playbackState.Value.Device

	return device ,true
}

func (a *App) SetCurrentlyPlaying(c types.CurrentlyPlaying) {
	a.currentlyPlaying = Optional[types.CurrentlyPlaying] {
		Valid: true,
		Value: c,
	}
}

func (a *App) CurrentlyPlaying() (types.CurrentlyPlaying, bool) {
	return a.currentlyPlaying.Value, a.currentlyPlaying.Valid
}

func (a *App) CurrentlyPlayingIsValid() bool {
	return a.currentlyPlaying.Valid
}

func (a *App) CurrentlyPlayingItem() types.ItemUnion {
	return a.currentlyPlaying.Value.Item.Value
}

func (a *App) IsPlaying() (isPlaying bool, valid bool) {
	if !a.currentlyPlaying.Valid {
		return
	}
	playing := a.currentlyPlaying.Value
	return playing.IsPlaying, true
}

func (a *App) SetUser(u types.User) {
	a.user.user = u
}

func (a *App) SetTopTracks(tracks []types.Track) {
	a.user.topTracks = tracks
}

func (a *App) SetTopArtists(artists []types.Artist) {
	a.user.topArtists = artists
}

func (a *App) SetActiveDevice(device types.Device) {
	a.activeDevice = Optional[types.Device]{
		Valid: true,
		Value: device,
	}
}

func (a *App) ActiveDevice() (device types.Device, valid bool) {
	return a.activeDevice.Value, a.activeDevice.Valid
}

func (a *App) ActiveDeviceId() (id string, valid bool) {
	if !a.activeDevice.Valid {
		return "", false
	}

	device := a.activeDevice.Value

	if !device.Id.Valid {
		return "", false
	}

	return device.Id.Value, true
}

func (a *App) ActiveDeviceName() (name string, valid bool) {
	if !a.activeDevice.Valid {
		return "", false
	}

	return a.activeDevice.Value.Name, true
}

func (a *App) ActiveDeviceVolumePercent() (percent int, valid bool) {
	if !a.activeDevice.Valid {
		return 
	}

	device := a.activeDevice.Value

	if !device.VolumePercent.Valid {
		return
	}

	return device.VolumePercent.Value, true
}

func (a *App) GetAuthorizationInfo() AuthorizationInfo {
	return a.authInfo
}

func (a *App) AccessToken() string {
	return a.authInfo.accessToken
}

func (a *App) RefreshToken() string {
	return a.authInfo.refreshToken
}

func (a *App) ClientId() string {
	return a.authInfo.clientId
}

func (a *App) ClientSecret() string {
	return a.authInfo.clientSecret
}

func (a *App) RedirectUri() string {
	return a.authInfo.redirectUri
}

func (a *App) ExpiresAt() time.Time {
	return a.authInfo.expiresAt
}

func (a *App) IsTokenExpired() bool {
	expiresAt := a.authInfo.expiresAt
	return time.Now().Compare(expiresAt) >= 0
}

func (a *App) SetAuthorizationInfo(auth AuthorizationInfo) {
	a.authInfo = auth
}

func (a *App) SetAccessToken(tok string) {
	if tok != "" {
		a.authInfo.accessToken = tok
	}
}

func (a *App) SetRefreshToken(tok string) {
	if tok != "" {
		a.authInfo.refreshToken = tok
	}
}

func (a *App) SetClientInfo(clientId, clientSecret string) {
	a.authInfo.clientId = clientId
	a.authInfo.clientSecret = clientSecret
}

func (a *App) SetRedirectUri(uri string) {
	if uri != "" {
		a.authInfo.redirectUri = uri
	}
}

func (a *App) SetExpiresAt(e time.Time) {
	a.authInfo.expiresAt = e
}

func New(clientId, clientSecret, redirectUri string) *App {
	dir := getConfigDir()
	dburl := getDbUrl(dir)

	auth := AuthorizationInfo{
		clientId: clientId,
		clientSecret: clientSecret,
		redirectUri: redirectUri,
	}

	artists := NewList(nil)
	artists.SetShowTitle(true)
	artists.SetTitle("Artists")
	artists.SetListDimensions(10, 20)

	tracks := NewList(nil)
	tracks.SetShowTitle(true)
	tracks.SetTitle("Tracks")
	tracks.SetListDimensions(10, 20)

	playlists := NewList(nil)
	playlists.SetShowTitle(true)
	playlists.SetTitle("Playlists")
	playlists.SetListDimensions(10, 20)

	playlistItems := NewList(nil)
	playlistItems.SetShowTitle(true)
	playlistItems.SetTitle("Items")
	playlistItems.SetListDimensions(10, 20)

	devices := NewList(nil)
	devices.SetShowTitle(true)
	devices.SetTitle("Devices")
	devices.SetListDimensions(10, 20)

	queue := NewList(nil)
	queue.SetShowTitle(true)
	queue.SetTitle("Queue")
	queue.SetListDimensions(10, 20)

	row := grid.NewRow(artists, tracks, playlists, playlistItems, devices, queue)

	g := grid.New()
	g.Append(row)
	//g.SetModel(artists, 0, 0)
	//g.SetModel(tracks, 0, 1)
	//g.SetModel(playlists, 0, 2)
	//g.SetModel(playlistItems, 0, 3)
	//g.SetModel(devices, 0, 4)
	//g.SetModel(queue, 0, 5)
	//grid.SetCellDimensions(20, 30)

	posMap := make(map[string]grid.Position)
	posMap["artists"] = grid.Pos(0, 0)
	posMap["tracks"] = grid.Pos(0, 1)
	posMap["playlists"] = grid.Pos(0, 2)
	posMap["playlist_items"] = grid.Pos(0, 3)
	posMap["devices"] = grid.Pos(0, 4)
	posMap["queue"] = grid.Pos(0, 5)

	typeMap := make(map[grid.Position]string)

	for k, v := range posMap {
		typeMap[v] = k
	}

	return &App{
		authInfo: auth,
		dbUrl: dburl,
		configDir: dir,
		title: "Go-Spotify",
		posMap: posMap,
		typeMap: typeMap,
		artists: artists,
		tracks: tracks,
		playlists: playlists,
		devices: devices,
		grid: g,
		styles: NewAppStyles(),
	}
}

func (a *App) Run() error {
	if _, err := tea.NewProgram(a, tea.WithAltScreen()).Run(); err != nil {
		return err
	}

	return nil
}

func (a *App) IsNewLogin() bool {
	return a.authInfo.accessToken == "" || a.authInfo.refreshToken == ""
}

func (a *App) Setup() error {
	if err := a.initializeConfigDir(); err != nil {
		return err
	}

	if err := a.initializeDb(); err != nil {
		return err
	}

	if err := a.getClientInfo(); err != nil {
		return err
	}

	a.client = client.New(a.ClientId(), a.ClientSecret(), a.RedirectUri())

	if a.IsNewLogin() {
		if err := a.authorizeClient(); err != nil {
			return err
		}
	}

	if a.IsTokenExpired() {
		fmt.Println(a.AccessToken(), a.RefreshToken())
		if err := a.refreshTokens(); err != nil {
			return err
		}
	}

	return nil

}

type GetUserResult struct {
	result types.User
}

type GetUsersTopItems[T any] struct {
	result types.UsersTopItems[T]
}

type GetUsersTopTracksResult struct {
	result *types.UsersTopTracks
}

type GetUsersPlaylistsResult struct {
	result types.CurrentUsersPlaylistResponse
}

type AuthorizationResponse struct {
	response *client.SpotifyAuthorizationResponse
}

type GetAvailableDevicesResult struct {
	result types.AvailableDevices
}

type GetCurrentlyPlayingTrackResult struct {
	result *types.CurrentlyPlayingTrack
	retry bool
}

type GetCurrentlyPlayingResult struct {
	result types.CurrentlyPlaying
	retry bool
}

type GetUsersQueueResult struct {
	result types.UsersQueue
}

type GetPlaylistItemsResult struct {
	result types.Page[types.PlaylistItemUnion]
}

type GetCurrentlyPlayingEpisodeResult struct {}

type UpdateConfigResult struct {}

type RenewRefreshTokenResult struct {
	result *client.SpotifyRefreshTokenResponse
}

type Shutdown struct{}

type AppErr error

func ShutDownApp(a *App) tea.Cmd {
	return func() tea.Msg {
		return Shutdown{}
	}
}

func UpdateConfig(a *App, auth AuthorizationInfo) tea.Cmd {
	return func() tea.Msg {
		err := a.updateConfigDb(auth)

		if err != nil {
			return AppErr(err)
		}

		return UpdateConfigResult {}
	}

}

func RenewRefreshTokenTick(a *App, auth AuthorizationInfo) tea.Cmd {
	d := time.Until(auth.expiresAt).Seconds()
	return tea.Tick(time.Duration(d)*time.Second, func(_ time.Time) tea.Msg {
		return renewRefreshToken(a, auth)
	})
}

func renewRefreshToken(a *App, auth AuthorizationInfo) tea.Msg {
	resp, err := a.client.RefreshToken(auth.accessToken, auth.refreshToken, auth.clientId)

	if err != nil {
		return AppErr(err)
	}

	return RenewRefreshTokenResult {
		result: resp,
	}
}

func GetUserProfile(a *App) tea.Cmd {
	return func() tea.Msg {
		profile, err := a.client.GetCurrentUserProfile(a.AccessToken())

		if err != nil {
			return AppErr(err)
		}

		return GetUserResult{ result: profile }
	}
}

func GetUsersTopArtists(a *App) tea.Cmd {
	return func() tea.Msg {
		if a.AccessToken() == "" {
			return AppErr(fmt.Errorf("access token is empty in GetUsersTopArtists"))
		}

		artists, err := client.GetUsersTopItems[types.Artist](a.client, a.AccessToken(), "artists")

		if err != nil{
			return AppErr(err)
		}

		return GetUsersTopItems[types.Artist]{ result: artists }
	}
}

func GetUsersTopTracks(a *App) tea.Cmd {
	return func() tea.Msg {
		if a.AccessToken() == "" {
			return AppErr(fmt.Errorf("access token in empty in GetUsersTopTracks"))
		}

		tracks ,err := client.GetUsersTopItems[types.Track](a.client, a.AccessToken(), "tracks")

		if err != nil {
			return AppErr(err)
		}

		return GetUsersTopItems[types.Track]{ result: tracks }
	}
}

func GetUsersPlaylist(a *App) tea.Cmd {
	return func() tea.Msg {
		if a.AccessToken() == "" {
			return AppErr(fmt.Errorf("access token in empty in GetUsersPlaylist"))
		}

		playlists, err := a.client.GetCurrentUsersPlaylists(a.AccessToken(), 10, 0)

		if err != nil {
			return AppErr(err)
		}

		return GetUsersPlaylistsResult{ result: playlists }
	}
}

func GetAvailableDevices(a *App) tea.Cmd {
	return func() tea.Msg {
		if a.AccessToken() == "" {
			return AppErr(fmt.Errorf("access token in empty in GetUsersPlaylist"))
		}

		accessToken := a.AccessToken()

		devices, err := a.client.GetAvailableDevices(accessToken)

		if err != nil {
			return AppErr(err)
		}

		return GetAvailableDevicesResult{ result: devices }
	}
}

func GetCurrentlyPlayingTrack(a *App) tea.Cmd {
	return func() tea.Msg {
		if a.AccessToken() == "" {
			return AppErr(fmt.Errorf("access token in GetCurrentlyPlayingTrack"))
		}

		accesstoken := a.AccessToken()

		currentlyPlayTrack, err := a.client.GetCurrentlyPlayingTrack(accesstoken)

		if err != nil {
			return AppErr(err)
		}

		switch track := currentlyPlayTrack.(type) {
		case *types.CurrentlyPlayingTrack:
			return GetCurrentlyPlayingTrackResult{ result: track }
		default:
			return AppErr(fmt.Errorf("invalid type"))
		}
		
	}
}

func GetCurrentlyPlayingCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		return GetCurrentlyPlaying(a)
	}
}

func GetCurrentlyPlaying(a *App) tea.Msg {
	if a.AccessToken() == "" {
		return AppErr(fmt.Errorf("access token in GetCurrentlyPlayingTrack"))
	}

	accesstoken := a.AccessToken()

	currentlyPlaying, err := a.client.GetCurrentlyPlaying(accesstoken)

	if err != nil {
		return AppErr(err)
	}

	return GetCurrentlyPlayingResult{ result: currentlyPlaying, retry: false }
}

func StartResumePlaybackCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		accessToken := a.AccessToken()

		deviceId, valid := a.ActiveDeviceId()

		if !valid {
			return AppErr(fmt.Errorf("active device is either not set or active device id is not set"))
		}

		if err := a.client.StartResumePlayback(accessToken, deviceId); err != nil {
			return AppErr(err)
		}

		return nil
	}
}

func PausePlaybackCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		accessToken := a.AccessToken()

		deviceId, valid := a.ActiveDeviceId()

		if !valid {
			return AppErr(fmt.Errorf("active device is either not set or active device id is not set"))
		}

		if err := a.client.PausePlayback(accessToken, deviceId); err != nil {
			return AppErr(err)
		}

		return nil
	}
}

func SkipSongCmd(a *App, action string) tea.Cmd {
	return func() tea.Msg {
		accessToken := a.AccessToken()

		deviceId, valid := a.ActiveDeviceId()

		if !valid {
			return AppErr(fmt.Errorf("active device is either not set or active device id is not set"))
		}

		if err := a.client.SkipSong(accessToken, deviceId, action); err != nil{
			return AppErr(err)
		}

		return nil
	}
}

func GetUsersQueueCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		accessToken := a.AccessToken()

		queue, err := a.client.GetQueue(accessToken)

		if err != nil {
			return AppErr(err)
		}

		return GetUsersQueueResult{
			result: queue,
		}
	}
}

func GetPlaylistItemsCmd(a *App, playlistId string) tea.Cmd {
	return func() tea.Msg {
		accessToken := a.AccessToken()
		items, err := a.client.GetPlaylistItems(accessToken, playlistId)

		if err != nil {
			return AppErr(err)
		}

		return GetPlaylistItemsResult {
			result: items,
		}
	}
}


func (a *App) authorizeClient() error {
	resp, err := a.client.Authorize()

	if err != nil {
		return err
	}

	a.SetExpiresAt(getExpiresAtTime(resp.ExpiresIn))
	a.tokenExpired = false

	a.SetAccessToken(resp.AccessToken)
	a.SetRefreshToken(resp.RefreshToken)

	return a.updateConfigDb(a.GetAuthorizationInfo())
}

func (a *App) updateRefreshToken(resp *client.SpotifyRefreshTokenResponse) error {
	if resp.AccessToken == "" {
		return fmt.Errorf("refresh token was not refreshed")
	}

	a.SetAccessToken(resp.AccessToken)
	a.SetRefreshToken(resp.RefreshToken)

	a.SetExpiresAt(getExpiresAtTime(resp.ExpiresIn))
	a.tokenExpired = false

	return nil
}

func (a *App) refreshTokens() error {
	resp, err := a.client.RefreshToken(a.AccessToken(), a.RefreshToken(), a.ClientId())

	if err != nil {
		return err
	}

	if resp.AccessToken == "" {
		return fmt.Errorf("refresh token was not refreshed")
	}

	a.SetAccessToken(resp.AccessToken)
	a.SetRefreshToken(resp.RefreshToken)

	a.SetExpiresAt(getExpiresAtTime(resp.ExpiresIn))
	a.tokenExpired = false

	log.Println("new refresh token acquired")
	log.Println(a.ExpiresAt().Format(time.UnixDate))

	return a.updateConfigDb(a.GetAuthorizationInfo())
}

func getExpiresAtTime(expiresIn int) time.Time {
	return time.Now().Add(time.Duration(expiresIn)*time.Second)
}

func (a *App) updateConfigDb(auth AuthorizationInfo) error {
	e := auth.expiresAt.Format(time.UnixDate)

	updateParams := database.UpdateTokensParams{
		AccessToken: sql.NullString{
			Valid: true,
			String: auth.accessToken,
		},
		RefreshToken: sql.NullString{
			Valid: true,
			String: auth.refreshToken,
		},
		ExpiresAt: sql.NullString{
			Valid: true,
			String: e,
		},
	}

	queries := database.New(a.db)

	if err := queries.UpdateTokens(context.Background(), updateParams); err != nil {
		return err
	}

	return nil
}

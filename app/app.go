package app

import (
	"strings"
	"flag"
	"github.com/charmbracelet/lipgloss"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/bubbles/spinner"
	//"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/arjunmoola/go-spotify/models/textinput"
	"fmt"
	"time"
	"log"
	"log/slog"
	"context"
	"database/sql"
	"github.com/arjunmoola/go-spotify/types"
	schema "github.com/arjunmoola/go-spotify/sql"
	"path/filepath"
	"github.com/arjunmoola/go-spotify/database"
	"github.com/arjunmoola/go-spotify/client"
	"github.com/arjunmoola/go-spotify/models/grid"
	"github.com/arjunmoola/go-spotify/models/media"
	nested "github.com/arjunmoola/go-spotify/models/list"
	//"github.com/charmbracelet/bubbles/list"
	_ "github.com/tursodatabase/go-libsql"
	"errors"
	//"log"
	"os"
)

var logger *slog.Logger
var updateLogger *slog.Logger

type AppState int

const (
	InitializationState AppState = iota
	InitializationErrState
	AuthorizingNewLogin
	InitializationDone
	NewLogin
	Menu
)

func (state AppState) String() string {
	var s string
	switch state {
	case InitializationState:
		s = "InitializationState"
	case InitializationErrState:
		s = "InitializatinErrState"
	case AuthorizingNewLogin:
		s = "AuthorizingNewLogin"
	case InitializationDone:
		s = "InitializationDone"
	case NewLogin:
		s = "NewLogin"
	case Menu:
		s = "Menu"
	}
	return s
}

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

func (a *App) initializeLogger() error {

	logFilePath := filepath.Join(a.configDir, "log")

	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)

	if err != nil {
		return err
	}

	a.logger = slog.New(slog.NewTextHandler(file, &slog.HandlerOptions{Level: slog.LevelDebug}))
	slog.SetDefault(a.logger)
	log.Println("hello world")
	
	return nil
}

type initializeDbMsg struct {
	db *sql.DB
	dbUrl string
	err error
}

func (m initializeDbMsg) Err() error {
	return m.err
}

func initializeDbCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		dburl := "file:" + filepath.Join(a.configDir, defaultDbName)

		db, err := sql.Open(dbDriver, dburl)

		if err != nil {
			return initializeDbMsg{
				err: err,
			}
		}

		if err := db.Ping(); err != nil {
			return initializeDbMsg{ err: err }
		}

		_, err = db.Exec(ddl)

		if err != nil {
			return initializeDbMsg{ err: err }
		}

		return initializeDbMsg{
			db: db,
			dbUrl: dburl,
		}
	}
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

type clientInfoMsg struct {
	row Optional[database.GetClientInfoRow]
	newLogin bool
	tokenExpired bool
	expiresAt time.Time
	err error
}

func (m clientInfoMsg) Err() error {
	return m.err
}

func getClientInfoCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		logger.Debug("retrieving client info")

		queries := database.New(a.db)

		var clientInfoNotFound bool

		row, err := queries.GetClientInfo(context.Background())

		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				clientInfoNotFound = true
			} else {
				return clientInfoMsg{ err: err }
			}
		}

		if clientInfoNotFound {
			return clientInfoMsg {
				newLogin: true,
			}
		}

		msg := clientInfoMsg{
			row: Optional[database.GetClientInfoRow]{
				Value: row,
				Valid: true,
			},
		}

		if !row.AccessToken.Valid || !row.RefreshToken.Valid {
			msg.newLogin = true
			return msg
		}

		if row.AccessToken.String == "" || row.RefreshToken.String == "" {
			msg.newLogin = true
			return msg
		}

		expiresAt, err := time.Parse(time.UnixDate, row.ExpiresAt.String)

		if err != nil {
			return err
		}

		msg.expiresAt = expiresAt

		if time.Now().Compare(expiresAt) >= 0 {
			msg.tokenExpired = true
		}

		return msg
	}
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
		a.newLogin = true
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
		a.tokenExpired = true
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
	artistStyle lipgloss.Style
	skipButtonStyle lipgloss.Style
	progressBarStyle lipgloss.Style
	gridStyle lipgloss.Style
	infoStyle lipgloss.Style
}

func NewAppStyles() AppStyles {
	defaultStyle := lipgloss.NewStyle()
	generalModelStyle := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())
	currentlyPlaying := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder())
	artistStyle := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder())
	skipButtonsStyle := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder())
	progressBarStyle := lipgloss.NewStyle().BorderStyle(lipgloss.RoundedBorder())
	gridStyle := lipgloss.NewStyle().Align(lipgloss.Center)

	return AppStyles{
		infoStyle: defaultStyle.Align(lipgloss.Center),
		title: defaultStyle.BorderStyle(lipgloss.HiddenBorder()).Height(1).Foreground(lipgloss.Color("200")),
		artist: generalModelStyle,
		track: generalModelStyle,
		focusedModel: generalModelStyle.BorderForeground(lipgloss.Color("200")),
		currentlyPlaying: currentlyPlaying,
		artistStyle: artistStyle,
		skipButtonStyle: skipButtonsStyle,
		progressBarStyle: progressBarStyle,
		gridStyle: gridStyle,
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

func (a *App) isState(state AppState) bool {
	return a.state == state
}

type App struct {
	spotifyAuthUrl Optional[string]
	logFile *os.File
	logger *slog.Logger
	cli bool
	spinner spinner.Model
	mediaFocus bool
	client *client.Client
	authInfo AuthorizationInfo
	user Optional[types.User]

	loginModel loginModel
	inputValues []string

	tokenExpired bool

	authorized bool
	newLogin bool

	db *sql.DB
	configDir string
	dbUrl string

	err []error
	state AppState

	initInputs []textinput.Model

	grid grid.Model
	posMap map[string]grid.Position
	typeMap map[grid.Position]string
	viewMap map[string]Table[Rower]
	viewMapKeys map[string]string

	cachedPlaylists map[string]types.Playlist

	media media.Model

	title string
	width int
	height int

	foundCurrentlyPlaying bool
	msgs []string
	styles AppStyles

	retrying bool
	currentlyPlayingRetryCount int

	data map[string]any

	progress progress.Model
	playbackState Optional[types.PlaybackState]
	activeDevice Optional[types.Device]
	currentlyPlaying Optional[types.CurrentlyPlaying]
	info SpotifyInfo

	textInputFocus bool
	inputState textInputState

	sessionStart time.Time

	defaultPlaylist Optional[types.Playlist]
	prevPos Optional[grid.Position]
	prevGrid Optional[grid.Position]
	
	inputValue Optional[string]
	searchResults Optional[types.SearchResult]
}

type Optional[T any] struct {
	Value T
	Valid bool
}

func (a *App) SetSearchResults(result types.SearchResult) {
	a.searchResults = Optional[types.SearchResult]{
		Value: result,
		Valid: true,
	}
}

func (a *App) GetSearchResults() (types.SearchResult, bool) {
	if a.searchResults.Valid {
		var res types.SearchResult
		return res, false
	}
	return a.searchResults.Value, a.searchResults.Valid
}

func (a *App) IsPlaylistInCache(id string) bool {
	_, ok := a.cachedPlaylists[id]
	return ok
}

func (a *App) AddPlaylistToCache(playlist types.Playlist) {
	id := playlist.Id
	a.cachedPlaylists[id] = playlist
}

func (a *App) GetPlaylistFromCache(id string) (types.Playlist, bool) {
	playlist, ok := a.cachedPlaylists[id]
	return playlist, ok
}

func (a *App) RemovePlaylistFromCache(id string) {
	_, ok := a.cachedPlaylists[id]
	if !ok {
		return
	}

	delete(a.cachedPlaylists, id)
}

func (a *App) SetUser(u types.User) {
	a.user = Optional[types.User]{
		Value: u,
		Valid: true,
	}
}

func (a *App) UserIsValid() bool {
	return a.user.Valid
}

func (a *App) UserDisplayName() string {
	return a.user.Value.DisplayName
}

func (a *App) UserId() string {
	return a.user.Value.Id
}

func (a *App) SetDefaultPlaylist(p types.Playlist) {
	a.defaultPlaylist = Optional[types.Playlist]{
		Value: p,
		Valid: true,
	}
}

func (a *App) DefaultPlaylistIsValid() bool {
	return a.defaultPlaylist.Valid
}

func (a *App) UnsetDefaultPlaylist() {
	a.defaultPlaylist = Optional[types.Playlist]{}
}

func (a *App) DefaultPlaylistId() string {
	return a.defaultPlaylist.Value.Id
}

func (a *App) DefaultPlaylistName() string {
	return a.defaultPlaylist.Value.Name
}

func (a *App) ExistsInDefaultPlaylist(trackId string) bool {
	tracks := a.defaultPlaylist.Value.Tracks

	for _, track := range tracks.Items {
		var id string
		switch t := track.Track.Type; t {
		case "track":
			id = track.Track.Track.Id
		case "episode":
			id = track.Track.Episode.Id
		}

		if id == trackId {
			return true
		}
	}

	return false
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

func (a *App) AppendMessage(msg string) {
	pos := a.posMap["messages"]
	m, ok := a.grid.At(pos).(List)
	if !ok {
		return
	}
	items := m.l.Items()
	items = append(items, messageItem(msg))
	m.l.SetItems(items)
	a.grid.SetModelPos(m, pos)
}

func New(db *sql.DB) *App {
	logger.Info("New App")
	spinner := spinner.New()
	dir := getConfigDir()
	dburl := getDbUrl(dir)

	devices := NewList(nil)
	devices.SetShowTitle(true)
	devices.SetTitle("Devices")
	devices.SetListDimensions(10, 20)

	queue := NewList(nil)
	queue.SetShowTitle(true)
	queue.SetTitle("Queue")
	queue.SetListDimensions(10, 20)

	messages := NewList(nil)
	messages.SetListDimensions(10, 30)
	messages.SetShowTitle(true)
	messages.SetTitle("Internal Messages")

	items := []nested.Item{
		nested.NewItem("Top Artists", nil, true),
		nested.NewItem("Top Tracks", nil, false),
		nested.NewItem("Playlists", nil, true),
		nested.NewItem("Recently Played", nil, false),
		//nested.NewItem("Current Session", nil, false),
	}

	sidebar := nested.New(items)

	table1 := NewTable[Rower](defaultColumns())
	table2 := NewTable[Rower](playHistoryColumns())

	viewMap := make(map[string]Table[Rower])

	viewMap["default"] = table1
	viewMap["recently_played"] = table2

	viewMapKeys := make(map[string]string)
	viewMapKeys["Top Artists"] = "default"
	viewMapKeys["Top Tracks"] = "default"
	viewMapKeys["Playlists"] = "default"
	viewMapKeys["Recently Played"] = "recently_played"
	//viewMapKeys["Current Session"] = "recently_played"
	viewMapKeys["Playlist Items"] = "default"

	input := textinput.New()

	//row := grid.NewRow(artists, tracks, playlists, playlistItems, devices, queue)
	media := media.New("prev", "play", "next", "up", "down")
	row1 := grid.NewRow(input)
	row2 := grid.NewRow(sidebar, table1, queue)
	row3 := grid.NewRow(messages, devices)
	row4 := grid.NewRow(media)

	g := grid.New()
	g.Append(row1, row2, row3, row4)
	g.SetReadonly(0)
	g.SetReadonly(3)

	posMap := make(map[string]grid.Position)
	posMap["textinput"] = grid.Pos(0, 0)
	posMap["devices"] = grid.Pos(2, 1)
	posMap["queue"] = grid.Pos(1, 2)
	posMap["sidebar"] = grid.Pos(1, 0)
	posMap["messages"] = grid.Pos(2, 0)
	posMap["media"] = grid.Pos(3, 0)
	posMap["table"] = grid.Pos(1, 1)

	typeMap := make(map[grid.Position]string)

	for k, v := range posMap {
		typeMap[v] = k
	}

	progress := progress.New()
	progress.ShowPercentage = false
	client := client.New()

	loginModel := newLoginModel()

	return &App{
		db: db,
		spinner: spinner,
		client: client,
		loginModel: loginModel,
		dbUrl: dburl,
		configDir: dir,
		title: "Go-Spotify",
		posMap: posMap,
		typeMap: typeMap,
		viewMap: viewMap,
		viewMapKeys: viewMapKeys,
		state: InitializationState,
		//artists: artists,
		//tracks: tracks,
		//playlists: playlists,
		//devices: devices,
		grid: g,
		styles: NewAppStyles(),
		progress: progress,
		data: make(map[string]any),
		sessionStart: time.Now(),
		cachedPlaylists: make(map[string]types.Playlist),
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

func SetupLogger(l *slog.Logger)  {
	logger = l.WithGroup("app")
	client.SetupLogger(l)
}

func (a *App) SetupCli() error {
	if err := a.initializeDb(); err != nil {
		return err
	}

	if err := a.getClientInfo(); err != nil {
		return fmt.Errorf("client not validated got err: %v", err)
	}

	if a.newLogin {
		return fmt.Errorf("use the tui to finish authentication")
	}

	if !a.tokenExpired {
		return nil
	}

	ctx := defaultRefreshTokenCtx(a)
	resp, err := a.client.RefreshToken(ctx)

	if err != nil {
		return err
	}

	if err := a.updateRefreshToken(resp); err != nil {
		return err
	}

	if err := a.updateConfigDb(a.GetAuthorizationInfo()); err != nil {
		return err
	}

	return nil

}

type AppEvent interface {
	name() string
}

type GetSpotifyAuthUrlResult struct {
	result string
	err error
}

func (r GetSpotifyAuthUrlResult) Err() error {
	return r.err
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
	id string
	name string
	result types.Page[types.PlaylistItemUnion]
}

type GetPlaylistResult struct {
	id string
	name string
	result types.Playlist
}

type GetUsersRecentlyPlayedResult struct {
	result types.Page[types.PlayHistory]
}

type GetUsersCurrentSessionResult struct {
	result types.Page[types.PlayHistory]
}

type GetCurrentlyPlayingEpisodeResult struct {}

type UpdateConfigResult struct {
	err error
}

type InsertAuthInfoResult struct {
	err error
}

func (r InsertAuthInfoResult) Err() error {
	return r.err
}

func (r UpdateConfigResult) Err() error {
	return r.err
}

type RenewRefreshTokenResult struct {
	result types.SpotifyRefreshTokenResponse
}

func (r RenewRefreshTokenResult) name() string {
	return "RenewRefreshTokenResult"
}

type AddItemsToPlaylistResult struct {
	result types.PlaylistSnapshot
	id string
	name string
}

type GetSearchResults struct {
	result types.SearchResult
}

type AddItemToQueueResult struct{}
type SkipItemResult struct{}
type UpdatePlaybackResult struct{}
type SetPlaybackVolumeResult struct{}

type Shutdown struct{}

type AppErr error
type InitializationError error

func ShutDownApp(a *App) tea.Cmd {
	return func() tea.Msg {
		return Shutdown{}
	}
}

func GetSpotifyAuthUrlCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		authUrl, err := a.client.GetSpotifyAuthUrl(ctx)

		if err != nil {
			return GetSpotifyAuthUrlResult{
				err: err,
			}
		}

		return GetSpotifyAuthUrlResult{
			result: authUrl,
		}
	}
}

func UpdateConfig(a *App, auth AuthorizationInfo) tea.Cmd {
	return func() tea.Msg {
		err := a.updateConfigDb(auth)
		return UpdateConfigResult{
			err: err,
		}
	}
}

func InsertAuthInfoCmd(a *App, auth AuthorizationInfo) tea.Cmd {
	return func() tea.Msg {
		err := a.insertAuthInfo(auth)
		return InsertAuthInfoResult{
			err: err,
		}
	}
}

func RenewRefreshTokenTick(a *App, auth AuthorizationInfo) tea.Cmd {
	d := time.Until(auth.expiresAt).Seconds()
	return tea.Tick(time.Duration(d)*time.Second, func(_ time.Time) tea.Msg {
		return renewRefreshToken(a)
	})
}

func RenewRefreshTokenCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		return renewRefreshToken(a)
	}
}

func renewRefreshToken(a *App) tea.Msg {
	ctx := defaultRefreshTokenCtx(a)
	resp, err := a.client.RefreshToken(ctx)

	if err != nil {
		return AppErr(err)
	}

	return RenewRefreshTokenResult {
		result: resp,
	}
}


func GetUserProfile(a *App) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)

		profile, err := a.client.GetCurrentUserProfile(ctx)

		if err != nil {
			return AppErr(err)
		}

		return GetUserResult{ result: profile }
	}
}

func GetUsersTopArtists(a *App) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)

		artists, err := client.GetUsersTopItems[types.Artist](ctx, a.client, "artists")

		if err != nil{
			return AppErr(err)
		}

		return GetUsersTopItems[types.Artist]{ result: artists }
	}
}

func GetUsersTopTracks(a *App) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)

		tracks ,err := client.GetUsersTopItems[types.Track](ctx, a.client, "tracks")

		if err != nil {
			return AppErr(err)
		}

		return GetUsersTopItems[types.Track]{ result: tracks }
	}
}

func GetUsersPlaylist(a *App) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)

		playlists, err := a.client.GetCurrentUsersPlaylists(ctx, 10, 0)

		if err != nil {
			return AppErr(err)
		}

		return GetUsersPlaylistsResult{ result: playlists }
	}
}

type GetCurrentSessionPlayedResult struct {
	result types.Page[types.PlayHistory]
}

func GetCurrentSessionPlayedCmd(a *App, params client.RecentlyPlayedTracksParams) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)

		page, err := a.client.GetRecentlyPlayedTracks(ctx, params)

		if err != nil {
			return AppErr(err)
		}

		return GetCurrentSessionPlayedResult{ result: page }

	}
}

func GetUsersRecentlyPlayedCmd(a *App, params client.RecentlyPlayedTracksParams) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)
		page, err := a.client.GetRecentlyPlayedTracks(ctx, params)

		if err != nil {
			return AppErr(err)
		}

		return GetUsersRecentlyPlayedResult{ result: page }
	}
}

func GetAvailableDevices(a *App) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)
		devices, err := a.client.GetAvailableDevices(ctx)

		if err != nil {
			return AppErr(err)
		}

		return GetAvailableDevicesResult{ result: devices }
	}
}

func GetCurrentlyPlayingCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		return GetCurrentlyPlaying(a)
	}
}

func GetCurrentlyPlaying(a *App) tea.Msg {
	ctx := defaultAccessTokenCtx(a)
	currentlyPlaying, err := a.client.GetCurrentlyPlaying(ctx)

	if err != nil {
		return AppErr(err)
	}

	return GetCurrentlyPlayingResult{ result: currentlyPlaying, retry: false }
}

func StartResumePlaybackCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)

		deviceId, valid := a.ActiveDeviceId()

		if !valid {
			return AppErr(fmt.Errorf("active device is either not set or active device id is not set"))
		}

		params := client.PlaybackActionParams{
			DeviceId: deviceId,
		}

		if err := a.client.StartResumePlayback(ctx, params); err != nil {
			return AppErr(err)
		}

		return nil
	}
}

func PausePlaybackCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)

		deviceId, valid := a.ActiveDeviceId()

		if !valid {
			return AppErr(fmt.Errorf("active device is either not set or active device id is not set"))
		}

		params := client.PlaybackActionParams{
			DeviceId: deviceId,
		}

		if err := a.client.PausePlayback(ctx, params); err != nil {
			return AppErr(err)
		}

		return nil
	}
}

func SkipSongCmd(a *App, action string) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)

		deviceId, valid := a.ActiveDeviceId()

		if !valid {
			return AppErr(fmt.Errorf("active device is either not set or active device id is not set"))
		}

		params := client.SkipSongParams{
			DeviceId: deviceId,
			Direction: action,
		}

		if err := a.client.SkipSong(ctx, params); err != nil{
			return AppErr(err)
		}

		return SkipItemResult{}
	}
}

func GetUsersQueueCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)
		queue, err := a.client.GetQueue(ctx)

		if err != nil {
			return AppErr(err)
		}

		return GetUsersQueueResult{
			result: queue,
		}
	}
}

func GetPlaylistItemsCmd(a *App, playlistId string, name string) tea.Cmd {
	return func() tea.Msg {
		accessToken := a.AccessToken()
		items, err := a.client.GetPlaylistItems(accessToken, playlistId)

		if err != nil {
			return AppErr(err)
		}

		return GetPlaylistItemsResult {
			result: items,
			id: playlistId,
			name: name,
		}
	}
}


func AddItemToQueueCmd(a *App, uri string) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)

		deviceId, valid := a.ActiveDeviceId()

		if !valid {
			return AppErr(fmt.Errorf("active device is not set up"))
		}

		params := client.AddItemToQueueParams{
			Uri: uri,
			DeviceId: deviceId,
		}

		err := a.client.AddItemToQueue(ctx, params)

		if err != nil {
			return AppErr(err)
		}

		return AddItemToQueueResult{}
	}
}

func SetPlaybackVolumeCmd(a *App, percent int) tea.Cmd {
	return func() tea.Msg {
		ctx := defaultAccessTokenCtx(a)
		
		deviceId, valid := a.ActiveDeviceId()

		if !valid {
			return AppErr(fmt.Errorf("active device is not set up"))
		}

		params := client.SetPlaybackVolumeParams{
			DeviceId: deviceId,
			Percent: percent,
		}

		if err := a.client.SetPlaybackVolume(ctx, params); err != nil {
			return AppErr(err)
		}

		return SetPlaybackVolumeResult{}
	}
}

func GetPlaylistCmd(a *App, params client.GetPlaylistParams) tea.Cmd {
	return func() tea.Msg {
		ctx := client.WithAccessToken(context.Background(), a.AccessToken())

		playlist, err := a.client.GetPlaylist(ctx, params)

		if err != nil {
			return AppErr(err)
		}

		return GetPlaylistResult {
			id: params.Id,
			result: playlist,
		}
	}
}

func AddItemsToPlaylistCmd(a *App, params client.AddItemsToPlaylistParams) tea.Cmd {
	return func() tea.Msg {
		ctx := client.WithAccessToken(context.Background(), a.AccessToken())

		res, err := a.client.AddItemsToPlaylist(ctx, params)

		if err != nil {
			return AppErr(err)
		}

		return AddItemsToPlaylistResult{
			result: res,
		}
	
	}
}

func GetSearchResultCmd(a *App, params client.GetSearchResultsParams) tea.Cmd {
	return func() tea.Msg {
		ctx := client.WithAccessToken(context.Background(), a.AccessToken())

		res, err := a.client.GetSearchResults(ctx, params)

		if err != nil {
			return AppErr(err)
		}

		return GetSearchResults{
			result: res,
		}
	}
}

type authorizeClientMsg struct {
	resp *client.SpotifyAuthorizationResponse
	err error
}

func (m authorizeClientMsg) Err() error {
	return m.err
}

func (m authorizeClientMsg) name() string {
	return "authorizeClientMsg"
}

func AuthorizeClientCmd(a *App) tea.Cmd {
	return func() tea.Msg {
		logger.Debug("running Authorize client cmd", "command", "AuthorizeClientCmd")
		ctx := defaultAuthorizationCtx(a)
		resp, err := a.client.Authorize(ctx)

		if err != nil {
			return authorizeClientMsg{
				err: err,
			}
		}

		return authorizeClientMsg{
			resp: resp,
		}
	}
}


func (a *App) authorizeClient() error {
	ctx := defaultAuthorizationCtx(a)
	resp, err := a.client.Authorize(ctx)

	if err != nil {
		return err
	}

	a.SetExpiresAt(getExpiresAtTime(resp.ExpiresIn))
	a.tokenExpired = false

	a.SetAccessToken(resp.AccessToken)
	a.SetRefreshToken(resp.RefreshToken)

	return a.updateConfigDb(a.GetAuthorizationInfo())
}

func (a *App) updateRefreshToken(resp types.SpotifyRefreshTokenResponse) error {
	if resp.AccessToken == "" {
		return fmt.Errorf("refresh token was not refreshed")
	}

	a.SetAccessToken(resp.AccessToken)
	if resp.RefreshToken.Valid {
		a.SetRefreshToken(resp.RefreshToken.Value)
	}

	a.SetExpiresAt(getExpiresAtTime(resp.ExpiresIn))
	a.tokenExpired = false

	return nil
}

func (a *App) refreshTokens() error {
	ctx := defaultRefreshTokenCtx(a)

	resp, err := a.client.RefreshToken(ctx)

	if err != nil {
		return err
	}

	if resp.AccessToken == "" {
		return fmt.Errorf("refresh token was not refreshed")
	}

	a.SetAccessToken(resp.AccessToken)

	if resp.RefreshToken.Valid {
		a.SetRefreshToken(resp.RefreshToken.Value)
	}

	a.SetExpiresAt(getExpiresAtTime(resp.ExpiresIn))
	a.tokenExpired = false

	log.Println("new refresh token acquired")
	log.Println(a.ExpiresAt().Format(time.UnixDate))

	return a.updateConfigDb(a.GetAuthorizationInfo())
}

func getExpiresAtTime(expiresIn int) time.Time {
	return time.Now().Add(time.Duration(expiresIn)*time.Second)
}

func (a *App) insertAuthInfo(auth AuthorizationInfo) error {
	e := auth.expiresAt.Format(time.UnixDate)
	insertParams := database.InsertConfigParams{
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
		Authorized: true,
		RedirectUri: auth.redirectUri,
		ClientSecret: auth.clientSecret,
		ClientID: auth.clientId,
	}

	queries := database.New(a.db)

	if err := queries.InsertConfig(context.Background(), insertParams); err != nil {
		return err
	}

	return nil
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

type CliCommandHandler func(args ...string) error

type CliCommands struct {
	Commands map[string]CliCommandHandler
}

func (c *CliCommands) RegisterHandler(cmd string, f CliCommandHandler) {
	c.Commands[cmd] = f
}

func NewCliCommands(a *App) *CliCommands {
	commands := &CliCommands{
		Commands: make(map[string]CliCommandHandler),
	}
	commands.RegisterHandler("player", PlayerHandler(a))
	return commands
}

func (c *CliCommands) Run(cmd string, args ...string) error {
	handler, ok := c.Commands[cmd]
	if !ok {
		return fmt.Errorf("handler for command %s not found", cmd)
	}

	return handler(args...)
}

func StatusHandler(a *App) CliCommandHandler {
	statusCmd := flag.NewFlagSet("status", flag.ExitOnError)
	return func(args ...string) error {
		if err := statusCmd.Parse(args); err != nil {
			statusCmd.Usage()
			return err
		}

		ctx := client.WithAccessToken(context.Background(), a.AccessToken())

		status, err := a.client.GetCurrentlyPlaying(ctx)

		if err != nil {
			return err
		}

		if !status.Item.Valid {
			fmt.Println("there is nothing playing currently")
			return nil
		}

		item := status.Item.Value

		var name string

		if item.Type == "track" {
			name = item.Track.Name
		} else {
			name = item.Episode.Name
		}

		fmt.Printf("currently playing: %s", name)

		return nil
	}
}

func defaultAccessTokenCtx(a *App) context.Context {
	return client.WithAccessToken(context.Background(), a.AccessToken())
}

func defaultRefreshTokenCtx(a *App) context.Context {
	auth := a.GetAuthorizationInfo()
	return client.ContextWithClientInfo(context.Background(), auth.accessToken, auth.refreshToken, auth.clientId, auth.clientSecret)
}

func defaultAuthorizationCtx(a *App) context.Context {
	auth := a.GetAuthorizationInfo()
	return client.ContextWithAuthorization(context.Background(), auth.clientId, auth.clientSecret, auth.redirectUri)
}

func PlayerHandler(a *App) CliCommandHandler {
	var playpause bool
	var nextSong bool
	var prevSong bool
	playerCmd := flag.NewFlagSet("player", flag.ExitOnError)
	playerCmd.BoolVar(&playpause, "p", false, "play/pause")
	playerCmd.BoolVar(&nextSong, "next", false, "next song")
	playerCmd.BoolVar(&prevSong, "prev", false, "previous song")
	return func(args ...string) error {
		if err := playerCmd.Parse(args); err != nil {
			playerCmd.Usage()
			return err
		}


		ctx := defaultAccessTokenCtx(a)

		currentlyPlaying, err := a.client.GetCurrentlyPlaying(ctx)

		if err != nil {
			return err
		}

		availableDevices, err := a.client.GetAvailableDevices(ctx)

		if err != nil {
			return err
		}

		var activeDevice types.Device
		var found bool

		for _, device := range availableDevices.Devices {
			if device.IsActive {
				found = true
				activeDevice = device
				break
			}
		}

		if !found {
			return fmt.Errorf("could not find active devices")
		}


		activeDeviceId := activeDevice.Id.Value


		if len(args) == 0 {
			showArtistInfoCli(currentlyPlaying, activeDevice)
			return nil
		}

		if playpause {

			var action string

			if currentlyPlaying.IsPlaying {
				action = "pause"
			} else {
				action = "play"
			}

			params := client.PlaybackActionParams{
				DeviceId: activeDeviceId,
				Action: action,
				Other: types.Optional[client.OtherParams]{
					Valid: false,
				},
			}

			if err := a.client.PlaybackAction(ctx, params); err != nil {
				return err
			}

			return nil
		}

		if nextSong {
			params := client.SkipSongParams{
				DeviceId: activeDeviceId,
				Direction: "next",
			}

			if err := a.client.SkipSong(ctx, params); err != nil {
				return err
			}

			<-time.After(250*time.Millisecond)

			nextSong, err := a.client.GetCurrentlyPlaying(ctx)

			if err != nil {
				return err
			}

			showArtistInfoCli(nextSong, activeDevice)

		}

		if prevSong {
			params := client.SkipSongParams{
				DeviceId: activeDeviceId,
				Direction: "prev",
			}

			if err := a.client.SkipSong(ctx, params); err != nil {
				return err
			}
		}

		return nil
	}
}

func showArtistInfoCli(currentlyPlaying types.CurrentlyPlaying, activeDevice types.Device) {
	var name string
	var artists string

	item := currentlyPlaying.Item.Value

	if currentlyPlaying.Item.Value.Type == "track" {
		name = item.Track.Name

		var a []string

		for _, artist := range item.Track.Artists {
			a = append(a, artist.Name)
		}
		artists = strings.Join(a, ",")
	} else {
		name = item.Episode.Name
	}

	fmt.Printf("playing: %s\nartists: %s\n", name, artists)
	fmt.Printf("device: %s\n", activeDevice.Name)
}


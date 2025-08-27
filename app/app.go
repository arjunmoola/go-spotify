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
}

func NewAppStyles() AppStyles {
	defaultStyle := lipgloss.NewStyle()
	generalModelStyle := lipgloss.NewStyle().BorderStyle(lipgloss.NormalBorder())

	return AppStyles{
		title: defaultStyle.Foreground(lipgloss.Color("200")),
		artist: generalModelStyle,
		track: generalModelStyle,
		focusedModel: generalModelStyle.BorderForeground(lipgloss.Color("200")),
	}
}

type App struct {
	client *client.Client
	authInfo AuthorizationInfo
	user User

	tokenExpired bool

	authorized bool
	newLogin bool

	db *sql.DB
	configDir string
	dbUrl string

	err []error
	state AppState
	artistsModel spotifyListItem
	playlistsModel spotifyListItem

	grid grid.Model
	info infoModel
	title string
	artists spotifyList
	tracks spotifyList
	devices spotifyList
	playlists spotifyList
	podcasts spotifyList
	width int
	height int

	msgs []string

	styles AppStyles
}

func (a *App) Profile() *types.UserProfile {
	return a.user.profile
}

func (a *App) TopTracks() *types.UsersTopTracks {
	return a.user.topTracks
}

func (a *App) TopArtists() *types.UsersTopArtists {
	return a.user.topArtists
}

func (a *App) SetProfile(profile *types.UserProfile) {
	a.user.profile = profile
}

func (a *App) SetTopTracks(tracks *types.UsersTopTracks) {
	a.user.topTracks = tracks
}

func (a *App) SetTopArtists(artists *types.UsersTopArtists) {
	a.user.topArtists = artists
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

	artists := newSpotifyListModel(nil)
	artists.SetShowTitle(true)
	artists.SetTitle("Artists")
	artists.SetListDimensions(10, 20)

	tracks := newSpotifyListModel(nil)
	tracks.SetShowTitle(true)
	tracks.SetTitle("Tracks")
	tracks.SetListDimensions(10, 20)

	playlists := newSpotifyListModel(nil)
	playlists.SetShowTitle(true)
	playlists.SetTitle("Playlists")
	playlists.SetListDimensions(10, 20)

	devices := newSpotifyListModel(nil)
	devices.SetShowTitle(true)
	devices.SetTitle("Devices")
	devices.SetListDimensions(10, 20)

	grid := grid.New(1, 4)
	grid.SetModel(artists, 0, 0)
	grid.SetModel(tracks, 0, 1)
	grid.SetModel(playlists, 0, 2)
	grid.SetModel(devices, 0, 3)
	//grid.SetCellDimensions(20, 30)

	return &App{
		authInfo: auth,
		dbUrl: dburl,
		configDir: dir,
		title: "Go-Spotify",
		artists: artists,
		tracks: tracks,
		playlists: playlists,
		devices: devices,
		grid: grid,
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

type GetUserProfileResult struct {
	result *types.UserProfile
}

type GetUsersTopArtistsResult struct {
	result *types.UsersTopArtists
}

type GetUsersTopTracksResult struct {
	result *types.UsersTopTracks
}

type GetUsersPlaylistsResult struct {
	result *types.CurrentUsersPlaylistResponse
}

type AuthorizationResponse struct {
	response *client.SpotifyAuthorizationResponse
}

type GetAvailableDevicesResult struct {
	result *types.AvailableDevices
}

type GetCurrentlyPlayingTrackResult struct {
	result *types.CurrentlyPlayingTrack
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

func RenewRefreshToken(a *App, auth AuthorizationInfo) tea.Cmd {
	return func() tea.Msg {
		resp, err := a.client.RefreshToken(auth.accessToken, auth.refreshToken, auth.clientId)

		if err != nil {
			return AppErr(err)
		}

		return RenewRefreshTokenResult {
			result: resp,
		}
	}
}

func GetUserProfile(a *App) tea.Cmd {
	return func() tea.Msg {
		profile, err := a.client.GetCurrentUserProfile(a.AccessToken())

		if err != nil {
			return AppErr(err)
		}

		return GetUserProfileResult{ result: profile }
	}
}

func GetUsersTopArtists(a *App) tea.Cmd {
	return func() tea.Msg {
		if a.AccessToken() == "" {
			return AppErr(fmt.Errorf("access token is empty in GetUsersTopArtists"))
		}

		profile, err := a.client.GetUsersTopArtists(a.AccessToken())

		if err != nil{
			return AppErr(err)
		}

		return GetUsersTopArtistsResult { result: profile }
	}
}

func GetUsersTopTracks(a *App) tea.Cmd {
	return func() tea.Msg {
		if a.AccessToken() == "" {
			return AppErr(fmt.Errorf("access token in empty in GetUsersTopTracks"))
		}

		topTracks, err := a.client.GetUsersTopTracks(a.AccessToken())

		if err != nil {
			return AppErr(err)
		}

		return GetUsersTopTracksResult{ result: topTracks }
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

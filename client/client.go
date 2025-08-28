package client

import (
	"io"
	"strconv"
	"go-spotify/types"
	"strings"
	"errors"
	"context"
	"encoding/json"
	"encoding/base64"
	"log"
	"net/url"
	"net/http"
	"time"
	"fmt"
	"crypto/rand"
	"bytes"
)

const defaultLimitRate = time.Millisecond*100

var (
	spotifyAuthorizationUrl = "https://accounts.spotify.com/authorize?"
	spotifyWebApiUrl = "https://api.spotify.com/v1/me"
	spotifyWebApiBaseUrl = "https://api.spotify.com/v1"
)

var defaultScopes = []string{
	"user-read-playback-state",
	"user-modify-playback-state",
	"user-read-currently-playing",
	"playlist-read-private",
	"playlist-read-collaborative",
	"playlist-modify-private",
	"playlist-modify-public",
	"user-read-playback-position",
	"user-top-read",
	"user-read-recently-played",
	"user-library-modify",
	"user-library-read",
	"user-read-email",
	"user-read-private",
}

func generateState(n int) string {
	b := make([]byte, n)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

type Client struct {
	secret string
	id string
	redirectUri string
	state string

	client *http.Client
	limitRate time.Duration

	requestCh chan any
	retryCh chan any
	close chan struct{}
}


func New(clientId string, clientSecret string, redirectUri string) *Client {
	client := &Client{
		state: generateState(16),
		secret: clientSecret,
		id: clientId,
		limitRate: defaultLimitRate,
		redirectUri: redirectUri,
		requestCh: make(chan any, 256),
		retryCh: make(chan any),
		client: &http.Client{},
	}

	go client.run()

	return client
}

func (c *Client) run() {
	ticker := time.NewTicker(c.limitRate)

	for {
		select {
		case <-ticker.C:
			select {
			case <-c.requestCh:
			default:
			}
		case <-c.close:
			c.requestCh = nil
			return
		}
	}
}

func (c *Client) Close() {
	close(c.close)
}

func (c *Client) Authorize() (*SpotifyAuthorizationResponse, error) {
	defer func() {
		log.Println("returing from Authorize")
	}()
	respCh := make(chan *SpotifyAuthorizationResponse, 1)
	mux := http.NewServeMux()
	mux.HandleFunc("/callback", callbackHandler(c, respCh))

	s := &http.Server{
		Addr: "127.0.0.1:8888",
		Handler: mux,
	}

	go func() {
		log.Println("listening of redirect uri")
		if err := s.ListenAndServe(); err != nil {
			if errors.Is(err, http.ErrServerClosed) {
				return
			}

			log.Println(err)
		}
	}()

	defer s.Shutdown(context.Background())

	<-time.After(50*time.Millisecond)

	//if err := c.login(); err != nil {
	//	return nil, err
	//}

	if err := c.authorize(); err != nil {
		return nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)

	defer cancel()

	var resp *SpotifyAuthorizationResponse

	log.Println("waiting for response")
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case resp = <-respCh:
	}

	log.Println(resp)

	return resp, nil
}

func (c *Client) authorize() error {
	log.Println("sending authorization request")
	u, err := url.Parse(spotifyAuthorizationUrl)
	
	if err != nil {
		return err
	}

	fmt.Println(c.redirectUri)

	scope := strings.Join(defaultScopes, " ")
	urlValues := make(url.Values)

	urlValues.Set("response_type", "code")
	urlValues.Set("client_id", c.id)
	urlValues.Set("scope", scope)
	urlValues.Set("redirect_uri", c.redirectUri)
	urlValues.Set("state", c.state)

	u.RawQuery = urlValues.Encode()

	fmt.Println("raw query", u.RawQuery)

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return err
	}

	resp, err := c.client.Do(req)

	if err != nil {
		return err
	}

	log.Println(u.String())

	defer resp.Body.Close()

	return nil
}

type SpotifyAuthorizationResponse struct {
	AccessToken string `json:"access_token"`
	TokenType string `json:"token_type"`
	Scope string `json:"scope"`
	ExpiresIn int `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

type SpotifyRefreshTokenResponse struct {
	AccessToken string `json:"access_token"`
	TokenType string `json:"token_type"`
	Scope string `json:"scope"`
	ExpiresIn int `json:"expires_in"`
	RefreshToken string `json:"refresh_token"`
}

func callbackHandler(c *Client, respCh chan *SpotifyAuthorizationResponse) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println("received callback request from spotify")
		values := r.URL.Query()

		code := values.Get("code")
		state := values.Get("state")
		error := values.Get("error")

		if error != "" {
			log.Println(error)
			return
		}

		if state != c.state {
			log.Println("incorrect state")
			return
		}

		if code == "" {
			log.Println("did not get code")
			return
		}

		log.Println("state: ", state, "code: ", code)

		values = make(url.Values)

		values.Set("grant_type", "authorization_code")
		values.Set("code", code)
		values.Set("redirect_uri", c.redirectUri)

		u := "https://accounts.spotify.com/api/token"

		buf := bytes.NewBufferString(values.Encode())

		req, err := http.NewRequest(http.MethodPost, u, buf)

		if err != nil { 
			log.Println(err)
			return
		}

		encodedClientInfo := encodeClientInfo(c.id, c.secret)

		req.Header.Set("content-type", "application/x-www-form-urlencoded")
		req.Header.Set("Authorization", encodedClientInfo)

		authorizationResp := SpotifyAuthorizationResponse{}

		if err := c.fetchResponse(req, &authorizationResp); err != nil {
			log.Println(err)
			return
		}

		respCh <- &authorizationResp
	}
}

func (c *Client) RefreshToken(accessToken string, refreshToken string, id string) (*SpotifyRefreshTokenResponse, error) {
	values := make(url.Values)

	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", refreshToken)
	values.Set("client_id", id)

	encodedClientInfo := encodeClientInfo(c.id, c.secret)

	buf := bytes.NewBufferString(values.Encode())

	url := "https://accounts.spotify.com/api/token"

	req, err := http.NewRequest(http.MethodPost, url, buf)

	if err != nil {
		return nil, err
	}
	req.Header.Set("content-type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", encodedClientInfo)

	var rsp SpotifyRefreshTokenResponse

	if err := c.fetchResponse(req, &rsp); err != nil {
		return nil, err
	}

	return &rsp, nil
}

func encodeClientInfo(id, secret string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(id + ":" + secret))
}

func (c *Client) GetCurrentUsersPlaylists(accessToken string, limit int, offset int) (types.CurrentUsersPlaylistResponse, error) {

	var playlists types.CurrentUsersPlaylistResponse

	value := make(url.Values)
	value.Set("limit", strconv.Itoa(limit))
	value.Set("offset", strconv.Itoa(offset))

	u, err := url.Parse(spotifyWebApiUrl + "/playlists")

	if err != nil {
		return playlists, err
	}

	u.RawQuery = value.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return playlists, err
	}

	setAuthorizationHeader(req, accessToken)

	if err := c.fetchResponse(req, &playlists); err != nil {
		return playlists, err
	}

	return playlists, nil
}

func (c *Client) GetCurrentUserProfile(accessToken string) (types.User, error) {
	var profile types.User
	u, err := url.Parse(spotifyWebApiUrl)

	if err != nil {
		return profile, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return profile, err
	}

	setAuthorizationHeader(req, accessToken)

	if err := c.fetchResponse(req, &profile); err != nil {
		return profile, err
	}

	return profile, nil
}

func GetUsersTopItems[T any](client *Client, accessToken string, itemType string) (types.UsersTopItems[T], error) {
	var items types.UsersTopItems[T]

	if itemType != "tracks" && itemType != "artists" {
		return items, fmt.Errorf("incorrent item type provided")
	}

	path, err := url.JoinPath(spotifyWebApiUrl, "top", itemType)

	if err != nil {
		return items, err
	}

	u, err := url.Parse(path)

	if err != nil {
		return items, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return items, err
	}

	setAuthorizationHeader(req, accessToken)

	if err := client.fetchResponse(req, &items); err != nil {
		return items, err
	}

	return items, nil

}

func (c *Client) GetUsersTopTracks(accessToken string) (types.UsersTopItems[types.Track], error) {
	var tracks types.UsersTopItems[types.Track]
	path, err := url.JoinPath(spotifyWebApiUrl, "top", "tracks")

	if err != nil {
		return tracks, err
	}

	u, err := url.Parse(path)

	if err != nil {
		return tracks, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return tracks, err
	}

	setAuthorizationHeader(req, accessToken)

	if err := c.fetchResponse(req, &tracks); err != nil {
		return tracks, err
	}

	return tracks, nil
}

func (c *Client) GetUsersTopArtists(accessToken string) (types.UsersTopItems[types.Artist], error) {
	var artists types.UsersTopItems[types.Artist]

	path, err := url.JoinPath(spotifyWebApiUrl, "top", "artists")

	if err != nil {
		return artists, err
	}

	u, err := url.Parse(path)

	if err != nil {
		return artists, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return artists, err
	}

	setAuthorizationHeader(req, accessToken)

	var topArtists types.UsersTopArtists

	if err := c.fetchResponse(req, &topArtists); err != nil {
		return artists, err
	}

	return artists, nil
}

func (c *Client) GetPlaybackStateTrack(accessToken string) (types.PlaybackState, error) {
	var state types.PlaybackState

	path, err := url.JoinPath(spotifyWebApiUrl, "player")

	if err != nil {
		return state, err
	}

	values := make(url.Values)
	values.Set("market", "US")

	u, err := url.Parse(path)

	if err != nil {
		return state, err
	}
	u.RawQuery = values.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return state, err
	}

	setAuthorizationHeader(req, accessToken)

	if err := c.fetchResponse(req, &state); err != nil {
		return state, err
	}

	return state, nil
}

type SpotifyError struct {
	Status int `json:"status"`
	Message string `json:"message"`
}

func (s SpotifyError) Error() string {
	return fmt.Sprintf("status: %d, message: %s", s.Status, s.Message)
}

func (c *Client) TransferPlayback(accessToken string, deviceId string, play bool) error {
	payload := types.TransferPlaybackRequest{
		DeviceIds: []string{ deviceId },
		Play: play,
	}

	data, err := json.Marshal(payload)

	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(data)

	path, err := url.JoinPath(spotifyWebApiUrl, "player")
	
	if err != nil {
		return err
	}

	u, err := url.Parse(path)

	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", u.String(), buf)

	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer " + accessToken)
	req.Header.Set("content-type", "application/json")

	return c.fetchResponse(req, nil)
}

func (c *Client) GetAvailableDevices(accessToken string) (types.AvailableDevices, error) {
	var devices types.AvailableDevices

	path, err := url.JoinPath(spotifyWebApiUrl, "player", "devices")

	if err != nil {
		return devices, err
	}

	u, err := url.Parse(path)

	if err != nil {
		return devices, err
	}

	req, err := http.NewRequest(http.MethodGet, u.String(), nil)

	if err != nil {
		return devices, err
	}

	setAuthorizationHeader(req, accessToken)

	if err := c.fetchResponse(req, &devices); err != nil {
		return devices, err
	}

	return devices, nil
}

func (c *Client) GetCurrentlyPlaying(accessToken string) (types.CurrentlyPlaying, error) {
	var item types.CurrentlyPlaying

	path, err := url.JoinPath(spotifyWebApiUrl, "player", "currently-playing")

	if err != nil {
		return item, err
	}

	u, err := url.Parse(path)

	if err != nil {
		return item, err
	}

	values := make(url.Values)
	setMarket(values, "US")
	encodeUrl(u, values)

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return item, err
	}

	setAuthorizationHeader(req, accessToken)

	if err := fetchResponse(c, req, &item); err != nil {
		return item, err
	}

	return item, nil
}

type StartResumePayload struct {
	ContextUri string `json:"context_uri,omitempty"`
	Uris string `json:"uris,omitempty"`
	Offset []string `json:"offset,omitempty"`
	PositionMs int `json:"position_ms"`
}

func (c *Client) StartResumePlayback(accessTokens string, deviceId string) error {
	u, err := createPlaybackUrl("play")

	if err != nil {
		return err
	}

	values := make(url.Values)
	setDeviceId(values, deviceId)
	encodeUrl(u, values)

	payload := StartResumePayload{
		PositionMs: 0,
	}

	data, err := json.Marshal(payload)

	if err != nil {
		return err
	}

	buf := bytes.NewBuffer(data)

	req, err := http.NewRequest("PUT", u.String(), buf)

	if err != nil {
		return err
	}

	setAuthorizationHeader(req, accessTokens)

	return fetchResponse(c, req, nil)
}

func (c *Client) PausePlayback(accessTokens string, deviceId string) error {
	u, err := createPlaybackUrl("pause")

	if err != nil {
		return err
	}

	values := make(url.Values)
	setDeviceId(values, deviceId)
	encodeUrl(u, values)

	req, err := http.NewRequest("PUT", u.String(), nil)

	if err != nil {
		return err
	}

	setAuthorizationHeader(req, accessTokens)

	return fetchResponse(c, req, nil)
}

func (c *Client) SkipSong(accessToken string , deviceId string, direction string) error {
	u, err := createPlaybackUrl(direction)

	if err != nil {
		return err
	}

	values := make(url.Values)
	setDeviceId(values, deviceId)
	encodeUrl(u, values)

	req, err := http.NewRequest("POST", u.String(), nil)

	if err != nil {
		return err
	}

	setAuthorizationHeader(req, accessToken)

	return fetchResponse(c, req, nil)
}

func (c *Client) GetQueue(accessToken string) (types.UsersQueue, error) {
	var queue types.UsersQueue

	u, err := createPlaybackUrl("queue")

	if err != nil {
		return queue, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return queue, err
	}

	setAuthorizationHeader(req, accessToken)

	if err := fetchResponse(c, req, &queue); err != nil {
		return queue, err
	}

	return queue, nil
}

func createPlaylistUrl(s ...string) (*url.URL, error) {
	path, err := url.JoinPath(spotifyWebApiBaseUrl, s...)
	
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(path)

	if err != nil {
		return nil, err
	}

	return u, nil
}

func (c *Client) GetPlaylistItems(accessToken string, playlistId string) (types.Page[types.PlaylistItemUnion], error) {
	var page types.Page[types.PlaylistItemUnion]

	u, err := createPlaylistUrl("playlists", playlistId, "tracks")

	if err != nil {
		return page, err
	}

	values := make(url.Values)
	setMarket(values, "US")
	encodeUrl(u, values)

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return page, err
	}

	setAuthorizationHeader(req, accessToken)

	if err := fetchResponse(c, req, &page); err != nil {
		return page, err
	}

	return page, nil
}

func (c *Client) AddItemToQueue(accessToken string, uri string, deviceId string) error {
	u, err := createPlaybackUrl("queue")

	if err != nil {
		return err
	}

	values := make(url.Values)
	setUri(values, uri)
	setDeviceId(values, deviceId)
	encodeUrl(u, values)

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return err
	}

	return fetchResponse(c, req, nil)
}


func setDeviceId(values url.Values, deviceId string) {
	values.Set("device_id", deviceId)
}

func setMarket(values url.Values, country string) {
	values.Set("market", country)
}

func setUri(values url.Values, uri string) {
	values.Set("uri", uri)
}


func createPlaybackUrl(endpoint string) (*url.URL, error) {
	path, err := url.JoinPath(spotifyWebApiUrl, "player", endpoint)
	
	if err != nil {
		return nil, err
	}

	u, err := url.Parse(path)

	if err != nil {
		return nil, err
	}

	return u, nil
}

func encodeUrl(u *url.URL, values url.Values) {
	u.RawQuery = values.Encode()
}

func (c *Client) GetCurrentlyPlayingTrack(accessToken string) (any, error) {
	path, err := url.JoinPath(spotifyWebApiUrl, "player", "currently-playing")

	if err != nil {
		return nil, err
	}

	values := make(url.Values)
	values.Set("market", "US")

	u, err := url.Parse(path)

	if err != nil {
		return nil, err
	}
	u.RawQuery = values.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return nil, err
	}

	setAuthorizationHeader(req, accessToken)

	var obj map[string]any

	data, err := c.fetchResponseBytes(req)

	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &obj); err != nil {
		return nil, fmt.Errorf("unable to marshal data into obj %v", err)
	}

	val, ok := obj["currently_playing_type"]

	if !ok {
		return nil, fmt.Errorf("track or episode could not be found")
	}

	playingType, ok := val.(string)

	if !ok {
		return nil, fmt.Errorf("incorrect value for the specified key")
	}

	if playingType == "track" {
		var track types.CurrentlyPlayingTrack

		if err := json.Unmarshal(data, &track); err != nil {
			return nil, fmt.Errorf("err in unmarshaling currently playing track %v", err)
		}

		return &track, nil
	} else {
		return nil, fmt.Errorf("invalid type")
	}
}

func fetchResponse(c *Client, req *http.Request, v any) error {
	resp, err := c.client.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if v == nil {
		return checkResponseCode(resp)
	}

	return handleResponse(resp, v)
}

func (c *Client) fetchResponse(req *http.Request, v any) error {
	resp, err := c.client.Do(req)

	if err != nil {
		return err
	}

	defer resp.Body.Close()

	if v == nil {
		return checkResponseCode(resp)
	}

	return handleResponse(resp, v)
}

func (c *Client) fetchResponseBytes(req *http.Request) ([]byte, error) {
	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if err := checkResponseCode(resp); err != nil {
		return nil, err
	}

	data, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, fmt.Errorf("error reading resp body %v", err)
	}

	return data, nil
}

func handleResponse(resp *http.Response, v any) error {
	if err := checkResponseCode(resp); err != nil {
		return err
	}

	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		return err
	}

	return nil
}

func checkResponseCode(resp *http.Response) error {
	if resp.StatusCode == http.StatusOK {
		return nil
	}

	if resp.StatusCode == http.StatusNoContent {
		return SpotifyError {
			Status: http.StatusNoContent,
			Message: "no content available",
		}
	}

	msg := SpotifyError{}

	data, err := io.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	msg.Message = string(data)
	msg.Status = resp.StatusCode

	//if err := json.NewDecoder(resp.Body).Decode(&msg); err != nil {
	//	return fmt.Errorf("unable to read resp body for err msg %v", err)
	//}

	return msg
}

func setAuthorizationHeader(req *http.Request, accessToken string) {
	req.Header.Set("Authorization", "Bearer " + accessToken)
}

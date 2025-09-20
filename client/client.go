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

const (
	contentTypeUrlEncoded = "application/x-www-form-urlencoded"
	contentTypeJson = "application/json"
)

var (
	spotifyAuthorizationUrl = "https://accounts.spotify.com/authorize?"
	spotifyWebApiUrl = "https://api.spotify.com/v1/me"
	spotifyWebApiBaseUrl = "https://api.spotify.com/v1"
	spotifyTokenUrl = "https://accounts.spotify.com/api/token"
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

func getAppScope() string {
	return strings.Join(defaultScopes, " ")
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

	if err := c.authorize(context.Background()); err != nil {
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

type urlValues struct {
	v url.Values
}

func newUrlValues() *urlValues {
	return &urlValues{
		v: make(url.Values),
	}
}

func (u *urlValues) setQ(q string) {
	u.v.Set("q", q)
}

func (u *urlValues) setType(types []string) {
	u.v.Set("type", strings.Join(types, ","))
}

func (u *urlValues) setMarket(m string) {
	u.v.Set("market", m)
}

func (u *urlValues) setAfter(after int) {
	u.v.Set("after", strconv.Itoa(after))
}

func (u *urlValues) setBefore(before int) {
	u.v.Set("before", strconv.Itoa(before))
}

func (u *urlValues) setLimit(limit int) {
	u.v.Set("limit", strconv.Itoa(limit))
}

func (u *urlValues) setOffset(offset int) {
	u.v.Set("offset", strconv.Itoa(offset))
}

func (u *urlValues) setState(state string) {
	u.v.Set("state", state)
}

func (u *urlValues) setContextState(state string) {
	u.v.Set("state", state)
}

func (u *urlValues) setDeviceId(deviceId string) {
	u.v.Set("device_id", deviceId)
}

func (u *urlValues) setDirection(direction string) {
	u.v.Set("direction", direction)
}

func (u *urlValues) setUri(uri string) {
	u.v.Set("uri", uri)
}

func (u *urlValues) setPercent(percent int) {
	u.v.Set("percent", strconv.Itoa(percent))
}

func (u *urlValues) setResponseType(t string) {
	u.v.Set("response_type", t)
}

func (u *urlValues) setScope(t string) {
	u.v.Set("scope", t)
}

func (u *urlValues) setRedirectUri(uri string) {
	u.v.Set("redirect_uri", uri)
}

func (u *urlValues) setCode(code string) {
	u.v.Set("code", code)
}

func (u *urlValues) setRefreshToken(token string) {
	u.v.Set("refresh_token", token)
}

func (u *urlValues) setClientId(id string) {
	u.v.Set("client_id", id)
}

func (u *urlValues) setGrantType(t string) {
	u.v.Set("grant_type", t)
}

func (u *urlValues) encode(url *url.URL) {
	url.RawQuery = u.v.Encode()
}

func (u *urlValues) encodeToBuffer() *bytes.Buffer {
	return bytes.NewBufferString(u.v.Encode())
}

func setAndEncodeUrl(url *url.URL, params spotifyUrlParameters) {
	values := newUrlValues()
	params.set(values)
	values.encode(url)
}

type spotifyUrlParameters interface {
	set(v *urlValues)
}

type requestFactory struct {
	h http.Header
	method string
	u string
	body io.Reader
}

func newRequestFactory(method string, u string, body io.Reader) *requestFactory {
	return &requestFactory{
		h: make(http.Header),
		u: u,
		method: method,
		body: body,
	}
}

func (h *requestFactory) setAuthorization(auth string) {
	h.h.Set("Authorization", auth)
}

func (h *requestFactory) setContentType(t string) {
	h.h.Set("content-type", t)
}

func (h *requestFactory) setAccessTokenAuthorizationFromCtx(ctx context.Context) error {
	accessToken, err := GetAccessToken(ctx)

	if err != nil {
		return err
	}

	h.h.Set("Authorization", "Bearer " + accessToken)

	return nil
}

func (h *requestFactory) setRefreshTokenHeadersFromCtx(ctx context.Context) error {
	authInfo, err := GetClientInfoFromContext(ctx)

	if err != nil {
		return err
	}

	encodedClientInfo := encodeClientInfo(authInfo.clientId, authInfo.clientSecret)

	h.h.Set("Authorization", encodedClientInfo)
	h.h.Set("content-type", contentTypeUrlEncoded)

	return nil
}

func (h *requestFactory) newRequestWithContext(ctx context.Context) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, h.method, h.u, h.body)

	if err != nil {
		return nil, err
	}

	req.Header = h.h

	return req, err
}

func (c *Client) authorize(ctx context.Context) error {
	log.Println("sending authorization request")
	u, err := url.Parse(spotifyAuthorizationUrl)
	
	if err != nil {
		return err
	}

	fmt.Println(c.redirectUri)

	scope := getAppScope()
	values := newUrlValues()
	values.setResponseType("code")
	values.setClientId(c.id)
	values.setScope(scope)
	values.setRedirectUri(c.redirectUri)
	values.setState(c.state)

	values.encode(u)

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)

	if err != nil {
		return err
	}

	if err := fetchResponse(c, req, nil); err != nil {
		return err
	}

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

		vals := newUrlValues()
		vals.setGrantType("authorization_code")
		vals.setCode(code)
		vals.setRedirectUri(c.redirectUri)
		buf := vals.encodeToBuffer()

		encodedClientInfo := encodeClientInfo(c.id, c.secret)

		reqFactory := newRequestFactory(http.MethodPost, spotifyTokenUrl, buf)
		reqFactory.setAuthorization(encodedClientInfo)
		reqFactory.setContentType(contentTypeUrlEncoded)

		req, err := reqFactory.newRequestWithContext(context.Background())

		if err != nil { 
			log.Println(err)
			return
		}

		authorizationResp := SpotifyAuthorizationResponse{}

		if err := fetchResponse(c, req, &authorizationResp); err != nil {
			log.Println(err)
			return
		}

		respCh <- &authorizationResp
	}
}


func (c *Client) RefreshToken(ctx context.Context) (types.SpotifyRefreshTokenResponse, error) {
	var response types.SpotifyRefreshTokenResponse

	authInfo, err := GetClientInfoFromContext(ctx)

	if err != nil  {
		return response, err
	}

	values := newUrlValues()
	values.setGrantType("refresh_token")
	values.setRefreshToken(authInfo.refreshToken)
	values.setClientId(authInfo.clientId)

	buf := values.encodeToBuffer()

	reqFactory := newRequestFactory(http.MethodPost, spotifyTokenUrl, buf)
	reqFactory.setRefreshTokenHeadersFromCtx(ctx)

	req, err := reqFactory.newRequestWithContext(ctx)

	if err != nil {
		return response, err
	}

	if err := fetchResponse(c, req, &response); err != nil {
		return response, err
	}

	return response, nil
}

func encodeClientInfo(id, secret string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(id + ":" + secret))
}

func (c *Client) GetCurrentUsersPlaylists(ctx context.Context, limit int, offset int) (types.CurrentUsersPlaylistResponse, error) {
	var playlists types.CurrentUsersPlaylistResponse

	u, err := createApiUrl("playlists")

	if err != nil {
		return playlists, err
	}

	value := newUrlValues()
	value.setLimit(limit)
	value.setOffset(offset)
	value.encode(u)

	req, err := NewRequestFromContext(ctx, http.MethodGet, u.String(), nil)

	if err != nil {
		return playlists, err
	}

	if err := fetchResponse(c, req, &playlists); err != nil {
		return playlists, err
	}

	return playlists, nil
}

func (c *Client) GetCurrentUserProfile(ctx context.Context) (types.User, error) {
	var profile types.User

	u, err := createApiUrl()

	if err != nil {
		return profile, err
	}

	req, err := NewRequestFromContext(ctx, http.MethodGet, u.String(), nil)

	if err != nil {
		return profile, err
	}

	if err := fetchResponse(c, req, &profile); err != nil {
		return profile, err
	}

	return profile, nil
}

func GetUsersTopItems[T any](ctx context.Context, client *Client, itemType string) (types.UsersTopItems[T], error) {
	var items types.UsersTopItems[T]

	if itemType != "tracks" && itemType != "artists" {
		return items, fmt.Errorf("incorrent item type provided")
	}

	u, err := createApiUrl("top", itemType)


	if err != nil {
		return items, err
	}

	req, err := NewRequestFromContext(ctx, http.MethodGet, u.String(), nil)

	if err != nil {
		return items, err
	}

	if err := fetchResponse(client, req, &items); err != nil {
		return items, err
	}

	return items, nil

}

func (c *Client) GetPlaybackStateTrack(ctx context.Context) (types.PlaybackState, error) {
	var state types.PlaybackState

	u, err := createApiUrl("player")

	if err != nil {
		return state, err
	}

	values := newUrlValues()
	values.setMarket("US")
	values.encode(u)

	req, err := NewRequestFromContext(ctx, http.MethodGet, u.String(), nil)

	if err != nil {
		return state, err
	}

	if err := fetchResponse(c, req, &state); err != nil {
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

func (c *Client) TransferPlayback(ctx context.Context, deviceId string, play bool) error {
	u, err := createApiUrl("player")

	if err != nil {
		return err
	}

	payload := types.TransferPlaybackRequest{
		DeviceIds: []string{ deviceId },
		Play: play,
	}

	buf, err := marshalRequestBody(payload)

	if err != nil {
		return err
	}

	reqFactory := newRequestFactory(http.MethodPut, u.String(), buf)
	reqFactory.setContentType("application/json")

	req, err := NewRequestFromContext(ctx, http.MethodPut, u.String(), buf)

	if err != nil {
		return err
	}
	
	setContentTypeHeader(req, "application/json")

	return c.fetchResponse(req, nil)
}

func (c *Client) GetAvailableDevices(ctx context.Context) (types.AvailableDevices, error) {
	var devices types.AvailableDevices

	u, err := createApiUrl("player", "devices")

	if err != nil {
		return devices, err
	}

	req, err := NewRequestFromContext(ctx, http.MethodGet, u.String(), nil)

	if err != nil {
		return devices, err
	}

	if err := c.fetchResponse(req, &devices); err != nil {
		return devices, err
	}

	return devices, nil
}

func NewRequestFromContext(ctx context.Context, method string, url string, body io.Reader) (*http.Request, error) {
	accessToken, err := GetAccessToken(ctx)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)

	if err != nil {
		return nil, err
	}

	setAuthorizationHeader(req, accessToken)

	return req, nil
}

type GetCurrentlyPlayingParams struct {
	Market string
}

func (p GetCurrentlyPlayingParams) set(u *urlValues) {
	u.setMarket(p.Market)
}

func (c *Client) GetCurrentlyPlaying(ctx context.Context) (types.CurrentlyPlaying, error) {
	var item types.CurrentlyPlaying

	u, err := createApiUrl("player", "currently-playing")

	if err != nil {
		return item, err
	}

	values := make(url.Values)
	setMarket(values, "US")
	encodeUrl(u, values)

	req, err := NewRequestFromContext(ctx, http.MethodGet, u.String(), nil)

	if err != nil {
		return item, err
	}

	if err := fetchResponse(c, req, &item); err != nil {
		return item, err
	}

	return item, nil
}

type StartResumePayload struct {
	ContextUri string `json:"context_uri,omitempty"`
	Uris []string `json:"uris,omitempty"`
	Offset []string `json:"offset,omitempty"`
	PositionMs int `json:"position_ms"`
}

type OtherParams struct {
	ContextUri types.Optional[string]
	Uris types.Optional[[]string]
	PositionMs types.Optional[int]
}

type PlaybackActionParams struct {
	DeviceId string
	Action string
	Other types.Optional[OtherParams]
}

func (p PlaybackActionParams) isValid() bool {
	return p.Other.Valid
}

func (p PlaybackActionParams) getPayload() (io.Reader, error) {
	if p.Action != "play" {
		return nil, fmt.Errorf("incorrect action for this payload")
	}

	payload := StartResumePayload{}

	other := p.Other.Value

	if value := other.ContextUri.Value; other.ContextUri.Valid {
		payload.ContextUri = value
	}

	if value := other.Uris.Value; other.Uris.Valid {
		payload.Uris = value
	}

	if value := other.PositionMs.Value; other.PositionMs.Valid {
		payload.PositionMs = value
	}

	return marshalRequestBody(payload)

}

func (p PlaybackActionParams) set(u *urlValues) {
	u.setDeviceId(p.DeviceId)
}

func marshalRequestBody(payload any) (io.Reader, error) {
	data, err := json.Marshal(payload)

	if err != nil {
		return nil, err
	}

	buf := bytes.NewBuffer(data)

	return buf, nil
}

func (c *Client) StartResumePlayback(ctx context.Context, params PlaybackActionParams) error {
	u, err := createApiUrl("player", "play")

	if err != nil {
		return err
	}

	setAndEncodeUrl(u, params)

	payload := StartResumePayload{
		PositionMs: 0,
	}

	buf, err := marshalRequestBody(payload)

	if err != nil {
		return err
	}

	req, err := NewRequestFromContext(ctx, http.MethodPut, u.String(), buf)

	if err != nil {
		return err
	}

	return fetchResponse(c, req, nil)
}

func (c *Client) PlaybackAction(ctx context.Context, params PlaybackActionParams) error {
	u, err := createApiUrl("player", params.Action)

	if err != nil {
		return err
	}

	setAndEncodeUrl(u, params)

	var r io.Reader

	if params.isValid() {
		r, err = params.getPayload()

		if err != nil {
			return err
		}
	}

	req, err := NewRequestFromContext(ctx, http.MethodPut, u.String(), r)

	if err != nil {
		return err
	}

	setContentTypeHeader(req, "application/json")

	return fetchResponse(c, req, nil)
	
}

func (c *Client) PausePlayback(ctx context.Context, params PlaybackActionParams) error {
	u, err := createApiUrl("player", "pause")

	if err != nil {
		return err
	}

	setAndEncodeUrl(u, params)

	req, err := NewRequestFromContext(ctx, http.MethodPut, u.String(), nil)

	if err != nil {
		return err
	}

	return fetchResponse(c, req, nil)
}

type SkipSongParams struct {
	DeviceId string
	Direction string
}

func (p SkipSongParams) set(u *urlValues) {
	u.setDeviceId(p.DeviceId)
}

func (c *Client) SkipSong(ctx context.Context, params SkipSongParams) error {
	u, err := createApiUrl("player", params.Direction)

	if err != nil {
		return err
	}

	setAndEncodeUrl(u, params)

	req, err := NewRequestFromContext(ctx, http.MethodPost, u.String(), nil)

	if err != nil {
		return err
	}

	return fetchResponse(c, req, nil)
}

func (c *Client) GetQueue(ctx context.Context) (types.UsersQueue, error) {
	var queue types.UsersQueue

	u, err := createApiUrl("player", "queue")

	if err != nil {
		return queue, err
	}

	req, err := NewRequestFromContext(ctx, http.MethodGet, u.String(), nil)

	if err != nil {
		return queue, err
	}

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

type GetPlaylistParams struct {
	Id string
	Market string
	Fields []string
	AdditionalTypes string
}

func (p GetPlaylistParams) set(u *urlValues) {
	u.setMarket(p.Market)
}

func (c *Client) GetPlaylist(ctx context.Context, params GetPlaylistParams) (types.Playlist, error) {
	var playlist types.Playlist

	u, err := createBaseApiUrl("playlists", params.Id)

	if err != nil {
		return playlist, err
	}

	setAndEncodeUrl(u, params)

	req, err := NewRequestFromContext(ctx, "GET", u.String(), nil)

	if err != nil {
		return playlist, err
	}


	if err := fetchResponse(c, req, &playlist); err != nil {
		return playlist, err
	}

	return playlist, nil
}

type AddItemsToPlaylistParams struct {
	Id string
	Position types.Optional[int]
	Uris []string
}

func (p AddItemsToPlaylistParams) set(u *urlValues) {}

func (c *Client) AddItemsToPlaylist(ctx context.Context, params AddItemsToPlaylistParams) (types.PlaylistSnapshot, error) {
	var snapshot types.PlaylistSnapshot

	u, err := createBaseApiUrl("playlists", params.Id, "tracks")

	if err != nil {
		return snapshot, err
	}

	payload := make(map[string]any)

	payload["uris"] = params.Uris

	data, err := json.Marshal(payload)

	if err != nil {
		return snapshot, err
	}

	buf := bytes.NewBuffer(data)

	req, err := NewRequestFromContext(ctx, "POST", u.String(), buf)

	if err != nil {
		return snapshot, err
	}

	if err := fetchResponse(c, req, &snapshot); err != nil {
		return snapshot, err
	}

	return snapshot, nil
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

type AddItemToQueueParams struct {
	Uri string
	DeviceId string
}

func (p AddItemToQueueParams) set(u *urlValues) {
	u.setDeviceId(p.DeviceId)
	u.setUri(p.Uri)
}

func (c *Client) AddItemToQueue(ctx context.Context, params AddItemToQueueParams) error {
	u, err := createPlaybackUrl("queue")

	if err != nil {
		return err
	}

	setAndEncodeUrl(u, params)

	req, err := NewRequestFromContext(ctx, http.MethodPost, u.String(), nil)

	if err != nil {
		return err
	}

	return fetchResponse(c, req, nil)
}

type SetPlaybackVolumeParams struct {
	DeviceId string
	Percent int
}

func (p SetPlaybackVolumeParams) set(u *urlValues) {
	u.setDeviceId(p.DeviceId)
	u.setPercent(p.Percent)
}

func (c *Client) SetPlaybackVolume(ctx context.Context, params SetPlaybackVolumeParams) error {
	u, err := createPlaybackUrl("volume")

	if err != nil {
		return err
	}

	setAndEncodeUrl(u, params)

	req, err := NewRequestFromContext(ctx, "PUT", u.String(), nil)

	if err != nil {
		return err
	}

	return fetchResponse(c, req, nil)
}

func setPercentage(values url.Values, percent int) {
	values.Set("volume_percent", strconv.Itoa(percent))
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

func createApiUrl(s ...string) (*url.URL, error) {
	path, err := url.JoinPath(spotifyWebApiUrl, s...)

	u, err := url.Parse(path)

	if err != nil {
		return nil, err
	}

	return u, nil

}

func createBaseApiUrl(s ...string) (*url.URL, error) {
	path, err := url.JoinPath(spotifyWebApiBaseUrl, s...)

	u, err := url.Parse(path)

	if err != nil {
		return nil, err
	}

	return u, nil
}

func createUsersTracksUrl() (*url.URL, error) {
	path, err := url.JoinPath(spotifyWebApiUrl, "tracks")

	if err != nil {
		return nil, err
	}

	u, err := url.Parse(path)

	if err != nil {
		return nil, err
	}

	return u, nil
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


func (c *Client) GetUsersSavedTracks(ctx context.Context) (types.Page[types.SavedTrack], error) {
	var page types.Page[types.SavedTrack]

	accessToken, err := GetAccessToken(ctx)

	if err != nil {
		return page, ErrAccessTokenNotFound
	}

	u, err := createUsersTracksUrl()

	if err != nil {
		return page, err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)

	if err != nil {
		return page, err
	}

	setAuthorizationHeader(req, accessToken)

	if err := fetchResponse(c, req, &page); err != nil {
		return page, err
	}

	return page, nil
}

type RecentlyPlayedTracksParams struct {
	Limit int
	After int
	Before int
}

func (p RecentlyPlayedTracksParams) set(u *urlValues) {
	if p.Limit != 0 {
		u.setLimit(p.Limit)
	}

	var afterSet bool

	if p.After != 0 {
		u.setAfter(p.After)
		afterSet = true
	}

	if p.Before != 0 && !afterSet {
		u.setBefore(p.Before)
	}
}

func (c *Client) GetRecentlyPlayedTracks(ctx context.Context, params RecentlyPlayedTracksParams) (types.Page[types.PlayHistory], error) {
	var page types.Page[types.PlayHistory]

	u, err := createPlaybackUrl("recently-played")

	if err != nil {
		return page, err
	}

	setAndEncodeUrl(u, params)

	req, err := NewRequestFromContext(ctx, "GET", u.String(), nil)

	if err != nil {
		return page, err
	}

	if err := fetchResponse(c, req, &page); err != nil {
		return page, err
	}

	return page, nil

}

type SetRepeatModeParams struct {
	State string // required
	DeviceId string
}

func (p SetRepeatModeParams) set(u *urlValues) {
	u.setContextState(p.State)
	if p.DeviceId != "" {
		u.setDeviceId(p.DeviceId)
	}
}

func (c *Client) SetRepeatMode(ctx context.Context, params SetRepeatModeParams) error {
	u, err := createPlaybackUrl("repeat")

	if err != nil {
		return err
	}

	setAndEncodeUrl(u, params)

	req, err := NewRequestFromContext(ctx, "PUT", u.String(), nil)

	if err != nil {
		return err
	}

	if err := fetchResponse(c, req, nil); err != nil {
		return err
	}

	return nil
}

type GetArtistsTopTracksParams struct {
	Id string
	Market string
}

func (p GetArtistsTopTracksParams) set(u *urlValues) {
	u.setMarket(p.Market)
}

func (c *Client) GetArtistsTopTracks(ctx context.Context, params GetArtistsTopTracksParams) ([]types.Track, error) {
	var tracks []types.Track

	u, err := createBaseApiUrl("artists", params.Id, "top-tracks")

	if err != nil {
		return nil, err
	}

	setAndEncodeUrl(u, params)

	req, err := NewRequestFromContext(ctx, "GET", u.String(), nil)

	if err != nil {
		return nil, err
	}

	if err := fetchResponse(c, req, &tracks); err != nil {
		return nil, err
	}

	return tracks, nil
}

type GetSearchResultsParams struct {
	Q string
	Type []string
	Market string
	Limit int
	Offset int
}

func (p GetSearchResultsParams) set(u *urlValues) {
	u.setQ(p.Q)

	if len(p.Type) == 0 {
		u.setType([]string{"artist", "album", "track", "playlist"})
	} else {
		u.setType(p.Type)
	}
}

func (c *Client) GetSearchResults(ctx context.Context, params GetSearchResultsParams) (types.SearchResult, error) {
	var result types.SearchResult

	u, err := createBaseApiUrl("search")

	if err != nil {
		return result, err
	}

	setAndEncodeUrl(u, params)

	req, err := NewRequestFromContext(ctx, http.MethodGet, u.String(), nil)

	if err != nil {
		return result, err
	}

	if err := fetchResponse(c, req, &result); err != nil {
		return result, err
	}

	return result, nil
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

func setContentTypeHeader(req *http.Request, s string) {
	req.Header.Set("Content-Type", s)
}

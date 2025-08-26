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

		resp, err := c.client.Do(req)

		if err != nil {
			log.Println(err)
			return
		}

		defer resp.Body.Close()

		authorizationResp := SpotifyAuthorizationResponse{}

		if err := json.NewDecoder(resp.Body).Decode(&authorizationResp); err != nil {
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

	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, fmt.Errorf("http bad request")
	}

	var rsp SpotifyRefreshTokenResponse

	data, err := io.ReadAll(resp.Body)

	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(data, &rsp); err != nil {
		return nil, err
	}

	fmt.Println(rsp)

	return &rsp, err

}

func encodeClientInfo(id, secret string) string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(id + ":" + secret))
}

func (c *Client) GetCurrentUsersPlaylists(accessToken string, limit int, offset int) (*types.CurrentUsersPlaylistResponse, error) {

	value := make(url.Values)
	value.Set("limit", strconv.Itoa(limit))
	value.Set("offset", strconv.Itoa(offset))

	u, err := url.Parse(spotifyWebApiUrl + "/playlists")

	if err != nil {
		return nil, err
	}

	u.RawQuery = value.Encode()

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer " + accessToken)

	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	if resp.StatusCode == http.StatusBadRequest {
		return nil, fmt.Errorf("http bad request")
	}

	//playlistResp := pb.GetCurrentUsersPlaylistsResponse{}
	playlistResp := types.CurrentUsersPlaylistResponse{}

	if err := json.NewDecoder(resp.Body).Decode(&playlistResp); err != nil {
		return nil, err
	}

	return &playlistResp, nil
}

func (c *Client) GetCurrentUserProfile(accessToken string) (*types.UserProfile, error) {
	u, err := url.Parse(spotifyWebApiUrl)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer " + accessToken)

	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	profile := types.UserProfile{}

	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, err
	}

	return &profile, nil
}

func (c *Client) GetUsersTopTracks(accessToken string) (*types.UsersTopTracks, error) {
	path, err := url.JoinPath(spotifyWebApiUrl, "top", "tracks")

	if err != nil {
		return nil, err
	}

	u, err := url.Parse(path)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return nil, err
	}


	req.Header.Set("Authorization", "Bearer " + accessToken)

	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusBadRequest {
		return nil, fmt.Errorf("bad request")
	}

	defer resp.Body.Close()

	var topTracks types.UsersTopTracks

	if err := json.NewDecoder(resp.Body).Decode(&topTracks); err != nil {
		return nil, err
	}

	return &topTracks, nil
}

func (c *Client) GetUsersTopArtists(accessToken string) (*types.UsersTopArtists, error) {
	path, err := url.JoinPath(spotifyWebApiUrl, "top", "artists")

	if err != nil {
		return nil, err
	}

	u, err := url.Parse(path)

	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", u.String(), nil)

	if err != nil {
		return nil, err
	}


	req.Header.Set("Authorization", "Bearer " + accessToken)

	resp, err := c.client.Do(req)

	if err != nil {
		return nil, err
	}

	if resp.StatusCode == http.StatusBadRequest {
		return nil, fmt.Errorf("bad request")
	}

	defer resp.Body.Close()

	var topArtists types.UsersTopArtists

	if err := json.NewDecoder(resp.Body).Decode(&topArtists); err != nil {
		return nil, err
	}

	return &topArtists, nil
}

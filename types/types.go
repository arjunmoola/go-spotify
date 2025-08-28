package types

import (
	"encoding/json"
)

type Optional[T any] struct {
	Value T
	Valid bool
}

func (o *Optional[T]) UnmarshalJSON(b []byte) error {
	if string(b) == "null" {
		var zero T
		o.Value = zero
		return nil
	}
	o.Valid = true
	return json.Unmarshal(b, &o.Value)
}

type ItemUnion struct {
	Type string
	Track *Track
	Episode *Episode
}

func (i *ItemUnion) UnmarshalJSON(b []byte) error {
	unmarshal := unmarshaler(b)

	var probe struct { Type string `json:"type"` }

	if err := unmarshal(&probe); err != nil {
		return err
	}

	i.Type = probe.Type

	switch i.Type {
	case "track":
		var track Track
		if err := unmarshal(&track); err != nil {
			return err
		}
		i.Track = &track
	case "episode":
		var episode Episode
		if err := unmarshal(&episode); err != nil {
			return err
		}
		i.Episode = &episode
	}
	return nil
}

func unmarshaler(b []byte) func(v any) error {
	return func(v any) error {
		return json.Unmarshal(b, v)
	}
}

type CurrentUsersPlaylistResponse struct {
	Href string `json:"href"`
	Limit int `json:"limit"`
	Next string `json:"next,omitempty"`
	Offset int `json:"offset"`
	Previous string `json:"previous,omitempty"`
	Total int `json:"total"`
	Items []SimplifiedPlaylistObject `json:"items"`
}

type SimplifiedPlaylistObject struct {
	Id string `json:"id"`
	Collaborative bool `json:"collaborative,omitempty"`
	Description string `json:"description,omitempty"`
	Href string `json:"href"`
	Name string `json:"name"`
	SnapshotId string `json:"snapshot_id"`
	Items []SimplifiedPlaylistTrack `json:"items"`
	Type string `json:"type"`
	Uri string `json:"uri"`
}

func (s SimplifiedPlaylistObject) FilterValue() string {
	return ""
}

type SimplifiedPlaylistTrack struct {
	Href string `json:"href"`
	Total int `json:"total"`
}

type User struct {
	Country string `json:"country"`
	DisplayName string `json:"display_name"`
	Email string `json:"email"`
	ExplicitContent ExplicitContent `json:"explicit_content"`
	ExternalUrls ExternalUrl `json:"external_urls"`
	Href string `json:"href"`
	Id string `json:"id"`
	Product string `json:"product"`
	Uri string `json:"uri"`
}

type ExplicitContent struct {
	FilteredEnabled bool `json:"filtered_enabled"`
	FilterLocked bool `json:"filtered_locked"`
}

type UsersTopItems[T any] struct {
	Href string `json:"href"`
	Limit int `json:"limit"`
	Next Optional[string] `json:"next"`
	Previous Optional[string] `json:"previous"`
	Total int `json:"total"`
	Items []T `json:"items"`
}

type UserProfile struct {
	Id string `json:"id"`
	Country string `json:"country"`
	DisplayName string `json:"display_name"`
	Email string `json:"email"`
	ExplicitContent UserProfileExplicitContent `json:"explicit_content"`
	Followers UserProfileFollowers `json:"followers"`
	Product string `json:"product"`
	Type string `json:"type"`
	Uri string `json:"uri"`
}

type UserProfileExplicitContent struct {
	FilterEnabled bool `json:"filter_enabled"`
	FilterLocked bool `json:"filter_locked"`
}

type UserProfileFollowers struct {
	Href string `json:"href"`
	Total int `json:"total"`
}

type UsersTopTracks struct {
	Href string `json:"href"`
	Limit int `json:"limit"`
	Next string `json:"next"`
	Prev string `json:"previous,omitempty"`
	Offset int `json:"offset"`
	Total int `json:"total"`
	Items []*TopTrack `json:"items"`
}

type UsersTopArtists struct {
	Href string `json:"href"`
	Limit int `json:"limit"`
	Next string `json:"next,omitempty"`
	Prev string `json:"previous,omitempty"`
	Offset int `json:"offset"`
	Total int `json:"total"`
	Items []*TopArtists `json:"items"`
}

type TopArtists struct {
	Genres []string `json:"genres"`
	Href string `json:"href"`
	Id string `json:"id"`
	Name string `json:"name"`
	Popularity int `json:"popularity"`
	Type string `json:"type"`
	Uri string `json:"uri"`
}

type Artist struct {
	Genres []string `json:"genres"`
	Href string `json:"href"`
	Id string `json:"id"`
	Name string `json:"name"`
	Popularity int `json:"popularity"`
	Type string `json:"type"`
	Uri string `json:"uri"`
}

func (a Artist) FilterValue() string {
	return ""
}

type TopTrack struct {
	Albums []*Album `json:"albums"`
	Artists []*SimplifiedArtist `json:"artists"`
	DiscNumber int `json:"disc_number"`
	DurationMs int `json:"duration_ms"`
	Explicit bool `json:"explicit"`
	Href string `json:"href"`
	Id string `json:"id"`
	IsPlayable bool `json:"is_playable"`
	Name string `json:"name"`
	Popularity int `json:"popularity"`
	TrackNumber int `json:"track_number"`
	Type string `json:"type"`
	Uri string `json:"uri"`
	IsLocal bool `json:"is_local"`
}

type Album struct {
	Type string `json:"album_type"`
	TotalTracks int `json:"total_tracks"`
	AvailableMarkets []string `json:"available_markets"`
	Href string `json:"href"`
	Id string `json:"id"`
	Name string `json:"name"`
	ReleaseDate string `json:"release_date"`
	ReleaseDatePrecision string `json:"release_date_precision"`
}

func (a Album) FilterValue() string {
	return ""
}

type SimplifiedArtist struct {
	Href string `json:"href"`
	Id string `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
	Uri string `json:"uri"`
}

type PlaylistItems struct {
	Href string `json:"href"`
	Limit int `json:"limit"`
	Next string `json:"next"`
	Prev string `json:"previous"`
	Total int `json:"total"`
	Items []*PlaylistTrack `json:"items"`
}

type PlaylistTrack struct {
	AddAt string `json:"added_at"`
	Track *Track
}

type Track struct {
	Album Album `json:"album"`
	Artists []SimplifiedArtist `json:"artists"`
	DiscNumber int `json:"disc_number"`
	DurationMs int `json:"duration_ms"`
	Explicit bool `json:"explicit"`
	Href string `json:"href"`
	Id string `json:"id"`
	IsPlayable bool `json:"is_playable"`
	Name string `json:"name"`
	Popularity int `json:"popularity"`
	TrackNumber int `json:"track_number"`
	Type string `json:"type"`
	Uri string `json:"uri"`
	IsLocal bool `json:"is_local"`
}

func (t Track) FilterValue() string {
	return ""
}

type PlaybackState struct {
	Device Device `json:"device"`
	RepeatState bool `json:"repeat_state"`
	ShuffleState bool `json:"shuffle_state"`
	Context Optional[Context] `json:"context"`
	Timestamp int `json:"timestamp"`
	ProgressMs int `json:"progress_ms"`
	IsPlaying bool `json:"is_playing"`
	CurrentlyPlayingType string `json:"currently_playing_type"`
	Actions Action `json:"actions"`
	Item Optional[ItemUnion] `json:"item"`
}

type Episode struct {
	Description string `json:"description"`
	HtmlDescription string `json:"html_description"`
	DurationMs int `json:"duration_ms"`
	Explicit bool `json:"explicit"`
	Href string `json:"href"`
	Id string `json:"id"`
	IsExternallyHosted bool `json:"is_externally_hosted"`
	IsPlayable bool `json:"is_playable"`
	Languages []string `json:"languages"`
	Name string `json:"name"`
	ReleaseDate string `json:"release_date"`
	ResumePoint ResumePoint `json:"resume_point"`
	Type string `json:"type"`
	Uri string `json:"uri"`
	Restrictions Restrictions `json:"restrictions"`
}

func (e Episode) FilterValue() string {
	return ""
}

type Restrictions struct {
	Reason string `json:"reason"`
}

type Show struct {
	AvailableMarkets []string `json:"availabe_markets"`
	CopyRights []CopyRight `json:"copyrights"`
	Description string `json:"description"`
	HtmlDescription string `json:"html_description"`
	Explicit bool `json:"explicit"`
	Href string `json:"href"`
	Id string `json:"id"`
	IsExternallyHosted bool `json:"is_externally_hosted"`
	Languages []string `json:"languages"`
	MediaType string `json:"media_type"`
	Name string `json:"name"`
	Publisher string `json:"publisher"`
	Type string `json:"type"`
	Uri string `json:"uri"`
	TotalEpisodes int `json:"total_episodes"`
}

func (s Show) FilterValue() string {
	return ""
}

type ExternalUrl struct {
	Spotify string `json:"spotify"`
}

type CopyRight struct {
	Text string `json:"text"`
	Type string `json:"type"`
}

type ResumePoint struct {
	FullyPlayed bool `json:"fully_played"`
	ResumePointMs int `json:"resume_point_ms"`
}

type PlayBackStateTrack struct {
	Device *Device `json:"device"`
	RepeatState string `json:"repeat_state"`
	ShuffleState bool `json:"shuffle_state"`
	Context *Context `json:"context"`
	Timestamp int `json:"timestamp"`
	ProgressMs int `json:"progress_ms"`
	IsPlaying bool `json:"is_playing"`
	Item *Track `json:"track"`
	CurrentyPlayingType string `json:"currently_playing_type"`
	Actions Action `json:"actions"`
}

type CurrentlyPlaying struct {
	Device Device `json:"device"`
	RepeatState string `json:"repeat_state"`
	ShuffleState bool `json:"shuffle_state"`
	Context Optional[Context] `json:"context"`
	Timestamp int `json:"timestamp"`
	ProgressMs Optional[int] `json:"progress_ms"`
	IsPlaying bool `json:"is_playing"`
	Item Optional[ItemUnion] `json:"item"`
	CurrentlyPlayingType string `json:"currently_playing_type"`
	Actions Action `json:"actions"`
}

type CurrentlyPlayingTrack struct {
	Device Device `json:"device"`
	RepeatState string `json:"repeat_state"`
	ShuffleState bool `json:"shuffle_state"`
	Context Optional[Context] `json:"context"`
	Timestamp int `json:"timestamp"`
	ProgressMs Optional[int] `json:"progress_ms"`
	IsPlaying bool `json:"is_playing"`
	Item *Track `json:"track"`
	CurrentyPlayingType string `json:"currently_playing_type"`
	Actions Action `json:"actions"`
}

type Action struct {
	InterruptingPlayback bool `json:"interrupting_playback"`
	Pausing bool `json:"pausing"`
	Resuming bool `json:"resuming"`
	Seeking bool `json:"seeking"`
	SkippingNext bool `json:"skipping_next"`
	SkippingPrev bool `json:"skipping_prev"`
	TogglingRepeatContext bool `json:"toggling_repeat_context"`
	TogglingShuffle bool `json:"toggling_shuffle"`
	TogglingRepeatTrack bool `json:"toggling_repeat_track"`
	TransferringPlayback bool `json:"transferring_playback"`
}

type Context struct {
	Type string `json:"type"`
	Href string `json:"href"`
	Uri string `json:"uri"`
}

type AvailableDevices struct {
	Devices []Device `json:"devices"`
}

type Device struct {
	Id Optional[string] `json:"id"`
	IsActive bool `json:"is_active"`
	IsPrivateSession bool `json:"is_private_session"`
	IsRestricted bool `json:"is_restricted"`
	Name string `json:"name"`
	Type string `json:"type"`
	VolumePercent Optional[int] `json:"int"`
	SupportsVolumne bool `json:"supports_volume"`
}

func (d Device) FilterValue() string {
	return ""
}

type TransferPlaybackRequest struct {
	DeviceIds []string `json:"device_ids"`
	Play bool `json:"play"`
}

type StartResumePlaybackRequest struct {
	ContextUri string `json:"context_uri"`
	Uris []string `json:"context_uris"`
	Offset *StartResumePlaybackOffset `json:"offset,omitempty"`
}

type UsersQueue struct {
	CurrentlyPlaying Optional[ItemUnion] `json:"currently_playing"`
	Queue []ItemUnion `json:"queue"`
}


type StartResumePlaybackOffset struct {
	Position string `json:"position,omitempty"`
	Uri string `json:"uri,omitempty"`
}

type RecentlyPlayedTracks struct {
	Href string `json:"href"`
	Limit int `json:"limit"`
	Next string `json:"next"`
	Total int `json:"total"`
	Cursors *Cursors `json:"cursors"`
}

type PlayHistoryObject struct {
	Track *Track `json:"track"`
	PlayedAt string `json:"played_at"`
	Context *Context `json:"context"`
}

type Cursors struct {
	After string `json:"after"`
	Before string `json:"before"`
}

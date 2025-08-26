package types

type CurrentUsersPlaylistResponse struct {
	Href string `json:"href"`
	Limit int `json:"limit"`
	Next string `json:"next,omitempty"`
	Offset int `json:"offset"`
	Previous string `json:"previous,omitempty"`
	Total int `json:"total"`
	Items []*SimplifiedPlaylistObject `json:"items"`
}

type SimplifiedPlaylistObject struct {
	Id string `json:"id"`
	Collaborative bool `json:"collaborative,omitempty"`
	Description string `json:"description,omitempty"`
	Href string `json:"href"`
	Name string `json:"name"`
	SnapshotId string `json:"snapshot_id"`
	Tracks []*SimplifiedPlaylistTrack `json:"items"`
	Type string `json:"type"`
	Uri string `json:"uri"`
}

type SimplifiedPlaylistTrack struct {
	Href string `json:"href"`
	Total int `json:"total"`
}

type UserProfile struct {
	Id string `json:"id"`
	Country string `json:"country"`
	DisplayName string `json:"display_name"`
	Email string `json:"email"`
	ExplicitContent *UserProfileExplicitContent `json:"explicit_content"`
	Followers *UserProfileFollowers `json:"followers"`
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
	Next string `json:"next,omitempty"`
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
	Album *Album `json:"album"`
	Artists []*SimplifiedArtist `json:"artists"`
	DiscNumber int `json:"disc_number"`
	DurationMs int `json:"duration_ms"`
	Explicit bool `json:"explicit"`
	Href string `json:"href"`
	Id string `json:"id"`
	IsPlayable bool `json:"is_playable"`
	Name string `json:"name"`
	Popularity string `json:"popularity"`
	TrackNumber int `json:"track_number"`
	Type string `json:"type"`
	Uri string `json:"uri"`
	IsLocal bool `json:"is_local"`
}

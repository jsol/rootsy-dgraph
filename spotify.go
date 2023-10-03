package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Spotify struct {
	id      string
	secret  string
	token   string
	expires int64
}

type LoginResponse struct {
	AccessToken string `json:"access_token"`
	Duration    int64  `json:"expires_in"`
}

type SearchResponse struct {
	Albums struct {
		Href  string `json:"href"`
		Items []struct {
			AlbumType string `json:"album_type"`
			Artists   []struct {
				ExternalUrls map[string]string `json:"external_urls"`
				Href         string            `json:"href"`
				ID           string            `json:"id"`
				Name         string            `json:"name"`
				Type         string            `json:"type"`
				URI          string            `json:"uri"`
			} `json:"artists"`
			ExternalUrls map[string]string `json:"external_urls"`
			Href         string            `json:"href"`
			ID           string            `json:"id"`
			Images       []struct {
				Height int    `json:"height"`
				URL    string `json:"url"`
				Width  int    `json:"width"`
			} `json:"images"`
			Name                 string `json:"name"`
			ReleaseDate          string `json:"release_date"`
			ReleaseDatePrecision string `json:"release_date_precision"`
			TotalTracks          int    `json:"total_tracks"`
			Type                 string `json:"type"`
			URI                  string `json:"uri"`
		} `json:"items"`
	} `json:"albums"`
}

type SpotifyOption struct {
	Url   string
	Name  string
	Image string
}

func NewSpotify(id, secret string) (*Spotify, error) {
	ctx := Spotify{
		id:     id,
		secret: secret,
	}

	err := ctx.Login()

	if err != nil {
		return nil, err
	}

	return &ctx, nil
}

func (sp *Spotify) Login() error {
	v := url.Values{}
	// rant_type=client_credentials&client_id=&client_secret=

	v.Set("grant_type", "client_credentials")
	v.Set("client_id", sp.id)
	v.Set("client_secret", sp.secret)
	resp, err := http.PostForm("https://accounts.spotify.com/api/token", v)

	if err != nil {
		return err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)

	if err != nil {
		return err
	}

	cred := LoginResponse{}

	err = json.Unmarshal(body, &cred)
	if err != nil {
		return err
	}

	sp.token = cred.AccessToken
	sp.expires = time.Now().Unix() + cred.Duration

	return nil
}

func (sp *Spotify) Search(artist, album string) ([]SpotifyOption, error) {

	if time.Now().Unix() > sp.expires-5 {
		err := sp.Login()
		if err != nil {
			return nil, err
		}
	}

	url := "https://api.spotify.com/v1/search"

	http_client := http.Client{
		Timeout: time.Second * 5, // Timeout after 5 seconds
	}

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", sp.token))

	q := req.URL.Query()
	if strings.HasPrefix(artist, "Blandade") || strings.HasPrefix(artist, "Various") {
		q.Add("q", album)
	} else {
		q.Add("q", fmt.Sprintf("%s artist:%s", album, artist))
	}
	q.Add("type", "album")
	q.Add("limit", "15")

	req.URL.RawQuery = q.Encode()

	res, getErr := http_client.Do(req)
	if getErr != nil {
		return nil, getErr
	}

	if res.Body != nil {
		defer res.Body.Close()
	}

	body, readErr := io.ReadAll(res.Body)
	if readErr != nil {
		return nil, readErr
	}

	//fmt.Println(string(body))

	found := SearchResponse{}

	jsonErr := json.Unmarshal(body, &found)
	if jsonErr != nil {
		return nil, jsonErr
	}

	opts := []SpotifyOption{}
	for _, a := range found.Albums.Items {
		if a.AlbumType != "album" {
			continue
		}

		names := []string{}

		for _, v := range a.Artists {
			names = append(names, v.Name)
		}

		opt := SpotifyOption{
			Name:  fmt.Sprintf("%s - %s", strings.Join(names, " & "), a.Name),
			Url:   a.ExternalUrls["spotify"],
			Image: a.Images[0].URL,
		}
		opts = append(opts, opt)
	}

	return opts, nil
}

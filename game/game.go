package game

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Game struct {
	ID         int
	Name       string
	StartingAt time.Time
	League     string
	Season     string
}

type FixtureResponse struct {
	Data struct {
		ID                  int    `json:"id"`
		Name                string `json:"name"`
		StartingAtTimestamp int64  `json:"starting_at_timestamp"`
		League              struct {
			Name string `json:"name"`
		} `json:"league"`
		Season struct {
			Name string `json:"name"`
		} `json:"season"`
	} `json:"data"`
}

var (
	sportmonksApiKey    = os.Getenv("SPORTMONKS_API_KEY")
	sportmonksBaseURL   = "https://api.sportmonks.com"
	authorizationHeader = "Authorization"
	contentTypeHeader   = "Content-Type"
)

func Fetch(ctx context.Context, gameID int) (*Game, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		sportmonksBaseURL+fmt.Sprintf("/v3/football/fixtures/%d/?include=league:name,image_path;season:name;round:name;participants.country:name,image_path;scores;venue;lineups.player;statistics;referees.referee;state", gameID),
		http.NoBody,
	)
	if err != nil {
		return nil, err
	}

	req.Header.Add(authorizationHeader, sportmonksApiKey)
	req.Header.Add(contentTypeHeader, "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var fixture FixtureResponse
	if err := json.Unmarshal(body, &fixture); err != nil {
		return nil, err
	}

	return &Game{
		ID:         fixture.Data.ID,
		Name:       fixture.Data.Name,
		StartingAt: time.Unix(fixture.Data.StartingAtTimestamp, 0),
		League:     fixture.Data.League.Name,
		Season:     fixture.Data.Season.Name,
	}, nil
}

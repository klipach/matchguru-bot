package fixture

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type League struct {
	Name    string
	Country string
}

type Venue struct {
	Name    string
	City    string
	Country string
}

type Team struct {
	Name    string
	Country string
}

type Fixture struct {
	ID         int
	Name       string
	StartingAt time.Time
	League     League
	Venue      Venue
	Season     string
	HomeTeam   Team
	AwayTeam   Team
}

type FixtureAPIResponse struct {
	Data struct {
		ID                  int    `json:"id"`
		Name                string `json:"name"`
		StartingAtTimestamp int64  `json:"starting_at_timestamp"`
		League              struct {
			Name    string `json:"name"`
			Country struct {
				Name string `json:"name"`
			} `json:"country"`
		} `json:"league"`
		Season struct {
			Name string `json:"name"`
		} `json:"season"`
		Venue struct {
			Name    string `json:"name"`
			City    string `json:"city_name"`
			Country struct {
				Name string `json:"name"`
			} `json:"country"`
		} `json:"venue"`
		Participants []struct {
			Meta struct {
				Location string `json:"location"`
			} `json:"meta"`
			Name    string `json:"name"`
			Country struct {
				Name string `json:"name"`
			} `json:"country"`
		} `json:"participants"`
	} `json:"data"`
}

var (
	sportmonksApiKey    = os.Getenv("SPORTMONKS_API_KEY")
	sportmonksBaseURL   = "https://api.sportmonks.com"
	authorizationHeader = "Authorization"
	contentTypeHeader   = "Content-Type"
)

func Fetch(ctx context.Context, fixtureID int) (*Fixture, error) {
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		sportmonksBaseURL+fmt.Sprintf("/v3/football/fixtures/%d/?include=league:name;season:name;round:name;league.country;participants.country:name;scores;venue;venue.country;lineups.player;referees.referee;state", fixtureID),
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

	var fixture FixtureAPIResponse
	if err := json.Unmarshal(body, &fixture); err != nil {
		return nil, err
	}

	var homeTeam, awayTeam Team
	for _, participant := range fixture.Data.Participants {
		if participant.Meta.Location == "home" {
			homeTeam = Team{
				Name:    participant.Name,
				Country: participant.Country.Name,
			}
		} else {
			awayTeam = Team{
				Name:    participant.Name,
				Country: participant.Country.Name,
			}
		}
	}

	return &Fixture{
		ID:         fixture.Data.ID,
		Name:       fixture.Data.Name,
		StartingAt: time.Unix(fixture.Data.StartingAtTimestamp, 0),
		League: League{
			Name:    fixture.Data.League.Name,
			Country: fixture.Data.League.Country.Name,
		},
		Venue: Venue{
			Name:    fixture.Data.Venue.Name,
			City:    fixture.Data.Venue.City,
			Country: fixture.Data.Venue.Country.Name,
		},
		HomeTeam: homeTeam,
		AwayTeam: awayTeam,
		Season:   fixture.Data.Season.Name,
	}, nil
}

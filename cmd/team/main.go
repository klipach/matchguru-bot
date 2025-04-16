package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
)

type TeamAPIResponse struct {
	Data []struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	} `json:"data"`
	Pagination struct {
		CurrentPage int  `json:"current_page"`
		HasMore     bool `json:"has_more"`
	} `json:"pagination"`
}

// SPORTMONKS_API_KEY=*** go run cmd/team/main.go > teams.txt
func main() {
	apiKey := os.Getenv("SPORTMONKS_API_KEY")
	ctx := context.Background()
	teams, err := FetchTeams(ctx, apiKey)
	if err != nil {
		log.Fatalf("Failed to fetch teams: %v", err)
	}
	// format teams as go map[string]int, convert names to lowercase
	for name, id := range teams {
		fmt.Printf("\"%s\": %d,\n", strings.ToLower(name), id)
	}
}

// FetchTeams fetches all teams from the SportMonks API and returns them as a map
// where the key is the team name and the value is the team ID
func FetchTeams(ctx context.Context, apiKey string) (map[string]int, error) {
	teams := make(map[string]int)
	page := 1

	for {
		url := fmt.Sprintf("https://api.sportmonks.com/v3/football/teams?include=country&per_page=50&page=%d", page)
		log.Printf("fetching page %d from URL: %s", page, url)

		req, err := http.NewRequestWithContext(
			ctx,
			http.MethodGet,
			url,
			http.NoBody,
		)
		if err != nil {
			return nil, err
		}

		req.Header.Add("Authorization", apiKey)
		req.Header.Add("Content-Type", "application/json")

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

		var teamResponse TeamAPIResponse
		if err := json.Unmarshal(body, &teamResponse); err != nil {
			return nil, err
		}

		log.Printf("page %d: Got %d teams", page, len(teamResponse.Data))

		// Add teams to the map
		for _, team := range teamResponse.Data {
			teams[team.Name] = team.ID
		}

		if !teamResponse.Pagination.HasMore || len(teamResponse.Data) == 0 {
			log.Printf("breaking loop: page=%d, dataLength=%d", page, len(teamResponse.Data))
			break
		}
		page++
	}

	log.Printf("total teams fetched: %d", len(teams))
	return teams, nil
}

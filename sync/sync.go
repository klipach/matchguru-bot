// sync leagues from sportmonks API to postgresql
// table structure:
// team:
// 	id (internal)
// 	created_at
// 	updated_at
// 	hash
// 	league_id (sportmonks)
// 	raw_json
// 	vector from OpenAI
// output:
// 	created count
// 	updated count
// 	deleted count

// what else need to be synced?
// fixtures
// teams
// players

// algorithm:
// 1. get all leagues from sportmonks API (respecting pagination)
// 2. for each league:
// 	- check if it exists in the database
// 	- update/create if necessary
// 3. delete leagues that are not in the sportmonks API response (how?)
// 4. update the vector column for each league (if data has changed)
// 5. return the counts

package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"

	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
	openai "github.com/sashabaranov/go-openai"
)

const (
	dbDriver = "postgres"
	dbSource = "user=user password=pass dbname=bot host=127.0.0.1 port=5432 sslmode=disable"
)

type Data struct {
	Data []Team `json:"data"`
}

type Team struct {
    ID            int     `json:"id"`
    SportID       int     `json:"sport_id"`
    CountryID     int     `json:"country_id"`
    VenueID       int     `json:"venue_id"`
    Gender        string  `json:"gender"`
    Name          string  `json:"name"`
    ShortCode     string  `json:"short_code"`
    ImagePath     string  `json:"image_path"`
    Founded       int     `json:"founded"`
    Type          string  `json:"type"`
    Placeholder   bool    `json:"placeholder"`
    LastPlayedAt  string  `json:"last_played_at"`
    Country       Country `json:"country"`
}

type Country struct {
    ID          int    `json:"id"`
    ContinentID int    `json:"continent_id"`
    Name        string `json:"name"`
}

var schema = `
CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE team (
    id SERIAL PRIMARY KEY,
    created_at TIMESTAMP NOT NULL,
	updated_at TIMESTAMP NOT NULL,
	hash TEXT NOT NULL,
	raw_json JSONB NOT NULL,
	vector VECTOR(1538)
);`

func main() {
	// https://us-central1-match-guru-0iqc9r.cloudfunctions.net/api/v3/football/leagues?include=country:name,image_path&per_page=50&page=1

	// parse JSON from file
	file, err := os.Open("teams.json")
	if err != nil {
		panic(err)
	}
	defer file.Close()

	// Read file content
	data, err := io.ReadAll(file)
	if err != nil {
		panic(err)
	}
	fmt.Println(string(data))

	teamsData := Data{}
	err = json.Unmarshal(data, &teamsData)
	if err != nil {
		panic(err)
	}
	fmt.Println(teamsData)

	ctx := context.Background()
	openaiAPIKey := ""
	openaiClient := openai.NewClient(openaiAPIKey)

	openAIReq := openai.EmbeddingRequest{
	for _, team := range teamsData.Data {
		teamDesc := fmt.Sprintf("The team %s is from %s", team.Name, team.Country.Name)
	}
	res, err := openaiClient.CreateEmbeddings(ctx, openai.EmbeddingRequest{
		Input: teamDesc,
		Model: openai.AdaEmbeddingV2,
	})
	if err != nil {
		panic(err)
	}
	fmt.Printf("Response: %+v\n", res.Data)	


	db, err := sqlx.Connect(dbDriver, dbSource)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Connected to the database")
	// db.MustExec(schema)
	defer db.Close()
}

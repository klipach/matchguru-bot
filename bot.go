package matchguru

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"text/template"
	"time"
	_ "time/tzdata"

	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	"github.com/klipach/matchguru/auth"
	"github.com/klipach/matchguru/chat"
	"github.com/klipach/matchguru/contract"
	"github.com/klipach/matchguru/game"
	"github.com/klipach/matchguru/logger"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	gcloudFuncSourceDir = "serverless_function_source_code"
)

var (
	openaiAPIKey     = os.Getenv("OPENAI_API_KEY")
	perplexityApiKey = os.Getenv("PERPLEXITY_API_KEY")
)

func init() {
	functions.HTTP("Bot", Bot)
	fixDir()
}

// in GCP Functions, source code is placed in a directory named "serverless_function_source_code"
// need to change the dir to get access to template file
func fixDir() {
	fileInfo, err := os.Stat(gcloudFuncSourceDir)
	if err == nil && fileInfo.IsDir() {
		_ = os.Chdir(gcloudFuncSourceDir)
	}
}

func Bot(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	logger := logger.FromContext(ctx)

	logger.Println("bot function called")

	if r.Method != http.MethodPost {
		logger.Printf("invalid method: %s", r.Method)
		http.Error(w, "Method Not Implemented", http.StatusNotImplemented)
		return
	}

	token, err := auth.Authenticate(r)
	if err != nil {
		logger.Printf("error while authenticating: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	mainPrompt, err := template.New("main.tmpl").ParseFiles("prompts/main.tmpl")
	if err != nil {
		logger.Printf("error while parsing mainPrompt: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var msg contract.BotRequest
	data, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Printf("error while reading request body: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	logger.Printf("message: %v", string(data))

	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Printf("error while decoding request: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// test message:
	if strings.TrimSpace(msg.Message) == "test" {
		resp := contract.BotResponse{
			Response: `<b> hi there</b>
			<a href="/team/346">Team</a>
			<a href="/player/4237">Player</a>
			<a href="/league/384">League</a>
			<a href="/fixture/19155228">Fixture</a>
			<s>strikethrough</s>
			<i>italic</i>
			`,
		}
		err = json.NewEncoder(w).Encode(resp)
		if err != nil {
			logger.Printf("error while encoding response: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}
	loc, err := time.LoadLocation(msg.Timezone)
	if err != nil {
		logger.Printf("error while loading location: %v", err)
	}
	today := time.Now().In(loc).Format("2006-01-02")
	_, offset := time.Now().In(loc).Zone()
	timeOffset := fmt.Sprintf("%+03d:%+02d", offset/3600, offset/60%60)

	gg := &game.Game{}
	if msg.GameID != 0 {
		gg, err = game.Fetch(ctx, msg.GameID)
		if err != nil {
			logger.Printf("error while fetching game: %v", err)
		}
		logger.Printf("game fetched %v+", gg)
	}

	var mainPromptStr strings.Builder
	err = mainPrompt.Execute(
		&mainPromptStr,
		struct {
			Today          string
			TimeOffset     string
			GameName       string
			GameStartingAt time.Time
			GameLeague     string
			Season         string
			Country        string
		}{
			Today:          today,
			TimeOffset:     timeOffset,
			GameName:       gg.Name,
			GameStartingAt: gg.StartingAt,
			GameLeague:     gg.League,
			Season:         gg.Season,
			Country:        gg.Country,
		})
	if err != nil {
		logger.Printf("error while executing mainPrompt: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shortPrompt, err := template.New("short.tmpl").ParseFiles("prompts/short.tmpl")
	if err != nil {
		logger.Printf("error while parsing shortPrompt: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	var shortPromptStr strings.Builder
	err = shortPrompt.Execute(
		&shortPromptStr,
		struct {
			Today          string
			TimeOffset     string
			GameName       string
			GameStartingAt time.Time
			GameLeague     string
			Season         string
			Country        string
		}{
			Today:          today,
			TimeOffset:     timeOffset,
			GameName:       gg.Name,
			GameStartingAt: gg.StartingAt,
			GameLeague:     gg.League,
			Season:         gg.Season,
			Country:        gg.Country,
		})

	if err != nil {
		logger.Printf("error while executing shortPrompt: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	chatHistory, err := chat.LoadChatHistory(ctx, token.UID, msg.ChatID)
	if err != nil {
		logger.Printf("error while loading chat history: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	chatHistory = append(chatHistory, llms.TextParts(llms.ChatMessageTypeHuman, msg.Message))

	gpt4o, err := openai.New(
		openai.WithModel("gpt-4o"),
		openai.WithToken(openaiAPIKey),
	)
	if err != nil {
		logger.Printf("error while creating gpt4Turbo: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shortResp, err := gpt4o.GenerateContent(
		ctx,
		append(
			[]llms.MessageContent{
				llms.TextParts(llms.ChatMessageTypeSystem, shortPromptStr.String()),
			},
			chatHistory...,
		),
		llms.WithTemperature(0.8),
		llms.WithMaxTokens(1000),
	)
	if err != nil {
		logger.Printf("CreateChatCompletion short error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	logger.Printf("short response: %s", shortResp.Choices[0].Content)
	if strings.ToLower(shortResp.Choices[0].Content) != "no" {
		logger.Printf("external knowledge required, perplexity request: %s", shortResp.Choices[0].Content)
		perplexityClient, err := openai.New(
			// Supported models: https://docs.perplexity.ai/docs/model-cards
			openai.WithModel("llama-3.1-sonar-small-128k-online"),
			openai.WithBaseURL("https://api.perplexity.ai"),
			openai.WithToken(perplexityApiKey),
		)
		if err != nil {
			logger.Printf("error while creating perplexity client: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		perplexityPrompt, err := template.New("perplexity.tmpl").ParseFiles("prompts/perplexity.tmpl")
		if err != nil {
			logger.Printf("error while parsing perplexityPrompt: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		var perplexityPromptStr strings.Builder
		err = perplexityPrompt.Execute(
			&perplexityPromptStr,
			struct {
				Today      string
				TimeOffset string
			}{
				Today:      time.Now().Format("2006-01-02"),
				TimeOffset: timeOffset,
			})
		if err != nil {
			logger.Printf("error while executing perplexityPrompt: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		perplexityResp, err := perplexityClient.GenerateContent(ctx,
			[]llms.MessageContent{
				llms.TextParts(llms.ChatMessageTypeSystem, perplexityPromptStr.String()),
				llms.TextParts(llms.ChatMessageTypeHuman, shortResp.Choices[0].Content),
			},
			llms.WithTemperature(0.8),
			llms.WithMaxTokens(1000),
		)
		if err != nil {
			logger.Printf("error while generating from single prompt: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		logger.Printf("perplexity response: %s", perplexityResp.Choices[0].Content)

		mainPromptStr.WriteString("Additional info for the request: " + perplexityResp.Choices[0].Content)
	}

	completion, err := gpt4o.GenerateContent(
		ctx,
		append(
			[]llms.MessageContent{
				llms.TextParts(llms.ChatMessageTypeSystem, mainPromptStr.String()),
			},
			chatHistory...,
		),
		llms.WithTemperature(0.8),
		llms.WithMaxTokens(1000),
	)

	if err != nil {
		logger.Printf("ChatCompletion error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	logger.Printf("response: %s", completion.Choices[0].Content)
	response := process(completion.Choices[0].Content)

	// replace newlines with <br /> for HTML
	response = strings.Replace(response, "\n", "<br />", -1)

	// convert input markdown to HTML to render in app
	unsafeHTML := blackfriday.Run([]byte(response))

	// allow only tags that are supported by app
	policy := bluemonday.NewPolicy()
	policy.AllowElements("br", "s", "i", "b", "a")

	safeHTML := policy.SanitizeBytes(unsafeHTML)
	response = string(safeHTML)

	logger.Printf("processed response: %s", response)
	err = json.NewEncoder(w).Encode(
		contract.BotResponse{
			Response: response,
		},
	)
	if err != nil {
		logger.Printf("error while encoding response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

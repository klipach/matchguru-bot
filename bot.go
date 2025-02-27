package matchguru

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
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
	"github.com/klipach/matchguru/fixture"
	"github.com/klipach/matchguru/log"
	"github.com/microcosm-cc/bluemonday"
	"github.com/russross/blackfriday/v2"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	gcloudFuncSourceDir = "serverless_function_source_code"
	ErrorMsgField       = "errorMsg"
)

var (
	openaiAPIKey     = os.Getenv("OPENAI_API_KEY")
	perplexityApiKey = os.Getenv("PERPLEXITY_API_KEY")
)

type ()

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
	logger := log.New()

	logger.Info("bot function called")

	if r.Method != http.MethodPost {
		logger.Error("invalid method: " + r.Method)
		http.Error(w, "Method Not Implemented", http.StatusNotImplemented)
		return
	}

	token, err := auth.Authenticate(r)
	if err != nil {
		logger.Error("error while authenticating", slog.String(ErrorMsgField, err.Error()))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	mainPrompt, err := template.New("main.tmpl").ParseFiles("prompts/main.tmpl")
	if err != nil {
		logger.Error("error while parsing mainPrompt", slog.String(ErrorMsgField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var msg contract.BotRequest
	data, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("error while reading request body", slog.String(ErrorMsgField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	logger.Info(fmt.Sprintf("incoming request payload: %s", string(data)))

	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Error("error while decoding request", slog.String(ErrorMsgField, err.Error()))
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// test message
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
			logger.Error("error while encoding response", slog.String(ErrorMsgField, err.Error()))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}
	loc, err := time.LoadLocation(msg.Timezone)
	if err != nil {
		logger.Error("error while loading location", slog.String(ErrorMsgField, err.Error()))
	}
	userNow := time.Now().In(loc).Format(time.RFC1123Z)

	f := &fixture.Fixture{}
	if msg.GameID != 0 {
		f, err = fixture.Fetch(ctx, msg.GameID)
		f.StartingAt = f.StartingAt.In(loc)
		if err != nil {
			logger.Error("error while fetching game", slog.String(ErrorMsgField, err.Error()))
		}
		logger.Info(fmt.Sprintf("fixture fetched %v+", f))
	}

	shortPrompt, err := template.New("short.tmpl").ParseFiles("prompts/short.tmpl")
	if err != nil {
		logger.Error("error while parsing shortPrompt", slog.String(ErrorMsgField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	var shortPromptStr strings.Builder
	err = shortPrompt.Execute(
		&shortPromptStr,
		struct {
			UserNow string
			Fixture *fixture.Fixture
		}{
			UserNow: userNow,
			Fixture: f,
		},
	)

	if err != nil {
		logger.Error("error while executing shortPrompt", slog.String(ErrorMsgField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	messages, err := chat.LoadHistory(ctx, token.UID, msg.ChatID)
	if err != nil {
		logger.Error("error while loading chat history", slog.String(ErrorMsgField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, msg.Message))

	gpt4o, err := openai.New(
		openai.WithModel("gpt-4o"),
		openai.WithToken(openaiAPIKey),
	)
	if err != nil {
		logger.Error("error while creating gpt4Turbo", slog.String(ErrorMsgField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	shortResp, err := gpt4o.GenerateContent(
		ctx,
		append(
			[]llms.MessageContent{
				llms.TextParts(llms.ChatMessageTypeSystem, shortPromptStr.String()),
			},
			messages...,
		),
		llms.WithTemperature(0.8),
		llms.WithMaxTokens(1000),
	)
	if err != nil {
		logger.Error("failed to ret response shortPrompt", slog.String(ErrorMsgField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	logger.Info(fmt.Sprintf("short response: %s", shortResp.Choices[0].Content))
	var additionalInfo string
	if strings.ToLower(shortResp.Choices[0].Content) != "no" {
		logger.Info(fmt.Sprintf("external knowledge required, perplexity request: %s", shortResp.Choices[0].Content))
		perplexityClient, err := openai.New(
			// Supported models: https://docs.perplexity.ai/docs/model-cards
			openai.WithModel("llama-3.1-sonar-small-128k-online"),
			openai.WithBaseURL("https://api.perplexity.ai"),
			openai.WithToken(perplexityApiKey),
		)
		if err != nil {
			logger.Error("error while creating perplexity client", slog.String(ErrorMsgField, err.Error()))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		perplexityPrompt, err := template.New("perplexity.tmpl").ParseFiles("prompts/perplexity.tmpl")
		if err != nil {
			logger.Error("error while parsing perplexityPrompt", slog.String(ErrorMsgField, err.Error()))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		var perplexityPromptStr strings.Builder
		err = perplexityPrompt.Execute(
			&perplexityPromptStr,
			struct {
				UserNow string
			}{
				UserNow: userNow,
			})
		if err != nil {
			logger.Error("error while executing perplexityPrompt", slog.String(ErrorMsgField, err.Error()))
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
			logger.Error("error while generating from single prompt", slog.String(ErrorMsgField, err.Error()))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		logger.Info(fmt.Sprintf("perplexity response: %s", perplexityResp.Choices[0].Content))

		additionalInfo = perplexityResp.Choices[0].Content
	}

	var mainPromptStr strings.Builder
	err = mainPrompt.Execute(
		&mainPromptStr,
		struct {
			UserNow        string
			AdditionalInfo string
			Fixture        *fixture.Fixture
		}{
			UserNow:        userNow,
			AdditionalInfo: additionalInfo,
			Fixture:        f,
		},
	)
	if err != nil {
		logger.Error("error while executing mainPrompt", slog.String(ErrorMsgField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	logger.Info(fmt.Sprintf("mainPrompt: %s", mainPromptStr.String()))

	completion, err := gpt4o.GenerateContent(
		ctx,
		append(
			[]llms.MessageContent{
				llms.TextParts(llms.ChatMessageTypeSystem, mainPromptStr.String()),
			},
			messages...,
		),
		llms.WithTemperature(0.8),
		llms.WithMaxTokens(1000),
	)

	if err != nil {
		logger.Error("ChatCompletion error", slog.String(ErrorMsgField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	logger.Info(fmt.Sprintf("response: %s", completion.Choices[0].Content))
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

	logger.Info(fmt.Sprintf("processed response: %s", response))
	err = json.NewEncoder(w).Encode(
		contract.BotResponse{
			Response: response,
		},
	)
	if err != nil {
		logger.Error("error while encoding response", slog.String(ErrorMsgField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

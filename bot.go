package matchguru

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strconv"
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
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"
)

const (
	ErrorMsgLogField = "errorMsg"
	promptLogField   = "prompt"
	bodyLogField     = "body"

	gcloudFuncSourceDir = "serverless_function_source_code"
	openAIModel         = "gpt-4o-search-preview"
)

var (
	openaiAPIKey = os.Getenv("OPENAI_API_KEY")
)

// ModifyingRoundTripper removes the "temperature" field and adds "web_search_options".
type modifyingRoundTripper struct {
	rt http.RoundTripper
}

func (mrt *modifyingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read the initial request body.
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
	}

	// Attempt to modify JSON.
	var modifiedBody []byte
	var jsonData map[string]any
	if err := json.Unmarshal(bodyBytes, &jsonData); err == nil {
		// Remove the "temperature" key.
		delete(jsonData, "temperature")
		// Add "web_search_options" field with an empty object.
		jsonData["web_search_options"] = map[string]any{}
		if mBody, err := json.Marshal(jsonData); err == nil {
			modifiedBody = mBody
		} else {
			// If marshalling fails, fallback to the original body.
			modifiedBody = bodyBytes
		}
	} else {
		// Not valid JSON, so fallback.
		modifiedBody = bodyBytes
	}

	// Reset req.Body so it can be read by downstream clients.
	req.Body = io.NopCloser(bytes.NewBuffer(modifiedBody))
	// Update ContentLength accordingly.
	req.ContentLength = int64(len(modifiedBody))
	req.Header.Set("Content-Length", strconv.Itoa(len(modifiedBody)))

	return mrt.rt.RoundTrip(req)
}

// LoggingRoundTripper logs the outgoing request details.
type loggingRoundTripper struct {
	rt     http.RoundTripper
	logger *slog.Logger
}

func (lrt *loggingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	// Read the current request body.
	var bodyBytes []byte
	if req.Body != nil {
		bodyBytes, _ = io.ReadAll(req.Body)
	}
	// Reset req.Body so it can be read downstream.
	req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	lrt.logger.Info("Outgoing openAI request",
		slog.String("url", req.URL.String()),
		slog.String(bodyLogField, string(bodyBytes)),
	)
	return lrt.rt.RoundTrip(req)
}

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
		logger.Error("error while authenticating", slog.String(ErrorMsgLogField, err.Error()))
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	var msg contract.BotRequest
	data, err := io.ReadAll(r.Body)
	if err != nil {
		logger.Error("error while reading request body", slog.String(ErrorMsgLogField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	logger.Info("incoming reques", slog.String(bodyLogField, string(data)))

	if err := json.Unmarshal(data, &msg); err != nil {
		logger.Error("error while decoding request", slog.String(ErrorMsgLogField, err.Error()))
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
			logger.Error("error while encoding response", slog.String(ErrorMsgLogField, err.Error()))
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		return
	}
	loc, err := time.LoadLocation(msg.Timezone)
	if err != nil {
		logger.Error("error while loading location", slog.String(ErrorMsgLogField, err.Error()))
	}
	userNow := time.Now().In(loc).Format(time.RFC1123Z)

	f := &fixture.Fixture{}
	if msg.GameID != 0 {
		f, err = fixture.Fetch(ctx, msg.GameID)
		if err != nil {
			logger.Error("error while fetching fixture", slog.String(ErrorMsgLogField, err.Error()))
		} else {
			f.StartingAt = f.StartingAt.In(loc)
			logger.Info(fmt.Sprintf("fixture fetched %v+", f))
		}
	}

	messages, err := chat.LoadHistory(ctx, token.UID, msg.ChatID)
	if err != nil {
		logger.Error("error while loading chat history", slog.String(ErrorMsgLogField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// append user message to the messages history
	messages = append(messages, llms.TextParts(llms.ChatMessageTypeHuman, msg.Message))

	openAIClient, err := openai.New(
		openai.WithModel(openAIModel),
		openai.WithToken(openaiAPIKey),
		openai.WithHTTPClient(
			&http.Client{
				Transport: &modifyingRoundTripper{
					rt: &loggingRoundTripper{
						rt:     http.DefaultTransport,
						logger: logger,
					},
				},
			},
		),
	)
	if err != nil {
		logger.Error("error while creating openAI client", slog.String(ErrorMsgLogField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	mainPrompt, err := template.New("main.tmpl").ParseFiles("prompts/main.tmpl")
	if err != nil {
		logger.Error("error while parsing mainPrompt", slog.String(ErrorMsgLogField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	var mainPromptStr strings.Builder
	err = mainPrompt.Execute(
		&mainPromptStr,
		struct {
			UserNow string
			Fixture *fixture.Fixture
		}{
			UserNow: userNow,
			Fixture: f,
		},
	)
	if err != nil {
		logger.Error("error while executing mainPrompt", slog.String(ErrorMsgLogField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Set SSE headers for streaming
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	flusher, ok := w.(http.Flusher)
	if !ok {
		logger.Error("streaming unsupported!")
		http.Error(w, "Streaming unsupported!", http.StatusInternalServerError)
		return
	}

	_, err = openAIClient.GenerateContent(
		ctx,
		append(
			[]llms.MessageContent{
				llms.TextParts(llms.ChatMessageTypeSystem, mainPromptStr.String()),
			},
			messages...,
		),
		llms.WithStreamingFunc(func(ctx context.Context, chunk []byte) error {
			msg := contract.BotResponse{Response: string(chunk)}
			jsonData, err := json.Marshal(msg)
			if err != nil {
				return err
			}
			sseData := fmt.Sprintf("data: %s\n\n", jsonData)
			if _, err := w.Write([]byte(sseData)); err != nil {
				return err
			}
			flusher.Flush()
			return nil
		}),
		llms.WithMaxTokens(1000),
	)

	if err != nil {
		logger.Error("ChatCompletion error", slog.String(ErrorMsgLogField, err.Error()))
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

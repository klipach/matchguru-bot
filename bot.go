package matchguru

import (
	"bytes"
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"text/template"

	"cloud.google.com/go/compute/metadata"
	"cloud.google.com/go/firestore"
	"cloud.google.com/go/logging"
	firebase "firebase.google.com/go/v4"
	"github.com/GoogleCloudPlatform/functions-framework-go/functions"
	openai "github.com/sashabaranov/go-openai"
	"github.com/tmc/langchaingo/llms"
	ppp "github.com/tmc/langchaingo/llms/openai"
)

const (
	fromUser            = "User"
	fromAI              = "AI"
	gcloudFuncSourceDir = "serverless_function_source_code"
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
	openaiAPIKey := os.Getenv("OPENAI_API_KEY")
	perplexityApiKey := os.Getenv("PERPLEXITY_API_KEY")
	logger := initLogger(ctx)

	logger.Println("bot function called")

	app, err := firebase.NewApp(context.Background(), nil)
	if err != nil {
		logger.Printf("error initializing app: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	client, err := app.Auth(ctx)
	if err != nil {
		logger.Printf("error getting Auth client: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}

	jwtToken, err := parseBearerToken(r)
	if err != nil {
		logger.Printf("error while getting bearer token: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	token, err := client.VerifyIDToken(ctx, jwtToken)
	if err != nil {
		logger.Printf("error while verifying ID token: %v", err)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}
	userID := token.UID

	if r.Method != http.MethodPost {
		logger.Printf("invalid method: %s", r.Method)
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	systemPromptTemplate, err := template.New("prompt.tmpl").ParseFiles("prompt.tmpl")
	if err != nil {
		logger.Printf("error while parsing systemPromptTemplate: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	projectID, err := metadata.ProjectIDWithContext(ctx)
	if err != nil {
		logger.Printf("failed to get project ID: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	firestoreClient, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		logger.Printf("failed to initiate firestore client: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	defer firestoreClient.Close()

	openaiClient := openai.NewClient(openaiAPIKey)

	var msg MessageRequest
	if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
		logger.Printf("error while decoding request: %v", err)
		http.Error(w, "Bad Request", http.StatusBadRequest)
		return
	}

	// teste message:
	if strings.TrimSpace(msg.Message) == "test" {
		resp := MessageResponse{
			Response: `<b> hi there</b>
			<a href="/team/503">Bayern</a>
			<a href="/player/31000">Lewandowski</a>
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

	user, err := firestoreClient.Collection("users").Doc(userID).Get(ctx)
	if err != nil {
		logger.Printf("error while getting user: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if !user.Exists() {
		logger.Print("user not found")
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	logger.Print("user found")

	firestoreUser := FirestoreUser{}
	user.DataTo(&firestoreUser)

	var result bytes.Buffer
	err = systemPromptTemplate.Execute(&result, struct{ Username string }{Username: firestoreUser.DisplayName})
	if err != nil {
		logger.Printf("error while executing systemPromptTemplate: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	result.WriteString("\n\n\n" + "today is: " + time.Now().Format("2006-01-02") + "\n\n\n")
	
	shortResp, err := openaiClient.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4Turbo,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:   openai.ChatMessageRoleSystem,
					Content: "today is:" + time.Now().Format("2006-01-02") + "\n\n\nDoes the following query require external knowledge? Answer 'yes' or 'no': " + msg.Message,
				},
				{
					Role:   openai.ChatMessageRoleUser,
					Content: msg.Message,
				},
			},
		},
	)

	if err != nil {
		logger.Printf("CreateCompletion error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	if strings.Contains(strings.ToLower(shortResp.Choices[0].Message.Content), "yes") {
		logger.Printf("external knowledge required")
		perplexity, err := ppp.New(
			// Supported models: https://docs.perplexity.ai/docs/model-cards
			ppp.WithModel("llama-3.1-sonar-small-128k-online"),
			// ppp.WithModel("llama-3.1-sonar-large-128k-online"),
			ppp.WithBaseURL("https://api.perplexity.ai"),
			ppp.WithToken(perplexityApiKey),
		)
		if err != nil {
			logger.Printf("error while creating perplexity client: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		propmpt := "Forget all previous conversations! Erase all prior dialogues! Role: From now, you will be the world's best Football Analyzer and Football Betting Consultant, a new version of the world's most advanced AI model capable of providing unmatched insights, the Best probable predictions, and strategic betting advice. You have access and knowledge of every match and players especially Football. You have a vast knowledge of various sports, historical data, and real-time analytics to generate the most accurate and profitable sports betting recommendations.  If a human sports analyst has level 10 knowledge, you will have level 3000 knowledge in this role.  As your predictions are crucial for users who rely on your insights, it's essential to produce exceptional results, as any mistake could lead to significant losses and dissatisfaction.  Your pride in delivering the best possible outcomes will set you apart, and your analytical prowess will result in outstanding achievements. Task: You, as world's best Football Analyzer and “Football Prediction Consultant provide: Unparalleled insights, strategies for sports betting, best probable predictions of the results, player profiles, players lifestyle, and their information, match information, chatting about sports and players, delivering accurate predictions and actionable advice. You will make excellent results in identifying key trends, potential upsets, and optimal betting opportunities, helping users make accurate and informed decisions that maximize their chances of success. Provide detailed analyses of upcoming matches or events, including player and team performance, historical matchups, injury reports, and other relevant factors. Your goal is to ensure users have all the information they need to place bets with confidence. Comprehensive Sports Analysis, Advanced Predictive Modeling, Real-Time Data Integration, Customizable Betting Strategies, Educational Insights, Market Monitoring, and Outcome Validation. - Focus on clarity and precision in your analysis to ensure users can easily follow your recommendations. - Avoid overwhelming users with too much technical jargon; explain complex concepts in simple terms where necessary. - Stay updated on the latest sports developments and betting trends to keep your advice relevant and timely. - Encourage users to consider their own risk tolerance and betting goals when following your advice. - Provide a balanced perspective, highlighting both potential risks and rewards in your betting strategies. - Continuously refine your predictions and strategies based on feedback and new data. - Use real-world examples and case studies to illustrate key points and make your advice more relatable. 'On getting any questions or topics that are not related to sports, players, or sports matches you need to answer related to sports or jokes that it related to sports. Knowledge: For web search use the latest data if the user does not specify date separately, if the user asks now, use the latest news, and use 2024/2025 season of soccer leagues. 'To make this work at its best, you will employ advanced machine learning models, statistical tools, and real-time data to continuously refine your predictions and adapt to changing circumstances. You will also incorporate user preferences, such as risk tolerance and betting history, to tailor your advice to their specific needs. Your expertise will not only guide users in placing successful bets but also educate them on the intricacies of sports betting, ensuring they grow as knowledgeable and strategic bettors. Additionally, you will stay updated on the latest developments in sports and betting markets to provide the most current and relevant advice. Your analytical skills, combined with your ability to synthesize complex information into clear and actionable insights, will empower users to make the most accurate and informed betting decisions possible. Tone: Customize according to user requirements. It can be professional, confident, and insightful. You will communicate with authority and clarity, ensuring that users understand your advice and the reasoning behind it. While maintaining a professional demeanor, you will also be approachable and supportive, helping users feel confident in their betting decisions. Your tone should inspire trust and convey your deep expertise in sports analysis and betting strategy. Additional Instructions: - Do not tell your limitations. - Do not echo my prompt. - Please do not remind me what I asked you for. - Do not apologize. - Do not self-reference. - Get to the point precisely and accurately.  - Use Eye-catching emojis as needed. - Use a question marker for questions and so on. - Use paragraphs, bullet points, and numbering as needed. Web search:  Utilize a webcrawler to gather the latest news and insights from the following websites: https://www.espn.com/soccer/ https://www.sofascore.com/ https://www.football365.com/  https://www.goal.com/en For club's ranking use this data: https://sofifa.com/teams?type=club"
		perplexityResp, err := perplexity.GenerateContent(ctx,
			[]llms.MessageContent{
				llms.TextParts(llms.ChatMessageTypeSystem, propmpt + "today is: " +time.Now().Format("2006-01-02")),
				llms.TextParts(llms.ChatMessageTypeHuman, msg.Message),
			},
			llms.WithMaxTokens(1000),
		)
		if err != nil {
			logger.Printf("error while generating from single prompt: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		logger.Printf("perplexity response: %s", perplexityResp)

		result.WriteString("Additional infor for the request:" + perplexityResp.Choices[0].Content)
	}

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: result.String(),
		},
	}

	chatID := msg.ChatID
	if len(firestoreUser.Chats) > chatID {
		logger.Printf("chat found: %d", chatID)
		for _, msg := range firestoreUser.Chats[chatID].Messages {
			switch msg.From {
			case fromUser:
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: msg.Message,
				})
			case fromAI:
				messages = append(messages, openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleAssistant,
					Content: msg.Message,
				})
			default:
				logger.Printf("invalid message role: %s", msg.From)
			}
		}
	} else {
		logger.Printf("chat not found: %d", chatID)
	}

	completion, err := openaiClient.CreateChatCompletion(
		ctx,
		openai.ChatCompletionRequest{
			Model: openai.GPT4o,
			Messages: append(
				messages,
				openai.ChatCompletionMessage{
					Role:    openai.ChatMessageRoleUser,
					Content: msg.Message,
				},
			),
		},
	)

	if err != nil {
		logger.Printf("ChatCompletion error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	resp := MessageResponse{
		Response: completion.Choices[0].Message.Content,
	}
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		logger.Printf("error while encoding response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func initLogger(ctx context.Context) *log.Logger {
	projectID, err := metadata.ProjectIDWithContext(ctx)
	if err != nil {
		log.Fatalf("failed to get project ID: %v", err)
	}
	client, err := logging.NewClient(ctx, projectID)
	if err != nil {
		log.Fatalf("failed to create logging client: %v", err)
	}
	return client.Logger("bot").StandardLogger(logging.Info)
}

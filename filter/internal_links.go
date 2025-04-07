package filter

import (
	"context"
	"log/slog"
	"regexp"
	"strconv"
	"strings"

	"github.com/klipach/matchguru/log"
)

var (
	internalLinkRegex = regexp.MustCompile(`\{([^}]+)\}`)
	// source https://api.sportmonks.com/v3/football/leagues?include=country:name&per_page=100
	leagueNameToID = map[string]int{ // league name should be in lowercase
		"english premier league":       8,
		"championship":                 9,
		"fa cup":                       24,
		"carabao cup":                  27,
		"eredivisie":                   72,
		"bundesliga":                   82,
		"austrian bundesliga":          181,
		"austrian football bundesliga": 181,
		"belgian pro league":           208,
		"pro league":                   208,
		"1. hnl":                       244,
		"superliga":                    271,
		"french ligue 1":               301,
		"ligue 1":                      301,
		"serie a":                      384,
		"italian serie a":              384,
		"serie b":                      387,
		"coppa italia":                 390,
		"eliteserien":                  444,
		"polish ekstraklasa":           453,
		"ekstraklasa":                  453,
		"portuguese primeira liga":     462,
		"liga portugal":                462,
		"premier league":               486,
		"premiership":                  501,
		"spanish la liga":              564,
		"la liga 2":                    567,
		"copa del rey":                 570,
		"allsvenskan":                  573,
		"swiss super league":           591,
		"super league":                 591,
		"turkish süper lig":            600,
		"super lig":                    600,
		"ukraine premier league":       609,
		"uefa europa league play-offs": 1371,
	}
)

type InternalLinkFilter struct {
	buffer    string
	buffering bool
}

func (ilf *InternalLinkFilter) ProcessChunk(ctx context.Context, chunk string) string {
	if chunk == "" { // empty chunk - end of stream
		ilf.buffering = false
		ret := ilf.buffer
		ilf.buffer = ""
		return ret
	}
	if internalLinkRegex.MatchString(chunk) { // if chunk is an internal link, trying to process
		ilf.buffering = false
		ret := ilf.buffer + chunk
		ilf.buffer = ""
		return convertInternalLinks(ctx, ret)
	}
	if strings.Contains(chunk, "{") {
		if ilf.buffering { // if we are in buffering state and see second {, flush buffer and start to buffer again
			ret := ilf.buffer
			ilf.buffer = chunk
			return ret
		}
		ilf.buffering = true
		ilf.buffer += chunk
		return ""
	}
	if strings.Contains(chunk, "}") && ilf.buffering { // potential internal link, trying to process
		ret := ilf.buffer + chunk
		ret = convertInternalLinks(ctx, ret)
		ilf.buffering = false
		ilf.buffer = ""
		return ret
	}
	if ilf.buffering { // if in buffering state, means there was a { symbol
		ilf.buffer += chunk
		return ""
	}
	return chunk
}

// convertInternalLinks converts text wrapped in curly braces to internal links
func convertInternalLinks(ctx context.Context, text string) string {
	return internalLinkRegex.ReplaceAllStringFunc(text, func(match string) string {
		logger := log.LoggerFromContext(ctx)
		// extract the text between curly braces
		content := match[1 : len(match)-1]

		parts := strings.Split(content, "|")
		if len(parts) != 2 {
			logger.Info("invalid internal link", slog.String("match", match))
			return ""
		}

		leagueName := parts[0]
		leagueNameInEnglish := parts[1]
		if leagueID, ok := leagueNameToID[strings.ToLower(leagueNameInEnglish)]; ok {
			return "[" + leagueName + "](leagues/" + strconv.Itoa(leagueID) + ")"
		}

		// if league not found, just return the content without braces
		logger.Info("league mapping not found", slog.String("leagueName", leagueNameInEnglish))
		return leagueName
	})
}

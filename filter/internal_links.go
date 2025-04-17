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

		linkTitle := parts[0]
		linkTitleInEnglish := parts[1]

		if leagueID, ok := leagueNameToID[strings.ToLower(linkTitleInEnglish)]; ok {
			if leagueID == 0 { // league found but no mapping yet
				return linkTitle
			}
			return "[" + linkTitle + "](leagues/" + strconv.Itoa(leagueID) + ")"
		}
		if teamID, ok := teamNameToID[strings.ToLower(linkTitleInEnglish)]; ok {
			return "[" + linkTitle + "](teams/" + strconv.Itoa(teamID) + ")"
		}

		// if link mapping not found, just return the content without braces
		logger.Info("link mapping not found", slog.String("linkTitle", strings.ToLower(linkTitleInEnglish)))
		return linkTitle
	})
}

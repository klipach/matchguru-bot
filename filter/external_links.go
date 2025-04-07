package filter

import (
	"context"
	"regexp"
	"strings"
)

var (
	externalLinkRegex = regexp.MustCompile(`\(?\[[^\]]*\]\([^)]*\)\)?`)
)

type ExternalLinkFilter struct {
	buffer    string
	buffering bool
}

func (ef *ExternalLinkFilter) ProcessChunk(_ context.Context, chunk string) string {
	if chunk == "" { // empty chunk - end of stream
		ef.buffering = false
		ret := ef.buffer
		ef.buffer = ""
		return externalLinkRegex.ReplaceAllString(ret, "")
	}
	if externalLinkRegex.MatchString(chunk) { // if chunk is a link, remove it and return the chunk
		ef.buffering = false
		ret := ef.buffer + chunk
		ef.buffer = ""
		return externalLinkRegex.ReplaceAllString(ret, "")
	}
	if strings.Contains(chunk, "[") {
		if ef.buffering { // if we are in buffering state and see second [, flush buffer and start to buffer again
			ret := ef.buffer
			ef.buffer = chunk
			return ret
		}
		ef.buffering = true
		ef.buffer += chunk
		return ""
	}
	if strings.Contains(chunk, "]") && !strings.Contains(chunk, "](") { // not a link, stop buffering
		ef.buffering = false
		ret := ef.buffer
		ef.buffer = ""
		return ret + chunk
	}
	if strings.Contains(chunk, ")") && ef.buffering { // potential link, trying to remove
		ret := ef.buffer + chunk
		ret = externalLinkRegex.ReplaceAllString(ret, "")
		ef.buffering = false
		ef.buffer = ""
		return ret
	}
	if ef.buffering { // in buffering state, means there was a [ symbol
		ef.buffer += chunk
		return ""
	}
	return chunk
}

package matchguru

import (
	"regexp"
	"strings"
)

var (
	markdownLinkRegex = regexp.MustCompile(`\(?\[[^\]]*\]\([^)]*\)\)?`)
)

type MarkdownLinkFilter struct {
	buffer    string
	buffering bool
}

func (mf *MarkdownLinkFilter) ProcessChunk(chunk string) string {
	if chunk == "" { // empty chunk - end of stream
		mf.buffering = false
		ret := mf.buffer
		mf.buffer = ""
		return ret
	}
	if markdownLinkRegex.MatchString(chunk) { // if chunk is a link, remove it and return the chunk
		mf.buffering = false
		ret := mf.buffer + chunk
		mf.buffer = ""
		return markdownLinkRegex.ReplaceAllString(ret, "")
	}
	if strings.Contains(chunk, "[") {
		if mf.buffering { // if we are in buffering state and see second [, flush buffer and start to buffer again
			ret := mf.buffer
			mf.buffer = chunk
			return ret
		}
		mf.buffering = true
		mf.buffer += chunk
		return ""
	}
	if strings.Contains(chunk, "]") && !strings.Contains(chunk, "](") { // not a link, stop buffering
		mf.buffering = false
		ret := mf.buffer
		mf.buffer = ""
		return ret + chunk
	}
	if strings.Contains(chunk, ")") && mf.buffering { // potential link, trying to remove
		ret := mf.buffer + chunk
		ret = markdownLinkRegex.ReplaceAllString(ret, "")
		mf.buffering = false
		mf.buffer = ""
		return ret
	}
	if mf.buffering { // if in buffering state, means there was a [ symbol
		mf.buffer += chunk
		return ""
	}
	return chunk
}

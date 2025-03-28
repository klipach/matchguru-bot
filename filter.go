package matchguru

import (
	"regexp"
	"strings"
)

var (
	markdownLinkRegex = regexp.MustCompile(`\[[^\]]*\]\([^)]*\)`)
)

type MarkdownLinkFilter struct {
	buffer    string
	buffering bool
}

func (mf *MarkdownLinkFilter) ProcessChunk(chunk string) string {
	if chunk == "" { // empty chunk - end of stream
		return mf.buffer
	}
	if markdownLinkRegex.MatchString(chunk) {
		mf.buffering = false
		ret := mf.buffer
		mf.buffer = ""
		return markdownLinkRegex.ReplaceAllString(ret+chunk, "")
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
		mf.buffer += chunk
		mf.buffer = markdownLinkRegex.ReplaceAllString(mf.buffer, "")
		mf.buffering = false
		ret := mf.buffer
		mf.buffer = ""
		return ret
	}
	if mf.buffering { // if in buffering state, means there was a [ symbol
		mf.buffer += chunk
		return ""
	}
	return chunk
}

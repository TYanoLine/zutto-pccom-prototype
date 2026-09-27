package worldrepo

import (
	"fmt"
	"hash/fnv"
	"strings"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

func (r *Repository) prepareBoardComposition(req BoardMaterializationRequest, selected world.Post) BoardMaterializationRequest {
	if req.Kind == "" {
		req.Kind = worldengine.PostKindNewPost
	}
	if selected.ParentID != 0 || selected.Intent.Action == "reply" {
		req.Kind = worldengine.PostKindReply
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte(fmt.Sprintf("%s|%s|%d|%s|%s|%s", req.Host.ID, req.BoardID, selected.ID, selected.Subject, req.WorldDate, req.Kind)))
	band := worldengine.SelectBodyLengthBand(h.Sum64(), req.Kind)
	if req.BodyMinChars == 0 {
		req.BodyMinChars = band.Min
	}
	if req.BodyMaxChars == 0 {
		req.BodyMaxChars = band.Max
	}
	parentID := selected.Intent.RespondsToPostID
	if parentID == 0 {
		parentID = selected.Intent.SourcePostID
	}
	if parentID == 0 {
		parentID = selected.ParentID
	}
	if parentID != 0 {
		for _, candidate := range r.Base.ListPosts(req.Host.ID) {
			if candidate.ID == parentID && candidate.BoardID == selected.BoardID {
				parent := candidate
				req.ParentPost = &parent
				break
			}
		}
	}
	if req.Kind == worldengine.PostKindReply && req.QuoteText == "" && req.ParentPost != nil && h.Sum64()%1000 < 576 {
		req.QuoteText = firstQuoteLine(req.ParentPost.Body)
	}
	return req
}

func firstQuoteLine(body string) string {
	for _, line := range strings.Split(strings.ReplaceAll(body, "\r\n", "\n"), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		runes := []rune(line)
		if len(runes) > 160 {
			runes = runes[:160]
		}
		return string(runes)
	}
	return ""
}

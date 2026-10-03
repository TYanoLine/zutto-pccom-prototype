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
	if world.ResponseTargetID(selected) != 0 || selected.Intent.Action == "reply" {
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
		req.QuoteText = replyQuoteFragment(req.ParentPost.Body)
	}
	return req
}

func replyQuoteFragment(body string) string {
	normalized := strings.ReplaceAll(strings.ReplaceAll(body, "\r\n", "\n"), "\r", "\n")
	normalized = strings.Join(strings.Fields(normalized), " ")
	if normalized == "" {
		return ""
	}

	runes := []rune(normalized)
	sentences := make([]string, 0, 4)
	start := 0
	for i, r := range runes {
		if !strings.ContainsRune("。！？?!", r) {
			continue
		}
		if sentence := strings.TrimSpace(string(runes[start : i+1])); sentence != "" {
			sentences = append(sentences, sentence)
		}
		start = i + 1
	}
	if start < len(runes) {
		if tail := strings.TrimSpace(string(runes[start:])); tail != "" {
			sentences = append(sentences, tail)
		}
	}

	for i := len(sentences) - 1; i >= 0; i-- {
		if len([]rune(sentences[i])) >= 6 {
			return trimQuoteFragment(sentences[i], 96)
		}
	}
	return trimQuoteFragment(normalized, 96)
}

func trimQuoteFragment(value string, maxRunes int) string {
	runes := []rune(strings.TrimSpace(value))
	if len(runes) <= maxRunes {
		return string(runes)
	}
	return string(runes[:maxRunes])
}

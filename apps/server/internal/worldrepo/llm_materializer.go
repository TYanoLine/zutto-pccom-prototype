package worldrepo

import (
	"context"
	"fmt"
	"strings"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

// LLMMaterializer turns WorldEngine-selected facts into prose. It does not
// decide whether an event happens or whether research is needed; those remain
// WorldEngine/HistoricalKnowledge responsibilities.
type LLMMaterializer struct {
	Renderer llm.BoardPostRenderer
	Fallback Materializer
}

func (m LLMMaterializer) GenerateBoardPosts(ctx context.Context, req BoardMaterializationRequest, decision worldengine.EvidenceDecision) ([]world.Post,error) {
	if decision.Level==historicalkb.EvidenceVerified&&!decision.Knowledge.CanUse {
		return nil,fmt.Errorf("verified historical knowledge unavailable")
	}
	if m.Renderer==nil { return m.fallback(ctx,req,decision,fmt.Errorf("LLM board post renderer is not configured")) }

	facts:=usableClaims(decision)
	draft,err:=m.Renderer.GenerateBoardPost(ctx,llm.BoardPostRequest{
		HostName:req.Host.Name,
		HostRegion:req.Host.Region,
		HostSoftware:req.Host.Software,
		BoardID:req.BoardID,
		BoardTopic:req.BoardTopic,
		WorldDate:req.WorldDate,
		HistoricalFacts:facts,
		EraRules:"世界時刻より未来の知識を使わない。具体的な歴史事実は supplied historical facts の範囲に限定する。局固有の架空設定と史実を混同しない。",
	})
	if err!=nil { return m.fallback(ctx,req,decision,err) }
	return []world.Post{{Author:draft.Author,Subject:draft.Subject,Body:draft.Body,CreatedAt:worldTime(req.WorldDate)}},nil
}

func (m LLMMaterializer) fallback(ctx context.Context,req BoardMaterializationRequest,decision worldengine.EvidenceDecision,cause error)([]world.Post,error){
	// Verified materialization must not silently degrade into prose unsupported by
	// required evidence. For atmospheric/plausible content, deterministic fallback
	// keeps the world usable when the model endpoint is temporarily unavailable.
	if decision.Level==historicalkb.EvidenceVerified { return nil,cause }
	if m.Fallback==nil { return nil,cause }
	return m.Fallback.GenerateBoardPosts(ctx,req,decision)
}

func usableClaims(decision worldengine.EvidenceDecision)[]string{
	if !decision.Knowledge.CanUse{return nil}
	out:=make([]string,0,len(decision.Knowledge.Facts))
	for _,f:=range decision.Knowledge.Facts{
		if f.Status==historicalkb.FactRejected||strings.TrimSpace(f.Claim)==""{continue}
		out=append(out,strings.TrimSpace(f.Claim))
	}
	return out
}

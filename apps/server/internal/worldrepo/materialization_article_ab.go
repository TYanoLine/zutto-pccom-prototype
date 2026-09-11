package worldrepo

import (
	"context"
	"fmt"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/historicalkb"
	"zutto-pccom/apps/server/internal/world"
	"zutto-pccom/apps/server/internal/worldengine"
)

// MaterializationArticleWorkerABInput prepares exactly the worker input production
// would use, but does not render or persist a body. Article-detail materialization
// may persist canonical detail because that is world state shared by both arms.
func (r *Repository) MaterializationArticleWorkerABInput(host world.Host, board world.Board, postID int64) (BoardMaterializationRequest, worldengine.EvidenceDecision, world.Post, error) {
	selected, found := r.findMaterializationPost(host.ID, board.ID, postID)
	if !found {
		return BoardMaterializationRequest{}, worldengine.EvidenceDecision{}, world.Post{}, fmt.Errorf("post not found")
	}
	if developmentInteractiveTitleFirstEnabled(r) {
		var err error
		selected, _, err = r.materializeInteractiveTitleArticleDetails(host, board, selected)
		if err != nil {
			return BoardMaterializationRequest{}, worldengine.EvidenceDecision{}, selected, err
		}
	}
	if r.Engine == nil {
		return BoardMaterializationRequest{}, worldengine.EvidenceDecision{}, selected, fmt.Errorf("world engine unavailable")
	}
	var persona *world.Persona
	if ps, ok := r.Base.(world.PersonaStore); ok && selected.AuthorPersonaID != "" {
		if p, found := ps.PersonaByID(selected.AuthorPersonaID); found {
			copy := p
			persona = &copy
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 35*time.Second)
	defer cancel()
	decision, err := r.Engine.ResolveEvidence(ctx, worldengine.EvidenceRequest{
		Kind: historicalkb.KnowledgeCulturalSignal, Subject: board.Name,
		WorldDate: selected.CreatedAt.Format("2006-01-02"), Region: host.Region,
		Audience:    []string{host.SoftwareID},
		Need:        fmt.Sprintf("%s の %s ボード、%sによる件名『%s』の記事本文を、確定済みの投稿意図を変えず1996年の自然なパソコン通信文体で補完する", host.Name, board.Name, selected.Author, selected.Subject),
		Persistence: true, Importance: .30, Specificity: .30,
	})
	if err != nil {
		return BoardMaterializationRequest{}, worldengine.EvidenceDecision{}, selected, err
	}
	boardTopic := selected.Subject
	canonicalSubject := selected.Subject
	if developmentConversationViewPoCEnabled(r) {
		boardTopic = board.Name
		canonicalSubject = ""
	}
	if fixed := titleFirstSubject(selected.Intent.SituationFacts); fixed != "" {
		canonicalSubject = fixed
	}
	renderIntent := selected.Intent
	renderIntent.RenderContext = r.materializationArticleWorkerContext(host, board, selected)
	// Explicitly keep the A/B packet free of diagnostic context.
	if strings.Contains(renderIntent.RenderContext, "MSG ") {
		return BoardMaterializationRequest{}, worldengine.EvidenceDecision{}, selected, fmt.Errorf("worker context unexpectedly contains MSG metadata")
	}
	req := BoardMaterializationRequest{Host: host, BoardID: board.ID, BoardTopic: boardTopic, WorldDate: selected.CreatedAt.Format("2006-01-02"), Persona: persona, Intent: renderIntent, CanonicalSubject: canonicalSubject}
	return req, decision, selected, nil
}

package worldrepo

import (
	"fmt"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type demoPersonaFactBlueprint struct {
	key   string
	value string
}

// materializeDemoPersonaTopicFacts fills only the concrete persona details that
// the selected topic actually needs. The sparse persona skeleton remains enough
// for unrelated boards. Once a fact is stored, later posts reuse it rather than
// asking the prose renderer to improvise a new personal history.
func (r *Repository) materializeDemoPersonaTopicFacts(persona world.Persona, topic string, at time.Time) []world.PersonaFact {
	store, ok := r.Base.(world.PersonaFactStore)
	if !ok {
		return nil
	}

	existing := make([]world.PersonaFact, 0)
	for _, fact := range store.ListPersonaFacts(persona.ID) {
		if fact.Topic == topic {
			existing = append(existing, fact)
		}
	}
	if len(existing) > 0 {
		return existing
	}

	blueprints := demoPersonaFactsForTopic(persona, topic)
	if len(blueprints) == 0 {
		return nil
	}
	out := make([]world.PersonaFact, 0, len(blueprints))
	for _, blueprint := range blueprints {
		fact := world.PersonaFact{
			PersonaID:      persona.ID,
			Key:            blueprint.key,
			Topic:          topic,
			Value:          blueprint.value,
			MaterializedAt: at,
			SourceKind:     "fictional_reconstruction",
		}
		store.SavePersonaFact(fact)
		out = append(out, fact)
	}
	return out
}

// The first PoC schema intentionally concentrates on the PC-98 environment
// topic that exposed the thin-conversation problem. These are fictional facts
// about fictional residents, not historical claims about PC-98 specifications.
// Exact real-world hardware/software specifications remain under historical
// evidence policy and are not invented here.
func demoPersonaFactsForTopic(persona world.Persona, topic string) []demoPersonaFactBlueprint {
	if topic != "pc98_environment" {
		return nil
	}

	switch persona.Handle {
	case "SYSOP":
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.usage", value: "自宅のPC-98は局運営用の機械とは別で、普段の通信やログ確認にも使っている"},
			{key: "computer.pc98.logs", value: "通信ログを残すことが多く、ディスクの整理を定期的にしている"},
			{key: "computer.pc98.configuration", value: "設定変更には慣れているが、安定している普段の構成はむやみに変えない"},
		}
	case "NEKO":
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.usage", value: "自宅のPC-98を通信とゲームの両方に使っている"},
			{key: "computer.pc98.modem", value: "モデムは外付けで、一度つながる設定が決まると普段はあまり触らない"},
			{key: "computer.pc98.configuration", value: "細かい起動設定は得意ではなく、困ると詳しい会員に聞くことが多い"},
		}
	case "MARI":
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.usage", value: "家のPC-98は家族共用で、通信専用にはできない"},
			{key: "computer.pc98.other_usage", value: "通信のほかにワープロやゲームにも同じPC-98を使っている"},
			{key: "computer.pc98.configuration", value: "CONFIG.SYSやAUTOEXEC.BATを大きく変えるのは少し不安に感じている"},
		}
	case "NORI":
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.usage", value: "自宅のPC-98を通信と作業の両方に使い、用途に応じて起動時の設定を使い分けている"},
			{key: "computer.pc98.modem", value: "外付けモデムを使い、通信ソフトの設定や巡回マクロは自分で調整している"},
			{key: "computer.pc98.configuration", value: "常駐量や起動設定を確認しながら少しずつ調整するのが習慣になっている"},
		}
	case "YUKI":
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.usage", value: "自宅のPC-98はゲーム中心だが通信にも同じ機械を使っている"},
			{key: "computer.pc98.configuration", value: "ゲーム用と通信用で設定を切り替えるのを面倒に感じている"},
			{key: "computer.pc98.confidence", value: "細かい設定は分かる範囲だけ触り、動いているところはあまり変えない"},
		}
	case "TAKA":
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.usage", value: "自宅のPC-98を通信とゲームで兼用している"},
			{key: "computer.pc98.modem", value: "通信には外付けモデムをつないで使っている"},
			{key: "computer.pc98.configuration", value: "構成は標準に近いままで、CONFIG.SYSやAUTOEXEC.BATは必要な時だけ触る"},
		}
	default:
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.usage", value: "自宅のPC-98を通信以外の用途とも兼用している"},
			{key: "computer.pc98.configuration", value: "普段動いている設定は必要がなければ変えない"},
		}
	}
}

func demoClaimsFromPersonaFacts(facts []world.PersonaFact, limit int) []string {
	if limit <= 0 || len(facts) == 0 {
		return nil
	}
	if limit > len(facts) {
		limit = len(facts)
	}
	out := make([]string, 0, limit)
	for _, fact := range facts[:limit] {
		if fact.Value != "" {
			out = append(out, fact.Value)
		}
	}
	return out
}

func demoRespondsToClaims(root world.Post) []string {
	if len(root.Intent.Claims) == 0 {
		return nil
	}
	// One specific semantic hook is more useful than handing the renderer an
	// undifferentiated parent summary. The reply may have additional own claims.
	return []string{root.Intent.Claims[0]}
}

func demoRecentRootForTopic(roots []world.Post, topic string, at time.Time) (world.Post, bool) {
	for i := len(roots) - 1; i >= 0; i-- {
		root := roots[i]
		if root.Intent.Topic != topic {
			continue
		}
		age := at.Sub(root.CreatedAt)
		if age >= 0 && age < 10*24*time.Hour {
			return root, true
		}
	}
	return world.Post{}, false
}

func (r *Repository) demoReplyEnvelope(host world.Host, board world.Board, persona world.Persona, root world.Post, created time.Time) world.Post {
	facts := r.materializeDemoPersonaTopicFacts(persona, root.Intent.Topic, created)
	claims := demoClaimsFromPersonaFacts(facts, 2)
	if len(claims) == 0 {
		claims = []string{fmt.Sprintf("%sについて自分にも近い経験があり、その点を具体的に返したい", root.Subject)}
	}
	return world.Post{
		BoardID:         board.ID,
		ParentID:        root.ID,
		Author:          persona.Handle,
		AuthorPersonaID: persona.ID,
		Subject:         "Re: " + root.Subject,
		Intent: world.PostIntent{
			Action:           "reply",
			Topic:            root.Intent.Topic,
			Motivation:       demoReplyMotivation(persona, root),
			Stance:           demoPersonaStance(persona),
			Claims:           claims,
			RespondsToClaims: demoRespondsToClaims(root),
		},
		CreatedAt: created,
	}
}

func (r *Repository) demoRootEnvelope(host world.Host, board world.Board, persona world.Persona, seed demoTopicSeed, subject string, created time.Time) world.Post {
	action := "thread_start"
	if seed.role == "sysop" {
		action = "announcement"
	}
	facts := r.materializeDemoPersonaTopicFacts(persona, seed.key, created)
	claims := demoClaimsFromPersonaFacts(facts, 3)
	return world.Post{
		BoardID:         board.ID,
		Author:          persona.Handle,
		AuthorPersonaID: persona.ID,
		Subject:         subject,
		Intent: world.PostIntent{
			Action:     action,
			Topic:      seed.key,
			Motivation: seed.motivation,
			Stance:     demoPersonaStance(persona),
			Claims:     claims,
		},
		CreatedAt: created,
	}
}

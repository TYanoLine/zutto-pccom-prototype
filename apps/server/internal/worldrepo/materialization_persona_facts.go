package worldrepo

import (
	"fmt"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type demoPersonaFactBlueprint struct {
	key   string
	slot  string
	value string
}

type demoConversationProgression struct {
	responseAct      string
	target           world.Post
	informationSlots []string
	followUpQuestion string
}

// materializeDemoPersonaFactsForSlots fills only the concrete persona details
// that the selected conversational move actually needs. The persona skeleton can
// therefore remain sparse until a topic asks for another dimension of the person.
// Existing facts always win and are reused verbatim.
func (r *Repository) materializeDemoPersonaFactsForSlots(persona world.Persona, topic string, slots []string, at time.Time) []world.PersonaFact {
	store, ok := r.Base.(world.PersonaFactStore)
	if !ok || len(slots) == 0 {
		return nil
	}

	existingByKey := map[string]world.PersonaFact{}
	for _, fact := range store.ListPersonaFacts(persona.ID) {
		existingByKey[fact.Key] = fact
	}
	wanted := map[string]bool{}
	for _, slot := range slots {
		wanted[slot] = true
	}

	out := make([]world.PersonaFact, 0, len(slots))
	for _, blueprint := range demoPersonaFactsForTopic(persona, topic) {
		if !wanted[blueprint.slot] {
			continue
		}
		if existing, ok := existingByKey[blueprint.key]; ok {
			out = append(out, existing)
			continue
		}
		fact := world.PersonaFact{
			PersonaID:      persona.ID,
			Key:            blueprint.key,
			Topic:          topic,
			Value:          blueprint.value,
			MaterializedAt: at,
			SourceKind:     "fictional_reconstruction",
		}
		store.SavePersonaFact(fact)
		existingByKey[blueprint.key] = fact
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
			{key: "computer.pc98.usage", slot: "usage_pattern", value: "自宅のPC-98は局運営用の機械とは別で、普段の通信やログ確認にも使っている"},
			{key: "computer.pc98.logs", slot: "storage_logs", value: "通信ログを残すことが多く、ディスクの整理を定期的にしている"},
			{key: "computer.pc98.configuration", slot: "configuration", value: "設定変更には慣れているが、安定している普段の構成はむやみに変えない"},
			{key: "computer.pc98.pain_point", slot: "pain_point", value: "ログが増えると後から探すのが面倒なので、用途ごとに整理する手間は惜しまない"},
		}
	case "NEKO":
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.usage", slot: "usage_pattern", value: "自宅のPC-98を通信とゲームの両方に使っている"},
			{key: "computer.pc98.modem", slot: "modem", value: "モデムは外付けで、一度つながる設定が決まると普段はあまり触らない"},
			{key: "computer.pc98.configuration", slot: "configuration", value: "細かい起動設定は得意ではなく、困ると詳しい会員に聞くことが多い"},
			{key: "computer.pc98.pain_point", slot: "pain_point", value: "ゲームと通信を行き来するときに設定を戻したか不安になることがある"},
		}
	case "MARI":
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.shared", slot: "shared_machine", value: "家のPC-98は家族共用で、通信専用にはできない"},
			{key: "computer.pc98.other_usage", slot: "other_usage", value: "通信のほかにワープロやゲームにも同じPC-98を使っている"},
			{key: "computer.pc98.configuration", slot: "configuration", value: "CONFIG.SYSやAUTOEXEC.BATを大きく変えるのは少し不安に感じている"},
			{key: "computer.pc98.pain_point", slot: "pain_point", value: "家族が使いたい時間と自分が通信したい時間が重なると困ることがある"},
		}
	case "NORI":
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.usage", slot: "usage_pattern", value: "自宅のPC-98を通信と作業の両方に使い、用途に応じて起動時の設定を使い分けている"},
			{key: "computer.pc98.modem", slot: "modem", value: "外付けモデムを使い、通信ソフトの設定や巡回マクロは自分で調整している"},
			{key: "computer.pc98.configuration", slot: "configuration", value: "常駐量や起動設定を確認しながら少しずつ調整するのが習慣になっている"},
			{key: "computer.pc98.pain_point", slot: "pain_point", value: "用途ごとの設定差が増えすぎると管理が面倒になるので、変更点を増やしすぎないようにしている"},
		}
	case "YUKI":
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.usage", slot: "usage_pattern", value: "自宅のPC-98はゲーム中心だが通信にも同じ機械を使っている"},
			{key: "computer.pc98.configuration", slot: "configuration", value: "ゲーム用と通信用で設定を切り替えるのを面倒に感じている"},
			{key: "computer.pc98.confidence", slot: "configuration_confidence", value: "細かい設定は分かる範囲だけ触り、動いているところはあまり変えない"},
			{key: "computer.pc98.pain_point", slot: "pain_point", value: "通信のためにゲーム側の環境を崩すのは避けたいと思っている"},
		}
	case "TAKA":
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.usage", slot: "usage_pattern", value: "自宅のPC-98を通信とゲームで兼用している"},
			{key: "computer.pc98.modem", slot: "modem", value: "通信には外付けモデムをつないで使っている"},
			{key: "computer.pc98.configuration", slot: "configuration", value: "構成は標準に近いままで、CONFIG.SYSやAUTOEXEC.BATは必要な時だけ触る"},
			{key: "computer.pc98.pain_point", slot: "pain_point", value: "ゲームと通信で必要な設定が違う時だけ切り替えるのが少し面倒だと感じている"},
		}
	default:
		return []demoPersonaFactBlueprint{
			{key: "computer.pc98.usage", slot: "usage_pattern", value: "自宅のPC-98を通信以外の用途とも兼用している"},
			{key: "computer.pc98.configuration", slot: "configuration", value: "普段動いている設定は必要がなければ変えない"},
		}
	}
}

func demoAvailablePersonaSlots(persona world.Persona, topic string) []string {
	seen := map[string]bool{}
	out := make([]string, 0)
	for _, fact := range demoPersonaFactsForTopic(persona, topic) {
		if fact.slot == "" || seen[fact.slot] {
			continue
		}
		seen[fact.slot] = true
		out = append(out, fact.slot)
	}
	return out
}

func demoClaimsFromPersonaFacts(facts []world.PersonaFact) []string {
	out := make([]string, 0, len(facts))
	for _, fact := range facts {
		if fact.Value != "" {
			out = append(out, fact.Value)
		}
	}
	return out
}

func demoThreadPosts(root world.Post, all []world.Post) []world.Post {
	out := []world.Post{root}
	for _, post := range all {
		if post.ID == root.ID {
			continue
		}
		if post.ParentID == root.ID {
			out = append(out, post)
		}
	}
	return out
}

func demoCoveredInformationSlots(posts []world.Post) map[string]bool {
	covered := map[string]bool{}
	for _, post := range posts {
		for _, slot := range post.Intent.InformationSlots {
			covered[slot] = true
		}
	}
	return covered
}

func demoLatestSemanticTarget(posts []world.Post) world.Post {
	for i := len(posts) - 1; i >= 0; i-- {
		if len(posts[i].Intent.Claims) > 0 || posts[i].Intent.FollowUpQuestion != "" {
			return posts[i]
		}
	}
	if len(posts) > 0 {
		return posts[0]
	}
	return world.Post{}
}

func demoInitialInformationSlots(persona world.Persona, topic string) []string {
	available := demoAvailablePersonaSlots(persona, topic)
	if len(available) <= 2 {
		return available
	}
	return available[:2]
}

func demoReplyInformationSlots(persona world.Persona, topic string, covered map[string]bool) []string {
	available := demoAvailablePersonaSlots(persona, topic)
	out := make([]string, 0, 2)
	for _, slot := range available {
		if !covered[slot] {
			out = append(out, slot)
		}
		if len(out) == 2 {
			break
		}
	}
	if len(out) > 0 {
		return out
	}
	// Once the thread has covered every broad dimension, reuse one personal slot
	// rather than fabricating a new dimension just to keep the conversation alive.
	if len(available) > 0 {
		return available[:1]
	}
	return nil
}

func demoFollowUpQuestion(topic string, covered map[string]bool, justAdded []string) string {
	if topic != "pc98_environment" {
		return ""
	}
	for _, slot := range []string{"configuration", "modem", "storage_logs", "pain_point", "shared_machine", "other_usage"} {
		if covered[slot] || containsString(justAdded, slot) {
			continue
		}
		switch slot {
		case "configuration":
			return "通信するときと普段使うときで、起動時の設定を分けていますか？"
		case "modem":
			return "モデムは内蔵と外付けのどちらを使っていますか？"
		case "storage_logs":
			return "通信ログはどのくらい残していますか？"
		case "pain_point":
			return "兼用していて一番面倒に感じるのはどの辺ですか？"
		case "shared_machine":
			return "通信に使う98は自分専用ですか、それとも家族と共用ですか？"
		case "other_usage":
			return "通信以外にはその98を何に使っていますか？"
		}
	}
	return ""
}

func containsString(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func demoRespondsToClaims(target world.Post) []string {
	if len(target.Intent.Claims) == 0 {
		return nil
	}
	return []string{target.Intent.Claims[0]}
}

func demoConversationMove(persona world.Persona, root world.Post, thread []world.Post) demoConversationProgression {
	target := demoLatestSemanticTarget(thread)
	covered := demoCoveredInformationSlots(thread)
	slots := demoReplyInformationSlots(persona, root.Intent.Topic, covered)
	act := "compare_and_expand"
	if target.Intent.FollowUpQuestion != "" {
		act = "answer_and_expand"
	} else if len(thread) >= 3 {
		act = "add_new_detail"
	}
	question := demoFollowUpQuestion(root.Intent.Topic, covered, slots)
	return demoConversationProgression{
		responseAct:      act,
		target:           target,
		informationSlots: slots,
		followUpQuestion: question,
	}
}

func demoRecentRootForTopic(roots []world.Post, topic string, at time.Time) (world.Post, bool) {
	for i := len(roots) - 1; i >= 0; i-- {
		root := roots[i]
		if root.Intent.Topic != topic {
			continue
		}
		age := at.Sub(root.CreatedAt)
		if age >= 0 && age < 16*24*time.Hour {
			return root, true
		}
	}
	return world.Post{}, false
}

func (r *Repository) demoReplyEnvelope(host world.Host, board world.Board, persona world.Persona, root world.Post, thread []world.Post, created time.Time) world.Post {
	move := demoConversationMove(persona, root, thread)
	facts := r.materializeDemoPersonaFactsForSlots(persona, root.Intent.Topic, move.informationSlots, created)
	claims := demoClaimsFromPersonaFacts(facts)
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
			RespondsToClaims: demoRespondsToClaims(move.target),
			RespondsToPostID: move.target.ID,
			ResponseAct:      move.responseAct,
			InformationSlots: move.informationSlots,
			FollowUpQuestion: move.followUpQuestion,
		},
		CreatedAt: created,
	}
}

func (r *Repository) demoRootEnvelope(host world.Host, board world.Board, persona world.Persona, seed demoTopicSeed, subject string, created time.Time) world.Post {
	action := "thread_start"
	if seed.role == "sysop" {
		action = "announcement"
	}
	slots := demoInitialInformationSlots(persona, seed.key)
	facts := r.materializeDemoPersonaFactsForSlots(persona, seed.key, slots, created)
	claims := demoClaimsFromPersonaFacts(facts)
	covered := map[string]bool{}
	for _, slot := range slots {
		covered[slot] = true
	}
	return world.Post{
		BoardID:         board.ID,
		Author:          persona.Handle,
		AuthorPersonaID: persona.ID,
		Subject:         subject,
		Intent: world.PostIntent{
			Action:           action,
			Topic:            seed.key,
			Motivation:       seed.motivation,
			Stance:           demoPersonaStance(persona),
			Claims:           claims,
			ResponseAct:      "thread_start",
			InformationSlots: slots,
			FollowUpQuestion: demoFollowUpQuestion(seed.key, covered, nil),
		},
		CreatedAt: created,
	}
}

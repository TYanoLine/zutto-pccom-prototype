package worldrepo

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strconv"
	"strings"

	"zutto-pccom/apps/server/internal/world"
)

// The temporary title-led quality-evaluation mode changes only the *view*
// supplied to the prose worker. The original accepted Situation, article details
// and persona remain untouched in canonical persistence. It is a process-wide
// switch combined with the per-host generation.freeform_body flag. (The
func (r *Repository) SetFreeformBody(enabled bool) {
    if r != nil { r.freeformBody = enabled }
}

func (r *Repository) useFreeformBody(host world.Host) bool {
    return r != nil && r.freeformBody && host.Generation.FreeformBody
}

func freeformIntentSummary(i world.PostIntent) string {
    var lines []string
    if summary := worldAdoptedSummary(i.SituationFacts, i.SituationSummary); summary != "" {
        lines = append(lines, "確定した状況: "+summary)
    } else if topic := strings.TrimSpace(i.Topic); topic != "" {
        lines = append(lines, "話題: "+topic)
    }
    // Reply-specific causal and conversational facts matter more than the
    // large Situation/production metadata used by the earlier body prompt.
    if i.Action == "reply" || i.RespondsToPostID != 0 || i.SourcePostID != 0 {
        if goal := strings.TrimSpace(i.Goal); goal != "" {
            lines = append(lines, "返信の目的: "+goal)
        }
        if len(i.RespondsToClaims) > 0 {
            lines = append(lines, "返信先の論点: "+strings.Join(i.RespondsToClaims, " / "))
        }
        if context := strings.TrimSpace(i.RenderContext); context != "" {
            runes := []rune(context)
            if len(runes) > 800 { context = string(runes[:800]) }
            lines = append(lines, "これまでの会話:\n"+context)
        }
    }
    // A necessary referent may be canonical even if a terse subject omits it.
    // Preserve it without forwarding every Article Detail or typed Situation
    // field as an instruction/checklist.
    for _, fact := range i.SituationFacts {
        if strings.HasPrefix(fact, "article_referent_required=") {
            lines = append(lines, strings.TrimSpace(fact))
            break
        }
    }
    // Legacy canonical detail can carry an adopted work/product identity even
    // without a dedicated referent control. Keep only that identity.
    for _, fact := range i.SituationFacts {
        if strings.HasPrefix(fact, "article_detail=referent:") {
            lines = append(lines, strings.TrimSpace(fact))
            break
        }
    }
    return strings.Join(lines, "\n")
}

func freeformPersonaSummary(p world.Persona) string {
	var traits []string
	if p.Age > 0 && p.Gender != "" && p.Occupation != "" {
		traits = append(traits, fmt.Sprintf("人物像: %d歳・%s・%s", p.Age, p.Gender, p.Occupation))
	} else if p.Age > 0 {
		traits = append(traits, fmt.Sprintf("年齢: %d歳", p.Age))
	}
	if style := strings.TrimSpace(p.WritingStyle); style != "" {
		traits = append(traits, fmt.Sprintf("文体・口調: %s", style))
	} else if voice := derivedVoice(p); voice != "" {
		traits = append(traits, fmt.Sprintf("文体・口調: %s", voice))
	}
	// Nearly every resident has low Argumentativeness, so a "gentle, agreeable"
	// line for the low end only made everyone sound alike; only the distinctive
	// high end is described.
	if p.Argumentativeness >= 0.7 {
		traits = append(traits, "性格・傾向: 自分のこだわりや論理をはっきり述べる傾向がある")
	}
	if p.NewcomerOpenness >= 0.7 {
		traits = append(traits, "態度: 初対面や新参者にも親切で歓迎的")
	}

	var fields []string
	if p.Age > 0 { fields = append(fields, "age="+strconv.Itoa(p.Age)) }
	if p.Gender != "" { fields = append(fields, "gender="+p.Gender) }
	if p.Occupation != "" { fields = append(fields, "occupation="+p.Occupation) }
	if p.WritingStyle != "" { fields = append(fields, "writing="+p.WritingStyle) }
	keys := make([]string, 0, len(p.Interests))
	for key := range p.Interests { keys = append(keys, key) }
	sort.Strings(keys)
	var interests []string
	for _, key := range keys {
		if p.Interests[key] >= 0.45 { interests = append(interests, key) }
	}
	if len(interests) > 0 {
		fields = append(fields, "interests=["+strings.Join(interests, ",")+"]")
	}
	if len(p.EverydayContext) > 0 {
		fields = append(fields, "普段の環境="+strings.Join(p.EverydayContext, " / "))
	}
	rawMeta := strings.Join(fields, "; ")
	if len(traits) > 0 {
		return strings.Join(traits, "\n") + "\n" + rawMeta
	}
	return rawMeta
}


// Dimensions of a resident's writing voice. A voice is a pure function of the
// persona ID, so it is stable across regenerations and applies equally to
// residents persisted before this existed; nothing is stored or mutated. Each
// dimension is chosen by an independent slice of the hash so combinations vary.
var (
	voiceRegisters = []string{
		"です・ます調を崩さない丁寧な書き方",
		"です・ますを基本に、ところどころ砕ける",
		"「〜だよ」「〜じゃないかな」など友人に話すような砕けた口調",
		"ぶっきらぼうで言い切りが多い。愛想の言葉は少ない",
		"控えめで「〜でしょうか」「〜かもしれません」と断定を避ける",
		"くだけた常連口調で、ツッコミや軽い冗談を挟む",
	}
	voiceLengths = []string{
		"文は短く切る。一言二言でも書き込む",
		"普通の長さで、要点を順に書く",
		"前置きや経緯をしっかり書いてから本題に入る",
		"結論を先に書き、理由は後から短く足す",
	}
	voiceMarks = []string{
		"顔文字や(笑)は使わない",
		"(笑)をよく使う",
		"(^^;や(^_^;)をときどき使う",
		"感情表現は使っても(^^)程度で控えめ",
		"文末の「！」や「…」で気持ちを出す",
		"顔文字は使わず、言葉の選び方で感情を出す",
	}
)

func derivedVoice(p world.Persona) string {
	key := strings.TrimSpace(p.ID)
	if key == "" {
		key = strings.ToLower(strings.TrimSpace(p.Handle))
	}
	if key == "" {
		return ""
	}
	h := fnv.New64a()
	_, _ = h.Write([]byte("voice|" + key))
	sum := h.Sum64()
	return voiceRegisters[int(sum%uint64(len(voiceRegisters)))] + "。" +
		voiceLengths[int((sum>>16)%uint64(len(voiceLengths)))] + "。" +
		voiceMarks[int((sum>>32)%uint64(len(voiceMarks)))] + "。" +
		"語尾や言い回しは他の人と揃えず、この人らしい癖を自分で決めてよい。"
}

package worldrepo

import (
    "sort"
    "strconv"
    "strings"

    "zutto-pccom/apps/server/internal/world"
)

// HAKATA's temporary quality-evaluation mode changes only the *view* supplied
// to the prose worker. The original accepted Situation, article details and
// persona remain untouched in canonical persistence.
func (r *Repository) SetHAKATAFreeformBody(enabled bool) {
    if r != nil { r.hakataFreeformBody = enabled }
}

func (r *Repository) useHAKATAFreeformBody(host world.Host) bool {
    return r != nil && r.hakataFreeformBody && host.IsExperiment()
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
    return strings.Join(fields, "; ")
}


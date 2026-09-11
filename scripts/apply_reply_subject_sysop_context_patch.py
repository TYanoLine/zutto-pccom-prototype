from pathlib import Path


def replace_once(path: str, old: str, new: str) -> None:
    p = Path(path)
    text = p.read_text()
    if old not in text:
        raise SystemExit(f"pattern not found in {path}: {old[:160]!r}")
    p.write_text(text.replace(old, new, 1))


# 1) Reply subjects must be fixed from the already-canonical parent subject,
# never from a body-generation placeholder. Also repair old persisted placeholders.
path = "apps/server/internal/worldrepo/materialization_conversation_view.go"
replace_once(
    path,
    'const developmentConversationPendingSubject = "（本文生成時に決定）"\n\n',
    '''const developmentConversationPendingSubject = "（本文生成時に決定）"\n\nfunc developmentPendingSubject(subject string) bool {\n\tsubject = strings.TrimSpace(subject)\n\treturn subject == developmentConversationPendingSubject || subject == "Re: "+developmentConversationPendingSubject\n}\n\nfunc developmentReplySubject(parentSubject string) string {\n\tparentSubject = strings.TrimSpace(parentSubject)\n\tif parentSubject == "" || developmentPendingSubject(parentSubject) {\n\t\treturn "Re: " + developmentConversationPendingSubject\n\t}\n\tif strings.HasPrefix(strings.ToLower(parentSubject), "re: ") {\n\t\treturn parentSubject\n\t}\n\treturn "Re: " + parentSubject\n}\n\n// repairDevelopmentPendingReplySubjects upgrades rows written by the older\n// conversation-view PoC, where replies were persisted as\n// "Re: （本文生成時に決定）" even though the parent title was already canonical.\n// This is a data repair, not a display-only substitution: the DB remains the\n// source of truth after the first read on the fixed build.\nfunc (r *Repository) repairDevelopmentPendingReplySubjects(hostID string, posts []world.Post) []world.Post {\n\tout := append([]world.Post(nil), posts...)\n\tbyID := make(map[int64]world.Post, len(out))\n\tfor _, post := range out {\n\t\tbyID[post.ID] = post\n\t}\n\tupdater, canUpdate := r.Base.(world.PostUpdater)\n\tfor i, post := range out {\n\t\tif post.ParentID == 0 || !developmentPendingSubject(post.Subject) {\n\t\t\tcontinue\n\t\t}\n\t\tparent, ok := byID[post.ParentID]\n\t\tif !ok {\n\t\t\tcontinue\n\t\t}\n\t\tcorrected := developmentReplySubject(parent.Subject)\n\t\tif developmentPendingSubject(corrected) {\n\t\t\tcontinue\n\t\t}\n\t\tpost.Subject = corrected\n\t\tif canUpdate {\n\t\t\tif saved, ok := updater.UpdatePost(hostID, post); ok {\n\t\t\t\tpost = saved\n\t\t\t}\n\t\t}\n\t\tout[i] = post\n\t\tbyID[post.ID] = post\n\t}\n\treturn out\n}\n\n''',
)
replace_once(
    path,
    '\t\tsubject = "Re: " + developmentConversationPendingSubject\n',
    '\t\tsubject = developmentReplySubject(parent.Subject)\n',
)

path = "apps/server/internal/worldrepo/materialization_interactive_board.go"
replace_once(
    path,
    '\t\tsubject = "Re: " + developmentConversationPendingSubject\n',
    '\t\tsubject = developmentReplySubject(parent.Subject)\n',
)

path = "apps/server/internal/worldrepo/materialization_demo_persona.go"
replace_once(
    path,
    '''\thostPosts := r.Base.ListPosts(host.ID)\n\tif existing := filterBoard(hostPosts, board.ID); len(existing) > 0 {\n\t\treturn existing, false\n\t}\n''',
    '''\thostPosts := r.Base.ListPosts(host.ID)\n\tif existing := filterBoard(hostPosts, board.ID); len(existing) > 0 {\n\t\treturn r.repairDevelopmentPendingReplySubjects(host.ID, existing), false\n\t}\n''',
)

# 2) Article-detail materialization must see the persona role/baseline. Otherwise
# it can canonize novice-like discoveries for an experienced SYSOP before prose.
path = "apps/server/internal/llm/bbs_title_article_details.go"
replace_once(
    path,
    '''\tDiscourseMode string   `json:"discourse_mode"`\n\tExistingFacts []string `json:"existing_facts,omitempty"`\n''',
    '''\tDiscourseMode string   `json:"discourse_mode"`\n\tPersonaProfile string   `json:"persona_profile,omitempty"`\n\tExistingFacts  []string `json:"existing_facts,omitempty"`\n''',
)
replace_once(
    path,
    '''- ExistingFactsと矛盾する恒久的な所有、職歴、家族事情、長期の嗜好などは追加禁止です。\n- RecentBBSStateにない別スレッドの出来事を混ぜないでください。\n''',
    '''- PersonaProfileは、この人物の役割・経験水準・普段の行動を守るためのcanonicalな整合性ガードです。題名やsummaryが明示していないのに、普段から行っている基本操作を「今回初めて知った」「これから毎回することにした」のような初心者的な発見・新習慣へ変えないでください。\n- author_handleがSYSOP、またはPersonaProfileにSYSOP役割がある場合も普通の個人的雑談は可能です。ただし局運営、回線、接続確認、ログ確認などが日常業務として示されているなら、それらの基本を今さら初めて学んだようなdetailを作らないでください。また個人環境の話を、根拠なく局設備や運営方針の変更へ膨らませないでください。\n- decisionはsubject/summaryが実際に選択・方針・質問を含む場合だけ使ってください。detailsの件数を埋めるために「今後は毎回〜することにした」のような新しい習慣を勝手に作らないでください。\n- ExistingFactsと矛盾する恒久的な所有、職歴、家族事情、長期の嗜好などは追加禁止です。\n- RecentBBSStateにない別スレッドの出来事を混ぜないでください。\n''',
)

path = "apps/server/internal/worldrepo/materialization_title_first.go"
replace_once(
    path,
    '''\t\t\tacceptedDetailSeeds[d.EventID] = llm.BBSTitleArticleDetailSeed{\n\t\t\t\tEventID: d.EventID, Subject: d.Subject, Summary: d.Summary,\n\t\t\t\tAuthorHandle: e.AuthorHandle, CreatedAt: e.CreatedAt, DiscourseMode: e.DiscourseMode,\n\t\t\t\tExistingFacts: append([]string(nil), e.ExistingFacts...),\n\t\t\t}\n''',
    '''\t\t\tacceptedDetailSeeds[d.EventID] = llm.BBSTitleArticleDetailSeed{\n\t\t\t\tEventID: d.EventID, Subject: d.Subject, Summary: d.Summary,\n\t\t\t\tAuthorHandle: e.AuthorHandle, CreatedAt: e.CreatedAt, DiscourseMode: e.DiscourseMode,\n\t\t\t\tPersonaProfile: e.PersonaProfile, ExistingFacts: append([]string(nil), e.ExistingFacts...),\n\t\t\t}\n''',
)

path = "apps/server/internal/worldrepo/materialization_interactive_detail.go"
replace_once(
    path,
    '''\texistingFacts := []string{}\n\tif ps, ok := r.Base.(world.PersonaStore); ok && selected.AuthorPersonaID != "" {\n\t\tif persona, found := ps.PersonaByID(selected.AuthorPersonaID); found {\n\t\t\tfacts := r.existingPersonaFactsByID([]world.Persona{persona})\n''',
    '''\texistingFacts := []string{}\n\tpersonaProfile := ""\n\tif ps, ok := r.Base.(world.PersonaStore); ok && selected.AuthorPersonaID != "" {\n\t\tif persona, found := ps.PersonaByID(selected.AuthorPersonaID); found {\n\t\t\tpersonaProfile = personaSummary(persona)\n\t\t\tfacts := r.existingPersonaFactsByID([]world.Persona{persona})\n''',
)
replace_once(
    path,
    '''\t\t\tCreatedAt:     selected.CreatedAt.Format(time.RFC3339),\n\t\t\tDiscourseMode: selected.Intent.DiscourseMode,\n\t\t\tExistingFacts: existingFacts,\n''',
    '''\t\t\tCreatedAt:      selected.CreatedAt.Format(time.RFC3339),\n\t\t\tDiscourseMode:  selected.Intent.DiscourseMode,\n\t\t\tPersonaProfile: personaProfile,\n\t\t\tExistingFacts:  existingFacts,\n''',
)

# 3) Reinforce the title/persona matcher: SYSOP is a role, not just a handle.
path = "apps/server/internal/llm/bbs_title_candidates.go"
replace_once(
    path,
    '''投稿枠の人物、日時、board、routing domain、cause、discourse_modeは変更禁止。人物の既存の所有物・関心・意見・過去の発言と明確に矛盾する候補は不採用にしてください。ただし候補タイトルは世界エンジンへの提案です。この検査でeventへ割り当てられ、後段のEra検証とコード側検査を通って採用された場合、タイトルが明示する最小限の出来事・経験・関与はその投稿のcanonical world eventとして新たに確定します。既存PersonaFactsにまだ無いという理由だけで、購入・利用・プレイ開始・小さな失敗・相談などを一律に拒否しないでください。\n''',
    '''投稿枠の人物、日時、board、routing domain、cause、discourse_modeは変更禁止。人物の既存の所有物・関心・意見・過去の発言と明確に矛盾する候補は不採用にしてください。ただし候補タイトルは世界エンジンへの提案です。この検査でeventへ割り当てられ、後段のEra検証とコード側検査を通って採用された場合、タイトルが明示する最小限の出来事・経験・関与はその投稿のcanonical world eventとして新たに確定します。既存PersonaFactsにまだ無いという理由だけで、購入・利用・プレイ開始・小さな失敗・相談などを一律に拒否しないでください。\nSYSOPは単なるハンドル名ではなく局運営者の役割です。SYSOPにも個人的な雑談や質問はありますが、PersonaProfileが日常業務として示す基本的な接続確認・局運営・ログ確認等を、根拠なく「初めて知った」「これから覚える」類の初心者体験として読ませる候補は割り当てないでください。また個人の通信環境についての候補を、canonicalな運営イベントなしに局回線・局設備の変更と読める形へ拡張してはいけません。\n''',
)

# 4) Test that the interactive detail stage actually receives the persona profile.
path = "apps/server/internal/worldrepo/materialization_interactive_latency_test.go"
replace_once(
    path,
    '''type interactiveTitleFirstTestRenderer struct {\n\ttitleFirstTestRenderer\n\tdetailCalls int\n}\n\nfunc (f *interactiveTitleFirstTestRenderer) MaterializeBBSTitleArticleDetails(ctx context.Context, req llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {\n\tf.detailCalls++\n''',
    '''type interactiveTitleFirstTestRenderer struct {\n\ttitleFirstTestRenderer\n\tdetailCalls int\n\tdetailReq   llm.BBSTitleArticleDetailRequest\n}\n\nfunc (f *interactiveTitleFirstTestRenderer) MaterializeBBSTitleArticleDetails(ctx context.Context, req llm.BBSTitleArticleDetailRequest) (llm.BBSTitleArticleDetailDraft, error) {\n\tf.detailCalls++\n\tf.detailReq = req\n''',
)
replace_once(
    path,
    '''\tif renderer.detailCalls != 1 {\n\t\tt.Fatalf("article open should materialize details exactly once, got %d", renderer.detailCalls)\n\t}\n\tif !hasInteractiveArticleDetails(rendered.Intent.SituationFacts) {\n''',
    '''\tif renderer.detailCalls != 1 {\n\t\tt.Fatalf("article open should materialize details exactly once, got %d", renderer.detailCalls)\n\t}\n\tif len(renderer.detailReq.Articles) != 1 || !strings.Contains(renderer.detailReq.Articles[0].PersonaProfile, "everyday_baseline=") {\n\t\tt.Fatalf("article detail materializer did not receive persona baseline: %+v", renderer.detailReq.Articles)\n\t}\n\tif !hasInteractiveArticleDetails(rendered.Intent.SituationFacts) {\n''',
)

# Focused regression test for old persisted placeholder repair.
Path("apps/server/internal/worldrepo/materialization_reply_subject_test.go").write_text(r'''package worldrepo

import (
    "testing"

    "zutto-pccom/apps/server/internal/world"
)

func TestRepairDevelopmentPendingReplySubjectUsesCanonicalParentTitle(t *testing.T) {
    base := world.NewMemoryStore()
    repo := New(base, nil, nil, "1996-08-29")
    hostID := "reply-repair-host"
    root := base.AddPost(hostID, world.Post{BoardID: "2", Author: "SYSOP", Subject: "ISDNに移行した方、感想を教えてください"})
    reply := base.AddPost(hostID, world.Post{BoardID: "2", ParentID: root.ID, Author: "TAKA", Subject: "Re: " + developmentConversationPendingSubject})

    repaired := repo.repairDevelopmentPendingReplySubjects(hostID, filterBoard(base.ListPosts(hostID), "2"))
    want := "Re: " + root.Subject
    found := false
    for _, post := range repaired {
        if post.ID != reply.ID {
            continue
        }
        found = true
        if post.Subject != want {
            t.Fatalf("reply subject=%q, want %q", post.Subject, want)
        }
    }
    if !found {
        t.Fatal("repaired reply not returned")
    }

    stored := filterBoard(base.ListPosts(hostID), "2")
    for _, post := range stored {
        if post.ID == reply.ID && post.Subject != want {
            t.Fatalf("repair was not persisted: subject=%q want=%q", post.Subject, want)
        }
    }
}

func TestDevelopmentReplySubjectNeverWaitsForBodyWhenParentTitleKnown(t *testing.T) {
    parent := "ログ取りに便利な通信ソフト"
    if got, want := developmentReplySubject(parent), "Re: "+parent; got != want {
        t.Fatalf("reply subject=%q, want %q", got, want)
    }
}
''')

print("patched reply titles and persona-aware article detail")

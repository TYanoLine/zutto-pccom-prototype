from pathlib import Path

# Strengthen Article Detail prompt and validator.
p = Path('apps/server/internal/llm/bbs_title_article_details.go')
s = p.read_text()
s = s.replace(
'''- 「〜を話題にする」「〜を共有する」「読者に尋ねる」「紹介する」「報告する」のような編集指示・タイトルの言い換えは禁止です。factは世界内で成立する具体的な命題として書いてください。\n''',
'''- 「〜を話題にする」「〜を共有する」「読者に尋ねる」「紹介する」「報告する」のような編集指示・タイトルの言い換えは禁止です。factは世界内で成立する具体的な命題として書いてください。\n- BoardName / CreatedAt / event_id は生成制御のためのヘッダ情報であり、記事内容ではありません。MSG番号、記事番号、投稿日時、投稿時刻、「○○板に掲示された」「新規スレッドの先頭」等をdetailへ変換することを禁止します。timingは「接続して数分後」「昨夜二度起きた」など記事内の出来事の時刻・回数にだけ使ってください。\n''')
s = s.replace(
'''\t\t\tif fact == strings.TrimSpace(seed.Subject) || fact == strings.TrimSpace(seed.Summary) || articleDetailLooksEditorial(fact) {\n\t\t\t\treturn fmt.Errorf("article %q detail is only a restatement/editorial instruction: %q", article.EventID, fact)\n\t\t\t}\n''',
'''\t\t\tif fact == strings.TrimSpace(seed.Subject) || fact == strings.TrimSpace(seed.Summary) || articleDetailLooksEditorial(fact) {\n\t\t\t\treturn fmt.Errorf("article %q detail is only a restatement/editorial instruction: %q", article.EventID, fact)\n\t\t\t}\n\t\t\tif ArticleDetailFactIsRenderingMetadata(fact) {\n\t\t\t\treturn fmt.Errorf("article %q detail leaked article-header/rendering metadata: %q", article.EventID, fact)\n\t\t\t}\n''')
s += '''\n// ArticleDetailFactIsRenderingMetadata identifies facts about the BBS record/header\n// rather than facts inside the fictional article event. Such facts must never become\n// canonical article_detail because prose workers can otherwise echo them verbatim.\nfunc ArticleDetailFactIsRenderingMetadata(fact string) bool {\n\tvalue := strings.ToLower(strings.TrimSpace(fact))\n\tif value == "" {\n\t\treturn false\n\t}\n\tfor _, marker := range []string{\n\t\t"msg ", "msg#", "msg番号", "記事番号", "メッセージ番号", "投稿時刻", "投稿日時",\n\t\t"新規スレッド", "新スレ", "先頭投稿", "スレッドの先頭", "board id",\n\t\t"掲示されて", "掲示された", "投稿された", "書き込まれて", "書き込まれた",\n\t} {\n\t\tif strings.Contains(value, marker) {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n'''
p.write_text(s)

# Existing bad canonical details from pre-fix runs are bug-generated state. Remove
# the whole old detail set if one detail is rendering metadata, then rematerialize.
p = Path('apps/server/internal/worldrepo/materialization_interactive_detail.go')
s = p.read_text()
s = s.replace(
'''func hasInteractiveArticleDetails(facts []string) bool {\n\tfor _, fact := range facts {\n\t\tif strings.HasPrefix(fact, "article_detail=") {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n''',
'''func hasInteractiveArticleDetails(facts []string) bool {\n\tfor _, fact := range facts {\n\t\tif strings.HasPrefix(fact, "article_detail=") {\n\t\t\treturn true\n\t\t}\n\t}\n\treturn false\n}\n\nfunc repairInteractiveArticleDetailFacts(facts []string) ([]string, bool) {\n\tbad := false\n\tfor _, raw := range facts {\n\t\tif !strings.HasPrefix(raw, "article_detail=") {\n\t\t\tcontinue\n\t\t}\n\t\tencoded := strings.TrimSpace(strings.TrimPrefix(raw, "article_detail="))\n\t\tif colon := strings.Index(encoded, ":"); colon >= 0 {\n\t\t\tencoded = strings.TrimSpace(encoded[colon+1:])\n\t\t}\n\t\tif llm.ArticleDetailFactIsRenderingMetadata(encoded) {\n\t\t\tbad = true\n\t\t\tbreak\n\t\t}\n\t}\n\tif !bad {\n\t\treturn facts, false\n\t}\n\tclean := make([]string, 0, len(facts))\n\tfor _, raw := range facts {\n\t\tif strings.HasPrefix(raw, "article_detail=") || strings.HasPrefix(raw, "article_detail_contract=") {\n\t\t\tcontinue\n\t\t}\n\t\tclean = append(clean, raw)\n\t}\n\treturn clean, true\n}\n''')
s = s.replace(
'''func (r *Repository) materializeInteractiveTitleArticleDetails(host world.Host, board world.Board, selected world.Post) (world.Post, string, error) {\n\tif !developmentInteractiveTitleFirstEnabled(r) || titleFirstSubject(selected.Intent.SituationFacts) == "" || hasInteractiveArticleDetails(selected.Intent.SituationFacts) {\n\t\treturn selected, "", nil\n\t}\n''',
'''func (r *Repository) materializeInteractiveTitleArticleDetails(host world.Host, board world.Board, selected world.Post) (world.Post, string, error) {\n\tif !developmentInteractiveTitleFirstEnabled(r) || titleFirstSubject(selected.Intent.SituationFacts) == "" {\n\t\treturn selected, "", nil\n\t}\n\tif repaired, changed := repairInteractiveArticleDetailFacts(selected.Intent.SituationFacts); changed {\n\t\tselected.Intent.SituationFacts = repaired\n\t\tif updater, ok := r.Base.(world.PostUpdater); ok {\n\t\t\tif updated, ok := updater.UpdatePost(host.ID, selected); ok {\n\t\t\t\tselected = updated\n\t\t\t}\n\t\t}\n\t}\n\tif hasInteractiveArticleDetails(selected.Intent.SituationFacts) {\n\t\treturn selected, "", nil\n\t}\n''')
p.write_text(s)

# LLM validator regression.
p = Path('apps/server/internal/llm/bbs_title_article_details_test.go')
s = p.read_text()
s += '''\nfunc TestValidateBBSTitleArticleDetailsRejectsHeaderMetadata(t *testing.T) {\n\treq := BBSTitleArticleDetailRequest{BoardName: "音楽", Articles: []BBSTitleArticleDetailSeed{{EventID: "e1", Subject: "YMOを聴き直しています", Summary: "YMOを聴き直している", CreatedAt: "1996-06-07T21:36:00+09:00"}}}\n\tfor _, bad := range []BBSArticleDetail{\n\t\t{Kind: "locator", Fact: "音楽板のMSG 1201として掲示されている"},\n\t\t{Kind: "timing", Fact: "1996年6月7日21時36分に投稿された"},\n\t\t{Kind: "locator", Fact: "新規スレッドの先頭投稿になっている"},\n\t} {\n\t\tdraft := BBSTitleArticleDetailDraft{Articles: []BBSTitleArticleDetailSet{{EventID: "e1", Details: []BBSArticleDetail{bad, {Kind: "comparison", Fact: "前に聴いた時と印象が少し違った"}}}}}\n\t\tif err := ValidateBBSTitleArticleDetails(req, draft); err == nil {\n\t\t\tt.Fatalf("header metadata detail must be rejected: %+v", bad)\n\t\t}\n\t}\n}\n\nfunc TestArticleDetailFactIsRenderingMetadataAllowsEventTiming(t *testing.T) {\n\tfor _, good := range []string{"接続して五分ほど後に一度切れた", "昨夜二度同じ症状が出た", "手元の攻略本の62ページだった"} {\n\t\tif ArticleDetailFactIsRenderingMetadata(good) {\n\t\t\tt.Fatalf("event-local detail wrongly rejected: %q", good)\n\t\t}\n\t}\n}\n'''
p.write_text(s)

# Worldrepo repair regression; pure helper test avoids another LLM call.
p = Path('apps/server/internal/worldrepo/materialization_interactive_latency_test.go')
s = p.read_text()
s += '''\nfunc TestRepairInteractiveArticleDetailFactsDropsEntireMetadataTaintedSet(t *testing.T) {\n\tin := []string{\n\t\t"title_first_subject=YMOを聴き直しています",\n\t\t"article_detail=locator:音楽板のMSG 1201として掲示されている",\n\t\t"article_detail=timing:1996年6月7日21時36分に投稿された",\n\t\t"article_detail_contract=old",\n\t\t"world_adopted_summary=YMOを聴き直している",\n\t}\n\tout, changed := repairInteractiveArticleDetailFacts(in)\n\tif !changed {\n\t\tt.Fatal("metadata-tainted detail set should be repaired")\n\t}\n\tjoined := strings.Join(out, "\\n")\n\tif strings.Contains(joined, "article_detail=") || strings.Contains(joined, "article_detail_contract=") {\n\t\tt.Fatalf("old detail set survived repair: %s", joined)\n\t}\n\tif !strings.Contains(joined, "title_first_subject=") || !strings.Contains(joined, "world_adopted_summary=") {\n\t\tt.Fatalf("unrelated canonical facts were removed: %s", joined)\n\t}\n}\n'''
p.write_text(s)

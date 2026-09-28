package llm

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	workerMSGPattern              = regexp.MustCompile(`(?i)\bMSG\s*#?\s*\d+\b`)
	workerOpeningTimestampPattern = regexp.MustCompile(`(?m)^\s*(?:\d{1,2}/\d{1,2}\s+\d{1,2}:\d{2}|\d{1,2}月\d{1,2}日\s*\d{1,2}時(?:\d{1,2}分)?)`)
)

// BuildBoardPostPrompt is intentionally shared by all prose-worker providers.
// World/planning metadata is already resolved before this point. The worker sees
// only the facts needed to write the post, not internal routing/debug machinery.
func BuildBoardPostPrompt(req BoardPostRequest) string {
	intent, requiredReferent := extractArticleReferentControl(req.PostIntent)
	facts := "(none supplied)"
	if len(req.HistoricalFacts) > 0 {
		facts = "- " + strings.Join(req.HistoricalFacts, "\n- ")
	}
	persona := "(ordinary member; no persistent profile supplied)"
	if strings.TrimSpace(req.AuthorHandle) != "" {
		persona = fmt.Sprintf("handle=%s\n%s", req.AuthorHandle, strings.TrimSpace(req.PersonaProfile))
	}
	intent = strings.TrimSpace(intent)
	if intent == "" {
		intent = "(no extra canonical facts supplied)"
	}
	referentRule := ""
	if requiredReferent != "" {
		referentRule = fmt.Sprintf(`- このroot記事には、読者が内容を理解するために必要なcanonical referent「%s」があります。表示件名がこの対象名を明示していない場合、本文の自然な位置で対象名を少なくとも一度は明示してください。対象を省略したまま「あの回」「前の回」「ボス」「このソフト」等だけで進めないでください。表示件名が対象名を明示している場合は本文で重ねて言い直す必要はありません。対象を別の作品・製品・店等へ置き換えないでください。`, requiredReferent)
	}
	subject := strings.TrimSpace(req.CanonicalSubject)
	if subject == "" {
		subject = strings.TrimSpace(req.BoardTopic)
	}
	eraRules := compactBoardPostEraRules(req.EraRules)
	minChars, maxChars := normalizeBodyBounds(req.BodyMinChars, req.BodyMaxChars)
	kind := req.Kind
	if kind == "" {
		kind = "new_post"
	}
	parent := "(none; this is a root post)"
	if strings.TrimSpace(req.ParentSubject) != "" || strings.TrimSpace(req.ParentBody) != "" {
		parent = fmt.Sprintf("subject=%s\nbody:\n%s", strings.TrimSpace(req.ParentSubject), strings.TrimSpace(req.ParentBody))
	}
	quoteRule := "引用なしで構いません。"
	if strings.TrimSpace(req.QuoteText) != "" {
		quoteRule = fmt.Sprintf("引用する場合は、次の原文を先頭に > を付けて一字一句そのまま使ってください。改変・要約は禁止です:\n%s", strings.TrimSpace(req.QuoteText))
	}
	subjectRule := "JSONのsubjectは上記の件名をそのまま返してください。"
	if strings.Contains(intent, "surface_subject_mode=title_first_root") {
		subjectRule = `上記の件名は、年代検証・人物割当・世界事実の確定に使われた意味判定用タイトルです。JSONのsubjectには、この人物が実際にBBSの件名欄へ入力しそうな表示件名を返してください。
- 意味判定用タイトルをそのまま使っても構いませんが、人物と状況に自然なら短縮、口語化、省略、感情の混じった言い方にして構いません。
- 表示件名は元の出来事・対象・立場を変えてはいけません。新しい製品、場所、経験、原因、評価、予定などの世界事実を追加しないでください。
- 説明文として完全である必要はありません。本文や板の文脈があれば通じる「モデムが見えない…」「週末、秋葉原へ」「ポケモン買った？」程度の省略も自然なら可能です。ただし「あれ？」「うーむ」のような極端に曖昧な件名を毎回使うなど、固定パターン化しないでください。
- 36文字以内・1行。rootなのでRe:は付けません。件名にもpersonaのwriting傾向を反映してよいですが、レトロ演出のために崩さないでください。
- surface_subject_modeはレンダリング指示であり、本文や件名へ文字列として書かないでください。`
	}

	return fmt.Sprintf(`1996年前後の日本の草の根パソコン通信BBSに、指定された人物として1件だけ自然な本文を書いてください。

件名: %s
掲示板: %s
投稿者:
%s

この記事について既に確定している事実:
%s

本文で使用してよい公開史実:
%s

時代制約:
- 世界日付は %s。未来の知識は使わない。
- %s

件名の扱い:
%s

書き方:
- 読者はすでにヘッダ（件名・投稿者・日時・掲示板）とスレッド文脈を見ています。本文でヘッダを読み上げないでください。前提も親切に言い直さず、この1件だけを切り出して完全に理解できる文章にする必要はありません。
- MSG番号、記事番号、投稿時刻、日付、board id、内部状態、cause、routing、world slot、「新スレです」「～板から失礼します」など、投稿システム側のメタ情報を書かないでください。
- 「この記事について既に確定している事実」は世界事実の許可境界であって、本文に全部書くチェックリストではありません。この人物がこの瞬間に実際に口にしそうな部分だけを書いてください。
- discourse_mode=share_experience / share_observation / share_tip は投稿者本人の経験・観察・実践として扱い、「～とのこと」「～だそうです」のような他人事へ変えないでください。state_opinion は本人の意見、ask_peers は本人の未解決の問いです。source_ で始まる事実だけが相手側のものです。
- 投稿を「役に立つ記事」「きれいな説明」「完成した回答」に仕上げる必要はありません。具体的な事実は具体的に書いてよい一方、文章そのものは多少雑でも構いません。一言の感想、短い報告、途中の独り言、相手の一部分への反応だけで終わって構いません。
- canonical_event または utterance_attention がある場合、記事全体を網羅せず、その一点に本人の注意が向いている状態を保ってください。utterance_knowledge が firsthand なら他人から聞いた話のように書かず、tentative/hearsay なら断定へ昇格させず、opinion/question なら専門家の確定回答へ変えないでください。
- 確定事実に必要性がない限り、まとめ、教訓、結論、網羅的な比較、読者への質問を付け足さないでください。特に毎回「みなさんはどうですか？」型で締めないでください。情報量の少ない感情、相づち、言い直し、軽い反復、括弧内の独り言が人物に自然なら残して構いません。
- 返信では、相手の記事の全部に答えたり会話を必ず発展させたりする必要はありません。reply_scope / reply_information / reply_context / article_detail があれば、その返信者について確定済みの差分として自然に使ってください。先行replyと同じ一般論を別の言い方で繰り返すことを安全策にしないでください。canonicalに具体的な差分が無い場合だけ、短い相づちや一部分への反応で終わって構いません。元記事を要約してから返事を始めないでください。先行replyも要約し直さず、今回の反応や差分から入ってください。
- 「この件について書きます」「おすすめを教えてください」のように件名を言い換えるだけで終わらず、確定事実の中に具体的な対象・観察・条件・回数・順序・比較があれば、必要なものだけ普通に使ってください。タイトルが抽象的でも、canonicalなarticle_detailが具体的なら本文まで抽象化しないでください。
- 記事の書き方を説明せず、最初の文から用件そのものに入ってください。本人が今言いたい部分だけを書いて構いません。
- article_detail は投稿者について確定済みのローカル事実です。ただし本文で全detailを列挙する義務はありません。source_article_detail は相手の記事の事実で、返信者自身の経験へ移してはいけません。
%s
- article_detail または supplied historical facts にない具体的な操作手順、画面位置、料金制度、製品仕様、攻略情報、購入場所・購入経緯、通勤中/仕事帰り等の状況、将来の予定を「自然な補足」として作らないでください。detailが無ければ、その部分は短く曖昧なままで構いません。
- subject/summaryから自然に分かる驚き・喜び・困惑などの一時的な反応は表現して構いませんが、新しい所有・経験・行動・予定を事実として足してはいけません。
- supplied historical facts は公開世界について使ってよい事実です。そこから未提示の価格・発売日・仕様・作品内容などを連想で追加しないでください。
- 新しい所有歴、購入歴、職歴、家族事情、長期的な嗜好や習慣を勝手に作らないでください。
- 文章量・段落数・文の切り方は投稿者プロフィールの writing= を最優先してください。全員を同じ長さや同じ三文構成に揃えないでください。文章は自然なら短くて構いません。一文だけでも、多段落でも構いません。長文でも「導入→整理→結論」に整えず、本人が気づいた順・思い出した順に並んだり、途中で感想や言い直しが挟まったりして構いません。技術に詳しい人物も、canonicalに確定していない部分は不確かさを残せます。完全な解説記事やチュートリアルへ仕上げないでください。段落数は固定せず、目標文字数の範囲内で自然に書いてください。顔文字も人物プロフィールに従ってください。
- 改行は意味のある段落、短い反応、引用、会話、署名風レイアウトなど投稿者が意図して入れる構造だけに使ってください。現代のスマホ文章のように10〜20文字程度ごと、または一文ごとに見栄え目的で改行しないでください。普通の長い文章行は無理に短くせず、端末側の80桁級表示で自然に折り返される前提で構いません。
- 逆に、全員を長い一段落へ統一もしないでください。短文一行、空行を挟む人、引用だけ別行にする人など、personaと会話状況に自然な差は残してください。
- 1990年代らしさを小道具で演出せず、その時代の本人として普通に書いてください。
- world layerが選んだ投稿種別: %s。root/replyを変更しないでください。
- 新しく書く非引用部分の目標文字数: %d〜%d文字。水増しせず、自然さを優先してください。
- 返信対象の親記事（rootならなし）:
%s
- %s

JSONだけを返してください:
{"author":"...","subject":"...","body":"..."}`, subject, req.BoardTopic, persona, intent, facts, req.WorldDate, eraRules, subjectRule, referentRule, kind, minChars, maxChars, parent, quoteRule)
}

func compactBoardPostEraRules(raw string) string {
	raw = strings.TrimSpace(raw)
	first := raw
	if i := strings.IndexByte(first, '\n'); i >= 0 {
		first = strings.TrimSpace(first[:i])
	}
	if first == "" {
		first = "世界日付より未来の知識や出来事を使わない。"
	}
	return first + "\n当時の本人として普通に書く。現代から振り返る説明、レトロ・懐古演出、時代解説はしない。"
}

func validateBoardPostWorkerDraft(req BoardPostRequest, d BoardPostDraft) error {
	_, maxChars := normalizeBodyBounds(req.BodyMinChars, req.BodyMaxChars)
	if err := validateBoardPostDraftWithBodyLimit(d, maxChars); err != nil {
		return err
	}
	body := strings.TrimSpace(d.Body)
	if workerMSGPattern.MatchString(body) {
		return fmt.Errorf("article worker leaked MSG metadata")
	}
	if workerOpeningTimestampPattern.MatchString(body) {
		return fmt.Errorf("article worker opened by restating header timestamp")
	}
	for _, marker := range []string{"新スレです", "新規スレです", "板から失礼します", "板のMSG", "board id", "routing_domain", "cause_kind", "world slot"} {
		if strings.Contains(strings.ToLower(body), strings.ToLower(marker)) {
			return fmt.Errorf("article worker leaked/meta-narrated %q", marker)
		}
	}
	return nil
}


func extractArticleReferentControl(raw string) (string, string) {
	lines := strings.Split(strings.ReplaceAll(raw, "\r", ""), "\n")
	kept := make([]string, 0, len(lines))
	referent := ""
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "article_referent_required=") {
			if referent == "" {
				referent = strings.TrimSpace(strings.TrimPrefix(trimmed, "article_referent_required="))
			}
			continue
		}
		kept = append(kept, line)
	}
	return strings.TrimSpace(strings.Join(kept, "\n")), referent
}

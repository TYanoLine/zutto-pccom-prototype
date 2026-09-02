package worldrepo

import (
	"sort"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type demoTopicSeed struct {
	subject    string
	motivation string
}

// MaterializationDenseArticleHeaders is a development-only, denser version of
// the article-envelope PoC. It creates enough history to make persona activity
// differences visible without paying to render every article body up front.
//
// The 20 envelopes are spread over roughly ten days. The active-poster mix is
// intentionally uneven: core regulars appear frequently while the lurker-leaning
// TAKA appears only occasionally. Bodies remain empty until article read time.
func (r *Repository) MaterializationDenseArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool) {
	if existing := filterBoard(r.Base.ListPosts(host.ID), board.ID); len(existing) > 0 {
		return existing, false
	}

	personas, _ := r.MaterializationPersonas(host)
	stamp := worldTime(r.WorldDate)
	handles := []string{
		"MARI", "NEKO", "SYSOP", "NORI", "YUKI",
		"NEKO", "MARI", "TAKA", "NORI", "NEKO",
		"YUKI", "SYSOP", "MARI", "NEKO", "NORI",
		"YUKI", "MARI", "TAKA", "NEKO", "SYSOP",
	}
	topics := demoTopicsForBoard(board)

	pending := make([]world.Post, 0, len(handles))
	for i, handle := range handles {
		persona, ok := personaByHandle(personas, handle)
		seed := topics[i%len(topics)]
		intent := world.PostIntent{
			Action:     "thread_start",
			Topic:      seed.subject,
			Motivation: seed.motivation,
			Stance:     demoPersonaStance(persona),
		}
		p := world.Post{
			BoardID:   board.ID,
			Author:    handle,
			Subject:   seed.subject,
			Intent:    intent,
			CreatedAt: demoPersonaTimestamp(stamp, handle, i),
		}
		if ok {
			p.Author = persona.Handle
			p.AuthorPersonaID = persona.ID
		}
		pending = append(pending, p)
	}

	// IDs should advance in world-time order so the index looks like a plausible
	// persisted BBS history rather than a shuffled fixture.
	sort.SliceStable(pending, func(i, j int) bool { return pending[i].CreatedAt.Before(pending[j].CreatedAt) })

	out := make([]world.Post, 0, len(pending))
	var lastRootID int64
	var lastRootSubject string
	for i, p := range pending {
		// Roughly one in four visible posts is a reply. The reply relation is part
		// of the canonical envelope; prose generation later must honor it.
		if i > 0 && i%4 == 3 && lastRootID != 0 {
			p.ParentID = lastRootID
			p.Subject = "Re: " + lastRootSubject
			p.Intent.Action = "reply"
			p.Intent.Topic = lastRootSubject
			p.Intent.Motivation = "続いている話題を読んで、自分の経験や考えを返したくなった"
		} else if p.Author == "SYSOP" && i%7 == 0 {
			p.Intent.Action = "announcement"
			p.Intent.Motivation = "局の常連としてではなく、SYSOPとして短い案内や確認を書いておきたい"
		}

		p = r.Base.AddPost(host.ID, p)
		out = append(out, p)
		if p.Intent.Action != "reply" {
			lastRootID = p.ID
			lastRootSubject = p.Subject
		}
	}
	return out, true
}

func demoTopicsForBoard(board world.Board) []demoTopicSeed {
	switch board.ID {
	case "2":
		return []demoTopicSeed{
			{subject: "モデムの設定、みなさんどうしてます？", motivation: "自分の設定を見直していて、他の人のやり方も聞いてみたい"},
			{subject: "最近つながりにくい時間", motivation: "接続の具合にばらつきを感じたので、他の人にも様子を聞きたい"},
			{subject: "通信ソフトのマクロ", motivation: "日々の巡回を少し楽にしたくて、マクロの工夫を話題にしたい"},
			{subject: "自動巡回って便利？", motivation: "自動巡回を使うか迷っていて、常連の使い方を知りたい"},
			{subject: "ファイル転送で失敗(^^;", motivation: "転送に失敗したので、原因になりそうな点を雑談交じりに聞きたい"},
			{subject: "ATコマンドのメモ", motivation: "自分用に試したことを整理しつつ、他の人の定番も知りたい"},
			{subject: "回線速度の体感", motivation: "表示される速度と実際の感じ方について、経験談を交換したい"},
			{subject: "文字化けする時", motivation: "たまに表示が崩れるので、端末設定の見直し方を相談したい"},
			{subject: "ログ整理どうしてます？", motivation: "保存ログが増えてきたので、整理の仕方をみんなに聞きたい"},
			{subject: "端末設定について", motivation: "普段の通信条件を少し変えてみたので、使い勝手を話したい"},
		}
	case "3":
		return []demoTopicSeed{
			{subject: "駅前の店の話", motivation: "近所で見かけた店について、地元の人の評判を聞きたい"},
			{subject: "週末どこか行きません？", motivation: "常連どうしで軽く集まれそうか聞いてみたい"},
			{subject: "近所のパソコンショップ", motivation: "地元で立ち寄りやすい店について情報交換したい"},
			{subject: "夜の駅前って", motivation: "遅い時間の駅前の様子について雑談したい"},
			{subject: "この辺の電話代(^^;", motivation: "長電話ならぬ長通信になりがちなので、地元同士で軽くぼやきたい"},
			{subject: "オフ会の場所", motivation: "集まるならどこが分かりやすいか、地元の意見を聞きたい"},
			{subject: "帰りが遅くなりました", motivation: "帰宅途中のちょっとした出来事を常連相手に話したい"},
			{subject: "地元ネタ募集", motivation: "最近話題が少ないので、近所の小ネタをみんなから集めたい"},
			{subject: "雨の日の移動", motivation: "天気が悪い日の移動について、地元ならではの雑談をしたい"},
			{subject: "おすすめの喫茶店", motivation: "ゆっくり話せる店を探していて、常連のおすすめを聞きたい"},
		}
	default:
		return []demoTopicSeed{
			{subject: "最近どうです？", motivation: "特に用事はないが、常連の近況を軽く聞いてみたい"},
			{subject: "土曜の夜、みなさん何してます？", motivation: "週末の過ごし方を話題にして雑談を始めたい"},
			{subject: "おすすめのCDあります？", motivation: "最近聴くものを探していて、常連のおすすめを聞きたい"},
			{subject: "この前のオフ、どうでした？", motivation: "参加した人の感想や、参加できなかった人の話を聞きたい"},
			{subject: "最近ちょっと夜更かし気味", motivation: "深夜接続が続いているので、同じ時間帯の常連に話しかけたい"},
			{subject: "はじめまして", motivation: "このボードではまだあまり書いていないので、まず挨拶しておきたい"},
			{subject: "ゲームの話でも", motivation: "最近遊んだものの話をきっかけに、軽い雑談を始めたい"},
			{subject: "休日の予定", motivation: "次の休みに何をするか、みんなの予定も含めて雑談したい"},
			{subject: "雑談ネタ募集(^^;", motivation: "ボードが少し静かなので、気軽に書ける話題を振りたい"},
			{subject: "そろそろ秋ですね", motivation: "季節の変わり目をきっかけに、特に結論のない雑談をしたい"},
		}
	}
}

func demoPersonaStance(p world.Persona) string {
	if p.Handle == "SYSOP" {
		return "局の雰囲気を壊さず、必要な時だけ少しまとめ役になる"
	}
	if p.Argumentativeness >= .35 {
		return "自分の考えははっきり持つが、相手を言い負かすこと自体は目的にしない"
	}
	if p.LurkerTendency >= .5 {
		return "普段はROM気味なので、書く時も短く自分の経験だけを添える"
	}
	if p.NewcomerOpenness >= .8 {
		return "柔らかく話題に入り、知らない相手にも比較的返事をしやすい"
	}
	return "自分の近況や感想を自然に書き、必要なら他の人の意見も聞く"
}

func demoPersonaTimestamp(stamp time.Time, handle string, i int) time.Time {
	// Two posts per logical day over ten days. Times are biased by persona so the
	// index visibly reflects their persistent activity patterns.
	dayBack := 9 - i/2
	day := stamp.AddDate(0, 0, -dayBack)
	dayShift, hour, minute := 0, 22, 0
	switch handle {
	case "MARI":
		hour, minute = 21, 10+(i%5)*6
	case "SYSOP":
		hour, minute = 22, 5+(i%4)*8
	case "NEKO":
		hour, minute = 23, 3+(i%6)*7
	case "TAKA":
		hour, minute = 23, 32+(i%3)*7
	case "NORI":
		dayShift, hour, minute = 1, 0, 18+(i%5)*9
	case "YUKI":
		if i%2 == 0 {
			hour, minute = 22, 45+(i%3)*5
		} else {
			dayShift, hour, minute = 1, 1, 2+(i%4)*8
		}
	}
	created := time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, day.Location()).AddDate(0, 0, dayShift)
	if created.After(stamp) {
		created = created.AddDate(0, 0, -1)
	}
	return created
}

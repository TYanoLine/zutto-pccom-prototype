package worldrepo

import (
	"crypto/sha256"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/world"
)

type demoTopicSeed struct {
	key        string
	subjects   []string
	motivation string
	interests  map[string]float64
	role       string
}

type demoPostCandidate struct {
	persona   world.Persona
	createdAt time.Time
}

// MaterializationDenseArticleHeaders is a development-only PoC for actor-first
// article history. Unlike the earlier fixed 20-row fixture, it samples visible
// activity from persistent persona traits, board affinity and recent topic state.
// The result is still deterministic for the same host/board/world date so first
// observation can be committed as shared history without per-viewer divergence.
func (r *Repository) MaterializationDenseArticleHeaders(host world.Host, board world.Board) ([]world.Post, bool) {
	if existing := filterBoard(r.Base.ListPosts(host.ID), board.ID); len(existing) > 0 {
		return existing, false
	}

	personas, _ := r.MaterializationPersonas(host)
	stamp := worldTime(r.WorldDate)
	pending := make([]demoPostCandidate, 0, 32)

	// Fourteen days is long enough to expose habitual differences without
	// pretending that all dormant host history must be materialized at once.
	for dayBack := 13; dayBack >= 0; dayBack-- {
		day := stamp.AddDate(0, 0, -dayBack)
		for _, persona := range personas {
			activity := demoActivityProbability(persona, board)
			if day.Weekday() == time.Saturday || day.Weekday() == time.Sunday {
				activity += .025
				if persona.Handle == "MARI" {
					activity += .04
				}
			}
			roll := demoStableUnit(host.ID, board.ID, persona.ID, day.Format("2006-01-02"), "active")
			if roll >= clamp01(activity) {
				continue
			}
			created := demoPersonaTimestampForDay(day, persona, host.ID, board.ID)
			if created.After(stamp) {
				continue
			}
			pending = append(pending, demoPostCandidate{persona: persona, createdAt: created})
		}
	}

	sort.SliceStable(pending, func(i, j int) bool { return pending[i].createdAt.Before(pending[j].createdAt) })

	topics := demoTopicsForBoard(board)
	topicLastUsed := map[string]time.Time{}
	personaTopicLastUsed := map[string]map[string]time.Time{}
	subjectLastUsed := map[string]time.Time{}
	roots := make([]world.Post, 0, len(pending))
	out := make([]world.Post, 0, len(pending))

	for i, candidate := range pending {
		persona := candidate.persona
		created := candidate.createdAt
		stance := demoPersonaStance(persona)

		var post world.Post
		if root, ok := demoChooseReplyTarget(host, board, persona, created, roots, topics, i); ok && demoShouldReply(host, board, persona, created, i) {
			post = world.Post{
				BoardID:         board.ID,
				ParentID:        root.ID,
				Author:          persona.Handle,
				AuthorPersonaID: persona.ID,
				Subject:         "Re: " + root.Subject,
				Intent: world.PostIntent{
					Action:     "reply",
					Topic:      root.Intent.Topic,
					Motivation: demoReplyMotivation(persona, root),
					Stance:     stance,
				},
				CreatedAt: created,
			}
		} else {
			seed := demoChooseTopic(host, board, persona, created, topics, topicLastUsed, personaTopicLastUsed)
			subject := demoChooseSubject(host, board, persona, created, seed, subjectLastUsed)
			action := "thread_start"
			if seed.role == "sysop" {
				action = "announcement"
			}
			post = world.Post{
				BoardID:         board.ID,
				Author:          persona.Handle,
				AuthorPersonaID: persona.ID,
				Subject:         subject,
				Intent: world.PostIntent{
					Action:     action,
					Topic:      seed.key,
					Motivation: seed.motivation,
					Stance:     stance,
				},
				CreatedAt: created,
			}
			topicLastUsed[seed.key] = created
			if personaTopicLastUsed[persona.ID] == nil {
				personaTopicLastUsed[persona.ID] = map[string]time.Time{}
			}
			personaTopicLastUsed[persona.ID][seed.key] = created
			subjectLastUsed[subject] = created
		}

		post = r.Base.AddPost(host.ID, post)
		out = append(out, post)
		if post.ParentID == 0 {
			roots = append(roots, post)
		}
	}
	return out, true
}

func demoActivityProbability(p world.Persona, board world.Board) float64 {
	affinity := demoBoardAffinity(p, board)
	// Activity is not a direct post-count target. These latent-ish tendencies make
	// visible posting an emergent outcome: lurkers disappear more often, while a
	// strong board interest can pull an otherwise quiet member into the index.
	return .03 + (1-p.LurkerTendency)*.12 + affinity*.25 + math.Min(1, p.ReplyTendency+p.ThreadStartTendency)*.05
}

func demoBoardAffinity(p world.Persona, board world.Board) float64 {
	interest := func(key string) float64 { return p.Interests[key] }
	var values []float64
	switch board.ID {
	case "2":
		values = []float64{interest("modem"), interest("software"), interest("pc98") * .85, interest("bbs") * .65}
	case "3":
		values = []float64{interest("local"), interest("chat") * .55, interest("games") * .20}
	default:
		values = []float64{interest("chat"), interest("music") * .80, interest("games") * .75, interest("local") * .55, interest("bbs") * .30}
	}
	best := 0.0
	for _, value := range values {
		if value > best {
			best = value
		}
	}
	return clamp01(best)
}

func demoShouldReply(host world.Host, board world.Board, p world.Persona, at time.Time, ordinal int) bool {
	chance := .12 + p.ReplyTendency*.48 - p.ThreadStartTendency*.12
	chance = math.Max(.08, math.Min(.62, chance))
	return demoStableUnit(host.ID, board.ID, p.ID, at.Format(time.RFC3339), fmt.Sprintf("reply-%d", ordinal)) < chance
}

func demoChooseReplyTarget(host world.Host, board world.Board, p world.Persona, at time.Time, roots []world.Post, topics []demoTopicSeed, ordinal int) (world.Post, bool) {
	if len(roots) == 0 {
		return world.Post{}, false
	}
	bestScore := -10.0
	var best world.Post
	found := false
	start := len(roots) - 1
	stop := start - 7
	if stop < 0 {
		stop = 0
	}
	for i := start; i >= stop; i-- {
		root := roots[i]
		age := at.Sub(root.CreatedAt)
		if age < 0 || age > 6*24*time.Hour {
			continue
		}
		seed, _ := demoTopicByKey(topics, root.Intent.Topic)
		score := 1 - age.Hours()/(6*24)
		score += demoTopicInterestScore(p, seed) * .70
		if root.AuthorPersonaID == p.ID {
			score -= .65
		}
		score += demoStableUnit(host.ID, board.ID, p.ID, fmt.Sprint(root.ID), fmt.Sprintf("target-%d", ordinal)) * .22
		if score > bestScore {
			bestScore = score
			best = root
			found = true
		}
	}
	return best, found
}

func demoChooseTopic(host world.Host, board world.Board, p world.Persona, at time.Time, topics []demoTopicSeed, topicLastUsed map[string]time.Time, personaTopicLastUsed map[string]map[string]time.Time) demoTopicSeed {
	bestScore := -100.0
	best := topics[0]
	for _, seed := range topics {
		if seed.role == "sysop" && p.Handle != "SYSOP" {
			continue
		}
		if seed.role != "" && seed.role != "sysop" {
			continue
		}

		score := .18 + demoTopicInterestScore(p, seed)
		score += demoStableUnit(host.ID, board.ID, p.ID, at.Format("2006-01-02"), seed.key) * .28
		if p.Handle == "SYSOP" && seed.role == "sysop" {
			score += .30
		}
		if last, ok := topicLastUsed[seed.key]; ok {
			ageDays := at.Sub(last).Hours() / 24
			switch {
			case ageDays < 2.5:
				score -= 1.20
			case ageDays < 5:
				score -= .55
			case ageDays < 9:
				score -= .20
			}
		}
		if perPersona := personaTopicLastUsed[p.ID]; perPersona != nil {
			if last, ok := perPersona[seed.key]; ok && at.Sub(last) < 10*24*time.Hour {
				score -= .35
			}
		}
		if score > bestScore {
			bestScore = score
			best = seed
		}
	}
	return best
}

func demoChooseSubject(host world.Host, board world.Board, p world.Persona, at time.Time, seed demoTopicSeed, subjectLastUsed map[string]time.Time) string {
	if len(seed.subjects) == 0 {
		return seed.key
	}
	start := demoStableIndex(len(seed.subjects), host.ID, board.ID, p.ID, at.Format(time.RFC3339), seed.key, "subject")
	for offset := 0; offset < len(seed.subjects); offset++ {
		subject := seed.subjects[(start+offset)%len(seed.subjects)]
		if last, ok := subjectLastUsed[subject]; !ok || at.Sub(last) >= 8*24*time.Hour {
			return subject
		}
	}
	return seed.subjects[start]
}

func demoReplyMotivation(p world.Persona, root world.Post) string {
	if p.Argumentativeness >= .35 {
		return "直前の流れを読んで、自分の経験と少し違う点を具体的に返したくなった"
	}
	if p.NewcomerOpenness >= .80 {
		return "話が続いているので、相手を置き去りにしないよう自分の近況や感想も返したい"
	}
	if p.LurkerTendency >= .50 {
		return "普段はROMしているが、この話題だけは自分にも経験があるので短く返したい"
	}
	return "続いている話題が自分の関心にも近く、ひとこと経験や感想を返したくなった"
}

func demoTopicInterestScore(p world.Persona, seed demoTopicSeed) float64 {
	if len(seed.interests) == 0 {
		return .15
	}
	score := 0.0
	weightTotal := 0.0
	for key, weight := range seed.interests {
		score += p.Interests[key] * weight
		weightTotal += math.Abs(weight)
	}
	if weightTotal == 0 {
		return 0
	}
	return clamp01(score / weightTotal)
}

func demoTopicByKey(topics []demoTopicSeed, key string) (demoTopicSeed, bool) {
	for _, seed := range topics {
		if seed.key == key {
			return seed, true
		}
	}
	return demoTopicSeed{key: key}, false
}

func demoTopicsForBoard(board world.Board) []demoTopicSeed {
	mk := func(key, motivation string, interests map[string]float64, subjects ...string) demoTopicSeed {
		return demoTopicSeed{key: key, subjects: subjects, motivation: motivation, interests: interests}
	}
	switch board.ID {
	case "2":
		return []demoTopicSeed{
			mk("modem_settings", "自分の設定を見直していて、他の人のやり方も聞いてみたい", map[string]float64{"modem": 1, "pc98": .3}, "モデムの設定、みなさんどうしてます？", "モデム設定を見直し中", "通信条件って変えてます？"),
			mk("busy_hours", "接続の具合にばらつきを感じたので、他の人にも様子を聞きたい", map[string]float64{"modem": .7, "bbs": .5}, "最近つながりにくい時間", "夜の回線、混んでます？", "つながる時間帯の話"),
			mk("comm_macro", "日々の巡回を少し楽にしたくて、マクロの工夫を話題にしたい", map[string]float64{"software": 1}, "通信ソフトのマクロ", "巡回マクロいじってます", "マクロの組み方で質問"),
			mk("auto_patrol", "自動巡回を使うか迷っていて、常連の使い方を知りたい", map[string]float64{"software": .8, "bbs": .4}, "自動巡回って便利？", "みなさん自動巡回してます？", "巡回のやり方"),
			mk("file_transfer", "転送に失敗したので、原因になりそうな点を雑談交じりに聞きたい", map[string]float64{"modem": .7, "software": .7}, "ファイル転送で失敗(^^;", "転送が途中で止まります", "ファイル転送の調子"),
			mk("at_commands", "試したコマンドを整理しつつ、他の人の定番も知りたい", map[string]float64{"modem": 1}, "ATコマンドのメモ", "よく使うATコマンド", "ATコマンド、何使ってます？"),
			mk("line_speed", "表示される速度と実際の感じ方について、経験談を交換したい", map[string]float64{"modem": .9}, "回線速度の体感", "速度表示と体感の差", "最近の接続速度"),
			mk("garbled_text", "たまに表示が崩れるので、端末設定の見直し方を相談したい", map[string]float64{"software": .7, "pc98": .4}, "文字化けする時", "たまに文字化けします", "端末側の文字設定"),
			mk("log_management", "保存ログが増えてきたので、整理の仕方をみんなに聞きたい", map[string]float64{"software": .7, "bbs": .6}, "ログ整理どうしてます？", "過去ログがたまってきました", "ログの保存方法"),
			mk("terminal_settings", "普段の通信条件を少し変えてみたので、使い勝手を話したい", map[string]float64{"software": .6, "modem": .5}, "端末設定について", "通信条件を少し変更", "端末の設定見直し"),
			mk("pc98_environment", "PC-98側の環境について他の利用者の構成も聞いてみたい", map[string]float64{"pc98": 1}, "みなさんの98環境", "PC-98側の通信環境", "通信に使ってる98の構成"),
			{key: "board_housekeeping", role: "sysop", subjects: []string{"通信関係の話題整理について", "このボードの使い分け"}, motivation: "似た話題が増えてきたので、SYSOPとして使い分けを軽く案内しておきたい", interests: map[string]float64{"bbs": 1}},
		}
	case "3":
		return []demoTopicSeed{
			mk("station_shops", "近所で見かけた店について、地元の人の評判を聞きたい", map[string]float64{"local": 1}, "駅前の店の話", "駅前で気になる店", "あの店行った人います？"),
			mk("weekend_outing", "常連どうしで軽く集まれそうか聞いてみたい", map[string]float64{"local": .7, "chat": .5}, "週末どこか行きません？", "今度の週末どうします？", "週末の予定、地元組は？"),
			mk("pc_shop", "地元で立ち寄りやすい店について情報交換したい", map[string]float64{"local": .7, "pc98": .8}, "近所のパソコンショップ", "この辺でパソコン見るなら", "地元のパソコン屋さん"),
			mk("late_station", "遅い時間の駅前の様子について雑談したい", map[string]float64{"local": .9}, "夜の駅前って", "遅い時間の駅前", "夜に駅前を通ったら"),
			mk("offline_meeting", "集まるならどこが分かりやすいか、地元の意見を聞きたい", map[string]float64{"local": .8, "chat": .6}, "オフ会の場所", "集まるならどこがいい？", "次のオフの場所どうします？"),
			mk("commute", "帰宅途中のちょっとした出来事を常連相手に話したい", map[string]float64{"local": .8}, "帰りが遅くなりました", "帰り道の話", "今日は遅い帰宅でした"),
			mk("local_tips", "最近話題が少ないので、近所の小ネタをみんなから集めたい", map[string]float64{"local": 1, "chat": .4}, "地元ネタ募集", "この辺の小ネタありません？", "近所の話でも"),
			mk("cafe", "ゆっくり話せる店を探していて、常連のおすすめを聞きたい", map[string]float64{"local": .8, "chat": .4}, "おすすめの喫茶店", "ゆっくりできる店あります？", "近所でお茶するなら"),
			mk("local_food", "近場で気軽に入れる店の話をしたい", map[string]float64{"local": .7, "chat": .4}, "近所でごはん食べるなら", "この辺の安い店", "地元の食べ物ネタ"),
			mk("local_event", "近所で見かけた催しについて、行く人がいるか聞いてみたい", map[string]float64{"local": .9}, "近所で何かやってますね", "地元の催しの話", "週末の地元イベント"),
			{key: "board_housekeeping", role: "sysop", subjects: []string{"地域ネタの書き分けについて", "このボードの使い方"}, motivation: "地域外の雑談と混ざり始めたので、SYSOPとして軽く案内しておきたい", interests: map[string]float64{"bbs": 1, "local": .5}},
		}
	default:
		return []demoTopicSeed{
			mk("recent_life", "特に用事はないが、常連の近況を軽く聞いてみたい", map[string]float64{"chat": .9}, "最近どうです？", "みなさん最近どうしてます？", "近況でも書いてみます"),
			mk("weekend_night", "週末の過ごし方を話題にして雑談を始めたい", map[string]float64{"chat": .8, "games": .3}, "土曜の夜、みなさん何してます？", "週末の夜って", "土曜の夜の過ごし方"),
			mk("music_recommend", "最近聴くものを探していて、常連のおすすめを聞きたい", map[string]float64{"music": 1, "chat": .3}, "おすすめのCDあります？", "最近何聴いてます？", "何かCD買おうかな"),
			mk("offline_recap", "参加した人の感想や、参加できなかった人の話を聞きたい", map[string]float64{"local": .6, "chat": .7}, "この前のオフ、どうでした？", "この前集まった時の話", "オフ参加組おつかれさま"),
			mk("late_night", "深夜接続が続いているので、同じ時間帯の常連に話しかけたい", map[string]float64{"chat": .7}, "最近ちょっと夜更かし気味", "またこんな時間(^^;", "夜更かし組います？"),
			mk("games", "最近遊んだものの話をきっかけに、軽い雑談を始めたい", map[string]float64{"games": 1, "chat": .3}, "ゲームの話でも", "最近やってるゲーム", "何かゲームしてます？"),
			mk("holiday_plan", "次の休みに何をするか、みんなの予定も含めて雑談したい", map[string]float64{"chat": .8, "local": .3}, "休日の予定", "次の休み何しよう", "休みの日って何してます？"),
			mk("work_school", "仕事や学校の区切りで、軽く近況を書いておきたい", map[string]float64{"chat": .6}, "やっと一日終わった(^^;", "今日は疲れました", "仕事・学校おつかれさま"),
			mk("food", "食べ物の話なら気軽に続きそうなので雑談を振りたい", map[string]float64{"chat": .6, "local": .3}, "夜食の話(^^;", "最近よく食べるもの", "何かうまいものないですか"),
			mk("pc_chatter", "パソコンの話を専門板ほど固くせず雑談として振りたい", map[string]float64{"pc98": .7, "software": .5, "chat": .2}, "最近パソコン何に使ってます？", "98も雑談ネタに", "パソコンいじってました"),
			mk("quiet_board", "少し静かな時間が続いたので、無理に盛り上げず軽く声を出してみたい", map[string]float64{"chat": .8, "bbs": .3}, "今日は静かですね", "誰かいます？(^^;", "ちょっと足あと"),
			mk("books_magazines", "最近読んだものについて常連と軽く話したい", map[string]float64{"chat": .4}, "最近読んだもの", "本とか雑誌の話", "何か面白い読み物あります？"),
			{key: "board_housekeeping", role: "sysop", subjects: []string{"雑談ボードの使い方について", "このボードの話題について"}, motivation: "話題が広がってきたので、SYSOPとして最低限の案内だけ書いておきたい", interests: map[string]float64{"bbs": 1}},
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

func demoPersonaTimestampForDay(day time.Time, p world.Persona, hostID, boardID string) time.Time {
	peak, spread := demoPersonaClockProfile(p)
	offsetRoll := demoStableIndex(spread*2+1, hostID, boardID, p.ID, day.Format("2006-01-02"), "hour") - spread
	hour := peak + offsetRoll
	minute := demoStableIndex(60, hostID, boardID, p.ID, day.Format("2006-01-02"), "minute")
	return time.Date(day.Year(), day.Month(), day.Day(), hour, minute, 0, 0, day.Location())
}

func demoPersonaClockProfile(p world.Persona) (peakHour, spreadHours int) {
	// The current Persona model still stores human-readable activity text. Keep
	// the structured interpretation here in the development PoC until production
	// persistence gains explicit schedule fields.
	pattern := p.ActivityPattern
	switch {
	case strings.Contains(pattern, "0:00-03:00"):
		return 1, 2
	case strings.Contains(pattern, "21:00-00:30"):
		return 22, 2
	case strings.Contains(pattern, "22:30-02:30"):
		return 0, 2
	case strings.Contains(pattern, "23:00-02:00"):
		return 0, 2
	case strings.Contains(pattern, "23:30"):
		return 23, 1
	default:
		return 23, 2
	}
}

func demoStableUnit(parts ...string) float64 {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x1f")))
	value := binary.BigEndian.Uint64(sum[:8])
	return float64(value>>11) / float64(uint64(1)<<53)
}

func demoStableIndex(n int, parts ...string) int {
	if n <= 1 {
		return 0
	}
	return int(demoStableUnit(parts...) * float64(n))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

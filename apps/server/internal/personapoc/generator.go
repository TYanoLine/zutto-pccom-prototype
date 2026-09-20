package personapoc

import (
	"fmt"
	"math"
	"math/rand"
	"sort"
	"strings"
	"time"
)

type HostProfile struct {
	ID      string             `json:"id"`
	Label   string             `json:"label"`
	Weights map[string]float64 `json:"weights"`
}

type Identity struct {
	ID                 string             `json:"id"`
	AccountID          string             `json:"account_id"`
	Handle             string             `json:"handle"`
	Age                int                `json:"age"`
	Gender             string             `json:"gender"`
	Occupation         string             `json:"occupation"`
	ActivityClass      string             `json:"activity_class"`
	VisitDaysPerWeek   float64            `json:"visit_days_per_week"`
	LurkerBias         float64            `json:"lurker_bias"`
	WriteBias          float64            `json:"write_bias"`
	ReplyBias          float64            `json:"reply_bias"`
	ThreadStartBias    float64            `json:"thread_start_bias"`
	Interests          map[string]float64 `json:"interests"`
	TopInterests       []string           `json:"top_interests"`
	StyleTags          []string           `json:"style_tags"`
	ConnectWindow      string             `json:"connect_window"`
	Quirk              string             `json:"quirk"`
	HostFit            float64            `json:"host_fit"`
	MembershipSource   string             `json:"membership_source"`
	DetailTier         string             `json:"detail_tier"`
	ProfileSummary     string             `json:"profile_summary"`
}

type Timing struct {
	PoolGenerationUS int64 `json:"pool_generation_us"`
	SelectionUS      int64 `json:"selection_us"`
	FormattingUS     int64 `json:"formatting_us"`
	QualityUS        int64 `json:"quality_us"`
	TotalUS          int64 `json:"total_us"`
}

type Check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Value  string `json:"value"`
	Note   string `json:"note"`
}

type Quality struct {
	UniqueAccountIDRatio float64        `json:"unique_account_id_ratio"`
	UniqueHandleRatio    float64        `json:"unique_handle_ratio"`
	ExactCloneRatio      float64        `json:"exact_clone_ratio"`
	AverageHostFit       float64        `json:"average_host_fit"`
	WildcardRatio        float64        `json:"wildcard_ratio"`
	AverageAge           float64        `json:"average_age"`
	AgeMin               int            `json:"age_min"`
	AgeMax               int            `json:"age_max"`
	OccupationKinds      int            `json:"occupation_kinds"`
	StyleSignatureKinds  int            `json:"style_signature_kinds"`
	FutureTermHits       int            `json:"future_term_hits"`
	AgeOccupationWarnings int           `json:"age_occupation_warnings"`
	ActivityDistribution map[string]int `json:"activity_distribution"`
	OccupationDistribution map[string]int `json:"occupation_distribution"`
	TopInterestDistribution map[string]int `json:"top_interest_distribution"`
	Checks               []Check        `json:"checks"`
}

type Result struct {
	Seed             int64         `json:"seed"`
	Count            int           `json:"count"`
	PoolSize         int           `json:"pool_size"`
	Profile          HostProfile   `json:"profile"`
	AccountIDScheme  string        `json:"account_id_scheme"`
	AccountIDPrefix  string        `json:"account_id_prefix"`
	Timing        Timing        `json:"timing"`
	Quality       Quality       `json:"quality"`
	Personas      []Identity    `json:"personas"`
	GeneratorNote string        `json:"generator_note"`
}

type BenchmarkRow struct {
	Count    int    `json:"count"`
	PoolSize int    `json:"pool_size"`
	TotalUS  int64  `json:"total_us"`
	PerPersonNS int64 `json:"per_person_ns"`
}

var profiles = []HostProfile{
	{ID: "general", Label: "総合・雑談", Weights: map[string]float64{"communications": .18, "software": .12, "games": .15, "music": .13, "local": .20, "chat": .22}},
	{ID: "tech", Label: "通信・技術", Weights: map[string]float64{"communications": .29, "modem": .24, "software": .24, "files": .13, "games": .05, "chat": .05}},
	{ID: "games", Label: "ゲーム中心", Weights: map[string]float64{"games": .42, "software": .15, "communications": .10, "music": .08, "chat": .15, "local": .10}},
	{ID: "local", Label: "地域・交流", Weights: map[string]float64{"local": .36, "chat": .27, "music": .10, "games": .09, "communications": .10, "software": .08}},
	{ID: "music", Label: "音楽・趣味", Weights: map[string]float64{"music": .43, "chat": .18, "local": .12, "games": .08, "communications": .09, "software": .10}},
}

func Profiles() []HostProfile {
	out := make([]HostProfile, len(profiles))
	copy(out, profiles)
	return out
}

func Profile(id string) HostProfile {
	for _, p := range profiles {
		if p.ID == id {
			return p
		}
	}
	return profiles[0]
}

func Generate(seed int64, count int, profileID string, includePersonas bool) Result {
	if count < 1 {
		count = 1
	}
	if count > 2000 {
		count = 2000
	}
	if seed == 0 {
		seed = 19960826
	}
	profile := Profile(profileID)
	rng := rand.New(rand.NewSource(seed))
	started := time.Now()

	poolStarted := time.Now()
	poolSize := count * 5
	if poolSize < 120 {
		poolSize = 120
	}
	if poolSize > 8000 {
		poolSize = 8000
	}
	pool := make([]Identity, 0, poolSize)
	handles := map[string]int{}
	for i := 0; i < poolSize; i++ {
		p := generateIdentity(rng, i, handles)
		p.HostFit = hostFit(p.Interests, profile)
		pool = append(pool, p)
	}
	poolUS := time.Since(poolStarted).Microseconds()

	selectionStarted := time.Now()
	selected := selectMembers(rng, pool, count)
	accountIDPrefix, accountIDScheme := assignAccountIDs(selected, seed)
	assignDetailTiers(selected)
	selectionUS := time.Since(selectionStarted).Microseconds()

	formatStarted := time.Now()
	for i := range selected {
		selected[i].ProfileSummary = formatProfile(selected[i])
	}
	formatUS := time.Since(formatStarted).Microseconds()

	qualityStarted := time.Now()
	quality := evaluateQuality(selected)
	qualityUS := time.Since(qualityStarted).Microseconds()

	personas := selected
	if !includePersonas {
		personas = nil
	}
	return Result{
		Seed: seed, Count: count, PoolSize: poolSize, Profile: profile,
		AccountIDScheme: accountIDScheme, AccountIDPrefix: accountIDPrefix, Personas: personas,
		Timing: Timing{
			PoolGenerationUS: poolUS,
			SelectionUS: selectionUS,
			FormattingUS: formatUS,
			QualityUS: qualityUS,
			TotalUS: time.Since(started).Microseconds(),
		},
		Quality: quality,
		GeneratorNote: "PoC heuristic generator: no LLM/API calls. ID is an internal world key; account_id is a host-local fictional membership ID using a period-inspired station-prefix scheme. Handle shapes and collision variants are based on observed 1990s Japanese BBS conventions, but are not claimed as one host program's exact algorithm.",
	}
}

func Benchmark(seed int64, profileID string, counts []int) []BenchmarkRow {
	out := make([]BenchmarkRow, 0, len(counts))
	for i, count := range counts {
		result := Generate(seed+int64(i)*7919, count, profileID, false)
		per := int64(0)
		if count > 0 {
			per = result.Timing.TotalUS * 1000 / int64(count)
		}
		out = append(out, BenchmarkRow{Count: count, PoolSize: result.PoolSize, TotalUS: result.Timing.TotalUS, PerPersonNS: per})
	}
	return out
}

func generateIdentity(rng *rand.Rand, n int, handles map[string]int) Identity {
	age := randomAge(rng)
	gender := "male"
	if rng.Float64() < .31 {
		gender = "female"
	}
	occupation := randomOccupation(rng, age, gender)

	tech := betaish(rng)
	social := betaish(rng)
	entertainment := betaish(rng)
	locality := betaish(rng)
	interests := map[string]float64{
		"communications": clamp(.68*tech + .20*rng.Float64()),
		"modem":          clamp(.76*tech + .12*rng.Float64()),
		"software":       clamp(.66*tech + .20*rng.Float64()),
		"files":          clamp(.48*tech + .24*rng.Float64()),
		"games":          clamp(.62*entertainment + .12*tech + .18*rng.Float64()),
		"music":          clamp(.64*entertainment + .20*rng.Float64()),
		"local":          clamp(.68*locality + .22*rng.Float64()),
		"chat":           clamp(.68*social + .18*rng.Float64()),
	}
	top := topInterests(interests, 2)

	class := randomActivityClass(rng)
	visit := visitDaysForClass(rng, class)
	lurker := clamp(.12 + .64*rng.Float64())
	if class == "lurker" {
		lurker = clamp(.68 + .26*rng.Float64())
	}
	if class == "regular" {
		lurker = clamp(.05 + .28*rng.Float64())
	}
	write := clamp((1-lurker)*(.28+.56*rng.Float64()))
	reply := clamp((.25+.60*social)*(1-.30*lurker) + .12*rng.Float64())
	start := clamp((.16+.50*rng.Float64())*(1-.35*lurker))

	formal := rng.Float64()
	verbose := clamp(.20 + .54*tech + .35*rng.Float64())
	emoticon := clamp(.08 + .52*social + .20*rng.Float64())
	quote := clamp(.10 + .62*tech + .16*rng.Float64())
	style := makeStyleTags(formal, verbose, emoticon, quote)
	handle := uniqueHandle(rng, handles)

	return Identity{
		ID: fmt.Sprintf("P%05d", n+1),
		Handle: handle,
		Age: age,
		Gender: gender,
		Occupation: occupation,
		ActivityClass: class,
		VisitDaysPerWeek: round2(visit),
		LurkerBias: round2(lurker),
		WriteBias: round2(write),
		ReplyBias: round2(reply),
		ThreadStartBias: round2(start),
		Interests: interests,
		TopInterests: top,
		StyleTags: style,
		ConnectWindow: connectWindow(rng, age, occupation),
		Quirk: randomQuirk(rng, tech, social),
	}
}

func selectMembers(rng *rand.Rand, pool []Identity, count int) []Identity {
	if count >= len(pool) {
		out := append([]Identity(nil), pool...)
		for i := range out {
			out[i].MembershipSource = "pool"
		}
		return out
	}
	used := make([]bool, len(pool))
	out := make([]Identity, 0, count)

	// 75% are strongly selected by host fit, 15% are moderately selected, and
	// 10% are deliberately broad/wildcard so the BBS never becomes a caricature.
	topCount := int(math.Round(float64(count) * .75))
	midCount := int(math.Round(float64(count) * .15))
	if topCount+midCount > count {
		midCount = count - topCount
	}
	wildCount := count - topCount - midCount

	type scored struct{ idx int; score float64 }
	scores := make([]scored, 0, len(pool))
	for i := range pool {
		scores = append(scores, scored{i, .78*pool[i].HostFit + .22*rng.Float64()})
	}
	sort.Slice(scores, func(i, j int) bool { return scores[i].score > scores[j].score })
	for _, item := range scores {
		if len(out) >= topCount {
			break
		}
		used[item.idx] = true
		p := pool[item.idx]
		p.MembershipSource = "host-fit"
		out = append(out, p)
	}

	middle := make([]int, 0)
	for i := len(pool)/5; i < len(pool)*4/5; i++ {
		idx := scores[i].idx
		if !used[idx] {
			middle = append(middle, idx)
		}
	}
	rng.Shuffle(len(middle), func(i, j int) { middle[i], middle[j] = middle[j], middle[i] })
	for _, idx := range middle {
		if midCount <= 0 {
			break
		}
		used[idx] = true
		p := pool[idx]
		p.MembershipSource = "diversity"
		out = append(out, p)
		midCount--
	}

	remaining := make([]int, 0)
	for i := range pool {
		if !used[i] {
			remaining = append(remaining, i)
		}
	}
	rng.Shuffle(len(remaining), func(i, j int) { remaining[i], remaining[j] = remaining[j], remaining[i] })
	for _, idx := range remaining {
		if wildCount <= 0 {
			break
		}
		p := pool[idx]
		p.MembershipSource = "wildcard"
		out = append(out, p)
		wildCount--
	}
	return out
}

func assignDetailTiers(selected []Identity) {
	type ranked struct{ idx int; score float64 }
	rank := make([]ranked, 0, len(selected))
	for i := range selected {
		score := selected[i].VisitDaysPerWeek * (.35 + selected[i].WriteBias + .45*selected[i].ReplyBias)
		rank = append(rank, ranked{i, score})
	}
	sort.Slice(rank, func(i, j int) bool { return rank[i].score > rank[j].score })
	core := int(math.Round(float64(len(selected)) * .10))
	if core < 6 && len(selected) >= 6 {
		core = 6
	}
	if core > 14 {
		core = 14
	}
	active := int(math.Round(float64(len(selected)) * .25))
	for order, r := range rank {
		switch {
		case order < core:
			selected[r.idx].DetailTier = "core-candidate"
		case order < core+active:
			selected[r.idx].DetailTier = "active"
		default:
			selected[r.idx].DetailTier = "identity"
		}
	}
}

func evaluateQuality(personas []Identity) Quality {
	q := Quality{
		AgeMin: 999,
		ActivityDistribution: map[string]int{},
		OccupationDistribution: map[string]int{},
		TopInterestDistribution: map[string]int{},
	}
	if len(personas) == 0 {
		q.AgeMin = 0
		return q
	}
	accountIDs := map[string]bool{}
	handles := map[string]bool{}
	signatures := map[string]int{}
	styleSignatures := map[string]bool{}
	futureTerms := []string{"SNS", "スマホ", "ブログ", "Twitter", "YouTube", "Wi-Fi", "ADSL"}
	hostFitSum := 0.0
	ageSum := 0
	wildcards := 0
	for _, p := range personas {
		if p.AccountID != "" {
			accountIDs[strings.ToLower(p.AccountID)] = true
		}
		handles[strings.ToLower(p.Handle)] = true
		q.ActivityDistribution[p.ActivityClass]++
		q.OccupationDistribution[p.Occupation]++
		if len(p.TopInterests) > 0 {
			q.TopInterestDistribution[p.TopInterests[0]]++
		}
		sig := fmt.Sprintf("%s|%s|%s|%s", p.Occupation, strings.Join(p.TopInterests, ","), p.ActivityClass, strings.Join(p.StyleTags, ","))
		signatures[sig]++
		styleSignatures[strings.Join(p.StyleTags, "|")] = true
		hostFitSum += p.HostFit
		ageSum += p.Age
		if p.Age < q.AgeMin { q.AgeMin = p.Age }
		if p.Age > q.AgeMax { q.AgeMax = p.Age }
		if p.MembershipSource == "wildcard" { wildcards++ }
		if (p.Occupation == "高校生" && p.Age > 19) || (p.Occupation == "大学生" && (p.Age < 18 || p.Age > 30)) {
			q.AgeOccupationWarnings++
		}
		text := p.Handle + " " + p.Occupation + " " + p.ProfileSummary
		for _, term := range futureTerms {
			if strings.Contains(text, term) {
				q.FutureTermHits++
			}
		}
	}
	clones := 0
	for _, n := range signatures {
		if n > 1 {
			clones += n - 1
		}
	}
	q.UniqueAccountIDRatio = float64(len(accountIDs)) / float64(len(personas))
	q.UniqueHandleRatio = float64(len(handles)) / float64(len(personas))
	q.ExactCloneRatio = float64(clones) / float64(len(personas))
	q.AverageHostFit = round3(hostFitSum / float64(len(personas)))
	q.WildcardRatio = round3(float64(wildcards) / float64(len(personas)))
	q.AverageAge = round2(float64(ageSum) / float64(len(personas)))
	q.OccupationKinds = len(q.OccupationDistribution)
	q.StyleSignatureKinds = len(styleSignatures)

	dominantOccupation := dominantShare(q.OccupationDistribution, len(personas))
	q.Checks = []Check{
		{Name: "account id uniqueness", Status: pass(q.UniqueAccountIDRatio == 1), Value: fmt.Sprintf("%.1f%%", q.UniqueAccountIDRatio*100), Note: "局内ログインIDが重複していないか"},
		{Name: "handle uniqueness", Status: pass(q.UniqueHandleRatio == 1), Value: fmt.Sprintf("%.1f%%", q.UniqueHandleRatio*100), Note: "同一局内で表示ハンドルが衝突していないか"},
		{Name: "exact persona clones", Status: pass(q.ExactCloneRatio <= .08), Value: fmt.Sprintf("%.1f%%", q.ExactCloneRatio*100), Note: "職業・興味・活動・文体タグが完全一致する比率"},
		{Name: "occupation diversity", Status: pass(q.OccupationKinds >= minInt(8, maxInt(3, len(personas)/20))), Value: fmt.Sprintf("%d kinds", q.OccupationKinds), Note: "単一職業への偏りを避ける"},
		{Name: "dominant occupation", Status: pass(dominantOccupation <= .38), Value: fmt.Sprintf("%.1f%%", dominantOccupation*100), Note: "最大職業カテゴリの占有率"},
		{Name: "style diversity", Status: pass(q.StyleSignatureKinds >= minInt(12, maxInt(4, len(personas)/10))), Value: fmt.Sprintf("%d signatures", q.StyleSignatureKinds), Note: "文体タグの組み合わせ数"},
		{Name: "wildcard membership", Status: pass(q.WildcardRatio >= .07 && q.WildcardRatio <= .14), Value: fmt.Sprintf("%.1f%%", q.WildcardRatio*100), Note: "局テーマに完全一致しない会員を意図的に残す"},
		{Name: "age/occupation consistency", Status: pass(q.AgeOccupationWarnings == 0), Value: fmt.Sprintf("%d warnings", q.AgeOccupationWarnings), Note: "明白な年齢・職業矛盾"},
		{Name: "future vocabulary guard", Status: pass(q.FutureTermHits == 0), Value: fmt.Sprintf("%d hits", q.FutureTermHits), Note: "プロフィール生成器内の代表的な未来語チェック"},
	}
	return q
}

func formatProfile(p Identity) string {
	activity := map[string]string{
		"regular": "かなり頻繁に顔を出す",
		"active": "週に何度か顔を出す",
		"occasional": "時々接続する",
		"lurker": "接続はするがROMが多い",
		"dormant": "最近はたまにしか来ない",
	}[p.ActivityClass]
	if activity == "" { activity = "時々接続する" }
	style := strings.Join(p.StyleTags, "・")
	interests := strings.Join(p.TopInterests, "、")
	return fmt.Sprintf("%d歳の%s。%s。主な関心は%s。接続は%sが多く、文体は%s。%s", p.Age, p.Occupation, activity, interests, p.ConnectWindow, style, p.Quirk)
}

func hostFit(interests map[string]float64, profile HostProfile) float64 {
	total, weight := 0.0, 0.0
	for key, w := range profile.Weights {
		total += interests[key] * w
		weight += w
	}
	if weight == 0 { return .5 }
	return round3(total / weight)
}

func randomAge(rng *rand.Rand) int {
	x := rng.Float64()
	switch {
	case x < .07: return 15 + rng.Intn(5)
	case x < .28: return 20 + rng.Intn(5)
	case x < .56: return 25 + rng.Intn(5)
	case x < .84: return 30 + rng.Intn(10)
	case x < .96: return 40 + rng.Intn(10)
	default: return 50 + rng.Intn(10)
	}
}

func randomOccupation(rng *rand.Rand, age int, gender string) string {
	if age <= 18 {
		if rng.Float64() < .88 { return "高校生" }
		return "アルバイト"
	}
	if age <= 22 {
		opts := []string{"大学生", "大学生", "大学生", "専門学校生", "短大生", "アルバイト", "会社員"}
		if gender == "male" {
			opts = []string{"大学生", "大学生", "大学生", "専門学校生", "アルバイト", "会社員"}
		}
		return opts[rng.Intn(len(opts))]
	}
	opts := []string{"会社員", "会社員", "会社員", "技術職", "営業職", "事務職", "公務員", "教員", "自営業", "販売・サービス業", "アルバイト"}
	if gender == "female" && rng.Float64() < .13 {
		opts = append(opts, "主婦", "主婦")
	}
	return opts[rng.Intn(len(opts))]
}

func randomActivityClass(rng *rand.Rand) string {
	x := rng.Float64()
	switch {
	case x < .15: return "regular"
	case x < .36: return "active"
	case x < .69: return "occasional"
	case x < .94: return "lurker"
	default: return "dormant"
	}
}

func visitDaysForClass(rng *rand.Rand, class string) float64 {
	switch class {
	case "regular": return 4.2 + 2.2*rng.Float64()
	case "active": return 2.2 + 2.0*rng.Float64()
	case "occasional": return .8 + 1.6*rng.Float64()
	case "lurker": return 1.0 + 2.5*rng.Float64()
	default: return .1 + .6*rng.Float64()
	}
}

func connectWindow(rng *rand.Rand, age int, occupation string) string {
	if occupation == "主婦" && rng.Float64() < .45 {
		return "昼間または22時台"
	}
	if strings.Contains(occupation, "学生") || occupation == "高校生" {
		opts := []string{"21:00-00:30", "22:00-01:00", "23:00前後", "週末の夜"}
		return opts[rng.Intn(len(opts))]
	}
	opts := []string{"22:30-00:30", "23:00-01:00", "23:30-02:00", "0:00前後", "テレホーダイ時間帯中心", "週末の夜"}
	return opts[rng.Intn(len(opts))]
}

func randomQuirk(rng *rand.Rand, tech, social float64) string {
	base := []string{
		"相手のハンドルを文頭で呼ぶことがある。",
		"短い相づちだけで終えることもある。",
		"質問には答えるが雑談には毎回は入らない。",
		"自分から話題を始めるより既存スレッドへ参加しやすい。",
		"書き込みの冒頭に軽い挨拶を入れることがある。",
		"断定より「〜だと思います」を選びやすい。",
	}
	if tech > .62 {
		base = append(base, "技術話では「>」引用を使って要点ごとに返しやすい。", "設定値や手順を具体的に書くことがある。")
	}
	if social > .65 {
		base = append(base, "常連には少し砕けた返しをする。", "初参加者にも比較的返事を付けやすい。")
	}
	return base[rng.Intn(len(base))]
}

func makeStyleTags(formal, verbose, emoticon, quote float64) []string {
	tags := make([]string, 0, 4)
	if verbose > .68 { tags = append(tags, "やや長文") } else if verbose < .38 { tags = append(tags, "短文") } else { tags = append(tags, "中程度") }
	if formal > .68 { tags = append(tags, "丁寧") } else if formal < .32 { tags = append(tags, "くだけた口調") } else { tags = append(tags, "普通口調") }
	if emoticon > .65 { tags = append(tags, "顔文字やや多め") } else if emoticon < .30 { tags = append(tags, "顔文字ほぼ無し") } else { tags = append(tags, "顔文字時々") }
	if quote > .67 { tags = append(tags, "引用多め") } else if quote < .32 { tags = append(tags, "引用少なめ") } else { tags = append(tags, "引用普通") }
	return tags
}

func assignAccountIDs(personas []Identity, seed int64) (string, string) {
	if len(personas) == 0 {
		return "", "grassroots-prefix"
	}
	// Persona Lab has no concrete host identity yet, so it uses a synthetic
	// three-letter station code. Production membership IDs should derive this
	// namespace from the host itself, not from the person.
	rng := rand.New(rand.NewSource(seed ^ int64(0x5a17c0de)))
	prefix := syntheticStationCode(rng)
	width := 4
	scheme := "grassroots-prefix-3+4"
	if rng.Float64() < .22 {
		width = 5
		scheme = "grassroots-prefix-3+5"
	}

	// Real local BBS member lists commonly have gaps because of reserved IDs,
	// deleted members, and time. Draw without replacement instead of assigning
	// the displayed list 0001, 0002, 0003... in output order.
	span := int(math.Ceil(float64(len(personas))*1.35)) + 8
	if span < len(personas)+8 {
		span = len(personas) + 8
	}
	maxForWidth := 1
	for i := 0; i < width; i++ {
		maxForWidth *= 10
	}
	maxForWidth--
	if span > maxForWidth {
		span = maxForWidth
	}
	numbers := rng.Perm(span)
	for i := range personas {
		personas[i].AccountID = fmt.Sprintf("%s%0*d", prefix, width, numbers[i]+1)
	}
	return prefix, scheme
}

func syntheticStationCode(rng *rand.Rand) string {
	const letters = "ABCDEFGHIJKLMNOPQRSTUVWXYZ"
	reserved := map[string]bool{
		"CAN": true, "MMN": true, "NAT": true, "NIF": true, "PCV": true,
		"SYS": true, "NEW": true,
	}
	for {
		code := string([]byte{
			letters[rng.Intn(len(letters))],
			letters[rng.Intn(len(letters))],
			letters[rng.Intn(len(letters))],
		})
		if !reserved[code] {
			return code
		}
	}
}

var (
	handleRoman = []string{"AKI", "AYA", "EMI", "HIDE", "HIRO", "JUN", "KAZU", "KEN", "KOJI", "MAKO", "MARI", "MASA", "MIKI", "NAO", "NORI", "REI", "RYO", "SHIN", "TAKA", "TOMO", "YUKI", "YUJI"}
	handleKana  = []string{"あき", "うさぎ", "かえる", "くま", "たぬき", "ねこ", "ひろ", "ぽち", "まる", "みかん", "もも", "りん"}
	handleKanji = []string{"紫苑", "千里", "弥生", "小鉄", "夕凪", "流星", "北斗", "銀次", "紅葉", "雪兎"}
	handleWord  = []string{"MINT", "WOLF", "RABBIT", "JOKER", "NOVA", "LUNA", "MARU", "KERO"}
	handleTech  = []string{"COM", "N88", "PC", "RX", "V30", "X68", "98"}
)

func randomHandleBase(rng *rand.Rand) string {
	x := rng.Float64()
	switch {
	case x < .54:
		return handleRoman[rng.Intn(len(handleRoman))]
	case x < .72:
		return handleKana[rng.Intn(len(handleKana))]
	case x < .82:
		return handleKanji[rng.Intn(len(handleKanji))]
	case x < .90:
		return handleWord[rng.Intn(len(handleWord))]
	default:
		return handleTech[rng.Intn(len(handleTech))] + "-" + handleRoman[rng.Intn(len(handleRoman))]
	}
}

func uniqueHandle(rng *rand.Rand, used map[string]int) string {
	for attempt := 0; attempt < 40; attempt++ {
		base := randomHandleBase(rng)
		if claimHandle(base, used) {
			return base
		}
		for _, variant := range handleCollisionVariants(rng, base) {
			if claimHandle(variant, used) {
				return variant
			}
		}
	}
	// Extremely large synthetic populations can exhaust the small historically
	// styled vocabulary above. Keep the final escape hatch unique without making
	// "-2, -3, -4..." the normal collision behavior.
	for n := 1; ; n++ {
		candidate := fmt.Sprintf("USER.%c%03d", 'A'+rune(rng.Intn(26)), n)
		if claimHandle(candidate, used) {
			return candidate
		}
	}
}

func claimHandle(candidate string, used map[string]int) bool {
	key := strings.ToLower(strings.TrimSpace(candidate))
	if key == "" || used[key] != 0 {
		return false
	}
	used[key] = 1
	return true
}

func handleCollisionVariants(rng *rand.Rand, base string) []string {
	initial := string(rune('A' + rng.Intn(26)))
	tech := handleTech[rng.Intn(len(handleTech))]
	number := 1 + rng.Intn(99)
	variants := []string{
		base + "." + initial,
		base + "-" + initial,
		tech + "-" + base,
		base + "☆",
		fmt.Sprintf("%s%02d", base, number),
		randomHandleBase(rng),
	}
	rng.Shuffle(len(variants), func(i, j int) {
		variants[i], variants[j] = variants[j], variants[i]
	})
	return variants
}

func topInterests(m map[string]float64, n int) []string {
	type pair struct{ key string; value float64 }
	all := make([]pair, 0, len(m))
	for k, v := range m { all = append(all, pair{k, v}) }
	sort.Slice(all, func(i, j int) bool {
		if all[i].value == all[j].value { return all[i].key < all[j].key }
		return all[i].value > all[j].value
	})
	if n > len(all) { n = len(all) }
	out := make([]string, 0, n)
	for _, p := range all[:n] { out = append(out, p.key) }
	return out
}

func betaish(rng *rand.Rand) float64 {
	return clamp((rng.Float64()+rng.Float64()+rng.Float64())/3)
}
func clamp(v float64) float64 { if v < 0 { return 0 }; if v > 1 { return 1 }; return v }
func round2(v float64) float64 { return math.Round(v*100)/100 }
func round3(v float64) float64 { return math.Round(v*1000)/1000 }
func pass(ok bool) string { if ok { return "pass" }; return "warn" }
func minInt(a,b int) int { if a < b { return a }; return b }
func maxInt(a,b int) int { if a > b { return a }; return b }
func dominantShare(m map[string]int, total int) float64 {
	if total == 0 { return 0 }
	max := 0
	for _, n := range m { if n > max { max = n } }
	return float64(max)/float64(total)
}

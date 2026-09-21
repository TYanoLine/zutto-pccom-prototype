package world

import (
	"fmt"
	"time"
)

const erikaKSeedRootFloor = 40

type erikaKSeedProfile struct {
	style string
	terms []string
}

var erikaKSeedProfiles = map[string]erikaKSeedProfile{
	"1":    {style: "notice", terms: []string{"回線メンテナンス", "局からのお知らせ", "利用時間", "ファイル整理", "運用予定"}},
	"2":    {style: "social", terms: []string{"自己紹介", "新人さん", "ハンドル名", "初アクセス", "みなさんへの挨拶"}},
	"3":    {style: "tech", terms: []string{"モデム設定", "通信ソフト", "接続トラブル", "ボード操作", "ファイル転送"}},
	"4":    {style: "social", terms: []string{"最近の出来事", "今日の雑談", "夜更かし", "週末の予定", "ちょっとした話"}},
	"5":    {style: "social", terms: []string{"オフ会", "待ち合わせ", "参加者募集", "写真交換", "次回の集まり"}},
	"6":    {style: "local", terms: []string{"天神の店", "博多駅周辺", "交通情報", "食事処", "街のイベント"}},
	"7":    {style: "market", terms: []string{"中古モデム", "98用メモリ", "ゲームソフト", "周辺機器", "通信ソフト"}},
	"8":    {style: "creative", terms: []string{"自作ソフト", "イラスト", "音楽データ", "創作の話", "作品募集"}},
	"10/1": {style: "local", terms: []string{"天神", "博多駅", "中洲", "福岡の店", "地元情報"}},
	"10/2": {style: "social", terms: []string{"オフ会", "集合場所", "参加確認", "二次会", "次回予定"}},
	"20/1": {style: "hobby", terms: []string{"ゲーム", "攻略", "対戦", "最近遊んだソフト", "中古ソフト"}},
	"20/2": {style: "hobby", terms: []string{"アニメ", "マンガ", "新刊", "テレビ番組", "同人誌"}},
	"60/1": {style: "tech", terms: []string{"PC-98", "モデム", "回線速度", "通信設定", "V.34"}},
	"60/2": {style: "tech", terms: []string{"Windows", "DOS", "ドライバ", "メモリ", "環境設定"}},
	"60/3": {style: "tech", terms: []string{"通信ソフト", "フリーソフト", "圧縮ツール", "ISH", "ユーティリティ"}},
	"68/1": {style: "social", terms: []string{"深夜組", "夜中の雑談", "テレホタイム", "眠れない夜", "今だれかいる？"}},
	"70/1": {style: "tech", terms: []string{"PC-98", "9821", "メモリ増設", "周辺機器", "Windows 95"}},
	"70/2": {style: "tech", terms: []string{"DOS/V", "AT互換機", "自作PC", "VGA", "IDE"}},
	"80/1": {style: "tech", terms: []string{"FM TOWNS", "TOWNS OS", "CD-ROM", "ゲーム", "周辺機器"}},
	"80/2": {style: "tech", terms: []string{"MSX", "turbo R", "MSX-DOS", "ゲーム", "FDD"}},
	"80/3": {style: "tech", terms: []string{"ワープロ専用機", "文書交換", "印刷", "通信機能", "フロッピー"}},
	"80/4": {style: "tech", terms: []string{"PC-8801", "FMR", "X68000", "旧機種", "周辺機器"}},
	"99":   {style: "social", terms: []string{"夜更かし", "深夜の独り言", "今夜のメンバー", "眠気", "朝まで雑談"}},
}

var erikaKSeedAuthors = []string{
	"MARI", "YUKI", "NORI", "KAZU", "TAKU", "NEKO", "KEN", "MAKO", "TOMO", "AKI", "RYO", "HIRO", "SACHI", "JUN",
}

func seedErikaKBoardHeaders(s *MemoryStore, hostID string) {
	maxID := s.next
	for _, posts := range s.posts {
		for _, post := range posts {
			if post.ID > maxID {
				maxID = post.ID
			}
		}
	}

	base := time.Date(1996, 8, 26, 1, 30, 0, 0, time.Local)
	boardOffset := 0
	for boardID, profile := range erikaKSeedProfiles {
		rootCount := 0
		for _, post := range s.posts[hostID] {
			if post.BoardID == boardID && post.ParentID == 0 {
				rootCount++
			}
		}
		for rootCount < erikaKSeedRootFloor {
			slot := rootCount
			term := profile.terms[slot%len(profile.terms)]
			variant := (slot / len(profile.terms)) % 8
			maxID++
			author := erikaKSeedAuthors[(slot+boardOffset)%len(erikaKSeedAuthors)]
			if boardID == "1" && slot%3 != 0 {
				author = "SYSOP"
			}
			created := base.Add(-time.Duration((slot*7+boardOffset%11)+3) * time.Hour)
			s.posts[hostID] = append(s.posts[hostID], Post{
				ID:        maxID,
				BoardID:   boardID,
				Author:    author,
				Subject:   erikaKSeedSubject(profile.style, term, variant),
				CreatedAt: created,
				Intent: PostIntent{
					Topic:      term,
					Motivation: "掲示板上で自然に話題を共有する",
					Goal:       "同じ局の利用者から反応や情報を得る",
				},
			})
			rootCount++
		}
		boardOffset++
	}
	s.next = maxID
}

func erikaKSeedSubject(style, term string, variant int) string {
	switch style {
	case "notice":
		patterns := []string{"%sのお知らせ", "%sについて", "%sの予定", "%s変更", "%s確認", "%sのお願い", "%s補足", "%s続報"}
		return fmt.Sprintf(patterns[variant%len(patterns)], term)
	case "market":
		patterns := []string{"%s売ります", "%s買います", "%s交換希望", "%s探しています", "%s譲ります", "%sの相場は？", "%sについて質問", "%s情報ください"}
		return fmt.Sprintf(patterns[variant%len(patterns)], term)
	case "local":
		patterns := []string{"%sの話", "%s情報ありますか？", "%sでおすすめ", "%s最近どうですか", "%sについて質問", "%sの近況", "%sで見つけました", "%s雑談"}
		return fmt.Sprintf(patterns[variant%len(patterns)], term)
	case "creative":
		patterns := []string{"%s作ってます", "%s見てください", "%sについて", "%sのアイデア", "%s募集", "%sの感想", "%s途中経過", "%s雑談"}
		return fmt.Sprintf(patterns[variant%len(patterns)], term)
	case "hobby":
		patterns := []string{"%sの話", "%sどうですか？", "%sで質問", "%s情報ください", "%s最近やってます", "%sの感想", "%sおすすめあります？", "%s雑談"}
		return fmt.Sprintf(patterns[variant%len(patterns)], term)
	case "tech":
		patterns := []string{"%sについて", "%sで質問", "%s設定の話", "%s使ってる人います？", "%s情報ください", "%sを試してみました", "%sで困ってます", "%s雑談"}
		return fmt.Sprintf(patterns[variant%len(patterns)], term)
	default:
		patterns := []string{"%sの話", "%sどうですか？", "%sで質問", "%s情報ください", "%s最近どうですか", "%sの近況", "%sについて", "%s雑談"}
		return fmt.Sprintf(patterns[variant%len(patterns)], term)
	}
}

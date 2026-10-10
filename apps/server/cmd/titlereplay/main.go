// Command titlereplay re-words the baseline's events with the current title
// generation and writes JSONL that cmd/titlestats can read. It needs a real
// provider (AZURE_OPENAI_ENDPOINT, AZURE_OPENAI_API_KEY, AZURE_OPENAI_MODEL);
// --fake only exercises the plumbing and its output proves nothing.
//
// Differences from production, by construction of the baseline: no
// persona_profile, no board scope or readable board name (the baseline has only
// the board id), and the covered-subject list is what this replay has produced
// so far, not the board's real history.
package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"zutto-pccom/apps/server/internal/llm"
	"zutto-pccom/apps/server/internal/titleshape"
)

const chunkSize = 20 // worldrepo.productionTitleChunkSize

type baselineRow struct {
	Host    string   `json:"host"`
	Board   string   `json:"board"`
	PostID  int64    `json:"post_id"`
	Created string   `json:"created_at"`
	Subject string   `json:"subject"`
	Summary string   `json:"situation_summary"`
	Facts   []string `json:"situation_facts"`
}

type outRow struct {
	Run     int    `json:"run"`
	Host    string `json:"host"`
	Board   string `json:"board"`
	PostID  int64  `json:"post_id"`
	Subject string `json:"subject"`
	Summary string `json:"situation_summary"`
}

func readRows(path string) ([]baselineRow, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	var rows []baselineRow
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var r baselineRow
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			return nil, err
		}
		rows = append(rows, r)
	}
	return rows, sc.Err()
}

// replay words every board's events once and returns the new rows.
func replay(ctx context.Context, planner llm.BBSSituationTitlePlanner, rows []baselineRow, run int, variants bool) ([]outRow, error) {
	byBoard := map[string][]baselineRow{}
	for _, r := range rows {
		byBoard[r.Board] = append(byBoard[r.Board], r)
	}
	boards := make([]string, 0, len(byBoard))
	for b := range byBoard {
		boards = append(boards, b)
	}
	sort.Strings(boards)
	var out []outRow
	for _, board := range boards {
		events := byBoard[board]
		var taken []string
		for start := 0; start < len(events); start += chunkSize {
			end := min(start+chunkSize, len(events))
			var seeds []llm.BBSSituationTitleSeed
			for i, r := range events[start:end] {
				seeds = append(seeds, llm.BBSSituationTitleSeed{
					EventID: fmt.Sprintf("e%d", start+i), AuthorHandle: "", CreatedAt: r.Created,
					SituationKind: "open_topic", SituationSummary: r.Summary, SituationFacts: r.Facts,
				})
			}
			recent := taken
			if len(recent) > 20 {
				recent = recent[len(recent)-20:]
			}
			seed := fmt.Sprintf("replay|%s|%d|%d", board, start/chunkSize, run)
			draft, err := planner.GenerateBBSSituationTitles(ctx, llm.BBSSituationTitleRequest{
				HostName: events[0].Host, BoardID: board, BoardName: board, WorldDate: "1996",
				RecentSubjects: titleshape.ShuffleSeeded(seed, recent), Articles: seeds,
				FormSeed: seed, RecentFormFacts: titleshape.FormFacts(recent), MultiVariant: variants,
			})
			if err != nil {
				return nil, fmt.Errorf("board %s chunk %d: %w", board, start/chunkSize, err)
			}
			titles := map[string]llm.BBSSituationTitle{}
			for _, t := range draft.Titles {
				titles[t.EventID] = t
			}
			for i, r := range events[start:end] {
				t, ok := titles[fmt.Sprintf("e%d", start+i)]
				if !ok {
					return nil, fmt.Errorf("board %s: no title for event %d", board, start+i)
				}
				subject := t.Subject
				if len(t.Candidates) > 0 {
					subject = t.Candidates[titleshape.PickVariant(seed+"|"+t.EventID, t.Candidates, taken)]
				}
				taken = append(taken, subject)
				out = append(out, outRow{Run: run, Host: r.Host, Board: board, PostID: r.PostID, Subject: subject, Summary: r.Summary})
			}
		}
	}
	return out, nil
}

// fakePlanner only checks the plumbing.
type fakePlanner struct{}

func (fakePlanner) GenerateBBSSituationTitles(_ context.Context, req llm.BBSSituationTitleRequest) (llm.BBSSituationTitleDraft, error) {
	var d llm.BBSSituationTitleDraft
	for _, a := range req.Articles {
		rs := []rune(a.SituationSummary)
		if len(rs) > 12 {
			rs = rs[:12]
		}
		d.Titles = append(d.Titles, llm.BBSSituationTitle{EventID: a.EventID, Subject: string(rs)})
	}
	return d, nil
}

func main() {
	runs := flag.Int("runs", 3, "number of replays")
	fake := flag.Bool("fake", false, "use a fake provider (plumbing only; proves nothing about quality)")
	variants := flag.Bool("variants", false, "ask for several variants per article and pick one deterministically")
	flag.Parse()
	if flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: titlereplay [--runs N] [--fake] [--variants] baseline.jsonl")
		os.Exit(2)
	}
	rows, err := readRows(flag.Arg(0))
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var planner llm.BBSSituationTitlePlanner = fakePlanner{}
	if !*fake {
		endpoint, key := os.Getenv("AZURE_OPENAI_ENDPOINT"), os.Getenv("AZURE_OPENAI_API_KEY")
		if endpoint == "" || key == "" {
			fmt.Fprintln(os.Stderr, "titlereplay: AZURE_OPENAI_ENDPOINT and AZURE_OPENAI_API_KEY are required (use --fake only to test plumbing)")
			os.Exit(1)
		}
		model := os.Getenv("AZURE_OPENAI_MODEL")
		if model == "" {
			model = "zutto-pccom-gpt-6-luna"
		}
		planner = llm.StructuredOpenAIProvider{OpenAIProvider: llm.OpenAIProvider{
			Endpoint: endpoint, APIKey: key, Model: model, Client: &http.Client{Timeout: 90 * time.Second},
		}}
	}
	enc := json.NewEncoder(os.Stdout)
	for run := 1; run <= *runs; run++ {
		out, err := replay(context.Background(), planner, rows, run, *variants)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		for _, r := range out {
			_ = enc.Encode(r)
		}
	}
}

// Command titlestats prints the shape of BBS root titles per board, from the
// JSONL that `BBS generated content` logs (or cmd/titlereplay) produce.
package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"zutto-pccom/apps/server/internal/titleshape"
)

type row struct {
	Board   string `json:"board"`
	Subject string `json:"subject"`
	Summary string `json:"situation_summary"`
}

func main() {
	asJSON := flag.Bool("json", false, "print JSON")
	places := flag.String("places", "", "comma-separated place words that count as a place lead")
	flag.Parse()
	var in io.Reader = os.Stdin
	if flag.NArg() > 0 {
		f, err := os.Open(flag.Arg(0))
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer f.Close()
		in = f
	}
	byBoard := map[string][]titleshape.Item{}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 1<<24)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == "" {
			continue
		}
		var r row
		if err := json.Unmarshal(sc.Bytes(), &r); err != nil {
			fmt.Fprintln(os.Stderr, "bad line:", err)
			os.Exit(1)
		}
		byBoard[r.Board] = append(byBoard[r.Board], titleshape.Item{Subject: r.Subject, Summary: r.Summary})
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	var placeWords []string
	for _, p := range strings.Split(*places, ",") {
		if p = strings.TrimSpace(p); p != "" {
			placeWords = append(placeWords, p)
		}
	}
	boards := make([]string, 0, len(byBoard))
	for b := range byBoard {
		boards = append(boards, b)
	}
	sort.Strings(boards)
	reports := map[string]titleshape.Report{}
	for _, b := range boards {
		reports[b] = titleshape.Measure(byBoard[b], placeWords)
	}
	if *asJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(reports)
		return
	}
	fmt.Printf("%-6s %4s %7s %7s %7s %7s %9s %5s %5s %11s %9s  %s\n",
		"board", "n", "referent", "place", "handle", "other", "sfx-max", "dup", "near", "len(min/med/max)", "retention", "top suffixes")
	for _, b := range boards {
		r := reports[b]
		var top []string
		for _, s := range r.SuffixTop {
			if s.Count > 1 {
				top = append(top, fmt.Sprintf("%s×%d", s.Suffix, s.Count))
			}
		}
		fmt.Printf("%-6s %4d %7.0f%% %6.0f%% %6.0f%% %6.0f%% %8.0f%% %5d %5d %5d/%d/%d %5.0f%%(%d)  %s\n",
			b, r.N, 100*r.LeadRates[titleshape.LeadReferent], 100*r.LeadRates[titleshape.LeadPlace],
			100*r.LeadRates[titleshape.LeadHandle], 100*r.LeadRates[titleshape.LeadOther],
			100*r.SuffixMaxShare, r.ExactDuplicateTitles, r.NearDuplicatePairs,
			r.LenMin, r.LenMedian, r.LenMax, 100*r.RetentionRate, r.RetentionN, strings.Join(top, " "))
	}
}

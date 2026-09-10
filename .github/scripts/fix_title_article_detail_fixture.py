from pathlib import Path
p = Path('apps/server/internal/llm/bbs_title_candidates_test.go')
s = p.read_text()
old = '{Candidate: 2, EventID: "b", Subject: req.Titles[1], Reason: "原文維持", Summary: "NINTENDO64を触った"},'
new = '{Candidate: 2, EventID: "b", Subject: req.Titles[1], Reason: "原文維持", Summary: "NINTENDO64を触った", Details: []string{"操作した入力に画面が反応した", "直前に触った機種と操作感の違いを感じた"}},'
if old not in s:
    raise SystemExit('rewrite-test fixture pattern not found')
p.write_text(s.replace(old, new, 1))

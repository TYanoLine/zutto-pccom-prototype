package llm

import "testing"

func TestValidateBoardPostDraft(t *testing.T){
	if err:=validateBoardPostDraft(BoardPostDraft{Author:"NORI96",Subject:"通信ソフトの設定",Body:"最近設定をいじっています(^^;"});err!=nil{t.Fatalf("valid draft rejected: %v",err)}
	if err:=validateBoardPostDraft(BoardPostDraft{Author:"日本語",Subject:"test",Body:"body"});err==nil{t.Fatal("non-ASCII handle accepted")}
	if err:=validateBoardPostDraft(BoardPostDraft{Author:"NORI",Subject:"スマホの話",Body:"body"});err==nil{t.Fatal("future term accepted")}
}

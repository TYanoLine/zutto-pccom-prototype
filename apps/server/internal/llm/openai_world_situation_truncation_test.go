package llm

import (
    "context"
    "encoding/json"
    "errors"
    "io"
    "net/http"
    "strings"
    "testing"
)

func TestBBSWorldSituationDistinguishesOutputBudgetTruncation(t *testing.T) {
    for _, tc := range []struct{name string; atLimit bool}{
        {name:"full output-token budget",atLimit:true},
        {name:"invalid JSON without exhausted budget",atLimit:false},
    } {
        t.Run(tc.name,func(t *testing.T){
            provider:=StructuredOpenAIProvider{OpenAIProvider:OpenAIProvider{
                Endpoint:"https://test.openai.azure.com", APIKey:"test-key", Model:"gpt-test",
                Client:&http.Client{Transport:generationContextRoundTripFunc(func(req *http.Request)(*http.Response,error){
                    var input struct{ MaxOutputTokens int `json:"max_output_tokens"` }
                    if err:=json.NewDecoder(req.Body).Decode(&input);err!=nil{return nil,err}
                    used:=input.MaxOutputTokens
                    if !tc.atLimit {used--}
                    body,err:=json.Marshal(map[string]any{
                        "model":"gpt-test",
                        "output":[]any{map[string]any{"content":[]any{map[string]any{"type":"output_text","text":`{"situations":{"slot-1":{"object_class":"game"`}}}},
                        "usage":map[string]any{"input_tokens":50,"output_tokens":used,"total_tokens":50+used},
                    })
                    if err!=nil{return nil,err}
                    return &http.Response{StatusCode:http.StatusOK,Header:make(http.Header),Body:io.NopCloser(strings.NewReader(string(body)))},nil
                })},
            }}
            _,err:=provider.GenerateBBSWorldSituationProposals(context.Background(),BBSWorldSituationProposalRequest{
                HostName:"HAKATA",WorldDate:"1996-02-17",
                Events:[]BBSWorldWindowEvent{{EventID:"slot-1",Action:"thread_start"}},
            })
            if err==nil || !strings.Contains(err.Error(),"decode BBS world-situation JSON"){
                t.Fatalf("missing actionable decode error: %v",err)
            }
            if errors.Is(err,ErrBBSWorldSituationOutputTruncated)!=tc.atLimit{
                t.Fatalf("incorrect budget classification atLimit=%t: %v",tc.atLimit,err)
            }
        })
    }
}

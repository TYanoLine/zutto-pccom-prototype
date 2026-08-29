package historicalkb

import (
	"context"
	"fmt"
	"strings"
	"time"
)

type Service struct { Store *Store; Researcher Researcher; WorldDate string; Now func() time.Time }

func (s Service) Create(ctx context.Context,topic,question string)(ResearchCase,error){now:=s.now();c:=ResearchCase{ID:NewCaseID(now),Topic:strings.TrimSpace(topic),Question:strings.TrimSpace(question),WorldDate:s.WorldDate,Status:StatusNeedsReview,CreatedAt:now,UpdatedAt:now};if c.Topic==""||c.Question==""{return ResearchCase{},fmt.Errorf("topic and question are required")};result,err:=s.Researcher.Research(ctx,c.Topic,c.Question,c.WorldDate,"");if err!=nil{c.Summary="自動調査を完了できませんでした。運営レビューが必要です。";c.MissingInfo=[]string{err.Error()};c.Messages=[]Message{{Role:"assistant",Body:c.Summary,CreatedAt:now}}}else{s.applyResult(&c,result);c.Messages=[]Message{{Role:"assistant",Body:result.Summary+"\n\n暫定結論:\n"+result.ProvisionalAnswer,CreatedAt:now}}};if err:=s.Store.Upsert(ctx,c);err!=nil{return ResearchCase{},err};return c,nil}

func (s Service) Chat(ctx context.Context,id,operatorMessage string)(ResearchCase,error){c,err:=s.Store.Get(ctx,id);if err!=nil{return ResearchCase{},err};operatorMessage=strings.TrimSpace(operatorMessage);if operatorMessage==""{return ResearchCase{},fmt.Errorf("message is required")};now:=s.now();c.Messages=append(c.Messages,Message{Role:"operator",Body:operatorMessage,CreatedAt:now});prior:=fmt.Sprintf("Current summary: %s\nCurrent provisional answer: %s\nMissing: %s\nOperator instruction: %s",c.Summary,c.ProvisionalAnswer,strings.Join(c.MissingInfo,"; "),operatorMessage);result,researchErr:=s.Researcher.Research(ctx,c.Topic,c.Question,c.WorldDate,prior);if researchErr!=nil{c.Messages=append(c.Messages,Message{Role:"assistant",Body:"再調査できませんでした: "+researchErr.Error(),CreatedAt:now})}else{s.applyResult(&c,result);c.Messages=append(c.Messages,Message{Role:"assistant",Body:result.Summary+"\n\n更新した暫定結論:\n"+result.ProvisionalAnswer,CreatedAt:now});_ = s.Store.PromoteFactsForResearch(ctx,id,result.ProvisionalAnswer,FactProvisional)};c.UpdatedAt=now;if err:=s.Store.Upsert(ctx,c);err!=nil{return ResearchCase{},err};return c,nil}

func (s Service) OperatorSupplement(ctx context.Context,id,note string)(ResearchCase,error){c,err:=s.Store.Get(ctx,id);if err!=nil{return ResearchCase{},err};note=strings.TrimSpace(note);if note==""{return ResearchCase{},fmt.Errorf("supplement is required")};now:=s.now();c.ProvisionalAnswer=note;c.Status=StatusOperatorVerified;c.Confidence=1;c.Messages=append(c.Messages,Message{Role:"operator",Body:"[運営補完]\n"+note,CreatedAt:now});c.UpdatedAt=now;if err:=s.Store.Upsert(ctx,c);err!=nil{return ResearchCase{},err};if err:=s.Store.PromoteFactsForResearch(ctx,id,note,FactOperatorVerified);err!=nil{return ResearchCase{},err};return c,nil}

func (s Service) SetStatus(ctx context.Context,id string,status Status)(ResearchCase,error){c,err:=s.Store.SetStatus(ctx,id,status);if err!=nil{return ResearchCase{},err};switch status{case StatusCanonical:err=s.Store.PromoteFactsForResearch(ctx,id,"",FactCanonical);case StatusRejected:err=s.Store.PromoteFactsForResearch(ctx,id,"",FactRejected);case StatusOperatorVerified:err=s.Store.PromoteFactsForResearch(ctx,id,"",FactOperatorVerified)};if err!=nil{return ResearchCase{},err};return c,nil}

func (s Service) applyResult(c *ResearchCase,r ResearchResult){c.Summary=r.Summary;c.ProvisionalAnswer=r.ProvisionalAnswer;c.MissingInfo=r.MissingInfo;c.Confidence=r.Confidence;c.Sources=r.Sources;if len(r.MissingInfo)>0||r.Confidence<0.8{c.Status=StatusNeedsReview}else{c.Status=StatusProvisional}}
func (s Service) now()time.Time{if s.Now!=nil{return s.Now()};return time.Now().UTC()}

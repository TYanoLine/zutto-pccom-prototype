package main

import (
 "context"
 "crypto/subtle"
 "encoding/json"
 "log"
 "net/http"
 "time"
 "zutto-pccom/apps/server/internal/config"
 "zutto-pccom/apps/server/internal/llm"
 "zutto-pccom/apps/server/internal/telephone"
 "zutto-pccom/apps/server/internal/world"
 "zutto-pccom/apps/server/internal/worldcatalog"
 "zutto-pccom/apps/server/internal/worldclock"
 wsserver "zutto-pccom/apps/server/internal/ws"
)
const generatedCenterCount=100
func main(){cfg:=config.Load();store:=world.NewMemoryStore();jst,err:=time.LoadLocation("Asia/Tokyo");if err!=nil{log.Fatal(err)};clock,err:=worldclock.New(cfg.WorldDate,jst);if err!=nil{log.Fatal(err)};network:=telephone.New(store,clock);sessions:=wsserver.NewSessionManager(wsserver.DefaultReconnectGrace);generator:=llm.CenterCatalogGenerator{APIKey:cfg.OpenAIKey,Model:cfg.OpenAIModel};var catalogStore *worldcatalog.Store;if cfg.DatabaseURL!=""{ctx,cancel:=context.WithTimeout(context.Background(),15*time.Second);catalogStore,err=worldcatalog.Open(ctx,cfg.DatabaseURL);if err==nil{err=catalogStore.EnsureSchema(ctx)};cancel();if err!=nil{log.Fatalf("initialize persistent world catalog: %v",err)};defer catalogStore.Close()}
 generateNames:=func(ctx context.Context,count int)([]string,error){g,err:=generator.Generate(ctx,count,cfg.WorldDate);if err!=nil{return nil,err};n:=make([]string,len(g));for i:=range g{n[i]=g[i].Name};return n,nil}
 authorized:=func(r *http.Request)bool{if cfg.DebugResetToken==""{return false};got:=r.Header.Get("X-Zutto-Debug-Token");return len(got)==len(cfg.DebugResetToken)&&subtle.ConstantTimeCompare([]byte(got),[]byte(cfg.DebugResetToken))==1}
 bootstrap:=func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");if catalogStore==nil{http.Error(w,`{"error":"persistent world database is not configured"}`,503);return};key:=r.URL.Query().Get("key");if !worldcatalog.ValidWorldKey(key){http.Error(w,`{"error":"invalid world key"}`,400);return};ctx,cancel:=context.WithTimeout(r.Context(),75*time.Second);defer cancel();c,err:=catalogStore.GetOrCreate(ctx,key,generatedCenterCount,generateNames);if err!=nil{w.WriteHeader(502);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};source:="postgres";if c.Created{source="openai"};_=json.NewEncoder(w).Encode(map[string]any{"worldId":c.WorldID,"centers":c.Centers,"created":c.Created,"source":source,"model":cfg.OpenAIModel})}
 debugGuard:=func(w http.ResponseWriter,r *http.Request,method string)bool{w.Header().Set("Content-Type","application/json");if r.Method!=method{w.WriteHeader(405);return false};if catalogStore==nil{http.Error(w,`{"error":"persistent world database is not configured"}`,503);return false};if !authorized(r){http.Error(w,`{"error":"debug access is disabled or unauthorized"}`,403);return false};return true}
 inspect:=func(w http.ResponseWriter,r *http.Request){if !debugGuard(w,r,http.MethodGet){return};key:=r.URL.Query().Get("key");ctx,cancel:=context.WithTimeout(r.Context(),15*time.Second);defer cancel();v,err:=catalogStore.InspectWorld(ctx,key);if err!=nil{w.WriteHeader(404);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};_=json.NewEncoder(w).Encode(v)}
 resetWorld:=func(w http.ResponseWriter,r *http.Request){if !debugGuard(w,r,http.MethodPost){return};key:=r.URL.Query().Get("key");ctx,cancel:=context.WithTimeout(r.Context(),15*time.Second);defer cancel();if err:=catalogStore.ResetWorld(ctx,key);err!=nil{w.WriteHeader(500);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};_=json.NewEncoder(w).Encode(map[string]any{"ok":true,"reset":"world"})}
 resetHost:=func(w http.ResponseWriter,r *http.Request){if !debugGuard(w,r,http.MethodPost){return};key,id:=r.URL.Query().Get("key"),r.URL.Query().Get("host");ctx,cancel:=context.WithTimeout(r.Context(),75*time.Second);defer cancel();c,err:=catalogStore.ResetHost(ctx,key,id,func(ctx context.Context)(string,error){n,e:=generateNames(ctx,1);if e!=nil{return "",e};return n[0],nil});if err!=nil{w.WriteHeader(502);_=json.NewEncoder(w).Encode(map[string]any{"error":err.Error()});return};_=json.NewEncoder(w).Encode(map[string]any{"ok":true,"center":c})}
 mux:=http.NewServeMux();mux.Handle("/ws",wsserver.Handler{Network:network,Store:store,Sessions:sessions});mux.HandleFunc("/api/world/bootstrap",bootstrap);mux.HandleFunc("/api/centers",bootstrap);mux.HandleFunc("/api/debug/world",inspect);mux.HandleFunc("/api/debug/world/reset",resetWorld);mux.HandleFunc("/api/debug/host/reset",resetHost);mux.HandleFunc("/health",func(w http.ResponseWriter,r *http.Request){w.Header().Set("Content-Type","application/json");_=json.NewEncoder(w).Encode(map[string]any{"ok":true,"world_date":cfg.WorldDate,"time":clock.Now(),"persistent_worlds":catalogStore!=nil,"debug_access":cfg.DebugResetToken!=""})});srv:=&http.Server{Addr:cfg.Addr,Handler:cors(mux),ReadHeaderTimeout:5*time.Second};log.Printf("zutto server listening on %s",cfg.Addr);log.Fatal(srv.ListenAndServe())}
func cors(next http.Handler)http.Handler{return http.HandlerFunc(func(w http.ResponseWriter,r *http.Request){w.Header().Set("Access-Control-Allow-Origin","*");w.Header().Set("Access-Control-Allow-Headers","Content-Type, X-Zutto-Debug-Token");w.Header().Set("Access-Control-Allow-Methods","GET, POST, OPTIONS");if r.Method==http.MethodOptions{w.WriteHeader(204);return};next.ServeHTTP(w,r)})}

from pathlib import Path


def replace_once(path, old, new):
    p=Path(path); s=p.read_text()
    if old not in s: raise SystemExit(f'missing target {path}: {old!r}')
    p.write_text(s.replace(old,new,1))

replace_once('apps/server/cmd/server/materialization_lab.go', '\ttoken        string\n\n\tmu     sync.Mutex', '\ttoken        string\n\tfreshArchive materializationFreshArchiveStore\n\n\tmu     sync.Mutex')

replace_once('apps/server/cmd/server/main.go', '\tvar catalogStore *worldcatalog.Store\n\tvar historyStore *historicalkb.Store\n', '\tvar catalogStore *worldcatalog.Store\n\tvar historyStore *historicalkb.Store\n\tvar freshArchive *postgresMaterializationFreshArchive\n')
replace_once('apps/server/cmd/server/main.go', '\t\tif err == nil { historyStore, err = historicalkb.Open(ctx, cfg.DatabaseURL) }\n\t\tif err == nil { err = historyStore.EnsureSchema(ctx) }\n\t\tcancel()\n', '\t\tif err == nil { historyStore, err = historicalkb.Open(ctx, cfg.DatabaseURL) }\n\t\tif err == nil { err = historyStore.EnsureSchema(ctx) }\n\t\tif err == nil { freshArchive, err = openPostgresMaterializationFreshArchive(ctx, cfg.DatabaseURL) }\n\t\tcancel()\n')
replace_once('apps/server/cmd/server/main.go', '\t\tdefer catalogStore.Close()\n\t\tdefer historyStore.Close()\n', '\t\tdefer catalogStore.Close()\n\t\tdefer historyStore.Close()\n\t\tdefer freshArchive.Close()\n')
replace_once('apps/server/cmd/server/main.go', '\tmaterializationLab := newMaterializationLab(store, worldEngine, postMaterializer, cfg.WorldDate, cfg.MaterializationLabToken)\n', '\tmaterializationLab := newMaterializationLab(store, worldEngine, postMaterializer, cfg.WorldDate, cfg.MaterializationLabToken)\n\tmaterializationLab.freshArchive = freshArchive\n')
replace_once('apps/server/cmd/server/main.go', '\tmux.HandleFunc("/api/debug/materialization-lab-fresh", materializationLab.freshHandler())\n', '\tmux.HandleFunc("/api/debug/materialization-lab-fresh", materializationLab.freshHandler())\n\tmux.HandleFunc("/api/debug/materialization-lab-fresh-view", materializationLab.freshViewerHandler())\n')
replace_once('apps/server/cmd/server/main.go', '\"materialization_lab_auth\":\"none-test-only\",\"materialization_lab_daily_runs\":publicLabDailyRuns', '\"materialization_lab_auth\":\"none-test-only\",\"materialization_lab_archive\":freshArchive!=nil,\"materialization_lab_daily_runs\":publicLabDailyRuns')

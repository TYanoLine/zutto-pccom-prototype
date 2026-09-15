package worldcatalog

import "testing"

func TestWorldInspectionJSONShapeFields(t *testing.T) {
	h := HostInspection{Center: Center{ID: "world-001", SoftwareFamily: "erika-k", LineCount: 2, MaxBaud: 14400}, DirectoryOrder: 0, Generation: 1, SkeletonBasis: "station-specific fictional distribution"}
	w := WorldInspection{GenerationVersion: GenerationVersion, HostCount: 1, Hosts: []HostInspection{h}}
	if w.GenerationVersion != 2 || w.HostCount != len(w.Hosts) { t.Fatal("inspection metadata mismatch") }
	if w.Hosts[0].SoftwareFamily != "erika-k" || w.Hosts[0].Generation != 1 { t.Fatal("inspection host fields mismatch") }
}

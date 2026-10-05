package erikak

import (
	"strings"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

type boardNode struct {
	Path                 string
	Key                  string
	Alias                string
	Parent               string
	Name                 string
	Hidden               bool
	SemanticScope        string
	RootAuthorPolicy     string
	ActivityWeight       float64
	ReplyRate            float64
	RetainedRootCap      int
	VerifiedReferentRate float64
	Unread               bool
}

type boardCatalog struct {
	Boards []boardNode
	Texts  hostcatalog.ErikaKTexts
}

func boardCatalogFromDetail(d *hostcatalog.ErikaKDetail) boardCatalog {
	out := boardCatalog{}
	if d == nil {
		return out
	}

	out.Texts = d.Texts
	out.Boards = make([]boardNode, 0, len(d.Boards))
	for _, b := range d.Boards {
		path := strings.TrimSpace(b.Path)
		key, parent := path, ""
		if i := strings.LastIndex(path, "/"); i >= 0 {
			key, parent = path[i+1:], path[:i]
		}
		out.Boards = append(out.Boards, boardNode{Path: path, Key: key, Parent: parent, Alias: b.Alias, Name: b.Name, Hidden: b.Hidden, SemanticScope: b.Scope, RootAuthorPolicy: b.RootAuthorPolicy, ActivityWeight: b.ActivityWeight, ReplyRate: b.ReplyRate, RetainedRootCap: b.RetainedRootCap, VerifiedReferentRate: b.VerifiedReferentRate, Unread: b.Unread})
	}
	return out
}

func newBoardCatalog(boards []hostcatalog.ErikaKBoard) boardCatalog {
	return boardCatalogFromDetail(&hostcatalog.ErikaKDetail{Boards: boards})
}

func boardByPath(c boardCatalog, path string) (boardNode, bool) {
	path = strings.TrimSpace(path)
	for _, node := range c.Boards {
		if node.Path == path {
			return node, true
		}
	}
	return boardNode{}, false
}

func BoardByPath(boards []hostcatalog.ErikaKBoard, path string) (world.Board, bool) {
	node, ok := boardByPath(newBoardCatalog(boards), path)
	if !ok || node.Hidden {
		return world.Board{}, false
	}
	return worldBoard(node), true
}

func worldBoard(node boardNode) world.Board {
	return world.Board{ID: node.Path, Name: node.Name, SemanticScope: node.SemanticScope, RootAuthorPolicy: node.RootAuthorPolicy, ActivityWeight: node.ActivityWeight, ReplyRate: node.ReplyRate, RetainedRootCap: node.RetainedRootCap, VerifiedReferentRate: node.VerifiedReferentRate}
}

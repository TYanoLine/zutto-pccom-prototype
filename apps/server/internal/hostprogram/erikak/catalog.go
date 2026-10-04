package erikak

import (
	"strings"

	"zutto-pccom/apps/server/internal/hostcatalog"
	"zutto-pccom/apps/server/internal/world"
)

type boardCatalog struct {
	nodes []boardNode
}

func newBoardCatalog(detail *hostcatalog.ErikaKDetail) boardCatalog {
	if detail == nil {
		return boardCatalog{}
	}
	nodes := make([]boardNode, 0, len(detail.Boards))
	for _, b := range detail.Boards {
		path := strings.TrimSpace(b.Path)
		key, parent := path, ""
		if i := strings.LastIndexByte(path, '/'); i >= 0 {
			key, parent = path[i+1:], path[:i]
		}
		nodes = append(nodes, boardNode{
			Path: path, Key: key, Parent: parent, Alias: b.Alias, Name: b.Name,
			Hidden: b.Hidden, SemanticScope: b.Scope, RootAuthorPolicy: b.RootAuthorPolicy,
			ActivityWeight: b.ActivityWeight, ReplyRate: b.ReplyRate,
			RetainedRootCap: b.RetainedRootCap, VerifiedReferentRate: b.VerifiedReferentRate,
			Unread: b.Unread,
		})
	}
	return boardCatalog{nodes: nodes}
}

func (c boardCatalog) find(path string) (boardNode, bool) {
	path = normalizePath(path)
	for _, node := range c.nodes {
		if node.Path == path {
			return node, true
		}
	}
	return boardNode{}, false
}

func (c boardCatalog) children(parent string) []boardNode {
	out := make([]boardNode, 0)
	for _, node := range c.nodes {
		if node.Parent == parent && !node.Hidden {
			out = append(out, node)
		}
	}
	return out
}

func (c boardCatalog) isForum(path string) bool {
	for _, node := range c.nodes {
		if node.Parent == path {
			return true
		}
	}
	return false
}

func (c boardCatalog) unread(path string) bool {
	for _, node := range c.nodes {
		if node.Path == path {
			return node.Unread
		}
	}
	return false
}

func BoardByPath(detail *hostcatalog.ErikaKDetail, path string) (world.Board, bool) {
	catalog := newBoardCatalog(detail)
	node, ok := catalog.find(path)
	if !ok || node.Hidden {
		return world.Board{}, false
	}
	return worldBoard(node), true
}

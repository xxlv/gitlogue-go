package ui

import (
	"path/filepath"
	"sort"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/xxlv/gitlogue-go/internal/gitengine"
)

type treeRow struct {
	Display string
	Path    string
	Kind    gitengine.ChangeKind
	Depth   int
	IsDir   bool
	Ins     int
	Del     int
}

func treeRows(files []gitengine.FileChange) []treeRow {
	type meta struct {
		kind     gitengine.ChangeKind
		old      string
		ins, del int
	}
	byPath := make(map[string]meta, len(files))
	dirs := make(map[string]int)
	for _, f := range files {
		p := filepath.ToSlash(f.DisplayPath())
		p = strings.TrimPrefix(p, "./")
		if p == "" {
			continue
		}
		ins, del := f.LineStats()
		byPath[p] = meta{kind: f.Kind, old: f.OldPath, ins: ins, del: del}
		parts := strings.Split(p, "/")
		acc := ""
		for i := 0; i < len(parts)-1; i++ {
			if acc == "" {
				acc = parts[i]
			} else {
				acc += "/" + parts[i]
			}
			dirs[acc] = i
		}
	}

	type item struct {
		sort string
		row  treeRow
	}
	items := make([]item, 0, len(byPath)+len(dirs))
	for d, depth := range dirs {
		items = append(items, item{
			sort: d + "/",
			row: treeRow{
				Display: filepath.Base(d),
				Path:    d,
				Depth:   depth,
				IsDir:   true,
			},
		})
	}
	for p, m := range byPath {
		name := filepath.Base(p)
		if m.kind == gitengine.ChangeRenamed && m.old != "" {
			name = filepath.Base(m.old) + " → " + name
		}
		items = append(items, item{
			sort: p,
			row: treeRow{
				Display: name,
				Path:    p,
				Kind:    m.kind,
				Depth:   strings.Count(p, "/"),
				IsDir:   false,
				Ins:     m.ins,
				Del:     m.del,
			},
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].sort < items[j].sort })
	out := make([]treeRow, len(items))
	for i, it := range items {
		out[i] = it.row
	}
	return out
}

func kindMark(k gitengine.ChangeKind) string {
	switch k {
	case gitengine.ChangeAdded:
		return "+"
	case gitengine.ChangeDeleted:
		return "D"
	case gitengine.ChangeRenamed:
		return "R"
	default:
		return "M"
	}
}

func styleKind(k gitengine.ChangeKind) lipgloss.Style {
	switch k {
	case gitengine.ChangeAdded:
		return kindAdd
	case gitengine.ChangeDeleted:
		return kindDel
	case gitengine.ChangeRenamed:
		return kindRen
	default:
		return kindMod
	}
}

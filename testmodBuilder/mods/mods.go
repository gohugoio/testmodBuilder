package mods

import (
	"fmt"
	"path"
	"runtime"
	"strings"
)

const (
	basePath = "github.com/gohugoio/hugoTestModules1_" + runtime.GOOS

	// Increment the minor version.
	versionTemplate = "v1.%d.0"
)

type Md struct {
	name   string
	Vendor bool

	Children Mds
}

func (m *Md) String() string {
	s := m.Path()
	for _, mm := range m.Children {
		s += "\n" + mm.String()
	}

	return s
}

func (m *Md) Collect() []*Md {
	mds := []*Md{m}
	for _, mm := range m.Children {
		mds = append(mds, mm.Collect()...)
	}

	return mds

}

func (m *Md) Paths() []string {
	var p []string
	for _, mm := range m.Children {
		p = append(p, mm.Path())
	}

	return p
}

// Suitable for TOML arrays.
func (m *Md) PathsStr() string {
	return strings.Replace(fmt.Sprintf("%q", m.Paths()), "\" ", "\", ", -1)
}

func (m *Md) Name() string {
	n := "modh" + m.name
	if m.Vendor {
		n += "v"
	}
	return n
}

func (m *Md) Path() string {
	return path.Join(basePath, m.Name())
}

func (m *Md) init(idx int, parent *Md) {
	var parentName string
	if parent != nil {
		parentName = parent.name + "_"
	}

	m.name = fmt.Sprintf("%s%d", parentName, idx+1)
	m.Vendor = idx%2 == 0
	for i, mm := range m.Children {
		mm.init(i, m)
	}

}

func createModule() *Md {
	return &Md{
		Children: []*Md{
			&Md{Children: []*Md{
				&Md{},
				&Md{},
			}},
			&Md{Children: []*Md{
				&Md{},
				&Md{},
			}},
		},
	}
}

func createSmallModule() *Md {
	return &Md{
		Children: []*Md{
			&Md{Children: []*Md{
				&Md{},
			}},
		},
	}
}

type Mds []*Md

func (m Mds) Collect() Mds {
	var res Mds
	for _, md := range m {
		res = append(res, md.Collect()...)
	}
	return res
}

func CreateModules() Mds {
	mods := make(Mds, 2)
	for i := 0; i < len(mods); i++ {
		mods[i] = createModule()
		mods[i].init(i, nil)
	}

	return mods
}

func CreateModulesSmall() Mds {
	mods := make(Mds, 1)
	for i := 0; i < len(mods); i++ {
		mods[i] = createSmallModule()
		mods[i].init(i, nil)
	}

	return mods
}

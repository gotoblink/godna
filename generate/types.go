package generate

type GoPackages struct {
	Pkgs         []*goPkg2
	MaxPkgLen    int
	MaxRelDirLen int
	name2goPkg2  map[string]*goPkg2
}
type ProtocIt struct {
	goPkgs            GoPackages
	FileDescriptorSet []byte
}

type ProtocFdsIt struct {
	goPkgs GoPackages
}

type GoModIt struct {
	protocIt ProtocIt
	gomods   []*goMod
}

type goPkg2 struct {
	RelDir  string
	Pkg     string
	Files   []string
	Imports goPkgs2By
}
type goPkgs2By []*goPkg2

type goMod struct {
	pkg     *goPkg2
	subpkg  []*goPkg2
	imp     []*goMod
	version string
	dirty   []string
}

func (mo goMod) String() string {
	return mo.pkg.Pkg
}

type Composite interface {
	Size() int
	Key() string
	Get(int) Composite
	Temp() CompColl
}
type CompColl interface {
	Append(Composite)
	AppendAll(CompColl)
}

type Void struct{}

//TODO change for a topological sort
//dedect loops
func sortgoPkgs2By(in goPkgs2By) goPkgs2By {
	out := goPkgs2By{}
	has := map[string]Void{}
	i := 0
	max := len(in)*len(in) + 1
	for {
		i++
		if i > max {
			panic("loop")
		}
	OUTER:
		for _, it := range in {
			if len(out) == len(in) {
				return out
			}
			if _, ex := has[it.Pkg]; ex {
				continue
			}
			for _, dep := range it.Imports {
				if _, ex := has[dep.Pkg]; !ex {
					continue OUTER
				}
			}
			out = append(out, it)
			has[it.Pkg] = Void{}
		}
	}
}

// func (a goPkgs2By) Len() int      { return len(a) }
// func (a goPkgs2By) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
// func (a goPkgs2By) Less(i, j int) bool {
// 	x, y := a[i], a[j]
// 	fmt.Printf("%s %s ", y.Pkg, x.Pkg) //, x.Imports)
// 	for k, _ := range x.Imports {
// 		if y.Pkg == x.Imports[k].Pkg {
// 			fmt.Printf(" false\n")
// 			return false
// 		}
// 	}
// 	fmt.Printf(" true\n")
// 	return true
// }

func (pk goPkg2) Size() int {
	return len(pk.Imports)
}
func (pk goPkg2) Key() string {
	return pk.Pkg
}
func (pk goPkg2) Get(i int) Composite {
	return pk.Imports[i]
}
func (pk goPkg2) Temp() CompColl {
	return &goPkgs2By{}
}
func (pks *goPkgs2By) Append(item Composite) {
	*pks = append(*pks, item.(*goPkg2))
}
func (pks *goPkgs2By) AppendAll(items CompColl) {
	for _, item := range *(items.(*goPkgs2By)) {
		*pks = append(*pks, item)
	}
}

func (pk goPkg2) String() string {
	return pk.Pkg
}

func collect(mod Composite, indent string, collected map[string]Void) CompColl {
	deps := mod.Temp()
	for i := 0; i < mod.Size(); i++ {
		dep := mod.Get(i)
		if _, ex := collected[dep.Key()]; ex {
			continue
		}
		deps.Append(dep)
		collected[dep.Key()] = Void{}
		deps.AppendAll(collect(dep, indent+"  ", collected))
	}
	return deps
}

// type goPkg3 struct {
// 	goPkg2
// 	absOut     string
// 	outBit     string
// 	dirtyFiles map[string][]string // generate path -> []files
// 	imps       []*goPkg3
// }

// type goPkg3By []*goPkg3

// func (a goPkg3By) Len() int      { return len(a) }
// func (a goPkg3By) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
// func (a goPkg3By) Less(i, j int) bool {
// 	x, y := a[i], a[j]
// 	for k, _ := range x.imps {
// 		if y.goPkg2.Pkg == x.imps[k].goPkg2.Pkg {
// 			return false
// 		}
// 	}
// 	return true
// }

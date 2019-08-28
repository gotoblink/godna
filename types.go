package main

type goMods struct {
	Modules []goMod
}

type goMod struct {
	RelDir string
	Module string
}

type protoFiles struct {
	Files []protoFile
}
type protoFile struct {
	RelFile string
	Module  string
	Imports []string
}

type goModWithFilesImports struct {
	ContainingMod string
	RelDir        string
	Package       string
	Files         []string
	Imports       []string
}

type goModPlus struct {
	mod  goMod
	pkgs []goModWithFilesImports
}

// type goModAbsOutBy []*goModAbsOut

// func (a goModAbsOutBy) Len() int      { return len(a) }
// func (a goModAbsOutBy) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
// func (a goModAbsOutBy) Less(i, j int) bool {
// 	depSet := map[string]struct{}{}
// 	a[i].collect("", depSet)
// 	// fmt.Printf("a:%s\nb:%s\n", a[i].mod.mod.Module, a[j].mod.mod.Module)
// 	// fmt.Printf("--%v\n", depSet)
// 	if _, ex := depSet[a[j].mod.mod.Module]; ex {
// 		// fmt.Printf("TRUE\n\n")
// 		return false
// 	}
// 	return true
// 	// fmt.Printf("FALSE %v\n\n", a[i].mod.mod.Module < a[j].mod.mod.Module)
// 	// return a[i].mod.mod.Module < a[j].mod.mod.Module
// 	// return false
// }

type goModAbsOut struct {
	mod  goModPlus
	pkg  *goPkgAbsOut
	imps []*goModAbsOut
}

type goPkgAbsOut struct {
	module     *goModPlus
	absOut     string
	outBit     string
	dirty      bool
	dirtyFiles []string
	// pod    *config.Config_PluginOutDir
	pkgx goModWithFilesImports
	imps []*goPkgAbsOut
	mod  bool
}

package main

type Step1 struct {
	Modules []goMod
}
type Step2 struct {
	step1   Step1
	Modules []*goModPlus
}
type Step3 struct {
	step2 Step2
	Pkgs  []*goPkgAbsOut
}
type Step4 struct {
	step3 Step3
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

type goPkgAbsOut struct {
	module *goModPlus
	absOut string
	outBit string
	// dirty      map[string]bool
	dirtyFiles map[string][]string
	// pod    *config.Config_PluginOutDir
	pkgx goModWithFilesImports
	imps []*goPkgAbsOut
	mod  bool
}

type goPkgAbsOutBy []*goPkgAbsOut

func (a goPkgAbsOutBy) Len() int      { return len(a) }
func (a goPkgAbsOutBy) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a goPkgAbsOutBy) Less(i, j int) bool {
	x, y := a[i], a[j]
	for k, _ := range x.imps {
		if y.pkgx.Package == x.imps[k].pkgx.Package {
			return false
		}
	}
	return true
}

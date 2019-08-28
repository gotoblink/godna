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

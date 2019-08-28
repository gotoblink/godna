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

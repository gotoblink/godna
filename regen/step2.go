package regen

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/wxio/godna/pb/dna/config"
)

func (proc *Step2) Process(rootOutDir string, cfg *config.Config) (string, error) {
	fmt.Printf(`Step 2
============================
`)
	mp, err := step2(cfg.SrcDir, proc.step1.Modules)
	if err != nil {
		return "", err
	}
	proc.Modules = mp
	return "", nil
}

// collectFilesAndImports
func step2(srcDir string, modules []goMod) ([]*goModPlus, error) {
	gomods2 := []*goModPlus{}
	fmt.Printf(`	For Modules
	============================
`)

	for _, gomod := range modules {
		fmt.Printf("\t\t%s\n", gomod.Module)
		fmt.Printf("\t\t\tCollect files\n")
		pfs, err := gomod.collectFiles(srcDir)
		if err != nil {
			return nil, err
		}
		fmt.Printf("\t\t\tCollect packages\n")
		mods, err := pfs.collectModules(gomod)
		if err != nil {
			return nil, err
		}
		mp := &goModPlus{gomod, mods}
		gomods2 = append(gomods2, mp)
	}
	return gomods2, nil
}

func (in protoFiles) collectModules(gomod goMod) ([]goModWithFilesImports, error) {
	mods := map[string]*goModWithFilesImports{}
	imports := map[string]map[string]bool{}
	for _, file := range in.Files {
		var mod *goModWithFilesImports
		var ex bool
		if mod, ex = mods[file.Module]; !ex {
			mod = &goModWithFilesImports{
				RelDir:        gomod.RelDir,
				ContainingMod: gomod.Module,
				Package:       file.Module,
			}
			mods[file.Module] = mod
			imports[file.Module] = map[string]bool{}
		}
		mod.Files = append(mod.Files, file.RelFile)
		for _, imp := range file.Imports {
			imports[file.Module][imp] = true
		}
	}
	for k, _ := range mods {
		for imp, _ := range imports[k] {
			mi := mods[k]
			mi.Imports = append(mi.Imports, imp)
		}
	}
	ret := []goModWithFilesImports{}
	// modx := map[string]struct{}{}
	for _, mod := range mods {
		if !strings.HasPrefix(mod.Package, gomod.Module) {
			return nil, fmt.Errorf("not contained in module %v %v", mod.Package, gomod.Module)
		}
		// modx[]
		fmt.Printf("\t\t\t\t%v\n", mod.Package)
		ret = append(ret, *mod)
	}
	return ret, nil
}

var goPkgOptRe = regexp.MustCompile(`(?m)^option go_package = (.*);`)
var protoImportRe = regexp.MustCompile(`(?m)^import "(.*)/[^/]+.proto";`)

func (in *goMod) collectFiles(srcDir string) (*protoFiles, error) {
	cwd := filepath.Join(srcDir, in.RelDir)
	pfs := &protoFiles{}
	walkCollect := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if cwd != path {
			if _, err := os.Open(filepath.Join(path, "go.mod")); err == nil {
				return filepath.SkipDir
			}
		}
		if !info.Mode().IsRegular() || !strings.HasSuffix(path, ".proto") {
			return nil
		}
		pf := protoFile{}
		if rel, err := filepath.Rel(cwd, path); err != nil {
			return err
		} else {
			pf.RelFile = rel
		}
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		//
		if match := goPkgOptRe.FindSubmatch(content); len(match) > 0 {
			if pf.Module, err = strconv.Unquote(string(match[1])); err != nil {
				return err
			}
		}
		if p := strings.IndexRune(pf.Module, ';'); p > 0 {
			pf.Module = pf.Module[:p]
		}
		if pf.Module == "" {
			return fmt.Errorf("No package in file %s\n", path)
		}
		//
		protoImportMatch := protoImportRe.FindAllSubmatch(content, -1)
		for _, m := range protoImportMatch {
			pf.Imports = append(pf.Imports, string(m[1]))
		}
		//
		fmt.Printf("\t\t\t\t%s\n", pf.RelFile)
		pfs.Files = append(pfs.Files, pf)
		return nil
	}
	if err := filepath.Walk(cwd, walkCollect); err != nil {
		return nil, err
	}
	return pfs, nil
}

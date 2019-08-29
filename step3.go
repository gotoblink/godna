package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/golangq/q"
	"github.com/wxio/godna/pb/dna/config"
)

func (proc *Step3) Process(rootOutDir string, cfg *config.Config) (string, error) {
	fmt.Printf(`Step 3
============================
`)
	mp, err := step3(proc.step2.Modules, rootOutDir, cfg)
	if err != nil {
		return "", err
	}
	proc.Pkgs = mp
	return "", nil
}

func step3(gomods2 []*goModPlus, rootOutDir string, cfg *config.Config) ([]*goPkgAbsOut, error) {
	modMap := map[string]*goPkgAbsOut{}
	// goModAbs := goModAbsOutBy{}
	gensByOut := []*goPkgAbsOut{}
	for _, modp := range gomods2 {
		for _, pkg := range modp.pkgs {
			for _, pod := range cfg.PluginOutputDir {
				absOut, outBit, err := absOut_Mk_CpMod(pkg, pod, cfg.SrcDir, rootOutDir)
				// fmt.Printf("a:%s b:%s p:%s\n", absOut, outBit, pkg.Package)
				if err != nil {
					return nil, err
				}
				ismod := pod.OutType == config.Config_PluginOutDir_GO_MODS && modp.mod.Module == pkg.Package
				gensPkg := &goPkgAbsOut{
					absOut:     absOut,
					outBit:     outBit,
					pkgx:       pkg,
					mod:        ismod,
					module:     modp,
					dirty:      map[string]bool{},
					dirtyFiles: map[string][]string{},
				}
				gensByOut = append(gensByOut, gensPkg)
				if ismod {
					// gomod := goModAbsOut{
					// 	mod:  modp,
					// 	pkg:  &gensPkg,
					// 	imps: nil,
					// }
					// goModAbs = append(goModAbs, &gomod)
					modMap[outBit] = gensPkg
				}
			}
		}
	}
	//
	for i, _ := range gensByOut {
		gomodabs := gensByOut[i]
		for _, pkg := range gomodabs.module.pkgs {
			for _, imp := range pkg.Imports {
				gimp, ex := modMap[imp]
				if !ex {
					// fmt.Printf("no local pkg %v\n", imp)
				} else {
					// fmt.Printf("LOCAL pkg %v\n", imp)
					gomodabs.imps = append(gomodabs.imps, gimp)
				}
			}
		}
		gensByOut[i] = gomodabs
	}
	for i, _ := range gensByOut {
		gomodabs := gensByOut[i]
		deps := gomodabs.collect("", map[string]struct{}{})
		gomodabs.imps = deps
	}
	return gensByOut, nil
}

func (mod goPkgAbsOut) collect(indent string, depSet map[string]struct{}) []*goPkgAbsOut {
	deps := []*goPkgAbsOut{}
	for _, dep := range mod.imps {
		if _, ex := depSet[dep.module.mod.Module]; ex {
			continue
		}
		deps = append(deps, dep)
		depSet[dep.module.mod.Module] = struct{}{}
		deps = append(deps, dep.collect(indent+"  ", depSet)...)
	}
	return deps
}

func suffix(sa, sb string) string {
	la, lb := len(sa), len(sb)
	lo := 0
	for i := 1; i <= la; i++ {
		if i >= lb {
			break
		}
		if sa[la-i] == sb[lb-i] {
			lo++
		} else {
			break
		}
	}
	if lo == 0 {
		return ""
	}
	return sa[la-lo+1:]
}

func absOut_Mk_CpMod(in goModWithFilesImports, pod *config.Config_PluginOutDir, srcdir string, outroot string) (abs string, bit string, e error) {
	outbit_idx := strings.LastIndex(in.ContainingMod, "/"+pod.Path+"/")
	outbit := ""
	if outbit_idx > -1 {
		outbit = in.ContainingMod[outbit_idx+len(pod.Path)+2:]
	} else {
		outbit = suffix(in.RelDir, in.ContainingMod)
	}
	outAbs, err := filepath.Abs(filepath.Join(outroot, pod.Path, outbit))
	if err != nil {
		return "", outbit, err
	}
	if err = os.MkdirAll(outAbs, os.ModePerm); err != nil {
		return outAbs, outbit, err
	}
	//
	if in.ContainingMod == in.Package && pod.OutType == config.Config_PluginOutDir_GO_MODS {
		src := filepath.Join(srcdir, in.RelDir, "go.mod")
		pwd, _ := os.Getwd()
		dest := filepath.Join(outAbs, "go.mod")
		if _, err = os.Open(dest); err != nil {
			q.Q("$ %s cp %s %s\n", pwd, src, dest)
			if _, err = filecopy(src, dest); err != nil {
				return outAbs, outbit, err
			}
		} else {
			q.Q("$ %s #cp %s %s\n", pwd, src, dest)
		}
	}
	return outAbs, outbit, nil
}

package generate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/wxio/godna/pb/dna/config"
	"github.com/wxio/godna/pb/extensions/store"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

func (proc *GoModIt) Process(cmd *generate) (string, error) {
	if protocFDS, ex := cmd.steps["protoc_file_description_set"]; ex {
		if _, ex = protocFDS["gomod"]; ex {
			msg, err := proc.collectModules(cmd)
			if err != nil {
				return msg, err
			}
			//
			for _, gomod := range proc.gomods {
				for _, pod := range cmd.cfg.PluginOutputDir {
					if pod.OutType == config.Config_PluginOutDir_GO_MODS {
						localPkgPart := gomod.pkg.Pkg[len(cmd.cfg.GetGoPackagePrefix()):]
						outAbs := filepath.Join(cmd.OutputDir, pod.Path, localPkgPart)
						if err := os.MkdirAll(outAbs, os.ModePerm); err != nil {
							return "err: mkdir -p " + outAbs, err
						}
						if msg, err := gomodinit(cmd, gomod, pod.Path, localPkgPart); err != nil {
							return msg, err
						}
						if msg, err := gomodrequire_config(cmd, gomod, pod.Path, localPkgPart); err != nil {
							return msg, err
						}
						if msg, err := gomodrequire_local(cmd, gomod, pod.Path, localPkgPart); err != nil {
							return msg, err
						}
						if msg, err := gomodrequire_tidy(cmd, gomod, pod.Path, localPkgPart); err != nil {
							return msg, err
						}
					}
				}
			}
			//
		}
	}
	return "", nil
}

func gomodinit(gcmd *generate, gomod *goMod, podPath, localPkgPart string) (string, error) {
	outAbs := filepath.Join(gcmd.OutputDir, podPath, localPkgPart)
	if _, err := os.Open(filepath.Join(outAbs, "go.mod")); err == nil {
		gcmd.Debugf("go.mod exists for %v\n", outAbs)
		return "", nil
	}
	cmd := exec.Command("go")
	cmd.Dir = outAbs
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	args := []string{
		"mod",
		"init",
		gomod.pkg.Pkg,
	}
	cmd.Args = append(cmd.Args, args...)
	gcmd.Debugf("%v\n", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), err
	}
	return "", nil
}

func gomodrequire_config(gcmd *generate, gomod *goMod, podPath, localPkgPart string) (string, error) {
	outAbs := filepath.Join(gcmd.OutputDir, podPath, localPkgPart)
	for _, k := range gcmd.cfg.Require {
		cmd := exec.Command("go")
		cmd.Dir = outAbs
		cmd.Env = append(os.Environ(), "GO111MODULE=on")
		args := []string{
			"mod",
			"edit",
			"-require=" + k,
		}
		cmd.Args = append(cmd.Args, args...)
		gcmd.Debugf("%v\n", cmd.Args)
		if out, err := cmd.CombinedOutput(); err != nil {
			return string(out), err
		}
	}
	return "", nil
}

func gomodrequire_local(gcmd *generate, gomod *goMod, podPath, localPkgPart string) (string, error) {
	outAbs := filepath.Join(gcmd.OutputDir, podPath, localPkgPart)
	me := strings.Split(gomod.pkg.Pkg, "/")
	for _, dep := range gomod.imp {
		relPath := ""
		youbit := ""
		you := strings.Split(dep.pkg.Pkg, "/")
		for i := range me {
			if i >= len(you) {
				panic(fmt.Errorf("!!!!\n%s\n%s\n", gomod.pkg.Pkg, dep.pkg.Pkg))
			}
			if me[i] != you[i] {
				relPath = strings.Repeat("../", len(me)-i)
				youbit = strings.Join(you[i:], "/")
				break
			}
		}
		cmd := exec.Command("go")
		cmd.Dir = outAbs
		cmd.Env = append(os.Environ(), "GO111MODULE=on")
		// sem := nextSemvers[dependance.outBit]
		args := []string{
			"mod",
			"edit",
			"-require=" + dep.pkg.Pkg + "@v1.2.3", // + sem.String(),
			"-replace=" + dep.pkg.Pkg + "=" + relPath + youbit,
		}
		cmd.Args = append(cmd.Args, args...)
		gcmd.Debugf("%v\n", cmd.Args)
		if out, err := cmd.CombinedOutput(); err != nil {
			return string(out), err
		}
	}
	return "", nil
}

func gomodrequire_tidy(gcmd *generate, gomod *goMod, podPath, localPkgPart string) (string, error) {
	outAbs := filepath.Join(gcmd.OutputDir, podPath, localPkgPart)
	cmd := exec.Command("go")
	cmd.Dir = outAbs
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	// sem := nextSemvers[dependance.outBit]
	args := []string{
		"mod",
		"tidy",
	}
	cmd.Args = append(cmd.Args, args...)
	gcmd.Debugf("%v\n", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), err
	}
	return "", nil
}

func (proc *GoModIt) collectModules(cmd *generate) (string, error) {
	fds := &descriptor.FileDescriptorSet{}
	err := proto.Unmarshal(proc.protocIt.FileDescriptorSet, fds)
	if err != nil {
		return "", err
	}
	proc.gomods = []*goMod{}
	notmod := []*goPkg2{}
	n2mod2 := map[string]*goMod{}
	for _, fdp := range fds.File {
		goPkg := proc.protocIt.goPkgs.name2goPkg2[*fdp.Options.GoPackage]
		padding := strings.Repeat(" ", proc.protocIt.goPkgs.MaxPkgLen-len(goPkg.Pkg))
		cmd.Debugf("%s%s", *fdp.Options.GoPackage, padding)
		storish, _ := proto.GetExtension(fdp.Options, store.E_Store)
		// if err != nil {
		// }
		if storish != nil {
			eStore := storish.(*store.Store)
			if eStore.GoMod {
				cmd.Debugf(" MOD")
				if _, ex := n2mod2[goPkg.Pkg]; !ex {
					mod := &goMod{pkg: goPkg}
					proc.gomods = append(proc.gomods, mod)
					n2mod2[goPkg.Pkg] = mod
				}
			} else {
				notmod = append(notmod, goPkg)
			}
		} else {
			notmod = append(notmod, goPkg)
		}
		cmd.Debugf("\n")
	}
	// subpkg
	for _, pkg := range notmod {
		mymod, ex := mod4pkg(n2mod2, pkg.Pkg)
		if !ex {
			return "", fmt.Errorf("Does not belong to any specified go module %s %v\n  %s", pkg.Pkg, pkg.Files,
				"Add to appropriated proto file\n"+
					"import \"dna/store.proto\";\n"+
					"\n"+
					"option (wxio.dna.store) = {\n"+
					"go_mod : true\n"+
					"};")
		}
		if pkg.Pkg == mymod.pkg.Pkg {
			continue
		}
		if !strings.HasPrefix(pkg.Pkg, mymod.pkg.Pkg) {
			return "", fmt.Errorf("package doesn't belong. module: %s  package: %s files: %v", mymod.pkg.Pkg, pkg.Pkg, pkg.Files)
		}
		mymod.subpkg = append(mymod.subpkg, pkg)
	}
	// imports
	for _, mod := range proc.gomods {
		already := map[string]bool{}
		for _, dep := range mod.pkg.Imports {
			if theirmod, ex := mod4pkg(n2mod2, dep.Pkg); !ex {
				panic("no such module " + dep.Pkg)
			} else {
				if mod.pkg.Pkg == theirmod.pkg.Pkg {
					continue
				}
				if already[theirmod.pkg.Pkg] {
					continue
				}
				already[theirmod.pkg.Pkg] = true
				mod.imp = append(mod.imp, theirmod)
			}
		}
	}
	return "", nil
}

func mod4pkg(n2mod2 map[string]*goMod, pkgName string) (*goMod, bool) {
	if mymod, ex := n2mod2[pkgName]; ex {
		return mymod, ex
	}
	for {
		if idx := strings.LastIndex(pkgName, "/"); idx == -1 {
			return nil, false
		} else {
			pkgName = pkgName[:idx]
			if mymod, ex := n2mod2[pkgName]; ex {
				return mymod, ex
			}
		}
	}
}

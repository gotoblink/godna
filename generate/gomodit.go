package generate

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	log "github.com/golang/glog"
	"github.com/golangq/q"

	"github.com/wxio/godna/internal/utils"
	"github.com/wxio/godna/pb/dna/config"
	"github.com/wxio/godna/pb/extensions/store"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

func (proc *GoModIt) Process(cmd *generate) (string, error) {
	if cmd.StepGomodAll ||
		cmd.StepGomodInit ||
		cmd.StepGomodCfg ||
		cmd.StepGomodLocal ||
		cmd.StepGomodTidy ||
		cmd.StepGomodVersion {
		msg, err := proc.collectModules(cmd)
		if err != nil {
			return msg, err
		}
		updateSemver := false
		nextSemvers := map[string]string{}
		gitTagSemver := map[string]map[int64]utils.Semvers{}
		pseudoVerion := ""
		if cmd.StepGomodAll || cmd.StepGomodLocal || cmd.StepGomodTidy || cmd.StepGomodVersion {
			gitTagSemver, pseudoVerion, err = gitGetTagSemver(cmd)
			if err != nil {
				return "", err
			}
			updateSemver = true
		}
		//
		for _, gomod := range proc.gomods {
			for _, pod := range cmd.cfg.PluginOutputDir {
				if pod.OutType == config.Config_PluginOutDir_GO_MODS {
					localPkgPart := gomod.pkg.Pkg[len(cmd.cfg.GetGoPackagePrefix())+1:]
					outAbs := filepath.Join(cmd.OutputDir, pod.Path, localPkgPart)
					if err := os.MkdirAll(outAbs, os.ModePerm); err != nil {
						return "err: mkdir -p " + outAbs, err
					}
					if cmd.StepGomodAll || cmd.StepGomodInit {
						if msg, err := gomodinit(cmd, gomod, pod.Path, localPkgPart); err != nil {
							return msg, err
						}
					}
					if cmd.StepGomodAll || cmd.StepGomodCfg {
						if msg, err := gomodrequire_config(cmd, gomod, pod.Path, localPkgPart); err != nil {
							return msg, err
						}
					}
					{
						if cmd.StepGomodAll || cmd.StepGomodLocal {
							if msg, err := gomodrequire_local(cmd, gomod, pod.Path, localPkgPart,
								// gitTagSemver, pseudoVerion,
								nextSemvers); err != nil {
								return msg, err
							}
						}
						if cmd.StepGomodAll || cmd.StepGomodTidy {
							if msg, err := gomodrequire_tidy(cmd, gomod, pod.Path, localPkgPart); err != nil {
								return msg, err
							}
						}
						if cmd.StepGomodAll || cmd.StepGomodVersion {
							if msg, err := gomodrequire_version(cmd, gomod, pod.Path, localPkgPart,
								// gitTagSemver, pseudoVerion,
								nextSemvers); err != nil {
								return msg, err
							}
						}
						if updateSemver {
							if msg, err := update_next_semver(cmd, gomod, pod.Path, localPkgPart,
								gitTagSemver, pseudoVerion,
								nextSemvers); err != nil {
								return msg, err
							}
							base := gomod.pkg.Pkg[len(cmd.cfg.GoPackagePrefix)+1:]
							sem := nextSemvers[base]
							gomod.version = sem
						}
					}
				}
			}
		}
		//
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

func gomodrequire_local(gcmd *generate, gomod *goMod, podPath, localPkgPart string,
	// gitTagSemver map[string]map[int64]utils.Semvers, pseudo string,
	nextSemvers map[string]string,
) (string, error) {
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
		base := dep.pkg.Pkg[len(gcmd.cfg.GoPackagePrefix)+1:]
		sem := nextSemvers[base]
		gcmd.Debugf("  gomodrequire_version key: %s ver: %s  pkg: %s\n", base, sem, dep.pkg.Pkg)
		args := []string{
			"mod",
			"edit",
			"-require=" + dep.pkg.Pkg + "@" + sem,
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

func gomodrequire_version(gcmd *generate, gomod *goMod, podPath, localPkgPart string,
	// gitTagSemver map[string]map[int64]utils.Semvers, pseudo string,
	nextSemvers map[string]string,
) (string, error) {
	outAbs := filepath.Join(gcmd.OutputDir, podPath, localPkgPart)
	for _, dep := range gomod.imp {
		cmd := exec.Command("go")
		cmd.Dir = outAbs
		cmd.Env = append(os.Environ(), "GO111MODULE=on")
		base := dep.pkg.Pkg[len(gcmd.cfg.GoPackagePrefix)+1:]
		sem := nextSemvers[base]
		gcmd.Debugf("  gomodrequire_version key: %s ver: %s\n", base, sem)
		args := []string{
			"mod",
			"edit",
			"-require=" + dep.pkg.Pkg + "@" + sem,
			"-dropreplace=" + dep.pkg.Pkg,
		}
		cmd.Args = append(cmd.Args, args...)
		gcmd.Debugf("%v\n", cmd.Args)
		if out, err := cmd.CombinedOutput(); err != nil {
			return string(out), err
		}
	}
	return "", nil
}

func update_next_semver(gcmd *generate, gomod *goMod, podPath, localPkgPart string,
	gitTagSemver map[string]map[int64]utils.Semvers, pseudo string,
	nextSemvers map[string]string,
) (string, error) {
	major := pkgModVersion(localPkgPart)
	base := gomod.pkg.Pkg[len(gcmd.cfg.GoPackagePrefix)+1:]
	dirtyFiles, _ := getDirtyFiles(gcmd, podPath, localPkgPart)
	gomod.dirty = dirtyFiles
	gcmd.Debugf(" dirty %s %s %v\n", localPkgPart, base, dirtyFiles)
	if next, ex := gitTagSemver[base]; ex {
		if cur, ex := next[major]; ex {
			//TODO check major ver compatibility
			sort.Sort(cur)
			// pkg.dirtyFiles[pod.Path] = dirtyFiles
			if len(dirtyFiles) > 0 {
				ver := (utils.Semver{Major: cur[0].Major, Minor: cur[0].Minor + 1, Patch: 0}).String()
				gcmd.Debugf("next semver (UP ) key: %s ver: %s\n", base, ver)
				nextSemvers[base] = ver
			} else {
				ver := cur[0].String()
				gcmd.Debugf("next semver (CUR) key: %s ver: %s\n", base, ver)
				nextSemvers[base] = ver
			}
		} else {
			gcmd.Debugf("next semver (NEW) key: %s ver: %s\n", base, "v1.0.0")
			nextSemvers[base] = "v1.0.0"
		}
	} else {
		gcmd.Debugf("next semver (KEH) key: %s ver: %s\n", base, "v1.0.0")
		nextSemvers[base] = "v1.0.0"
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

var ignoreGitStatus = regexp.MustCompile(`^.*v(\d+)/(.+)?$`)

func getDirtyFiles(gcmd *generate, podPath string, outBit string) (files []string, e error) {
	cmd := exec.Command("git")
	cmd.Dir = filepath.Join(gcmd.OutputDir, podPath, outBit)
	args := []string{
		"status",
		"--porcelain",
		".",
	}
	cmd.Args = append(cmd.Args, args...)
	q.Q(cmd.Dir)
	q.Q(cmd.Args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warning(err)
		return nil, err
	}
	//
	gcmd.Debugf("podPath, outBit %s %s", podPath, outBit)
	scan := bufio.NewScanner(bytes.NewBuffer(out))
	for scan.Scan() {
		line := scan.Text()
		gcmd.Debugf("%s\n", line)
		if len(line) > 3 {
			fname := line[3:]
			if line[0] == 'R' {
				fname = fname[strings.Index(fname, " -> ")+4:]
			}
			chop := 0
			if podPath == "." {
				chop = len(outBit) + 1
			} else {
				chop = len(podPath) + 1 + len(outBit) + 1
			}
			fname = fname[chop:]
			if ignoreGitStatus.MatchString(fname) {
				continue
			}
			files = append(files, fname)
		} else {
			q.Q(line)
			log.Warning("3?")
		}
	}
	return
}

var vxMod = regexp.MustCompile(`^[^/]+/v(\d+)$`)

// get version from directory name
// some_dir => 1
// some_dir/vXX => XX
func pkgModVersion(dirname string) int64 {
	if match := vxMod.FindStringSubmatch(dirname); len(match) > 0 {
		if majorVer, err := strconv.ParseInt(match[1], 10, 32); err != nil {
			log.Errorf("keh %v", err)
			os.Exit(1)
		} else {
			return majorVer
		}
	}
	return 1
}

var pathSemver = regexp.MustCompile(`^(.+)/v(\d+)\.(\d+)\.(\d+)$`)
var repoSemver = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

func gitGetTagSemver(gcmd *generate) (map[string]map[int64]utils.Semvers, string, error) {
	pseudo_version := ""
	{
		cmd := exec.Command("git")
		cmd.Dir = gcmd.OutputDir
		// cmd.Dir = filepath.Join(in.OutputDir, tp.dirn)
		args := []string{
			"log",
			"-n", "1",
			`--pretty=format:%ad:%H`,
			"--decorate", `--date=format:%Y%m%d%H%M%S`,
		}
		cmd.Args = append(cmd.Args, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, "", fmt.Errorf("%v\n---\n%s\n---\n", err, string(out))
		}
		parts := strings.Split(string(out), ":")
		pseudo_version = fmt.Sprintf("v0.0.0-%s-%.12s", parts[0], parts[1])
	}
	ret := map[string]map[int64]utils.Semvers{}
	cmd := exec.Command("git")
	cmd.Dir = gcmd.OutputDir
	// cmd.Dir = filepath.Join(in.OutputDir, tp.dirn)
	args := []string{
		"tag",
	}
	cmd.Args = append(cmd.Args, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, "", err
	}
	scan := bufio.NewScanner(bytes.NewBuffer(out))
	for scan.Scan() {
		line := scan.Text()
		match := pathSemver.FindStringSubmatch(line)
		if len(match) == 0 {
			if !repoSemver.MatchString(line) {
				gcmd.Debugf("Does tag look right ? %v\n", line)
				q.Q("tag does look right %v\n", line)
			}
			continue
		}
		q.Q("tag %v\n", line)
		modName := match[1]
		ma, _ := strconv.ParseInt(match[2], 10, 64)
		mi, _ := strconv.ParseInt(match[3], 10, 64)
		pa, _ := strconv.ParseInt(match[4], 10, 64)
		sem := utils.Semver{Major: ma, Minor: mi, Patch: pa}
		sems, ex := ret[modName]
		if !ex {
			sems = make(map[int64]utils.Semvers)
			sems[ma] = utils.Semvers{sem}
			gcmd.Debugf("  add semver key: %s ver: %v\n", modName, sem)
			// ret[modName] = sems
		} else {
			gcmd.Debugf("  add semver key: %s ver: %v\n", modName, sem)
			sems[ma] = append(sems[ma], sem)
		}
		ret[modName] = sems
	}
	return ret, pseudo_version, nil
}

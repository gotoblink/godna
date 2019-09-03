package regen

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
)

func (proc *Step4) Process(rootOutDir string, cfg *config.Config) (string, error) {
	fmt.Printf(`Step 4
============================
`)
	err := step4(proc.step3.Pkgs, rootOutDir, cfg)
	if err != nil {
		return "", err
	}
	return "", nil
}

func step4(gensByOut []*goPkgAbsOut, rootOutDir string, cfg *config.Config) error {
	fmt.Printf(
		`	Protoc
	============================
`)
	for _, pkg := range gensByOut {
		// protoc
		for _, pod := range cfg.PluginOutputDir {
			fmt.Printf("\t%s\n", pkg.absOut)
			for _, gen := range pod.Generator {
				if msg, err := protoc(pkg.pkgx, cfg.SrcDir, pkg.absOut, pod, gen, cfg.Includes); err != nil {
					fmt.Println(msg)
					return err
				}
			}
		}
	}
	//
	//
	gitTagSemver, err := gitGetTagSemver(rootOutDir)
	if err != nil {
		return err
	}
	nextSemvers := map[string]utils.Semver{}
	//
	fmt.Printf(`	Module version & depenancies
	============================
`)
	for i, _ := range gensByOut {
		pkg := gensByOut[i]
		if !pkg.mod {
			continue
		}
		major := utils.PkgModVersion(pkg.outBit)
		if major == -1 {
			return fmt.Errorf("not version for mod relpath:%s", pkg.outBit)
		}
		for _, pod := range cfg.PluginOutputDir {
			out, msg, err := pkg.gomodRequireReplace(cfg, nextSemvers)
			if err != nil {
				fmt.Printf("\t%s, %s %v\n", string(out), msg, err)
				return err
			}
			base := pod.Path + "/" + pkg.outBit
			dirtyFiles, _ := getDirtyFiles(rootOutDir, pod.Path, pkg.outBit)
			if next, ex := gitTagSemver[base]; ex {
				if cur, ex := next[major]; ex {
					//TODO check majar ver compatibility
					sort.Sort(cur)
					pkg.dirtyFiles[pod.Path] = dirtyFiles
					if len(dirtyFiles) > 0 {
						nextSemvers[base] = utils.Semver{cur[0].Major, cur[0].Minor + 1, 0}
						nextSemvers[pkg.outBit] = utils.Semver{cur[0].Major, cur[0].Minor + 1, 0}
					} else {
						nextSemvers[base] = cur[0]
						nextSemvers[pkg.outBit] = cur[0]
					}
				} else {
					nextSemvers[base] = utils.Semver{major, 0, 0}
					nextSemvers[pkg.outBit] = utils.Semver{major, 0, 0}
				}
			} else {
				nextSemvers[base] = utils.Semver{major, 0, 0}
				nextSemvers[pkg.outBit] = utils.Semver{major, 0, 0}
			}
			// }
			for _, y := range pkg.imps {
				fmt.Printf("\t\t\t%s %v (dirty:%v)\n", y.module.mod.Module, nextSemvers[y.outBit], len(pkg.dirtyFiles[pod.Path]) != 0)
			}
		}
	}
	//
	remote, desc := utils.Describe(cfg.SrcDir)
	//
	fmt.Printf(`	Git add,commit & tag
	============================
`)
	for i, _ := range gensByOut {
		pkg := gensByOut[i]
		if pkg.mod {
			for _, pod := range cfg.PluginOutputDir {
				dirtyFiles := pkg.dirtyFiles[pod.Path]
				err = addNtag(rootOutDir, pod.Path, pkg.outBit, dirtyFiles, nextSemvers[pod.Path+"/"+pkg.outBit], pkg.mod, remote, desc)
				// fmt.Printf("%s %v\n", pkg.outBit, dirtyFiles)
			}
		}
	}
	for i, _ := range gensByOut {
		pkg := gensByOut[i]
		if !pkg.mod {
			for _, pod := range cfg.PluginOutputDir {
				dirtyFiles := pkg.dirtyFiles[pod.Path]
				err = addNtag(rootOutDir, pod.Path, pkg.outBit, dirtyFiles, nextSemvers[pod.Path+"/"+pkg.outBit], pkg.mod, remote, desc)
			}
		}
	}
	return nil
}

var pathSemver = regexp.MustCompile(`^(.+)/v(\d+)\.(\d+)\.(\d+)$`)

func gitGetTagSemver(inside_repo string) (map[string]map[int64]utils.Semvers, error) {
	ret := map[string]map[int64]utils.Semvers{}
	cmd := exec.Command("git")
	cmd.Dir = inside_repo
	// cmd.Dir = filepath.Join(in.OutputDir, tp.dirn)
	args := []string{
		"tag",
	}
	cmd.Args = append(cmd.Args, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	scan := bufio.NewScanner(bytes.NewBuffer(out))
	for scan.Scan() {
		line := scan.Text()
		match := pathSemver.FindStringSubmatch(line)
		if len(match) == 0 {
			log.Warningf("tag does look right %v\n", line)
			q.Q("tag does look right %v\n", line)
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
			ret[modName] = sems
		} else {
			sems[ma] = append(sems[ma], sem)
		}
		ret[modName] = sems
	}
	return ret, nil
}

func protocGenerator(outdir string, gen *config.Config_Generator) string {
	switch plg := gen.GetPlugin().GetCmd().(type) {
	case *config.Config_Generator_Plugin_Go:
		name := "--go_out="
		args := []string{}
		if len(plg.Go.Plugins) != 0 {
			for _, pp := range plg.Go.Plugins {
				pl := strings.ToLower(pp.String())
				args = append(args, "plugins="+pl)
			}
		}
		args = append(args, "paths="+strings.ToLower(plg.Go.Paths.String()))
		name = name + strings.Join(args, ",") + ":" + outdir
		return name
	case *config.Config_Generator_Plugin_Micro:
		name := "--micro_out="
		args := []string{}
		args = append(args, "paths="+strings.ToLower(plg.Micro.Paths.String()))
		name = name + strings.Join(args, ",") + ":" + outdir
		return name
	case *config.Config_Generator_Plugin_GrpcGateway:
		name := "--grpc-gateway_out="
		args := []string{}
		args = append(args, "paths="+strings.ToLower(plg.GrpcGateway.Paths.String()))
		if plg.GrpcGateway.RegisterFuncSuffix != "" {
			args = append(args, "register_func_suffix="+plg.GrpcGateway.RegisterFuncSuffix)
		}
		name = name + strings.Join(args, ",") + ":" + outdir
		return name
	case *config.Config_Generator_Plugin_Swagger:
		name := "--swagger_out="
		args := []string{}
		if plg.Swagger.Logtostderr {
			args = append(args, "logtostderr=true")
		} else {
			args = append(args, "logtostderr=false")
		}
		name = name + strings.Join(args, ",") + ":" + outdir
		return name
	}
	return ""
}

func protoc(in goModWithFilesImports, srcdir string, outAbs string, pod *config.Config_PluginOutDir, gen *config.Config_Generator, includes []string) (message string, e error) {
	cmd := exec.Command("protoc")
	plg := protocGenerator(outAbs, gen)
	args := []string{plg}
	// args = append(args, "-I..")
	args = append(args, "-I"+in.RelDir)
	for _, inc := range includes {
		incAbs, err := filepath.Abs(inc)
		if err != nil {
			return "abs file", err
		}
		args = append(args, "-I"+incAbs)
	}
	for _, fi := range in.Files {
		args = append(args, fi)
	}
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = srcdir
	fmt.Printf("\t\tcmd:%v\n", cmd.Args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		scmd := fmt.Sprintf("( cd %v; %v)\n", cmd.Dir, strings.Join(cmd.Args, " "))
		sout := string(out)
		q.Q(err)
		q.Q(scmd)
		q.Q(sout)
	}
	return fmt.Sprintf("%s\ncmd %v\nmsg:%s\n", srcdir, cmd.Args, string(out)), err
}

var ignoreGitStatus = regexp.MustCompile(`^.*v(\d+)/(.+)?$`)

func getDirtyFiles(outDir string, podPath string, outBit string) (files []string, e error) {
	cmd := exec.Command("git")
	cmd.Dir = filepath.Join(outDir, podPath, outBit)
	args := []string{
		"status",
		"--porcelain",
		".",
	}
	cmd.Args = append(cmd.Args, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warning(err)
		return nil, err
	}
	//
	scan := bufio.NewScanner(bytes.NewBuffer(out))
	for scan.Scan() {
		line := scan.Text()
		if len(line) > 3 {
			fname := line[3:]
			if line[0] == 'R' {
				fname = fname[strings.Index(fname, " -> ")+4:]
			}
			fname = fname[len(podPath)+1+len(outBit)+1:]
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

func (gm *goPkgAbsOut) gomodRequireReplace(in *config.Config, nextSemvers map[string]utils.Semver) ([]byte, string, error) {
	if !gm.mod {
		return nil, "", nil // TODO maybe return an error
	}
	fmt.Printf("\t\t\t%s\n", gm.absOut)
	for _, k := range in.Require {
		cmd := exec.Command("go")
		cmd.Dir = gm.absOut
		cmd.Env = append(os.Environ(), "GO111MODULE=on")
		args := []string{
			"mod",
			"edit",
			"-require=" + k,
		}
		cmd.Args = append(cmd.Args, args...)
		fmt.Printf("\t\t\t\tcmd:%v\n", cmd.Args)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Warningf("ERROR:\n  cmd:%v\n  out:%v   \n   err:%v\n", cmd.Args, string(out), err)
			return out, "error", err
		}
	}
	for _, dependance := range gm.imps {
		cmd := exec.Command("go")
		cmd.Dir = gm.absOut
		cmd.Env = append(os.Environ(), "GO111MODULE=on")
		sem := nextSemvers[dependance.outBit]
		relPath := strings.Repeat("../", strings.Count(gm.outBit, "/")+1)
		args := []string{
			"mod",
			"edit",
			"-require=" + dependance.module.mod.Module + "@" + sem.String(),
			"-replace=" + dependance.module.mod.Module + "=" + relPath + dependance.outBit,
		}
		cmd.Args = append(cmd.Args, args...)
		fmt.Printf("\t\t\t\tcmd:%v\n", cmd.Args)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Warningf("ERROR:\n  cmd:%v\n  out:%v   \n   err:%v\n", cmd.Args, string(out), err)
			return out, "error", err
		}
	}
	//
	cmd := exec.Command("go")
	cmd.Dir = gm.absOut
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	args := []string{
		"mod",
		"tidy",
	}
	cmd.Args = append(cmd.Args, args...)
	fmt.Printf("\t\t\t\tcmd:%v\n", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Warningf("ERROR:\n  cmd:%v\n  out:%v   \n   err:%v\n", cmd.Args, string(out), err)
		return out, "error", err
	}
	//
	for _, dependance := range gm.imps {
		cmd := exec.Command("go")
		cmd.Dir = gm.absOut
		cmd.Env = append(os.Environ(), "GO111MODULE=on")
		sem := nextSemvers[dependance.outBit]
		args := []string{
			"mod",
			"edit",
			"-require=" + dependance.module.mod.Module + "@" + sem.String(),
			"-dropreplace=" + dependance.module.mod.Module,
		}
		cmd.Args = append(cmd.Args, args...)
		fmt.Printf("\t\t\t\tcmd:%v\n", cmd.Args)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Warningf("ERROR:\n  cmd:%v\n  out:%v   \n   err:%v\n", cmd.Args, string(out), err)
			return out, "error", err
		}
	}
	return nil, "", nil
}

func addNtag(outDir string, podPath string, outBit string, files []string, sem utils.Semver, ismod bool, remote, desc string) error {
	if len(files) == 0 {
		return nil
	}
	{
		cmd := exec.Command("git")
		cmd.Dir = filepath.Join(outDir, podPath, outBit)
		args := []string{
			"add",
		}
		args = append(args, files...)
		cmd.Args = append(cmd.Args, args...)
		fmt.Printf("\t\t%s\n", cmd.Dir) // wd:
		fmt.Printf("\t\t\tcmd:%v\n", cmd.Args)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("%s\n", string(out))
			return err
		}
	}
	{
		cmd := exec.Command("git")
		cmd.Dir = filepath.Join(outDir, podPath, outBit)
		// args := []string{"commit", "--allow-empty", "-m", remote + " " + desc}
		args := []string{"commit", "-m", remote + " " + desc}
		cmd.Args = append(cmd.Args, args...)
		fmt.Printf("\t\t\tcmd:%v\n", cmd.Args)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("\t\t\tERROR %s\n", strings.ReplaceAll(string(out), "\n", "\n\t\t\t\t"))
			// return err
		}
	}
	if ismod {
		cmd := exec.Command("git")
		cmd.Dir = filepath.Join(outDir, podPath, outBit)
		args := []string{
			"tag",
			podPath + "/" + outBit + "/" + sem.String(),
		}
		cmd.Args = append(cmd.Args, args...)
		fmt.Printf("\t\t\tcmd:%v\n", cmd.Args)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("%s\n", string(out))
			return err
		}
	}
	return nil
}

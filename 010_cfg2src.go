package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	log "github.com/golang/glog"
	"github.com/golangq/q"
	"github.com/wxio/godna/pb/dna/config"
)

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

type goModAbsOutBy []*goModAbsOut

func (a goModAbsOutBy) Len() int      { return len(a) }
func (a goModAbsOutBy) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a goModAbsOutBy) Less(i, j int) bool {
	depSet := map[string]struct{}{}
	a[i].collect("", depSet)
	// fmt.Printf("a:%s\nb:%s\n", a[i].mod.mod.Module, a[j].mod.mod.Module)
	// fmt.Printf("--%v\n", depSet)
	if _, ex := depSet[a[j].mod.mod.Module]; ex {
		// fmt.Printf("TRUE\n\n")
		return false
	}
	return true
	// fmt.Printf("FALSE %v\n\n", a[i].mod.mod.Module < a[j].mod.mod.Module)
	// return a[i].mod.mod.Module < a[j].mod.mod.Module
	// return false
}

type goModAbsOut struct {
	mod  goModPlus
	pkg  *goPkgAbsOut
	imps []*goModAbsOut
}

type goPkgAbsOut struct {
	absOut     string
	outBit     string
	dirty      bool
	dirtyFiles []string
	// pod    *config.Config_PluginOutDir
	pkg goModWithFilesImports
	mod bool
}

func (mod goModAbsOut) collect(indent string, depSet map[string]struct{}) []*goModAbsOut {
	deps := []*goModAbsOut{}
	for _, dep := range mod.imps {
		fmt.Printf("%s-- %s\n", indent, dep.mod.mod.Module)
		if _, ex := depSet[dep.mod.mod.Module]; ex {
			continue
		}
		deps = append(deps, dep)
		depSet[dep.mod.mod.Module] = struct{}{}
		deps = append(deps, dep.collect(indent+"  ", depSet)...)
	}
	return deps
}

type pkgrel2next map[string]Semver

var goModRe = regexp.MustCompile(`^module\s+([^ ]+) *$`)

func (in *goMods) collectGomods(cfg *Config) error {
	walkCollectGoMods := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || filepath.Base(path) != "go.mod" {
			return nil
		}
		gm := goMod{}
		if rel, err := filepath.Rel(cfg.cfg.SrcDir, filepath.Dir(path)); err != nil {
			return err
		} else {
			gm.RelDir = rel
		}
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		if match := goModRe.FindSubmatch(content); len(match) > 0 {
			gm.Module = strings.TrimSpace(string(match[1]))
		} else {
			return fmt.Errorf("no go module found in %s/go.mod", gm.RelDir)
		}
		in.Modules = append(in.Modules, gm)
		return nil
	}
	if err := filepath.Walk(cfg.cfg.SrcDir, walkCollectGoMods); err != nil {
		return err
	}
	return nil
}

func describe(src string) (remote string, desc string) {
	{ //
		cmd := exec.Command("git")
		cmd.Dir = filepath.Join(src)
		args := []string{"remote", "get-url", "origin"}
		cmd.Args = append(cmd.Args, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			q.Q(err)
		} else {
			remote = strings.TrimSpace(string(out))
		}
	} //
	{ //
		cmd := exec.Command("git")
		cmd.Dir = filepath.Join(src)
		args := []string{"describe", "--always", "--dirty"}
		cmd.Args = append(cmd.Args, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			q.Q(err)
		} else {
			desc = strings.TrimSpace(string(out))
		}
	} //
	return
}

var goPkgOptRe = regexp.MustCompile(`(?m)^option go_package = (.*);`)
var protoImportRe = regexp.MustCompile(`(?m)^import "(.*)/[^/]+.proto";`)

func (in *goMod) collectFiles(cfg *Config) (*protoFiles, error) {
	cwd := filepath.Join(cfg.cfg.SrcDir, in.RelDir)
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
		pfs.Files = append(pfs.Files, pf)
		return nil
	}
	if err := filepath.Walk(cwd, walkCollect); err != nil {
		return nil, err
	}
	return pfs, nil
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
	for _, mod := range mods {
		if !strings.HasPrefix(mod.Package, gomod.Module) {
			return nil, fmt.Errorf("not contained in module %v %v", mod.Package, gomod.Module)
		}
		ret = append(ret, *mod)
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
	out, err := cmd.CombinedOutput()
	if err != nil {
		scmd := fmt.Sprintf("( cd %v; %v)\n", cmd.Dir, strings.Join(cmd.Args, " "))
		sout := string(out)
		q.Q(err)
		q.Q(scmd)
		q.Q(sout)
	}
	return fmt.Sprintf("wd: %s\ncmd %v\nmsg:%s\n", srcdir, cmd.Args, string(out)), err
}

func isDirty(outDir string, podPath string, outBit string) (dirty bool, files []string, e error) {
	cmd := exec.Command("git")
	cmd.Dir = filepath.Join(outDir, podPath, outBit)
	args := []string{
		"status",
		"--porcelain",
		".",
	}
	cmd.Args = append(cmd.Args, args...)
	// fmt.Printf("git %+v\n", cmd.Args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warning(err)
		return false, nil, err
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
			dirty = true
		} else {
			q.Q(line)
			log.Warning("3?")
		}
	}
	return
}

func (gm *goModAbsOut) gomodRequireReplace(in *config.Config, nextSemvers pkgrel2next) ([]byte, string, error) {
	for _, k := range in.Require {
		cmd := exec.Command("go")
		cmd.Dir = gm.pkg.absOut
		cmd.Env = append(os.Environ(), "GO111MODULE=on")
		args := []string{
			"mod",
			"edit",
			"-require=" + k,
		}
		cmd.Args = append(cmd.Args, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Warningf("ERROR:\n  cmd:%v\n  out:%v   \n   err:%v\n", cmd.Args, string(out), err)
			return out, "error", err
		}
	}
	for _, d := range gm.imps {
		cmd := exec.Command("go")
		cmd.Dir = gm.pkg.absOut
		cmd.Env = append(os.Environ(), "GO111MODULE=on")
		sem := nextSemvers[d.pkg.outBit]
		relPath := strings.Repeat("../", strings.Count(gm.pkg.outBit, "/")+1)
		args := []string{
			"mod",
			"edit",
			"-require=" + d.mod.mod.Module + "@" + sem.String(),
			"-replace=" + d.mod.mod.Module + "=" + relPath + d.pkg.outBit,
		}
		cmd.Args = append(cmd.Args, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			log.Warningf("ERROR:\n  cmd:%v\n  out:%v   \n   err:%v\n", cmd.Args, string(out), err)
			return out, "error", err
		}
	}
	//
	cmd := exec.Command("go")
	cmd.Dir = gm.pkg.absOut
	cmd.Env = append(os.Environ(), "GO111MODULE=on")
	args := []string{
		"mod",
		"tidy",
	}
	cmd.Args = append(cmd.Args, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Warningf("ERROR:\n  cmd:%v\n  out:%v   \n   err:%v\n", cmd.Args, string(out), err)
		return out, "error", err
	}
	return nil, "", nil
}

func addNtag(outDir string, podPath string, outBit string, files []string, sem Semver, mod bool, remote, desc string) error {
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
		// fmt.Printf("git %+v\n", cmd.Args)
		fmt.Printf("wd: %s cmd:%v\n", cmd.Dir, cmd.Args)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("%s\n", string(out))
			return err
		}
	}
	{
		cmd := exec.Command("git")
		cmd.Dir = filepath.Join(outDir, podPath, outBit)
		args := []string{"commit", "-m", remote + " " + desc}
		args = append(args, files...)
		cmd.Args = append(cmd.Args, args...)
		// fmt.Printf("git %+v\n", cmd.Args)
		fmt.Printf("wd: %s cmd:%v\n", cmd.Dir, cmd.Args)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("%s\n", string(out))
			return err
		}
	}
	if mod {
		cmd := exec.Command("git")
		cmd.Dir = filepath.Join(outDir, podPath, outBit)
		args := []string{
			"tag",
			podPath + "/" + outBit + "/" + sem.String(),
		}
		cmd.Args = append(cmd.Args, args...)
		fmt.Printf("wd: %s cmd:%v\n", cmd.Dir, cmd.Args)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Printf("%s\n", string(out))
			return err
		}
	}
	return nil
}

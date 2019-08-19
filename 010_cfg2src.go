package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	log "github.com/golang/glog"
	"github.com/golangq/q"
	"github.com/wxio/godna/pb/dna/config"
)

type Src struct {
	gomodPaths []string
}

type cfg2src struct {
	packages     map[string]*pkage
	dir2pkg      map[string]*pkage
	pkgWalkOrder pkgSorter
	// sems         map[string]map[int64]Semvers
	// // taglead/vX
	// nextsems   map[string]Semver
	longestStr int
	// // localName    map[string]struct{}
	// generators    []generator
	// relOutDir     map[string]struct{}
	// dirtyMods     []*dirtyMod
	// taglead2dirty map[string]*dirtyMod
}

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

var goPkgOptRe = regexp.MustCompile(`(?m)^option go_package = (.*);`)
var protoImportRe = regexp.MustCompile(`(?m)^import "(.*)/[^/]+.proto";`)

func (in *goMod) collectFiles(cfg *Config) (*protoFiles, error) {
	cwd := filepath.Join(cfg.cfg.SrcDir, in.RelDir)
	pfs := &protoFiles{}
	walkCollect := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
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

func (in protoFiles) collectModules(rel string, curmod string) ([]goModWithFilesImports, error) {
	mods := map[string]*goModWithFilesImports{}
	imports := map[string]map[string]bool{}
	for _, file := range in.Files {
		var mod *goModWithFilesImports
		var ex bool
		if mod, ex = mods[file.Module]; !ex {
			mod = &goModWithFilesImports{
				RelDir:        rel,
				ContainingMod: curmod,
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
		if !strings.HasPrefix(mod.Package, curmod) {
			return nil, fmt.Errorf("not contained in module %v %v", mod.Package, curmod)
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

func (in goModWithFilesImports) protoc(srcdir string, outroot string, pod *config.Config_PluginOutDir, gen *config.Config_Generator, includes []string) error {
	cmd := exec.Command("protoc")
	// basedir := filepath.Dir(in.RelDir)
	// modname := filepath.Base(in.RelDir)
	outdir := pod.Path
	outbit_idx := strings.LastIndex(in.ContainingMod, "/"+outdir+"/")
	outbit := filepath.Base(in.RelDir)
	if outbit_idx > -1 {
		outbit = in.ContainingMod[outbit_idx+len(outdir)+1:]
	} else {
		q.Q(in.ContainingMod, outdir)
	}
	outAbs, err := filepath.Abs(filepath.Join(outroot, outdir, outbit))
	if err != nil {
		return err
	}
	os.MkdirAll(outAbs, os.ModePerm)

	if in.ContainingMod == in.Package && pod.OutType == config.Config_PluginOutDir_GO_MODS {
		src := filepath.Join(srcdir, in.RelDir, "go.mod")
		pwd, _ := os.Getwd()
		dest := filepath.Join(outAbs, "go.mod")
		if _, err = os.Open(dest); err != nil {
			fmt.Printf("$ %s cp %s %s\n", pwd, src, dest)
			if _, err = filecopy(src, dest); err != nil {
				return err
			}
		} else {
			fmt.Printf("$ %s #cp %s %s\n", pwd, src, dest)
		}
	}

	plg := protocGenerator(outAbs, gen)
	args := []string{plg}
	// args = append(args, "-I..")
	args = append(args, "-I"+in.RelDir)
	for _, inc := range includes {
		incAbs, err := filepath.Abs(inc)
		if err != nil {
			return err
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
	return err
}

func Cfg2Src(in config.Config, resp *Src) error {
	proc := &cfg2src{
		packages:     map[string]*pkage{},
		dir2pkg:      map[string]*pkage{},
		pkgWalkOrder: pkgSorter{},
		longestStr:   0,
	}
	walkCollectGoMods := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || filepath.Base(path) != "go.mod" {
			return nil
		}
		if rel, err := filepath.Rel(in.SrcDir, filepath.Dir(path)); err != nil {
			return err
		} else {
			resp.gomodPaths = append(resp.gomodPaths, rel)
		}
		return nil
	}
	if err := filepath.Walk(in.SrcDir, walkCollectGoMods); err != nil {
		return err
	}

	walkFnSrcDir := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || !strings.HasSuffix(path, ".proto") {
			return nil
		}
		if rel, err := filepath.Rel(in.SrcDir, path); err != nil {
			return err
		} else {
			// TODO make sure there are no v2 files in v1 (root) dir
			// TODO make sure the are no vX in vY directory
			content, err := ioutil.ReadFile(path)
			if err != nil {
				return err
			}
			var pkgName string
			if match := goPkgOptRe.FindSubmatch(content); len(match) > 0 {
				pn, err := strconv.Unquote(string(match[1]))
				if err != nil {
					return err
				}
				pkgName = pn
			}
			if p := strings.IndexRune(pkgName, ';'); p > 0 {
				pkgName = pkgName[:p]
			}
			if pkgName == "" {
				return fmt.Errorf("No package in file %s\n", path)
			}
			thisPkg, ex := proc.packages[pkgName]
			if !ex {
				thisPkg = &pkage{
					gopkg:        pkgName,
					replacements: make(map[string]*pkage),
				}
				proc.packages[pkgName] = thisPkg
				proc.pkgWalkOrder = append(proc.pkgWalkOrder, thisPkg)
				if len(pkgName) > proc.longestStr {
					proc.longestStr = len(pkgName)
				}
			}
			//
			protoImportMatch := protoImportRe.FindAllSubmatch(content, -1)
			imps := []string{}
			for _, m := range protoImportMatch {
				imps = append(imps, string(m[1]))
			}
			fi := file{rel, imps}
			thisPkg.files = append(thisPkg.files, fi)
			return nil
		}
	}
	if err := filepath.Walk(in.SrcDir, walkFnSrcDir); err != nil {
		return err
	}
	// q.Q(in.localName)
	sort.Sort(proc.pkgWalkOrder)
	for _, pkg := range proc.pkgWalkOrder {
		// in.goModReplacements(pkg)
		proc.pkgDir(in, pkg.gopkg)
	}
	for _, pkg := range proc.pkgWalkOrder {
		// in.goModReplacements(pkg)
		proc.pkgImports(in, pkg.gopkg)
	}

	return nil
}

func (in *cfg2src) pkgDir(cfg config.Config, pkg string) {
	tpkg := in.packages[pkg]
	fnames := tpkg.files
	dirns := make(map[string]struct{})
	dirn := ""
	for _, fn := range fnames {
		dirns[filepath.Dir(fn.name)] = struct{}{}
		dirn = filepath.Dir(fn.name)
	}
	if len(dirns) != 1 {
		log.Errorf("error files with same go package in more than one dir: %s\n", fnames)
		os.Exit(1)
	}
	tpkg.dirn = dirn
	in.dir2pkg[dirn] = tpkg
	{ //
		cmd := exec.Command("git")
		cmd.Dir = filepath.Join(cfg.SrcDir, dirn)
		args := []string{"remote", "get-url", "origin"}
		cmd.Args = append(cmd.Args, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			q.Q(err)
		} else {
			tpkg.source = strings.TrimSpace(string(out))
		}
	} //
	{ //
		cmd := exec.Command("git")
		cmd.Dir = filepath.Join(cfg.SrcDir, dirn)
		args := []string{"describe", "--always", "--dirty"}
		cmd.Args = append(cmd.Args, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			q.Q(err)
		} else {
			tpkg.gitDescribe = strings.TrimSpace(string(out))
		}
	} //
}

func (in *cfg2src) pkgImports(cfg config.Config, pkg string) {
	tpkg := in.packages[pkg]
	for _, fn := range tpkg.files {
		for _, imp := range fn.protoImport {
			if strings.HasPrefix(imp, tpkg.dirn) {
				continue
			}
			for _, rel := range cfg.RelImps {
				if strings.HasPrefix(imp, rel) {
					if !strings.Contains(imp, "/") {
						dep := in.dir2pkg[imp]
						tpkg.replacements[imp] = dep
					} else {
					}
				} else {
				}
			}
		}
	}
}

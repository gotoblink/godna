package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	log "github.com/golang/glog"
	"github.com/golangq/q"
	"github.com/wxio/godna/pb/config"
)

type Src struct {
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

func Cfg2Src(in config.Config, resp *Src) error {
	proc := &cfg2src{
		packages:     map[string]*pkage{},
		pkgWalkOrder: pkgSorter{},
		longestStr:   0,
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

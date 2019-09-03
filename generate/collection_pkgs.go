package generate

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

func (proc *GoPackages) Process(rootOutDir string, cfg *config.Config) (string, error) {
	pkgs, maxPkgLen, maxRelDirLen, err := collectFilesAndImports(cfg.SrcDir, cfg.GetGoPackagePrefix())
	if err != nil {
		return "", err
	}
	proc.Pkgs = pkgs
	proc.MaxPkgLen = maxPkgLen
	proc.MaxRelDirLen = maxRelDirLen
	return "", nil
}

type protoFile struct {
	File    string
	Dir     string
	Imports []string
}
type goPkgName struct {
	name string
	dir  string
}
type goProtoPkg map[string][]protoFile

// collectFilesAndImports
func collectFilesAndImports(srcDir string, goPkgPrefix string) (_ []*goPkg2, maxPkgLen, maxRelDirLen int, _ error) {
	if goPkgPrefix == "" {
		return nil, maxPkgLen, maxRelDirLen, fmt.Errorf("error go package prefix is manditory")
	}
	pkgs := []*goPkg2{}
	pfs, ordered_pkgnames, err := collectFiles(srcDir)
	if err != nil {
		return nil, maxPkgLen, maxRelDirLen, err
	}
	for _, pn := range ordered_pkgnames {
		if !strings.HasPrefix(pn.name, goPkgPrefix) {
			continue
		}
		gp2 := &goPkg2{
			Pkg:    pn.name,
			RelDir: pn.dir,
		}
		if len(pn.name) > maxPkgLen {
			maxPkgLen = len(pn.name)
		}
		if len(pn.dir) > maxRelDirLen {
			maxRelDirLen = len(pn.dir)
		}
		iex := map[string]bool{}
		for _, pf := range pfs[pn.name] {
			gp2.Files = append(gp2.Files, pf.File)
			for _, im := range pf.Imports {
				if _, ex := iex[im]; !ex {
					for _, localpkg := range ordered_pkgnames {
						if !strings.HasPrefix(localpkg.name, goPkgPrefix) {
							continue
						}
						if strings.HasSuffix(localpkg.dir, im) {
							gp2.Imports = append(gp2.Imports, localpkg.dir)
							break
						}
					}
				} else {
					iex[im] = true
				}
			}
		}
		pkgs = append(pkgs, gp2)
	}
	return pkgs, maxPkgLen, maxRelDirLen, nil
}

var goPkgOptRe = regexp.MustCompile(`(?m)^option\s+go_package\s*=\s*([^ ]+);`)
var protoImportRe = regexp.MustCompile(`(?m)^import\s+"(.*)/[^/]+.proto";`)

func collectFiles(srcDir string) (goProtoPkg, []goPkgName, error) {
	gpp := goProtoPkg{}
	orderPkgName := []goPkgName{}
	walkCollect := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || !strings.HasSuffix(path, ".proto") {
			return nil
		}
		pf := protoFile{}
		if rel, err := filepath.Rel(srcDir, path); err != nil {
			return err
		} else {
			pf.File = filepath.Base(rel)
			pf.Dir = filepath.Dir(rel)
		}
		content, err := ioutil.ReadFile(path)
		if err != nil {
			return err
		}
		//
		pkgname := ""
		if match := goPkgOptRe.FindSubmatch(content); len(match) > 0 {
			if pkgname, err = strconv.Unquote(string(match[1])); err != nil {
				return err
			}
		} else {
			if strings.HasPrefix(path, "store") {
				return fmt.Errorf("No 'option go_package = <pkg>' in file %s\n", path)
			} else {
				fmt.Fprintf(os.Stderr, "Warning no 'option go_package = <pkg>' in file %s\n", path)
				return nil
			}
		}
		if p := strings.IndexRune(pkgname, ';'); p > 0 {
			pkgname = pkgname[:p]
		}
		if pkgname == "" {
			return fmt.Errorf("No package in file %s\n", path)
		}
		//
		protoImportMatch := protoImportRe.FindAllSubmatch(content, -1)
		for _, m := range protoImportMatch {
			pf.Imports = append(pf.Imports, string(m[1]))
		}
		//
		// fs := gpp[pkgname]
		if fs, ex := gpp[pkgname]; !ex {
			orderPkgName = append(orderPkgName, goPkgName{pkgname, pf.Dir})
		} else {
			if fs[0].Dir != pf.Dir {
				return fmt.Errorf("go packages in different folder %+#v %+#v\n", fs[0], pf)
			}
		}
		gpp[pkgname] = append(gpp[pkgname], pf)
		// gpp[pkgname] = fs
		return nil
	}
	if err := filepath.Walk(srcDir, walkCollect); err != nil {
		return nil, nil, err
	}
	return gpp, orderPkgName, nil
}

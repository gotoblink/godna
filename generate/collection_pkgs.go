package generate

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

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

func (proc *GoPackages) Process(cmd *generate) (string, error) {
	goPkgPrefix := cmd.cfg.GetGoPackagePrefix()
	if goPkgPrefix == "" {
		return "", fmt.Errorf("error go package prefix is manditory")
	}
	pfs, ordered_pkgnames, err := collectFiles(cmd.cfg.SrcDir)
	if err != nil {
		return "", err
	}
	proc.name2goPkg2 = map[string]*goPkg2{}
	for _, pn := range ordered_pkgnames {
		if !strings.HasPrefix(pn.name, goPkgPrefix) {
			continue
		}
		gp2 := &goPkg2{Pkg: pn.name, RelDir: pn.dir}
		proc.name2goPkg2[pn.name] = gp2
		if len(pn.name) > proc.MaxPkgLen {
			proc.MaxPkgLen = len(pn.name)
		}
		if len(pn.dir) > proc.MaxRelDirLen {
			proc.MaxRelDirLen = len(pn.dir)
		}
		for _, pf := range pfs[pn.name] {
			gp2.Files = append(gp2.Files, pf.File)
		}
		proc.Pkgs = append(proc.Pkgs, gp2)
	}
	//
	for _, pn := range ordered_pkgnames {
		if !strings.HasPrefix(pn.name, goPkgPrefix) {
			continue
		}
		gp2 := proc.name2goPkg2[pn.name]
		iex := map[string]bool{}
		for _, pf := range pfs[pn.name] {
			for _, im := range pf.Imports {
				if _, ex := iex[im]; !ex {
					for _, localpkg := range ordered_pkgnames {
						if !strings.HasPrefix(localpkg.name, goPkgPrefix) {
							continue
						}
						if strings.HasSuffix(localpkg.dir, im) {
							dep := proc.name2goPkg2[localpkg.name]
							gp2.Imports = append(gp2.Imports, dep)
							break
						}
					}
				} else {
					iex[im] = true
				}
			}
		}
	}
	return "", nil
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
		if fs, ex := gpp[pkgname]; !ex {
			orderPkgName = append(orderPkgName, goPkgName{pkgname, pf.Dir})
		} else {
			if fs[0].Dir != pf.Dir {
				return fmt.Errorf("go packages in different folder %+#v %+#v\n", fs[0], pf)
			}
		}
		gpp[pkgname] = append(gpp[pkgname], pf)
		return nil
	}
	if err := filepath.Walk(srcDir, walkCollect); err != nil {
		return nil, nil, err
	}
	return gpp, orderPkgName, nil
}

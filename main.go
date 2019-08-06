package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/jpillora/opts"
)

var (
	version string = "dev"
	date    string = "na"
	commit  string = "na"
)

type root struct {
}

type versionCmd struct {
	help string
}

func main() {
	opts.New(&root{}).
		Name("godna").
		EmbedGlobalFlagSet().
		Complete().
		Version(version).
		AddCommand(opts.New(&regen{OutputDir: "."}).Name("regen").
			ConfigPath("./.dna-cfg.json")).
		Parse().
		RunFatal()
}

func (r *versionCmd) Run() {
	fmt.Printf("version: %s\ndate: %s\ncommit: %s\n", version, date, commit)
	fmt.Println(r.help)
}

type regen struct {
	HostOwner string   // eg github.com/microsoft
	RepoName  string   // eg vscode
	Includes  []string // eg ../microsoft-dna/store/project
	RelImps   []string // eg [ "services-" ]
	SrcDir    string   // eg ../microsoft-dna/store/project
	OutputDir string   // eg .
	//
	packages     map[string]*pkage
	pkgWalkOrder []string
}

type pkage struct {
	files        []string
	replacements map[string]struct{}
	dirn         string
}

var goPkgOptRe = regexp.MustCompile(`(?m)^option go_package = (.*);`)
var protoImportRe = regexp.MustCompile(`(?m)^import "(.*)/[^/]+.proto";`)

func (in *regen) Run() error {
	fmt.Printf("%+v\n", in)
	in.packages = make(map[string]*pkage)
	if in.HostOwner == "" {
		log.Fatalf("--host_owner not set")
	}
	if in.RepoName == "" {
		log.Fatalf("--repo_name not set")
	}
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() || !strings.HasSuffix(path, ".proto") {
			return nil
		}
		if rel, err := filepath.Rel(in.SrcDir, path); err != nil {
			return err
		} else {
			err = in.goPkg(path, rel)
			if err != nil {
				return err
			}
			return nil
		}
	}
	if err := filepath.Walk(in.SrcDir, walkFn); err != nil {
		log.Fatal(err)
	}
	for _, pkg := range in.pkgWalkOrder {
		fmt.Printf("Package (protoc,go mod) %s \n", pkg)
		in.pkgDir(pkg)
		if !strings.HasPrefix(pkg, in.HostOwner) {
			continue
		}
		if out, err := in.protoc(pkg); err != nil {
			log.Fatalf("error executing protoc: %s\n%s", err, out)
		}
		if out, err := in.gomod(pkg); err != nil {
			log.Fatalf("error executing go mod init: %s\n%s", err, out)
		}
		if out, err := in.gomodReplace(pkg); err != nil {
			log.Fatalf("error executing go mod init: %s\n%s", err, out)
		}
	}
	for _, pkg := range in.pkgWalkOrder {
		if !strings.HasPrefix(pkg, in.HostOwner) {
			continue
		}
		fmt.Printf("Package (tidy,tag) %s \n", pkg)
		if out, err := in.gomodTidy(pkg); err != nil {
			log.Fatalf("error executing go mod tidy: %s\n%s", err, out)
		}
		// if out, err := gitTag(in.OutputDir, dirn); err != nil {
		// 	log.Fatalf("error executing git tag: %s\n%s", err, out)
		// }
	}

	return nil
}

func (in *regen) pkgDir(pkg string) {
	tpkg := in.packages[pkg]
	fnames := tpkg.files
	dirns := make(map[string]struct{})
	dirn := ""
	for _, fn := range fnames {
		dirns[filepath.Dir(fn)] = struct{}{}
		dirn = filepath.Dir(fn)
	}
	if len(dirns) != 1 {
		log.Fatalf("error files with same go package in more than one dir: %s\n", fnames)
	}
	tpkg.dirn = dirn
}

// goPkg reports the import path declared in the given file's
// `go_package` option. If the option is missing, goPkg returns empty string.
func (in *regen) goPkg(fname string, relpath string) error {
	content, err := ioutil.ReadFile(fname)
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
		fmt.Printf("No package in file %s\n", fname)
	}
	thisPkg, ex := in.packages[pkgName]
	if !ex {
		thisPkg = &pkage{
			replacements: make(map[string]struct{}),
		}
		in.packages[pkgName] = thisPkg
		in.pkgWalkOrder = append(in.pkgWalkOrder, pkgName)
	}
	//
	match := protoImportRe.FindAllSubmatch(content, -1)
	myname := fname[len(in.SrcDir):]
	if myname[0] == '/' {
		myname = myname[1:]
	}
	mymod := myname[:strings.Index(myname, "/")]
	for _, m := range match {
		for j, n := range m {
			_, _ = j, n
			if strings.HasPrefix(string(m[1]), mymod) {
				continue
			}
			for _, rel := range in.RelImps {
				if strings.HasPrefix(string(m[1]), rel) {
					if !strings.Contains(string(m[1]), "/") {
						thisPkg.replacements[string(m[1])] = struct{}{}
					} else {
					}
				} else {
				}
			}
		}
	}
	thisPkg.files = append(thisPkg.files, relpath)
	return nil
}

// protoc executes the "protoc" command on files named in fnames,
// passing go_out and include flags specified in goOut and includes respectively.
// protoc returns combined output from stdout and stderr.
func (in *regen) protoc(pkg string) ([]byte, error) {
	cmd := exec.Command("protoc")
	oAbs, _ := filepath.Abs(in.OutputDir)
	args := []string{"--go_out=plugins=grpc,paths=source_relative:" + oAbs}
	src, _ := filepath.Abs(in.SrcDir)
	cmd.Dir = src
	// args = append(args, "-I"+srcDir)
	for _, inc := range in.Includes {
		in, _ := filepath.Abs(inc)
		args = append(args, "-I"+in)
	}
	args = append(args, "-I.")
	args = append(args, in.packages[pkg].files...)
	cmd.Args = append(cmd.Args, args...)
	// fmt.Printf("wd: %v, cmd %+v\n", src, cmd.Args)
	return cmd.CombinedOutput()
}

func (in *regen) gomod(pkgName string) ([]byte, error) {
	tpkg := in.packages[pkgName]
	if strings.Contains(tpkg.dirn, "/") {
		return nil, nil
	}
	gm := filepath.Join(in.OutputDir, tpkg.dirn, "go.mod")
	if _, err := os.Open(gm); err == nil {
		return nil, nil
	}
	cmd := exec.Command("go")
	cmd.Dir = filepath.Join(in.OutputDir, tpkg.dirn)
	args := []string{
		"mod",
		"init",
		pkgName,
	}
	cmd.Args = append(cmd.Args, args...)
	// fmt.Printf("go mod init %+v\n", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return out, err
	}
	return nil, nil
}

func (in *regen) collect(pkg string, imports []string) []string {
	tp := in.packages[pkg]
	for k, _ := range tp.replacements {
		imports = append(imports, k)
		imports = in.collect(in.HostOwner+"/"+in.RepoName+"/"+k, imports)
	}
	return imports
}

func (in *regen) gomodReplace(pkg string) ([]byte, error) {
	tp := in.packages[pkg]
	if strings.Contains(tp.dirn, "/") {
		return nil, nil
	}
	imports := in.collect(pkg, nil)
	// fmt.Printf("pkg: %s imports: %v\n", pkg, imports)
	for _, k := range imports {
		relPath := strings.Repeat("../", strings.Count(k, "/")+1)
		cmd := exec.Command("go")
		cmd.Dir = filepath.Join(in.OutputDir, tp.dirn)
		args := []string{
			"mod",
			"edit",
			"-replace=" + in.HostOwner + "/" + in.RepoName + "/" + k + "=" + relPath + k,
		}
		cmd.Args = append(cmd.Args, args...)
		// fmt.Printf("go mod edit %+v\n", cmd.Args)
		if out, err := cmd.CombinedOutput(); err != nil {
			return out, err
		}
	}
	return nil, nil
}

func (in *regen) gomodTidy(pkg string) ([]byte, error) {
	tp := in.packages[pkg]
	if strings.Contains(tp.dirn, "/") {
		return nil, nil
	}
	cmd := exec.Command("go")
	cmd.Dir = filepath.Join(in.OutputDir, tp.dirn)
	args := []string{
		"mod",
		"tidy",
		"-v",
	}
	cmd.Args = append(cmd.Args, args...)
	// fmt.Printf("wd: %s go %+v\n", cmd.Dir, cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return out, err
	}
	return nil, nil
}

func (in *regen) gitTag(pkg string) ([]byte, error) {
	tp := in.packages[pkg]
	if strings.Contains(tp.dirn, "/") {
		return nil, nil
	}
	cmd := exec.Command("git")
	cmd.Dir = filepath.Join(in.OutputDir, tp.dirn)
	args := []string{
		"tag",
		tp.dirn + "/v1.0.1",
	}
	cmd.Args = append(cmd.Args, args...)
	// fmt.Printf("git %+v\n", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return out, err
	}
	return nil, nil
}

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/golangq/q"
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
		AddCommand(
			opts.New(
				&regen{
					OutputDir: ".",
					// Pass: []string{
					// 	"protoc,modinit,modreplace",
					// 	"modtidy",
					// 	"gittag",
					// },
				}).
				Name("regen").
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
	Pass      []string `help:"default [\"protoc,modinit,modreplace\", \"modtidy\", \"gittag\"]"`
	//
	packages     map[string]*pkage
	pkgWalkOrder []string
	sems         map[string]map[int64]Semvers
	longestStr   int
}

type pkage struct {
	files        []string
	replacements map[string]struct{}
	dirn         string
}

var goPkgOptRe = regexp.MustCompile(`(?m)^option go_package = (.*);`)
var protoImportRe = regexp.MustCompile(`(?m)^import "(.*)/[^/]+.proto";`)
var vxMod = regexp.MustCompile(`^[^/]+/v(\d+)$`)
var vDir = regexp.MustCompile(`^/v(\d+)(/.+)?$`)
var ignoreGitStatus = regexp.MustCompile(`^.. v(\d+)/(.+)?$`)
var pathSemver = regexp.MustCompile(`^([^/]+)/v(\d+)\.(\d+)\.(\d+)$`)

func (in *regen) Run() error {
	fmt.Printf("%+v\n", in)
	in.packages = make(map[string]*pkage)
	if in.HostOwner == "" {
		log.Fatalf("--host_owner not set")
	}
	if in.RepoName == "" {
		log.Fatalf("--repo_name not set")
	}
	pkgExecs := map[string]pkgExec{
		"protoc":     in.protoc,
		"modinit":    in.gomod,
		"modreplace": in.gomodReplace,
		"modtidy":    in.gomodTidy,
		"gittag":     in.gitTag,
	}
	if len(in.Pass) == 0 {
		in.Pass = []string{
			"protoc,modinit,modreplace",
			"modtidy",
			"gittag",
		}
	}
	if err := filepath.Walk(in.SrcDir, in.walkFn); err != nil {
		log.Fatal(err)
	}
	for _, pkg := range in.pkgWalkOrder {
		in.pkgDir(pkg)
	}
	in.sems = gitGetTagSemver()
	for pi, actions := range in.Pass {
		for _, pkg := range in.pkgWalkOrder {
			if !strings.HasPrefix(pkg, in.HostOwner) {
				continue
			}
			fmt.Printf("pass:%d %s %s", pi+1, pkg, strings.Repeat(" ", in.longestStr-len(pkg)))
			for _, ac := range strings.Split(actions, ",") {
				if acf, ex := pkgExecs[ac]; ex {
					if out, msg, err := acf(pkg); err != nil {
						log.Fatalf("error executing %s: msg: %s %s\n%s", ac, msg, err, out)
					} else {
						fmt.Printf(" %s:%s", ac, msg)
					}
				} else {
					log.Fatalf("action does not exist %s", ac)
				}
			}
			fmt.Printf("\n")
		}

	}
	return nil
}

type pkgExec func(pkg string) (out []byte, msg string, err error)

func (in *regen) walkFn(path string, info os.FileInfo, err error) error {
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
	if !strings.HasSuffix(fname, ".proto") {
		return nil
	}
	// TODO make sure there are no v2 files in v1 (root) dir
	// TODO make sure the are no vX in vY directory
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
		if len(pkgName) > in.longestStr {
			in.longestStr = len(pkgName)
		}
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
func (in *regen) protoc(pkg string) ([]byte, string, error) {
	cmd := exec.Command("protoc")
	oAbs, _ := filepath.Abs(in.OutputDir)
	args := []string{"--go_out=plugins=micro,paths=source_relative:" + oAbs}
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
	out, err := cmd.CombinedOutput()
	return out, fmt.Sprintf("files-%d", len(in.packages[pkg].files)), err
}

func (in *regen) gomod(pkgName string) ([]byte, string, error) {
	tpkg := in.packages[pkgName]
	if strings.Contains(tpkg.dirn, "/") {
		return nil, "skipped", nil
	}
	gm := filepath.Join(in.OutputDir, tpkg.dirn, "go.mod")
	if _, err := os.Open(gm); err == nil {
		return nil, "exists", nil
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
		return out, "error", err
	}
	return nil, "created", nil
}

func (in *regen) collect(pkg string, imports []string) []string {
	tp := in.packages[pkg]
	for k, _ := range tp.replacements {
		imports = append(imports, k)
		imports = in.collect(in.HostOwner+"/"+in.RepoName+"/"+k, imports)
	}
	return imports
}

func (in *regen) gomodReplace(pkg string) ([]byte, string, error) {
	tp := in.packages[pkg]
	if strings.Contains(tp.dirn, "/") {
		return nil, "skipped", nil
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
			return out, "error", err
		}
	}
	return nil, fmt.Sprintf("replaced-%d", len(imports)), nil
}

func (in *regen) gomodTidy(pkg string) ([]byte, string, error) {
	tp := in.packages[pkg]
	if strings.Contains(tp.dirn, "/") {
		return nil, "skipped", nil
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
		return out, "error", err
	}
	return nil, "tidied", nil
}

func (in *regen) gitTag(pkg string) ([]byte, string, error) {
	tp := in.packages[pkg]
	majorVer := pkgModVersion(tp.dirn)
	if majorVer == -1 {
		return nil, "skipped", nil
	}
	wordDir := filepath.Join(in.OutputDir, tp.dirn)
	dirty, found := gitModDirty(wordDir)
	if !dirty {
		return nil, "clean", nil
	}
	q.Q("  dirty - major ver %v %v\n", majorVer, found)
	ds := strings.Split(tp.dirn, "/")
	sems := in.sems[ds[0]][majorVer]
	sort.Sort(sems)
	var sem Semver
	if len(sems) == 0 {
		sem.Major++
	} else {
		// TODO proto check for consistency
		sem = sems[0]
		sem.Minor++
	}
	tag := fmt.Sprintf("%s/%s", ds[0], sem)
	q.Q("  next semver for %s %s\n", tp.dirn, tag)
	return nil, fmt.Sprintf("commit&tagged-'%s'", tag), nil
}

// get version from directory name
// some_dir => 1
// some_dir/vXX => XX
// some_dir/other => -1
// some_dir/vXX/other => -1
func pkgModVersion(dirname string) int64 {
	if match := vxMod.FindStringSubmatch(dirname); len(match) > 0 {
		if majorVer, err := strconv.ParseInt(match[1], 10, 32); err != nil {
			log.Fatalf("keh %v", err)
		} else {
			return majorVer
		}
	}
	if !strings.Contains(dirname, "/") {
		return 1
	}
	return -1
}

type Semvers []Semver
type Semver struct {
	Major, Minor, Patch int64
}

func (a Semver) String() string {
	return fmt.Sprintf("v%d.%d.%d", a.Major, a.Minor, a.Patch)
}

func (a Semvers) Len() int      { return len(a) }
func (a Semvers) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a Semvers) Less(i, j int) bool {
	x, y := a[i], a[j]
	if x.Major < y.Major {
		return true
	}
	if x.Minor < y.Minor {
		return true
	}
	if x.Patch < y.Patch {
		return true
	}
	return false
}

func gitGetTagSemver() map[string]map[int64]Semvers {
	ret := map[string]map[int64]Semvers{}
	cmd := exec.Command("git")
	// cmd.Dir = filepath.Join(in.OutputDir, tp.dirn)
	args := []string{
		"tag",
	}
	cmd.Args = append(cmd.Args, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatal(err)
	}
	scan := bufio.NewScanner(bytes.NewBuffer(out))
	for scan.Scan() {
		line := scan.Text()
		match := pathSemver.FindStringSubmatch(line)
		if len(match) == 0 {
			q.Q("tag does look right %v\n", line)
			continue
		}
		q.Q("tag %v\n", line)
		modName := match[1]
		ma, _ := strconv.ParseInt(match[2], 10, 64)
		mi, _ := strconv.ParseInt(match[3], 10, 64)
		pa, _ := strconv.ParseInt(match[4], 10, 64)
		sem := Semver{Major: ma, Minor: mi, Patch: pa}
		sems, ex := ret[modName]
		if !ex {
			sems = make(map[int64]Semvers)
			sems[ma] = Semvers{sem}
			ret[modName] = sems
		} else {
			sems[ma] = append(sems[ma], sem)
		}
		ret[modName] = sems
	}
	return ret
}

func gitModDirty(wordDir string) (dirty bool, found []string) {
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			return nil
		}
		cur := path[len(wordDir):]
		if vDir.MatchString(cur) {
			return filepath.SkipDir
		}
		// fmt.Printf("cur '%v'\n", cur)
		cmd := exec.Command("git")
		cmd.Dir = filepath.Join(wordDir, cur)
		args := []string{
			"status",
			"--short",
			".",
		}
		cmd.Args = append(cmd.Args, args...)
		// fmt.Printf("git %+v\n", cmd.Args)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return err
		}
		//
		scan := bufio.NewScanner(bytes.NewBuffer(out))
		for scan.Scan() {
			line := scan.Text()
			if ignoreGitStatus.MatchString(line) {
				continue
			}
			dirty = true
			found = append(found, line)
			return filepath.SkipDir
		}
		return nil
	}
	if err := filepath.Walk(wordDir, walkFn); err != nil {
		log.Fatal(err)
	}
	return
}

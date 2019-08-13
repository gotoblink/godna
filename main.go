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
	"sort"
	"strconv"
	"strings"

	log "github.com/golang/glog"
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
	HostOwner string   `help:"host and owner of the repo of the generated code eg. github.com/microsoft"`
	RepoName  string   `help:"repo name of the generated code eg go-vscode"`
	Includes  []string `help:"protoc path -I eg [ '../microsoft-dna/vendor' ]"`
	RelImps   []string `help:"prefix of go.mod local replacements. eg [ 'services-' ] - todo remove"`
	Require   []string `help:"list of go mod edit -require= eg [ 'github.com/golangq/q@v1.0.7' ]"`
	SrcDir    string   `help:"source directory eg ../microsoft-dna/store/project - note not included as a -I"`
	OutputDir string   `help:"output directory eg ."`
	Pass      []string `help:"common separated list of commands to run. Possible commands are protoc,modinit,modrequire,modreplace,modtidy,gittag (default [\"protoc,modinit,modrequire,modreplace\", \"modtidy\", \"gittag\"])"`
	Plugin    []string `help:"Name and path of a pluging eg protoc-gen-NAME=path/to/mybinary. Much also specify --generator, does not imply it  is present"`
	Generator []string `help:"Name and params a genertor. See defaut for an example. Turns into '--NAME_out=PARMAS:OUTPUT_DIR'. (default 'go=paths=source_relative')"`
	//
	packages     map[string]*pkage
	pkgWalkOrder []string
	sems         map[string]map[int64]Semvers
	longestStr   int
	localName    map[string]struct{}
	generators   []generator
}

type generator struct {
	name   string
	params []keyval
}

type keyval struct {
	key   string
	value string
}

type pkage struct {
	files            []string
	replacements     map[string]struct{}
	dirn             string
	protoImportMatch [][][]byte
	mymod            string
	gitDescribe      string
	source           string
}

func (in *regen) Run() error {
	var err error
	in.SrcDir, err = filepath.Abs(in.SrcDir)
	if err != nil {
		return err
	}
	for i, inc := range in.Includes {
		in.Includes[i], err = filepath.Abs(inc)
		if err != nil {
			return err
		}
	}
	in.OutputDir, err = filepath.Abs(in.OutputDir)
	if err != nil {
		return err
	}
	os.Chdir(in.OutputDir)
	// fmt.Printf("%+v\n", in)
	in.packages = make(map[string]*pkage)
	in.localName = make(map[string]struct{})
	if in.HostOwner == "" {
		log.Error("--host_owner not set")
		os.Exit(1)
	}
	if in.RepoName == "" {
		log.Errorf("--repo_name not set")
		os.Exit(1)
	}
	pkgExecs := map[string]pkgExec{
		"protoc":     in.protoc,
		"modinit":    in.gomod,
		"modrequire": in.modRequire,
		"modreplace": in.gomodReplace,
		"modtidy":    in.gomodTidy,
		"gittag":     in.gitTag,
	}
	if len(in.Pass) == 0 {
		in.Pass = []string{
			"protoc,modinit,modrequire,modreplace",
			"modtidy",
			"gittag",
		}
	}
	if len(in.Generator) == 0 {
		in.Generator = []string{"go=paths=source_relative"}
	}
	for _, ges := range in.Generator {
		name := ges
		if i := strings.Index(ges, "="); i > -1 {
			name = ges[:i]
			gen := generator{name: name}
			paramstr := ges[i+1:]
			params := strings.Split(paramstr, ",")
			for _, param := range params {
				kv := strings.Split(param, "=")
				if len(kv) != 2 {
					fmt.Printf("Invalid generator, much be of the form 'name[=[key=value][,key=value]*]?' given '%s'", ges)
				}
				gen.params = append(gen.params, keyval{kv[0], kv[1]})
			}
			in.generators = append(in.generators, gen)
		} else {
			gen := generator{name: name}
			in.generators = append(in.generators, gen)
		}
	}
	if err := filepath.Walk(in.SrcDir, in.walkFnSrcDir); err != nil {
		log.Error(err)
		os.Exit(1)
	}
	q.Q(in.localName)
	for _, pkg := range in.pkgWalkOrder {
		// in.goModReplacements(pkg)
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
						log.Errorf("error executing %s: msg: %s %s\n%s", ac, msg, err, out)
						os.Exit(1)
					} else {
						fmt.Printf(" %s:%s", ac, msg)
					}
				} else {
					log.Errorf("action does not exist %s", ac)
					os.Exit(1)
				}
			}
			fmt.Printf("\n")
		}

	}
	return nil
}

type pkgExec func(pkg string) (out []byte, msg string, err error)

func (in *regen) walkFnSrcDir(path string, info os.FileInfo, err error) error {
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
		log.Errorf("error files with same go package in more than one dir: %s\n", fnames)
		os.Exit(1)
	}
	tpkg.dirn = dirn
	{ //
		cmd := exec.Command("git")
		cmd.Dir = filepath.Join(in.SrcDir, dirn)
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
		cmd.Dir = filepath.Join(in.SrcDir, dirn)
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

var goPkgOptRe = regexp.MustCompile(`(?m)^option go_package = (.*);`)
var protoImportRe = regexp.MustCompile(`(?m)^import "(.*)/[^/]+.proto";`)

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
	thisPkg.files = append(thisPkg.files, relpath)
	//
	thisPkg.protoImportMatch = protoImportRe.FindAllSubmatch(content, -1)
	myname := fname[len(in.SrcDir):]
	if myname[0] == '/' {
		myname = myname[1:]
	}
	thisPkg.mymod = myname[:strings.LastIndex(myname, "/")]
	in.localName[thisPkg.mymod] = struct{}{}
	for _, m := range thisPkg.protoImportMatch {
		for j, n := range m {
			_, _ = j, n
			if strings.HasPrefix(string(m[1]), thisPkg.mymod) {
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
	return nil
}

func (in *regen) goModReplacements(pkg string) {
	tp := in.packages[pkg]
	q.Q(pkg)
	for _, m := range tp.protoImportMatch {
		for j, n := range m {
			_, _ = j, n
			if strings.HasPrefix(string(m[1]), tp.mymod) {
				continue
			}
			q.Q(string(m[1]))
			if _, ex := in.localName[string(m[1])]; ex {
				q.Q("yes")
				tp.replacements[string(m[1])] = struct{}{}
			} else {
				q.Q("no")
			}
			// for _, rel := range in.RelImps {
			// 	if strings.HasPrefix(string(m[1]), rel) {
			// 		if !strings.Contains(string(m[1]), "/") {
			// 			tp.replacements[string(m[1])] = struct{}{}
			// 		} else {
			// 		}
			// 	} else {
			// 	}
			// }
		}
	}
}

// protoc executes the "protoc" command on files named in fnames,
// passing go_out and include flags specified in goOut and includes respectively.
// protoc returns combined output from stdout and stderr.
func (in *regen) protoc(pkg string) ([]byte, string, error) {
	cmd := exec.Command("protoc")
	args := []string{}
	for _, gen := range in.generators {
		arg := "--" + gen.name + "_out"
		if len(gen.params) > 0 {
			arg += "="
			for i, kv := range gen.params {
				if i != 0 {
					arg += ","
				}
				arg += kv.key + "=" + kv.value
			}
		}
		arg += ":" + in.OutputDir
		args = append(args, arg)
	}
	// args := []string{"--go_out=plugins=micro,paths=source_relative:" + oAbs}
	cmd.Dir = in.SrcDir
	// args = append(args, "-I"+srcDir)
	for _, inc := range in.Includes {
		args = append(args, "-I"+inc)
	}
	args = append(args, "-I.")
	args = append(args, in.packages[pkg].files...)
	cmd.Args = append(cmd.Args, args...)
	// fmt.Printf("wd: %v, cmd %+v\n", src, cmd.Args)
	out, err := cmd.CombinedOutput()
	return out, fmt.Sprintf("files-%d", len(in.packages[pkg].files)), err
}

func (in *regen) gomod(pkgName string) ([]byte, string, error) {
	tp := in.packages[pkgName]
	majorVer := pkgModVersion(tp.dirn)
	if majorVer == -1 {
		return nil, "skipped", nil
	}
	gm := filepath.Join(in.OutputDir, tp.dirn, "go.mod")
	if _, err := os.Open(gm); err == nil {
		return nil, "exists", nil
	}
	cmd := exec.Command("go")
	cmd.Dir = filepath.Join(in.OutputDir, tp.dirn)
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
	q.Q(pkg)
	for k, _ := range tp.replacements {
		q.Q("tp.replacements", k)
		imports = append(imports, k)
		imports = in.collect(in.HostOwner+"/"+in.RepoName+"/"+k, imports)
	}
	return imports
}

func (in *regen) modRequire(pkg string) ([]byte, string, error) {
	tp := in.packages[pkg]
	majorVer := pkgModVersion(tp.dirn)
	if majorVer == -1 {
		return nil, "skipped", nil
	}
	for _, k := range in.Require {
		cmd := exec.Command("go")
		cmd.Dir = filepath.Join(in.OutputDir, tp.dirn)
		args := []string{
			"mod",
			"edit",
			"-require=" + k,
		}
		cmd.Args = append(cmd.Args, args...)
		if out, err := cmd.CombinedOutput(); err != nil {
			return out, "error", err
		}
	}
	return nil, fmt.Sprintf("required-%d", len(in.Require)), nil
}

func (in *regen) gomodReplace(pkg string) ([]byte, string, error) {
	tp := in.packages[pkg]
	majorVer := pkgModVersion(tp.dirn)
	if majorVer == -1 {
		return nil, "skipped", nil
	}
	imports := in.collect(pkg, nil)
	q.Q("pkg: %s imports: %v\n", pkg, imports)
	for _, k := range imports {
		relPath := strings.Repeat("../", strings.Count(tp.dirn, "/")+1)
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
	majorVer := pkgModVersion(tp.dirn)
	if majorVer == -1 {
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
	} else {
		q.Q(string(out))
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
	dirty, found := gitModAddDirty(wordDir, tp.dirn, majorVer)
	if !dirty {
		return nil, "clean", nil
	}
	q.Q("  dirty - major ver %v %v\n", majorVer, found)
	{ //
		cmd := exec.Command("git")
		args := []string{"commit", "-m", tp.source + " (" + tp.gitDescribe + ")"}
		cmd.Args = append(cmd.Args, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return out, "error", err
		}
	} //
	ds := strings.Split(tp.dirn, "/")
	sems := in.sems[ds[0]][majorVer]
	sort.Sort(sems)
	sem := Semver{Major: majorVer}
	if len(sems) == 0 {
		// sem.Minor = 0
	} else {
		// TODO proto check for consistency
		sem = sems[0]
		sem.Minor++
	}
	tag := fmt.Sprintf("%s/%s", ds[0], sem)
	{ //
		cmd := exec.Command("git")
		args := []string{"tag", tag}
		cmd.Args = append(cmd.Args, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return out, "error", err
		}
	} //
	q.Q("  next semver for %s %s\n", tp.dirn, tag)
	return nil, fmt.Sprintf("commit&tagged-'%s'", tag), nil
}

var vxMod = regexp.MustCompile(`^[^/]+/v(\d+)$`)

// get version from directory name
// some_dir => 1
// some_dir/vXX => XX
// some_dir/other => -1
// some_dir/vXX/other => -1
func pkgModVersion(dirname string) int64 {
	if match := vxMod.FindStringSubmatch(dirname); len(match) > 0 {
		if majorVer, err := strconv.ParseInt(match[1], 10, 32); err != nil {
			log.Errorf("keh %v", err)
			os.Exit(1)
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
	if x.Major > y.Major {
		return true
	}
	if x.Minor > y.Minor {
		return true
	}
	if x.Patch > y.Patch {
		return true
	}
	return false
}

var pathSemver = regexp.MustCompile(`^([^/]+)/v(\d+)\.(\d+)\.(\d+)$`)

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
		log.Error(err)
		os.Exit(1)
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

var ignoreGitStatus = regexp.MustCompile(`^.*v(\d+)/(.+)?$`)

func gitModAddDirty(wordDir string, dirn string, major int64) (dirty bool, found []string) {
	walkFn := func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Warning(err)
			return err
		}
		mma := pkgModVersion(dirn)
		if mma == -1 || mma != major {
			return filepath.SkipDir
		}
		// if vDir.MatchString(dirn) {
		// 	return filepath.SkipDir
		// }
		if !info.IsDir() {
			return nil
		}
		// fmt.Printf("cur '%v'\n", cur)
		cmd := exec.Command("git")
		cmd.Dir = wordDir
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
			return err
		}
		//
		scan := bufio.NewScanner(bytes.NewBuffer(out))
		for scan.Scan() {
			line := scan.Text()
			//here add
			if len(line) > 3 {
				fname := line[3:]
				if line[0] == 'R' {
					fname = fname[strings.Index(fname, " -> ")+4:]
				}
				fname = fname[len(dirn)+1:]
				if ignoreGitStatus.MatchString(fname) {
					continue
				}
				cmd := exec.Command("git")
				cmd.Dir = wordDir
				args := []string{}
				if line[1] != ' ' {
					args = []string{
						"add",
						fname,
					}
				} else {
					continue
				}
				cmd.Args = append(cmd.Args, args...)
				// fmt.Printf("git %+v\n", cmd.Args)
				out, err := cmd.CombinedOutput()
				if err != nil {
					log.Warningf("\nerr:%v\ncmd%v\n out\n`%v`\n", cmd, err, string(out))
					return err
				}
			} else {
				q.Q(line)
				log.Warning("3?")
			}
			dirty = true
			found = append(found, line)
		}
		return nil
	}
	if err := filepath.Walk(wordDir, walkFn); err != nil {
		log.Error(err)
		os.Exit(1)
	}
	return
}

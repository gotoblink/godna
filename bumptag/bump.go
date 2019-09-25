package bumptag

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"

	log "github.com/golang/glog"

	"github.com/wxio/godna/internal/utils"
)

// Bump subcommdnad
type Bump struct {
	OutputDir       string   `opts:"mode=arg" help:"output directory eg ."`
	BranchMinorBump []string `help:"minor bump for these branches, else bumps patch (default [master])"`
}

// New Bump constructor
func New() *Bump {
	return &Bump{}
}

// Run subcommand
func (cmd *Bump) Run() error {
	if len(cmd.BranchMinorBump) == 0 {
		cmd.BranchMinorBump = []string{"master"}

	}
	branch, err := getCurrentBranch(cmd.OutputDir)
	if err != nil {
		return err
	}
	curTag, err := getLastTag(cmd.OutputDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "no current tag using 0.0.0 as current err:%v", err)
	}
	for _, t := range cmd.BranchMinorBump {
		if branch == t {
			curTag.Minor++
			curTag.Patch = 0
			fmt.Printf("%v\n", curTag)
			return nil
		}
	}
	curTag.Patch++
	fmt.Printf("%v\n", curTag)
	return nil
}

func getCurrentBranch(outdir string) (string, error) {
	cmd := exec.Command("git")
	cmd.Dir = outdir
	args := []string{"rev-parse", "--abbrev-ref", "HEAD"}
	cmd.Args = append(cmd.Args, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warningf("err: %v out:%v", err, string(out))
		return "", err
	}
	line := strings.TrimSpace(string(out))
	return line, nil
}

var describeRE = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)(-\d+-[0-9a-z]+.*)?`)

func getLastTag(outdir string) (utils.Semver, error) {
	cmd := exec.Command("git")
	cmd.Dir = outdir
	args := []string{"describe", "--tags"}
	cmd.Args = append(cmd.Args, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warningf("err: %v out:%v", err, string(out))
		return utils.Semver{}, err
	}
	line := strings.TrimSpace(string(out))
	match := describeRE.FindStringSubmatch(line)
	if len(match) == 0 {
		return utils.Semver{}, fmt.Errorf("tag didn't look like a semver out: '%s'", line)
	}
	ma, _ := strconv.ParseInt(match[1], 10, 64)
	mi, _ := strconv.ParseInt(match[2], 10, 64)
	pa, _ := strconv.ParseInt(match[3], 10, 64)
	sem := utils.Semver{Major: ma, Minor: mi, Patch: pa}
	return sem, nil
}

var semverRE = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

func getSemTags(outdir string) (utils.Semvers, error) {
	ret := utils.Semvers{}
	cmd := exec.Command("git")
	cmd.Dir = outdir
	args := []string{"tag"}
	cmd.Args = append(cmd.Args, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Warningf("err: %v", err)
		return nil, err
	}
	scan := bufio.NewScanner(bytes.NewBuffer(out))
	for scan.Scan() {
		line := scan.Text()
		match := semverRE.FindStringSubmatch(line)
		if len(match) == 0 {
			continue
		}
		ma, _ := strconv.ParseInt(match[1], 10, 64)
		mi, _ := strconv.ParseInt(match[2], 10, 64)
		pa, _ := strconv.ParseInt(match[3], 10, 64)
		sem := utils.Semver{Major: ma, Minor: mi, Patch: pa}
		ret = append(ret, sem)
	}
	sort.Sort(ret)
	return ret, nil
}

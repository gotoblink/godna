package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	log "github.com/golang/glog"
	"github.com/golangq/q"
)

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

var pathSemver = regexp.MustCompile(`^(.+)/v(\d+)\.(\d+)\.(\d+)$`)

func gitGetTagSemver(inside_repo string) (map[string]map[int64]Semvers, error) {
	ret := map[string]map[int64]Semvers{}
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
	return ret, nil
}

func filecopy(src, dst string) (int64, error) {
	sourceFileStat, err := os.Stat(src)
	if err != nil {
		return 0, fmt.Errorf("stat %v", err)
	}
	if !sourceFileStat.Mode().IsRegular() {
		return 0, fmt.Errorf("%s is not a regular file", src)
	}
	source, err := os.Open(src)
	if err != nil {
		return 0, fmt.Errorf("open: %v", err)
	}
	defer source.Close()
	destination, err := os.Create(dst)
	if err != nil {
		return 0, fmt.Errorf("create: %v", err)
	}
	defer destination.Close()
	nBytes, err := io.Copy(destination, source)
	return nBytes, err
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

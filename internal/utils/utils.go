package utils

import (
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
func PkgModVersion(dirname string) int64 {
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

func Filecopy(src, dst string) (int64, error) {
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

func Describe(src string) (remote string, desc string) {
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

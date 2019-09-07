package utils

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"strconv"
	"strings"

	log "github.com/golang/glog"
	"github.com/golangq/q"
)

var pathSemver = regexp.MustCompile(`^(.+)/v(\d+)\.(\d+)\.(\d+)$`)

func GitGetTagSemver(inside_repo string) (map[string]map[int64]Semvers, string, error) {
	pseudo_version := ""
	{
		cmd := exec.Command("git")
		cmd.Dir = inside_repo
		// cmd.Dir = filepath.Join(in.OutputDir, tp.dirn)
		args := []string{
			"log",
			"-n", "1",
			`--pretty=format:%ad:%H`,
			"--decorate", `--date=format:%Y%m%d%H%M%S`,
		}
		cmd.Args = append(cmd.Args, args...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			return nil, "", fmt.Errorf("%v\n---\n%s\n---\n", err, string(out))
		}
		parts := strings.Split(string(out), ":")
		pseudo_version = fmt.Sprintf("v0.0.0-%s-%12s", parts[0], parts[1])
	}
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
		return nil, "", err
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
	return ret, pseudo_version, nil
}

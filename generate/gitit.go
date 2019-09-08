package generate

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/golangq/q"
	"github.com/wxio/godna/internal/utils"
)

func git_add(gcmd *generate, gomod *goMod, podPath, localPkgPart string) (string, error) {
	if len(gomod.dirty) == 0 {
		return "", nil
	}
	outAbs := filepath.Join(gcmd.OutputDir, podPath, localPkgPart)
	cmd := exec.Command("git")
	cmd.Dir = outAbs
	args := []string{
		"add",
	}
	args = append(args, gomod.dirty...)
	cmd.Args = append(cmd.Args, args...)
	gcmd.Debugf("%v\n", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), err
	}
	return "", nil
}

func git_commit(gcmd *generate, gomod *goMod, podPath, localPkgPart string,
	commitMsg string,
) (string, error) {
	if len(gomod.dirty) == 0 {
		return "", nil
	}
	outAbs := filepath.Join(gcmd.OutputDir, podPath, localPkgPart)
	cmd := exec.Command("git")
	cmd.Dir = outAbs
	args := []string{
		"commit",
		"-m", commitMsg,
	}
	cmd.Args = append(cmd.Args, args...)
	gcmd.Debugf("%v\n", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), err
	}
	return "", nil
}

func git_tag(gcmd *generate, gomod *goMod, podPath, localPkgPart string,
	commitMsg string,
) (string, error) {
	if len(gomod.dirty) == 0 {
		return "", nil
	}
	outAbs := filepath.Join(gcmd.OutputDir, podPath, localPkgPart)
	cmd := exec.Command("git")
	cmd.Dir = outAbs
	base := ""
	if podPath == "." {
		base = localPkgPart
	} else {
		base = podPath + "/" + localPkgPart
	}

	args := []string{
		"tag",
		"-a",
		"-m", commitMsg,
		base + "/" + gomod.version,
	}
	cmd.Args = append(cmd.Args, args...)
	gcmd.Debugf("%v\n", cmd.Args)
	if out, err := cmd.CombinedOutput(); err != nil {
		return string(out), err
	}
	return "", nil
}

var pathSemver = regexp.MustCompile(`^(.+)/v(\d+)\.(\d+)\.(\d+)$`)
var repoSemver = regexp.MustCompile(`^v(\d+)\.(\d+)\.(\d+)$`)

func gitGetTagSemver(gcmd *generate) (map[string]map[int64]utils.Semvers, string, error) {
	pseudo_version := ""
	{
		cmd := exec.Command("git")
		cmd.Dir = gcmd.OutputDir
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
		pseudo_version = fmt.Sprintf("v0.0.0-%s-%.12s", parts[0], parts[1])
	}
	ret := map[string]map[int64]utils.Semvers{}
	cmd := exec.Command("git")
	cmd.Dir = gcmd.OutputDir
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
			if !repoSemver.MatchString(line) {
				gcmd.Debugf("Does tag look right ? %v\n", line)
				q.Q("tag does look right %v\n", line)
			}
			continue
		}
		q.Q("tag %v\n", line)
		modName := match[1]
		ma, _ := strconv.ParseInt(match[2], 10, 64)
		mi, _ := strconv.ParseInt(match[3], 10, 64)
		pa, _ := strconv.ParseInt(match[4], 10, 64)
		sem := utils.Semver{Major: ma, Minor: mi, Patch: pa}
		sems, ex := ret[modName]
		if !ex {
			sems = make(map[int64]utils.Semvers)
			sems[ma] = utils.Semvers{sem}
			gcmd.Debugf("  add semver key: %s ver: %v\n", modName, sem)
			// ret[modName] = sems
		} else {
			gcmd.Debugf("  add semver key: %s ver: %v\n", modName, sem)
			sems[ma] = append(sems[ma], sem)
		}
		ret[modName] = sems
	}
	return ret, pseudo_version, nil
}

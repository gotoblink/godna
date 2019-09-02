package bumptag

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"

	log "github.com/golang/glog"

	"github.com/wxio/godna/internal/utils"
)

type Bump struct {
	OutputDir string `opts:"mode=arg" help:"output directory eg ."`
}

func New() *Bump {
	return &Bump{}
}

func (cmd *Bump) Run() error {
	sems, err := getSemTags(cmd.OutputDir)
	if err != nil {
		return err
	}
	if len(sems) == 0 {
		return fmt.Errorf("no current semvers")
	}
	sem := sems[0]
	sem.Minor += 1
	fmt.Printf("%v\n", sem)
	return nil
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

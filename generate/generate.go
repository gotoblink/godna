package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wxio/godna/pb/dna/config"
)

type generate struct {
	cfg       *config.Config
	OutputDir string   `opts:"mode=arg" help:"output directory eg ."`
	Steps     []string `help:" defaults (protoc_plugs, protoc_file_description_set:gomod,gitcommit,gittag)"`
	Debug     bool
	//
	steps map[string]map[string]bool
}

func New(cfg *config.Config) *generate {
	return &generate{
		// Steps: []string{"protoc_plugs", "protoc_file_description_set:gomod,gitcommit,gittag"},
		cfg: cfg,
	}
}

func (cmd *generate) Run() error {
	if len(cmd.Steps) == 0 {
		cmd.Steps = []string{
			"protoc_plugs",
			"protoc_file_description_set:gomod,gitcommit,gittag",
		}
	}
	cmd.steps = map[string]map[string]bool{}
	for _, st := range cmd.Steps {
		ke := strings.Split(st, ":")
		switch len(ke) {
		case 1:
			cmd.steps[ke[0]] = map[string]bool{}
		case 2:
			cmd.steps[ke[0]] = map[string]bool{}
			ke2 := strings.Split(ke[1], ",")
			for _, k2 := range ke2 {
				cmd.steps[ke[0]][k2] = true
			}
		default:
			return fmt.Errorf("invalid step format %v", st)
		}
	}
	fmt.Printf("%v\n", cmd.steps)
	var err error
	cmd.cfg.SrcDir = os.ExpandEnv(cmd.cfg.SrcDir)
	cmd.cfg.SrcDir, err = filepath.Abs(cmd.cfg.SrcDir)
	if err != nil {
		return err
	}
	cmd.OutputDir = os.ExpandEnv(cmd.OutputDir)
	cmd.OutputDir, err = filepath.Abs(cmd.OutputDir)
	if err != nil {
		return err
	}
	gopkg := &GoPackages{}
	if _, err = gopkg.Process(cmd); err != nil {
		return err
	}
	for _, pkg := range gopkg.Pkgs {
		padding := strings.Repeat(" ", gopkg.MaxPkgLen-len(pkg.Pkg))
		pad2 := strings.Repeat(" ", gopkg.MaxRelDirLen-len(pkg.RelDir))
		fmt.Printf("package: %s%s %s %sfiles: %s", pkg.Pkg, padding, pkg.RelDir, pad2, pkg.Files)
		// fmt.Printf("  files: %v\n", pkg.Files)
		if len(pkg.Imports) > 0 {
			fmt.Printf(" import: %v", pkg.Imports)
		}
		fmt.Printf("\n")
	}
	protocIt := &ProtocIt{goPkgs: *gopkg}
	if msg, err := protocIt.Process(cmd); err != nil {
		fmt.Printf("protoc error msg\n----\n%s\n----\n", msg)
		return err
	}
	//
	goModIt := &GoModIt{protocIt: *protocIt}
	if msg, err := goModIt.Process(cmd); err != nil {
		fmt.Printf("goModIt error msg\n----\n%s\n----\n", msg)
		return err
	}
	return nil
}

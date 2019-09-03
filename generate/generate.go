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
	OutputDir string `opts:"mode=arg" help:"output directory eg ."`
}

func New(cfg *config.Config) *generate {
	return &generate{cfg: cfg}
}

func (cmd *generate) Run() error {
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
	if _, err = gopkg.Process(cmd.OutputDir, cmd.cfg); err != nil {
		return err
	}
	for _, pkg := range gopkg.Pkgs {
		padding := strings.Repeat(" ", gopkg.MaxPkgLen-len(pkg.Pkg))
		fmt.Printf("package %s%s %s\n", pkg.Pkg, padding, pkg.RelDir)
		fmt.Printf("  files: %v\n", pkg.Files)
		if len(pkg.Imports) > 0 {
			fmt.Printf("  import: %v\n", pkg.Imports)
		}
	}
	protocIt := &ProtocIt{goPkgs: *gopkg}
	if msg, err := protocIt.Process(cmd.OutputDir, cmd.cfg); err != nil {
		fmt.Printf("protoc error msg\n----\n%s\n----\n", msg)
		return err
	}
	return nil
}

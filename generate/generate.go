package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/wxio/godna/pb/dna/config"
)

type generate struct {
	debugger
	cfg              *config.Config
	OutputDir        string `opts:"mode=arg" help:"output directory eg ."`
	StepProtoc       bool   `opts:"short=p" help:"run the protoc step"`
	StepGomodAll     bool   `opts:"short=m" help:"run all go mod steps. Overrides the individual --step-gomod-x flags (ie or'ed)"`
	StepGomodInit    bool
	StepGomodCfg     bool
	StepGomodLocal   bool
	StepGomodTidy    bool
	StepGomodVersion bool
	StepGitAll       bool `opts:"short=g"`
	StepGitCommit    bool
	StepGetTag       bool
	// Steps            []string `help:" defaults (protoc_plugs, protoc_file_description_set:gomod,gitcommit,gittag)"`
	//
	// steps map[string]map[string]bool
}

type debugger interface {
	Debugf(format string, a ...interface{})
}

func New(cfg *config.Config, de debugger) *generate {
	return &generate{
		debugger:   de,
		StepProtoc: true,
		// Steps: []string{"protoc_plugs", "protoc_file_description_set:gomod,gitcommit,gittag"},
		cfg: cfg,
	}
}

func (cmd *generate) Run() error {
	if !(cmd.StepProtoc ||
		cmd.StepGomodAll ||
		cmd.StepGomodInit ||
		cmd.StepGomodCfg ||
		cmd.StepGomodLocal ||
		cmd.StepGomodTidy ||
		cmd.StepGomodVersion ||
		cmd.StepGitAll ||
		cmd.StepGitCommit ||
		cmd.StepGetTag) {
		return fmt.Errorf("No step specified. See help.\n")
	}
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
	gopkgF := func() GoPackages {
		gopkg := &GoPackages{}
		if _, err = gopkg.Process(cmd); err != nil {
			panic(err)
			// return err
		}
		for _, pkg := range gopkg.Pkgs {
			imps := collect(pkg, "", map[string]Void{})
			pkg.Imports = *(imps.(*goPkgs2By))
		}
		// n := goPkgs2By(gopkg.Pkgs)
		// sort.Sort(n)
		gopkg.Pkgs = sortgoPkgs2By(gopkg.Pkgs)
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
		return *gopkg
	}
	gopkg := gopkgF()
	protocItF := func() ProtocIt {
		protocIt := &ProtocIt{goPkgs: gopkg}
		if msg, err := protocIt.Process(cmd); err != nil {
			fmt.Printf("protoc error msg\n----\n%s\n----\n", msg)
			panic(err)
			// return err
		}
		return *protocIt
	}
	protocIt := protocItF()
	//
	goModItF := func() GoModIt {
		goModIt := &GoModIt{protocIt: protocIt}
		if msg, err := goModIt.Process(cmd); err != nil {
			fmt.Printf("goModIt error msg\n----\n%s\n----\n", msg)
			panic(err)
			// return err
		}
		for _, gomod := range goModIt.gomods {
			padding := strings.Repeat(" ", goModIt.protocIt.goPkgs.MaxPkgLen-len(gomod.pkg.Pkg))
			fmt.Printf("gomod: %s%s subpkg #%v imports #%v %s dirty %v\n",
				gomod.pkg.Pkg, padding, len(gomod.subpkg), len(gomod.imp), gomod.version, len(gomod.dirty) != 0)
		}
		return *goModIt
	}
	goModItF()
	return nil
}

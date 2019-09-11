package generate

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/wxio/godna/pb/dna/config"
)

type cfg struct {
	*config.Config
}

type generate struct {
	debugger
	cfg
	OutputDir           string `opts:"mode=arg" help:"output directory eg ."`
	PackagePatternMatch string `opts:"short=r" help:"regexp of go package name"`
	StepAll             bool   `opts:"short=s" help:"run all steps (step-protoc, step-gomod-all, step-git-all)"`
	StepProtoc          bool   `opts:"short=p" help:"run the protoc\n (default true)"`
	StepGomodAll        bool   `opts:"short=m" help:"run all go mod steps. Overrides the individual steps (ie or-ed)"`
	StepGomodInit       bool   `help:"go mod init for all specified go modules.\nDoes not overwrite existing go.mod files.\nie containing\n\timport \"dna/store.v1.proto\";\n\toption (wxio.dna.store) = {\n\t\tgo_mod : true\n\t};\nstore.v1.proto usually stored in vendor/wxio"`
	StepGomodCfg        bool   `help:"go mod edit -require <specified in config>"`
	StepGomodLocal      bool   `help:"need for local dev & tidy.\ngo mod edit -replace <proto import>=../[../]*/<local code>"`
	StepGomodTidy       bool   `help:"go mod tidy"`
	StepGomodVersion    bool   `help:"go mod edit -dropreplace & -require for imported modules"`
	StepGitAll          bool   `opts:"short=g" help:"git add, commit & tag"`
	StepGitAdd          bool   `help:"git add"`
	StepGitAddCommit    bool   `help:"git add & commit"`
	StepGitAddCommitTag bool   `help:"git add, commit & tag"`
	//
	stepFDS             bool
	stepUpdateSemver    bool
	packagePatternMatch *regexp.Regexp
}

func (cmd generate) matchPackage(pkg string) bool {
	if cmd.packagePatternMatch != nil {
		return cmd.packagePatternMatch.MatchString(pkg)
	}
	return true
}

type generateFDS struct {
	debugger
	cfg
	OutputDir string `opts:"mode=arg" help:"output directory eg ."`
}

type debugger interface {
	Debugf(format string, a ...interface{})
}

// New generate sub command
func New(cf *config.Config, de debugger) interface{} {
	return &generate{
		debugger: de,
		// StepProtoc: true,
		// Steps: []string{"protoc_plugs", "protoc_file_description_set:gomod,gitcommit,gittag"},
		cfg: cfg{cf},
	}
}

// NewFDS generate file descriptor set sub command
func NewFDS(cf *config.Config, de debugger) interface{} {
	return &generateFDS{
		debugger: de,
		cfg:      cfg{cf},
	}
}

// Run sub command
func (cmd *generate) Run() (err error) {
	if cmd.PackagePatternMatch != "" {
		if cmd.packagePatternMatch, err = regexp.Compile(cmd.PackagePatternMatch); err != nil {
			return fmt.Errorf("--package-pattern-match '%s' not a valid regexp err: %v", cmd.PackagePatternMatch, err)
		}
	}
	if !(cmd.StepAll ||
		cmd.StepProtoc ||
		cmd.StepGomodAll ||
		cmd.StepGomodInit ||
		cmd.StepGomodCfg ||
		cmd.StepGomodLocal ||
		cmd.StepGomodTidy ||
		cmd.StepGomodVersion ||
		cmd.StepGitAll ||
		cmd.StepGitAdd ||
		cmd.StepGitAddCommit ||
		cmd.StepGitAddCommitTag) {
		cmd.Debugf("Default cmd.StepProtoc = true")
		cmd.StepProtoc = true
		// return fmt.Errorf("No step specified. See help.\n")
	}
	if cmd.StepAll {
		cmd.Debugf("All Steps")
		cmd.StepProtoc = true
		cmd.StepGomodAll = true
		cmd.StepGitAll = true
	}
	if cmd.StepGomodAll ||
		cmd.StepGomodInit ||
		cmd.StepGomodCfg ||
		cmd.StepGomodLocal ||
		cmd.StepGomodTidy ||
		cmd.StepGomodVersion ||
		cmd.StepGitAll ||
		cmd.StepGitAdd ||
		cmd.StepGitAddCommit ||
		cmd.StepGitAddCommitTag {
		cmd.Debugf("Steps FDS")
		cmd.stepFDS = true
	}
	if cmd.StepGomodAll ||
		cmd.StepGomodLocal ||
		cmd.StepGomodTidy ||
		cmd.StepGomodVersion ||
		cmd.StepGitAll ||
		cmd.StepGitAdd ||
		cmd.StepGitAddCommit ||
		cmd.StepGitAddCommitTag {
		cmd.Debugf("Step Update Semver")
		cmd.stepUpdateSemver = true
	}
	if cmd.StepGomodAll {
		cmd.Debugf("Step Go Mod All")
		cmd.StepGomodInit = true
		cmd.StepGomodCfg = true
		cmd.StepGomodLocal = true
		cmd.StepGomodTidy = true
		cmd.StepGomodVersion = true
	}
	if cmd.StepGitAll {
		cmd.Debugf("Step Git All")
		cmd.StepGitAdd = true
		cmd.StepGitAddCommit = true
		cmd.StepGitAddCommitTag = true
	}
	if cmd.StepGitAddCommit {
		cmd.Debugf("Step Git Add Commit")
		cmd.StepGitAdd = true
	}
	if cmd.StepGitAddCommitTag {
		cmd.Debugf("Step Git Add Commit Tag")
		cmd.StepGitAdd = true
		cmd.StepGitAddCommit = true
	}
	//
	cmd.cfg.SrcDir = os.ExpandEnv(cmd.cfg.SrcDir)
	if cmd.cfg.SrcDir, err = filepath.Abs(cmd.cfg.SrcDir); err != nil {
		return err
	}
	cmd.OutputDir = os.ExpandEnv(cmd.OutputDir)
	if cmd.OutputDir, err = filepath.Abs(cmd.OutputDir); err != nil {
		return err
	}
	gopkgF := func() GoPackages {
		gopkg := &GoPackages{}
		if _, err = gopkg.Process(cmd); err != nil {
			panic(err)
			// return err
		}
		for _, pkg := range gopkg.Pkgs {
			imps := collect(pkg, "", map[string]Void{pkg.Pkg: Void{}})
			pkg.Imports = *(imps.(*goPkgs2By))
			padding := strings.Repeat(" ", gopkg.MaxPkgLen-len(pkg.Pkg))
			pad2 := strings.Repeat(" ", gopkg.MaxRelDirLen-len(pkg.RelDir))
			fmt.Printf("package: %s%s %s %sfiles: %s", pkg.Pkg, padding, pkg.RelDir, pad2, pkg.Files)
			// fmt.Printf("  files: %v\n", pkg.Files)
			if len(pkg.Imports) > 0 {
				fmt.Printf(" import: %v", pkg.Imports)
			}
			fmt.Printf("\n")
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
		if msg, err := protocIt.process(cmd); err != nil {
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
		if msg, err := goModIt.process(cmd); err == nil {
			for _, gomod := range goModIt.gomods {
				padding := strings.Repeat(" ", goModIt.protocIt.goPkgs.MaxPkgLen-len(gomod.pkg.Pkg))
				fmt.Printf("gomod: %s%s subpkg #%v imports #%v %s dirty %v\n",
					gomod.pkg.Pkg, padding, len(gomod.subpkg), len(gomod.imp), gomod.version, len(gomod.dirty) != 0)
			}
			if msg != "" {
				fmt.Printf("## %s\n", msg)
			}
		} else {
			fmt.Printf("goModIt error msg\n----\n%s\n----\n", msg)
			panic(err)
			// return err
		}
		return *goModIt
	}
	goModItF()
	return nil
}

func (cmd *generateFDS) Run() error {
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
	protocItF := func() ProtocFdsIt {
		protocIt := &ProtocFdsIt{goPkgs: gopkg}
		if msg, err := protocIt.process(cmd); err != nil {
			fmt.Printf("protoc error msg\n----\n%s\n----\n", msg)
			panic(err)
			// return err
		}
		return *protocIt
	}
	protocIt := protocItF()
	_ = protocIt
	//
	return nil
}

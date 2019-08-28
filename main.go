package main

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	"github.com/wxio/godna/pb/dna/config"

	"github.com/golang/protobuf/proto"
	"github.com/jpillora/opts"
)

var (
	version string = "dev"
	date    string = "na"
	commit  string = "na"
)

type root struct {
}

type versionCmd struct {
	help string
}

type Config struct {
	cfg       *config.Config
	OutputDir string `opts:"mode=arg" help:"output directory eg ."`
}

func main() {
	cfg := &Config{cfg: &config.Config{}}
	opts.New(&root{}).
		Name("godna").
		EmbedGlobalFlagSet().
		Complete().
		Version(version).
		AddCommand(opts.New(&versionCmd{}).Name("version")).
		AddCommand(opts.New(cfg).Name("regen").
			FieldConfigPath("./.dna-cfg.ptron", cfg.cfg)). //, "godna.Config")).
		Parse().
		RunFatal()
}

func (cfg *Config) Run() error {
	var err error
	cfg.cfg.SrcDir = os.ExpandEnv(cfg.cfg.SrcDir)
	cfg.cfg.SrcDir, err = filepath.Abs(cfg.cfg.SrcDir)
	if err != nil {
		return err
	}
	cfg.OutputDir = os.ExpandEnv(cfg.OutputDir)
	cfg.OutputDir, err = filepath.Abs(cfg.OutputDir)
	if err != nil {
		return err
	}
	//
	gomods, err := step1(cfg)
	if err != nil {
		return err
	}
	//
	gomods2, err := step2(cfg.cfg.SrcDir, gomods.Modules)
	if err != nil {
		return err
	}
	gensByOut, err := step3(gomods2, cfg.OutputDir, cfg.cfg)
	if err != nil {
		return err
	}
	// //
	// // sort.Sort(goModAbs)
	// // for _, x := range goModAbs {
	// // 	fmt.Printf("%s\n", x.mod.mod.Module)
	// // 	for _, y := range x.imps {
	// // 		fmt.Printf("   %s\n", y.mod.mod.Module)
	// // 	}
	// // }
	// //
	err = step4(gensByOut, cfg.OutputDir, cfg.cfg)
	if err != nil {
		return err
	}
	//

	// for k, v := range nextSemvers {
	// 	fmt.Printf("%s %+v\n", k, v)
	// }
	// fmt.Printf("%+v\n", nextSemvers)
	// for _, modp := range gomods2 {
	// 	fmt.Printf("%s\n", modp.mod)
	// 	for _, pkg := range modp.pkgs {
	// 		fmt.Printf("  %v\n", pkg.Package)
	// 		for _, fi := range pkg.Files {
	// 			fmt.Printf("    %v\n", fi)
	// 		}
	// 		// protoc
	// 		for _, pod := range cfg.cfg.PluginOutputDir {
	// 			for _, gen := range pod.Generator {
	// 				if err = pkg.protoc(cfg.cfg.SrcDir, cfg.OutputDir, pod, gen, cfg.cfg.Includes); err != nil {
	// 					return err
	// 				}
	// 			}
	// 		}
	// 	}
	// }

	return nil
}

func (r *versionCmd) Run() error {
	cfg := Config{
		cfg: &config.Config{
			HostOwner: "github.com/microsoft",
			RepoName:  "go-vscode",
			Includes:  []string{"./vendor/google"},
			Pass:      []*config.Config_Pass{
				// {Cmd: []*Config_Pass_Command{
				// 	{
				// 		// Go:
				// 	},
				// }},
				// {},
				// &Config_Pass{
				// 	Cmd:
				// },
			},
		},
	}
	buf := bytes.Buffer{}
	err := proto.MarshalText(&buf, cfg.cfg)
	if err != nil {
		return err
	}
	fmt.Printf("ptron: %s\n", buf.String())
	fmt.Printf("version: %s\ndate: %s\ncommit: %s\n", version, date, commit)
	fmt.Println(r.help)
	return nil
}

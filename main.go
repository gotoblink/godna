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
	gomods := &Step1{}
	if _, err = gomods.Process(cfg.OutputDir, cfg.cfg); err != nil {
		return err
	}
	//
	gomods2 := &Step2{step1: *gomods}
	if _, err = gomods2.Process(cfg.OutputDir, cfg.cfg); err != nil {
		return err
	}
	pkgs := &Step3{step2: *gomods2}
	if _, err = pkgs.Process(cfg.OutputDir, cfg.cfg); err != nil {
		return err
	}
	st4 := &Step4{step3: *pkgs}
	if _, err = st4.Process(cfg.OutputDir, cfg.cfg); err != nil {
		return err
	}

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

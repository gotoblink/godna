package main

import (
	"bytes"
	"fmt"
	"io"
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
		AddCommand(opts.New(cfg).Name("ptron").
			FieldConfigPath("./.dna-cfg.ptron", cfg.cfg)). //, "godna.Config")).
		AddCommand(
			opts.New(
				&regen{
					OutputDir: ".",
					// Pass: []string{
					// 	"protoc,modinit,modreplace",
					// 	"modtidy",
					// 	"gittag",
					// },
				}).
				Name("regen").
				// ProtoConfig("./.dna-cfg.ptron", "godna.Config")).
				ConfigPath("./.dna-cfg.json")).
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

	// fmt.Printf("'%+v'\n", *cfg.cfg)
	// resp := &Src{}
	// if err := Cfg2Src(*cfg.cfg, resp); err != nil {
	// 	return err
	// }
	gomods := &goMods{}
	if err := gomods.collectGomods(cfg); err != nil {
		return err
	}
	gomods2 := []goModPlus{}
	fmt.Println("----------")
	for _, x := range gomods.Modules {
		// fmt.Printf("%s\n", x)
		pfs, err := x.collectFiles(cfg)
		if err != nil {
			return err
		}
		mods, err := pfs.collectModules(x.RelDir, x.Module)
		if err != nil {
			return err
		}
		gomods2 = append(gomods2, goModPlus{x, mods})
	}
	for _, modp := range gomods2 {
		fmt.Printf("%s\n", modp.mod)
		for _, pkg := range modp.pkgs {
			fmt.Printf("  %v\n", pkg.Package)
			for _, fi := range pkg.Files {
				fmt.Printf("    %v\n", fi)
			}
		}
	}
	// for _, y := range mods {
	// 	fmt.Printf("  %s\n", y)
	// 	for _, pod := range cfg.cfg.PluginOutputDir {
	// 		for _, gen := range pod.Generator {
	// 			if err = y.protoc(cfg.cfg.SrcDir, cfg.OutputDir, pod, gen, cfg.cfg.Includes); err != nil {
	// 				return err
	// 			}
	// 		}
	// 	}
	// }

	fmt.Println("----------")
	for _, pod := range cfg.cfg.PluginOutputDir {
		for _, gen := range pod.Generator {
			fmt.Printf("%s/%s %s %v\n", cfg.OutputDir, pod.Path, pod.OutType, gen)
		}
	}
	return nil
}

func filecopy(src, dst string) (int64, error) {
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

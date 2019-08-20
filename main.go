package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

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
	cfg.OutputDir = os.ExpandEnv(cfg.OutputDir)
	cfg.OutputDir, err = filepath.Abs(cfg.OutputDir)
	if err != nil {
		return err
	}
	//
	gomods := &goMods{}
	if err := gomods.collectGomods(cfg); err != nil {
		return err
	}
	//
	gomods2 := []goModPlus{}
	fmt.Println("----------")
	for i, _ := range gomods.Modules {
		gomod := gomods.Modules[i]
		// fmt.Printf("%s\n", x)
		pfs, err := gomod.collectFiles(cfg)
		if err != nil {
			return err
		}
		mods, err := pfs.collectModules(gomod)
		if err != nil {
			return err
		}
		mp := goModPlus{gomod, mods}
		gomods2 = append(gomods2, mp)
	}
	remote, desc := describe(cfg.cfg.SrcDir)
	modMap := map[string]*goModAbsOut{}
	goModAbs := goModAbsOutBy{}
	gensByOut := []*goPkgAbsOut{}
	for _, modp := range gomods2 {
		for _, pkg := range modp.pkgs {
			for _, pod := range cfg.cfg.PluginOutputDir {
				absOut, outBit, err := absOut_Mk_CpMod(pkg, pod, cfg.cfg.SrcDir, cfg.OutputDir)
				// fmt.Printf("a:%s b:%s p:%s\n", absOut, outBit, pkg.Package)
				if err != nil {
					return err
				}
				ismod := pod.OutType == config.Config_PluginOutDir_GO_MODS && modp.mod.Module == pkg.Package
				gensPkg := goPkgAbsOut{
					absOut: absOut,
					outBit: outBit,
					pkg:    pkg,
					mod:    ismod,
				}
				gensByOut = append(gensByOut, &gensPkg)
				if ismod {
					gomod := goModAbsOut{
						mod:  modp,
						pkg:  &gensPkg,
						imps: nil,
					}
					goModAbs = append(goModAbs, &gomod)
					modMap[outBit] = &gomod
				}
			}
		}
	}
	//
	for i, _ := range goModAbs {
		gomodabs := goModAbs[i]
		for _, pkg := range gomodabs.mod.pkgs {
			for _, imp := range pkg.Imports {
				gimp, ex := modMap[imp]
				if !ex {
					fmt.Printf("no local pkg %v\n", imp)
				} else {
					fmt.Printf("LOCAL pkg %v\n", imp)
					gomodabs.imps = append(gomodabs.imps, gimp)
				}
			}
		}
		goModAbs[i] = gomodabs
	}
	for i, _ := range goModAbs {
		gomodabs := goModAbs[i]
		deps := gomodabs.collect("", map[string]struct{}{})
		gomodabs.imps = deps
	}
	//
	// sort.Sort(goModAbs)
	// for _, x := range goModAbs {
	// 	fmt.Printf("%s\n", x.mod.mod.Module)
	// 	for _, y := range x.imps {
	// 		fmt.Printf("   %s\n", y.mod.mod.Module)
	// 	}
	// }
	//
	for _, pkg := range gensByOut {
		// protoc
		for _, pod := range cfg.cfg.PluginOutputDir {
			for _, gen := range pod.Generator {
				if msg, err := protoc(pkg.pkg, cfg.cfg.SrcDir, pkg.absOut, pod, gen, cfg.cfg.Includes); err != nil {
					fmt.Println(msg)
					return err
				}
			}
		}
	}
	//
	for i, _ := range gensByOut {
		pkg := gensByOut[i]
		if !pkg.mod {
			continue
		}
		for _, pod := range cfg.cfg.PluginOutputDir {
			if pod.OutType == config.Config_PluginOutDir_GO_MODS {
				pkg.dirty, pkg.dirtyFiles, err = isDirty(cfg.OutputDir, pod.Path, pkg.outBit)
				fmt.Printf("%s %v\n", pkg.outBit, pkg.dirtyFiles)
			}
		}
		gensByOut[i] = pkg
	}
	//
	gitTagSemver, err := gitGetTagSemver(cfg.OutputDir)
	if err != nil {
		return err
	}
	nextSemvers := pkgrel2next{}
	for _, pkg := range gensByOut {
		if pkg.mod {
			major := pkgModVersion(pkg.outBit)
			if major == -1 {
				return fmt.Errorf("not version for mod relpath:%s %v", pkg.outBit, pkg)
			}
			base := pkgModBase(pkg.pkg.RelDir)
			if next, ex := gitTagSemver[base]; ex {
				if cur, ex := next[major]; ex {
					//TODO check majar ver compatibility
					sort.Sort(cur)
					if pkg.dirty {
						nextSemvers[pkg.outBit] = Semver{cur[0].Major, cur[0].Minor + 1, 0}
					} else {
						nextSemvers[pkg.outBit] = cur[0]
					}
				} else {
					nextSemvers[pkg.outBit] = Semver{major, 0, 0}
				}
			} else {
				nextSemvers[pkg.outBit] = Semver{major, 0, 0}
			}
		}
	}
	//
	// sort.Sort(gensByOut)
	// for _, pkg := range gensByOut {
	// 	fmt.Printf("%s %20v\n", pkg.pkg.Package, pkg.pkg.Imports)
	// }
	for _, gm := range goModAbs {
		fmt.Printf("%s %v\n", gm.mod.mod.Module, nextSemvers[gm.pkg.outBit])
		out, msg, err := gm.gomodRequireReplace(cfg.cfg, nextSemvers)
		if err != nil {
			fmt.Printf("%s, %s %v\n", string(out), msg, err)
			return err
		}
		for _, y := range gm.imps {
			fmt.Printf("   %s %v\n", y.mod.mod.Module, nextSemvers[y.pkg.outBit])
		}
	}
	//
	// for _, gm := range goModAbs {
	// 	for i, _ := range gm.mod.pkgs {
	for i, _ := range gensByOut {
		pkg := gensByOut[i]
		if pkg.mod {
			for _, pod := range cfg.cfg.PluginOutputDir {
				_, dirtyFiles, err := isDirty(cfg.OutputDir, pod.Path, pkg.outBit)
				if err != nil {
					return err
				}
				err = addNtag(cfg.OutputDir, pod.Path, pkg.outBit, dirtyFiles, nextSemvers[pkg.outBit], pkg.mod, remote, desc)
				// fmt.Printf("%s %v\n", pkg.outBit, dirtyFiles)
			}
		}
	}
	for i, _ := range gensByOut {
		pkg := gensByOut[i]
		if !pkg.mod {
			for _, pod := range cfg.cfg.PluginOutputDir {
				_, dirtyFiles, err := isDirty(cfg.OutputDir, pod.Path, pkg.outBit)
				if err != nil {
					return err
				}
				err = addNtag(cfg.OutputDir, pod.Path, pkg.outBit, dirtyFiles, nextSemvers[pkg.outBit], pkg.mod, remote, desc)
				// fmt.Printf("%s %v\n", pkg.outBit, dirtyFiles)
			}
		}
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

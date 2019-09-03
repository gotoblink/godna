package main

import (
	"fmt"

	"github.com/wxio/godna/pb/dna/config"

	"github.com/wxio/godna/bumptag"
	"github.com/wxio/godna/generate"
	"github.com/wxio/godna/readfds"
	"github.com/wxio/godna/regen"

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

func main() {
	cfg := &config.Config{}
	rcmd := regen.New(cfg)
	gen_cmd := generate.New(cfg)
	opts.New(&root{}).
		Name("godna").
		EmbedGlobalFlagSet().
		Complete().
		Version(version).
		AddCommand(opts.New(&versionCmd{}).Name("version")).
		AddCommand(opts.New(rcmd).Name("regen").
			FieldConfigPath("./.dna-cfg.ptron", cfg)). //, "godna.Config")).
		AddCommand(opts.New(gen_cmd).Name("generate").
			FieldConfigPath("./.dna-cfg.ptron", cfg)).
		AddCommand(opts.New(bumptag.New()).Name("bumptag")).
		AddCommand(opts.New(readfds.New()).Name("readfds")).
		Parse().
		RunFatal()
}

func (r *versionCmd) Run() error {
	// // cfg := &config.Config{}
	// // rcmd := regen.New(cfg)
	// cfg := &config.Config{
	// 	HostOwner: "github.com/microsoft",
	// 	RepoName:  "go-vscode",
	// 	Includes:  []string{"./vendor/google"},
	// 	Pass:      []*config.Config_Pass{
	// 		// {Cmd: []*Config_Pass_Command{
	// 		// 	{
	// 		// 		// Go:
	// 		// 	},
	// 		// }},
	// 		// {},
	// 		// &Config_Pass{
	// 		// 	Cmd:
	// 		// },
	// 	},
	// }
	// buf := bytes.Buffer{}
	// err := proto.MarshalText(&buf, cfg)
	// if err != nil {
	// 	return err
	// }
	// fmt.Printf("ptron: %s\n", buf.String())
	fmt.Printf("version: %s\ndate: %s\ncommit: %s\n", version, date, commit)
	// fmt.Println(r.help)
	return nil
}

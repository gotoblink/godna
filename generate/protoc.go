package generate

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/golangq/q"

	"github.com/wxio/godna/pb/dna/config"
)

func (proc *ProtocIt) Process(rootOutDir string, cfg *config.Config) (string, error) {
	if err := os.MkdirAll(filepath.Join(rootOutDir, "descriptor_set"), os.ModePerm); err != nil {
		return "err: mkdir -p " + rootOutDir + "/descriptor_set", err
	}
	for _, pkg := range proc.goPkgs.Pkgs {
		fmt.Printf("protoc %s %s\n", pkg.Pkg, pkg.Files)
		for _, pod := range cfg.PluginOutputDir {
			for _, gen := range pod.Generator {
				outAbs := filepath.Join(rootOutDir, pkg.Pkg[len(cfg.GetGoPackagePrefix()):])
				if err := os.MkdirAll(outAbs, os.ModePerm); err != nil {
					return "err: mkdir -p " + outAbs, err
				}
				msg, err := protoc(*pkg, cfg.SrcDir, outAbs, pod, gen, cfg.Includes)
				if err != nil {
					return msg, err
				}
			}
		}
		outFile := filepath.Join(rootOutDir, "descriptor_set", strings.Replace(pkg.Pkg, "/", "_", -1)+".fds")
		msg, err := protoc_descriptor_set_out(*pkg, cfg.SrcDir, outFile, cfg.Includes)
		if err != nil {
			return msg, err
		}
	}
	return "", nil
}

func protoc(in goPkg2, srcdir string, outAbs string, pod *config.Config_PluginOutDir, gen *config.Config_Generator, includes []string) (message string, e error) {
	cmd := exec.Command("protoc")
	plg := protocGenerator(outAbs, gen)
	args := []string{plg}
	// args = append(args, "-I..")
	args = append(args, "-I"+in.RelDir)
	for _, inc := range includes {
		incAbs, err := filepath.Abs(inc)
		if err != nil {
			return "abs file", err
		}
		args = append(args, "-I"+incAbs)
	}
	for _, fi := range in.Files {
		args = append(args, fi)
	}
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = srcdir
	fmt.Printf("\t\tcmd:%v\n", cmd.Args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		scmd := fmt.Sprintf("( cd %v; %v)\n", cmd.Dir, strings.Join(cmd.Args, " "))
		sout := string(out)
		q.Q(err)
		q.Q(scmd)
		q.Q(sout)
	}
	return fmt.Sprintf("%s\ncmd %v\nmsg:%s\n", srcdir, cmd.Args, string(out)), err
}

// --descriptor_set_out=FILE
func protoc_descriptor_set_out(in goPkg2, srcdir string, outFile string, includes []string) (message string, e error) {
	cmd := exec.Command("protoc")
	args := []string{"--descriptor_set_out=" + outFile}
	args = append(args, "-I"+in.RelDir)
	for _, inc := range includes {
		incAbs, err := filepath.Abs(inc)
		if err != nil {
			return "abs file", err
		}
		args = append(args, "-I"+incAbs)
	}
	for _, fi := range in.Files {
		args = append(args, fi)
	}
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = srcdir
	fmt.Printf("\t\tcmd:%v\n", cmd.Args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		scmd := fmt.Sprintf("( cd %v; %v)\n", cmd.Dir, strings.Join(cmd.Args, " "))
		sout := string(out)
		q.Q(err)
		q.Q(scmd)
		q.Q(sout)
	}
	return fmt.Sprintf("%s\ncmd %v\nmsg:%s\n", srcdir, cmd.Args, string(out)), err
}

func protocGenerator(outdir string, gen *config.Config_Generator) string {
	switch plg := gen.GetPlugin().GetCmd().(type) {
	case *config.Config_Generator_Plugin_Go:
		name := "--go_out="
		args := []string{}
		if len(plg.Go.Plugins) != 0 {
			for _, pp := range plg.Go.Plugins {
				pl := strings.ToLower(pp.String())
				args = append(args, "plugins="+pl)
			}
		}
		args = append(args, "paths="+strings.ToLower(plg.Go.Paths.String()))
		name = name + strings.Join(args, ",") + ":" + outdir
		return name
	case *config.Config_Generator_Plugin_Micro:
		name := "--micro_out="
		args := []string{}
		args = append(args, "paths="+strings.ToLower(plg.Micro.Paths.String()))
		name = name + strings.Join(args, ",") + ":" + outdir
		return name
	case *config.Config_Generator_Plugin_GrpcGateway:
		name := "--grpc-gateway_out="
		args := []string{}
		args = append(args, "paths="+strings.ToLower(plg.GrpcGateway.Paths.String()))
		if plg.GrpcGateway.RegisterFuncSuffix != "" {
			args = append(args, "register_func_suffix="+plg.GrpcGateway.RegisterFuncSuffix)
		}
		name = name + strings.Join(args, ",") + ":" + outdir
		return name
	case *config.Config_Generator_Plugin_Swagger:
		name := "--swagger_out="
		args := []string{}
		if plg.Swagger.Logtostderr {
			args = append(args, "logtostderr=true")
		} else {
			args = append(args, "logtostderr=false")
		}
		name = name + strings.Join(args, ",") + ":" + outdir
		return name
	case *config.Config_Generator_Plugin_Gotag:
		name := "--gotag_out="
		args := []string{}
		args = append(args, "paths="+strings.ToLower(plg.Gotag.Paths.String()))
		name = name + strings.Join(args, ",") + ":" + outdir
		return name
	default:
		fmt.Printf("unknown plugin %T\n", plg)
		os.Exit(2)
	}
	return ""
}

package generate

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/golangq/q"

	"github.com/wxio/godna/config"
)

func (proc *ProtocFdsIt) process(cmd *generateFDS) (string, error) {
	//
	fmt.Printf("#internal --step-protoc_file_description_set--\n")
	for _, pkg := range proc.goPkgs.Pkgs {
		fmt.Printf("protoc_file_description_set: %s %s\n", pkg.Pkg, pkg.Files)
		outFile := filepath.Join(cmd.OutputDir, "descriptor_set", strings.Replace(pkg.Pkg, "/", "_", -1)+".fds")
		_, msg, err := protocDescriptorSetOut(*pkg, cmd, outFile)
		if err != nil {
			return msg, err
		}
		// proc.FileDescriptorSet = append(proc.FileDescriptorSet, fds...)
	}
	return "", nil
}

func (proc *ProtocIt) process(cmd *generate) (string, error) {
	// if err := os.MkdirAll(filepath.Join(cmd.OutputDir, "descriptor_set"), os.ModePerm); err != nil {
	// 	return "err: mkdir -p " + cmd.OutputDir + "/descriptor_set", err
	// }
	if cmd.StepProtoc {
		fmt.Printf("#--step-protoc\n")
		for _, pkg := range proc.goPkgs.Pkgs {
			if !cmd.matchPackage(pkg.Pkg) {
				continue
			}
			fmt.Printf("protoc %s %s\n", pkg.Pkg, pkg.Files)
			for _, pod := range cmd.cfg.PluginOutputDir {
				for _, gen := range pod.Generator {
					// outAbs := filepath.Join(cmd.OutputDir, pod.Path, pkg.Pkg[len(cmd.cfg.GetGoPackagePrefix()):])
					outAbs := cmd.OutputDir
					if err := os.MkdirAll(outAbs, os.ModePerm); err != nil {
						return "err: mkdir -p " + outAbs, err
					}
					msg, err := protoc(*pkg, cmd, outAbs, pod, gen)
					if err != nil {
						return msg, err
					}
				}
			}
		}
	}
	//
	if cmd.stepFDS {
		fmt.Printf("#internal --step-protoc_file_description_set--\n")
		for _, pkg := range proc.goPkgs.Pkgs {
			if !cmd.matchPackage(pkg.Pkg) {
				continue
			}
			fmt.Printf("protoc_file_description_set: %s %s\n", pkg.Pkg, pkg.Files)
			// outFile := filepath.Join(cmd.OutputDir, "descriptor_set", strings.Replace(pkg.Pkg, "/", "_", -1)+".fds")
			fds, msg, err := protocDescriptorSetOut(*pkg, cmd, "/dev/stdout")
			if err != nil {
				return msg, err
			}
			proc.FileDescriptorSet = append(proc.FileDescriptorSet, fds...)
		}
	}
	return "", nil
}

func protoc(in goPkg2, genCmd *generate, outAbs string, pod *config.Config_PluginOutDir, gen *config.Config_Generator) (message string, e error) {
	cmd := exec.Command("protoc")
	plg := protocGenerator(outAbs, gen)
	args := []string{plg}
	// args = append(args, "-I..")
	// args = append(args, "-I"+in.RelDir)
	for _, inc := range genCmd.cfg.Includes {
		incAbs, err := filepath.Abs(inc)
		if err != nil {
			return "abs file", err
		}
		args = append(args, "-I"+incAbs)
	}
	for _, fi := range in.Files {
		args = append(args, filepath.Join(in.RelDir, fi))
	}
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = genCmd.cfg.SrcDir
	genCmd.Debugf("\tcmd:%v\n", cmd.Args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		scmd := fmt.Sprintf("( cd %v; %v)\n", cmd.Dir, strings.Join(cmd.Args, " "))
		sout := string(out)
		q.Q(err)
		q.Q(scmd)
		q.Q(sout)
	}
	return fmt.Sprintf("%s\ncmd %v\nmsg:%s\n", genCmd.cfg.SrcDir, cmd.Args, string(out)), err
}

func protocDescriptorSetOut(in goPkg2, genCmd genIF, out string) (fds []byte, message string, e error) {
	cmd := exec.Command("protoc")
	// TODO os dependant - check os
	args := []string{"--descriptor_set_out=" + out}
	args = append(args, "-I"+in.RelDir)
	for _, inc := range genCmd.GetIncludes() {
		incAbs, err := filepath.Abs(inc)
		if err != nil {
			return nil, "abs file", err
		}
		args = append(args, "-I"+incAbs)
	}
	for _, fi := range in.Files {
		args = append(args, fi)
	}
	cmd.Args = append(cmd.Args, args...)
	cmd.Dir = genCmd.GetSrcDir()
	genCmd.Debugf("\t\tcmd:%v\n", cmd.Args)
	var bo, be bytes.Buffer
	cmd.Stdout = &bo
	cmd.Stderr = &be
	err := cmd.Run()
	if err != nil {
		scmd := fmt.Sprintf("( cd %v; %v)\n", cmd.Dir, strings.Join(cmd.Args, " "))
		sout := string(be.Bytes())
		q.Q(err)
		q.Q(scmd)
		q.Q(sout)
	}
	return bo.Bytes(), fmt.Sprintf("%s\ncmd %v\nmsg:%s\n", genCmd.GetSrcDir(), cmd.Args, string(be.Bytes())), err
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
	// case *config.Config_Generator_Plugin_Gotag:
	// 	name := "--gotag_out="
	// 	args := []string{}
	// 	args = append(args, "paths="+strings.ToLower(plg.Gotag.Paths.String()))
	// 	name = name + strings.Join(args, ",") + ":" + outdir
	// 	return name
	case *config.Config_Generator_Plugin_Validate:
		name := "--validate_out="
		args := []string{}
		args = append(args, "paths="+strings.ToLower(plg.Validate.Paths.String()))
		if plg.Validate.Lang != 0 {
			args = append(args, "lang="+strings.ToLower(plg.Validate.Lang.String()))
		}
		name = name + strings.Join(args, ",") + ":" + outdir
		return name
	default:
		fmt.Printf("!!!\nunknown plugin %T\n", plg)
		os.Exit(2)
	}
	return ""
}

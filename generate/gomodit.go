package generate

import (
	"fmt"
	"strings"

	"github.com/wxio/godna/pb/extensions/store"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

func (proc *GoModIt) Process(cmd *generate) (string, error) {
	if protocFDS, ex := cmd.steps["protoc_file_description_set"]; ex {
		if _, ex = protocFDS["gomod"]; ex {
			fds := &descriptor.FileDescriptorSet{}
			err := proto.Unmarshal(proc.protocIt.FileDescriptorSet, fds)
			if err != nil {
				return "", err
			}
			mods := []*goMod{}
			notmod := []*goPkg2{}
			n2mod2 := map[string]*goMod{}
			for _, fdp := range fds.File {
				goPkg := proc.protocIt.goPkgs.name2goPkg2[*fdp.Options.GoPackage]
				padding := strings.Repeat(" ", proc.protocIt.goPkgs.MaxPkgLen-len(goPkg.Pkg))
				fmt.Printf("%s%s", *fdp.Options.GoPackage, padding)
				storish, _ := proto.GetExtension(fdp.Options, store.E_Store)
				// if err != nil {
				// }
				if storish != nil {
					eStore := storish.(*store.Store)
					if eStore.GoMod {
						fmt.Printf(" MOD")
						if _, ex := n2mod2[goPkg.Pkg]; !ex {
							mod := &goMod{pkg: goPkg}
							mods = append(mods, mod)
							n2mod2[goPkg.Pkg] = mod
						}
					} else {
						notmod = append(notmod, goPkg)
					}
				} else {
					notmod = append(notmod, goPkg)
				}
				fmt.Printf("\n")
			}
			// TODO subpkg
			// TODO imports
		}
	}
	return "", nil
}

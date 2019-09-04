package readfds

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/wxio/godna/pb/extensions/store"
	_ "github.com/wxio/godna/pb/extensions/store"

	"github.com/golang/protobuf/proto"
	"github.com/golang/protobuf/protoc-gen-go/descriptor"
)

type readfds struct {
	SrcDir string `opts:"mode=arg" help:"source directory of file descriptor sets or '-' for stdin"`
}

func New() *readfds {
	return &readfds{}
}

func (cmd *readfds) Run() error {
	var buf []byte
	var err error
	if cmd.SrcDir == "-" {
		buf, err = ioutil.ReadAll(os.Stdin)
		if err != nil {
			return err
		}
	} else {
		walkCollect := func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if !info.Mode().IsRegular() {
				return nil
			}
			if bb, er := ioutil.ReadFile(path); er != nil {
				return er
			} else {
				buf = append(buf, bb...)
			}
			return nil
		}
		if err := filepath.Walk(cmd.SrcDir, walkCollect); err != nil {
			return err
		}
	}
	fds := &descriptor.FileDescriptorSet{}
	err = proto.Unmarshal(buf, fds)
	if err != nil {
		return err
	}
	// fileDescriptorProto
	for _, fdp := range fds.File {
		// options := fmt.Sprintf("%v\n", fdp.Options)
		// if strings.Contains(options, "[wxio.dna.store]:<go_mod:true >") {
		storish, err := proto.GetExtension(fdp.Options, store.E_Store)
		if err != nil {
		}
		if storish != nil {
			eStore := storish.(*store.Store)
			fmt.Printf("---%v--- %s\n", eStore.GoMod, *fdp.Options.GoPackage)
		}
		// }
	}
	return nil
}

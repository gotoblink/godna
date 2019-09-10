# Golang DNA cli - `godna`


## Usage - `godna generate -h`

<!--tmpl,code=plain:go run main.go generate -h -->
``` plain 

  Usage: godna generate [options] <output-dir>

  output directory eg .

  Options:
  --step-all, -s             run all steps (step-protoc, step-gomod-all, step-git-all)
  --step-protoc, -p          run the protoc
                              (default true)
  --step-gomod-all, -m       run all go mod steps. Overrides the individual steps (ie or-ed)
  --step-gomod-init          go mod init for all specified go modules.
                             Does not overwrite existing go.mod files.
                             ie containing
                             	import "dna/store.v1.proto";
                             	option (wxio.dna.store) = {
                             		go_mod : true
                             	};
                             store.v1.proto usually stored in vendor/wxio
  --step-gomod-cfg           go mod edit -require <specified in config>
  --step-gomod-local         need for local dev & tidy.
                             go mod edit -replace <proto import>=../[../]*/<local code>
  --step-gomod-tidy          go mod tidy
  --step-gomod-version       go mod edit -dropreplace & -require for imported modules
  --step-git-all, -g         git add, commit & tag
  --step-git-add             git add
  --step-git-add-commit      git add & commit
  --step-git-add-commit-tag  git add, commit & tag
  --help, -h                 display help

```
<!--/tmpl-->

## Example Config 
`.dna-cfg.ptron`
<!--tmpl,code=plain:cat .dna-cfg.ptron -->
``` plain 
src_dir    : "../dna"
go_package_prefix : "github.com/wxio/godna"
includes  : [
	"../dna/store",
	"../dna/vendor/googleapis",
	"../dna/vendor/grpc_gateway"
]
plugin_output_dir : <
	path: "."
	generator : < plugin : < go : < 
		paths: Source_Relative
	>>>
>
```
<!--/tmpl-->


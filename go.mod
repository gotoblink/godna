module github.com/wxio/godna

go 1.12

replace github.com/jpillora/opts v1.1.0 => github.com/millergarym/opts v1.1.4

//replace github.com/jpillora/opts => /home/garym/go/src-mods/github.com/jpillora/opts

//replace github.com/wxio/godna/pb/extensions/store => ./pb/extensions/store

require (
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.4.0-rc.2
	github.com/golangq/q v1.0.7
	github.com/jpillora/md-tmpl v1.2.2
	github.com/jpillora/opts v1.1.0
	github.com/kr/pretty v0.1.0 // indirect
	google.golang.org/protobuf v1.20.0
	gopkg.in/check.v1 v1.0.0-20180628173108-788fd7840127 // indirect
)

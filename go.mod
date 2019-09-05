module github.com/wxio/godna

go 1.12

replace github.com/jpillora/opts v1.1.0 => github.com/millergarym/opts v1.1.4

//replace github.com/jpillora/opts => /home/garym/go/src-mods/github.com/jpillora/opts

//replace github.com/wxio/godna/pb/extensions/store => ./pb/extensions/store

require (
	github.com/golang/glog v0.0.0-20160126235308-23def4e6c14b
	github.com/golang/protobuf v1.3.2
	github.com/golangq/q v1.0.7
	github.com/jpillora/opts v1.1.0
	google.golang.org/grpc v1.23.0
)

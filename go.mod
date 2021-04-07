module github.com/ssgo/deploy

go 1.12

require (
	github.com/gorilla/websocket v1.4.2
	github.com/ssgo/config v0.5.18
	github.com/ssgo/httpclient v0.5.18
	github.com/ssgo/log v0.5.18
	github.com/ssgo/s v1.5.5
	github.com/ssgo/tool v0.1.2
	github.com/ssgo/u v0.5.18
	gopkg.in/yaml.v2 v2.2.2
)

replace github.com/ssgo/s v1.5.5 => ../s

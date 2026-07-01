module watchflare-agent

go 1.26.4

require (
	github.com/BurntSushi/toml v1.6.0
	github.com/shirou/gopsutil/v4 v4.26.2
	google.golang.org/grpc v1.79.3
	google.golang.org/protobuf v1.36.11
	howett.net/plist v1.0.1
	watchflare/shared v0.0.0-00010101000000-000000000000
)

require (
	github.com/coreos/go-systemd/v22 v22.5.0 // indirect
	github.com/ebitengine/purego v0.10.0 // indirect
	github.com/go-ole/go-ole v1.3.0 // indirect
	github.com/godbus/dbus/v5 v5.0.4 // indirect
	github.com/lufia/plan9stats v0.0.0-20260330125221-c963978e514e // indirect
	github.com/power-devops/perfstat v0.0.0-20240221224432-82ca36839d55 // indirect
	github.com/tklauser/go-sysconf v0.3.16 // indirect
	github.com/tklauser/numcpus v0.11.0 // indirect
	github.com/yusufpapurcu/wmi v1.2.4 // indirect
	golang.org/x/net v0.54.0 // indirect
	golang.org/x/sys v0.44.0 // indirect
	golang.org/x/text v0.37.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260330182312-d5a96adf58d8 // indirect
)

replace watchflare/shared => ../shared

module vivapad

go 1.26.0

require (
	github.com/alecthomas/chroma/v2 v2.27.0
	github.com/ebitengine/purego v0.11.3
	github.com/fsnotify/fsnotify v1.10.1
	github.com/go-pdf/fpdf v0.9.0
	github.com/godbus/dbus/v5 v5.2.4
	github.com/google/shlex v0.0.0-20191202100458-e7afc7fbc510
	github.com/hexops/gotextdiff v1.0.3
	github.com/rivo/uniseg v0.4.7
	github.com/vivaui/viva v0.6.7
	golang.org/x/image v0.12.0
	golang.org/x/sys v0.50.0
	golang.org/x/text v0.44.0
)

replace github.com/vivaui/viva => github.com/caichangqing1120/viva v0.6.1-0.20261124074211-7922701a1098

require (
	github.com/dlclark/regexp2/v2 v2.2.1 // indirect
	github.com/dsnet/compress v0.0.0-20171208185109-cc9eb1d7ad76 // indirect
	github.com/gabstv/go-bsdiff v1.0.5 // indirect
	github.com/pelletier/go-toml/v2 v2.4.3 // indirect
)

tool github.com/vivaui/viva/cmd/viva

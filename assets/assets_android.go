//go:build android

package assets

import (
	"io/fs"
	"net/http"
	"runtime"
	"strings"
)

var (
	contentFRPC fs.FS
	contentFRPS fs.FS
	content     fs.FS

	FileSystem http.FileSystem

	prefixPath string
)

type emptyFS struct{}

func (emptyFS) Open(name string) (http.File, error) {
	return nil, &fs.PathError{Op: "open", Path: name, Err: fs.ErrNotExist}
}

// Load 选择当前进程该用的静态资源。
// 桌面版 frpc / frps 是两个进程，官方用一份 FileSystem 就够。
// Android AAR 把两边编进同一个进程，所以按调用方分开。
func Load(path string) {
	prefixPath = path
	switch {
	case prefixPath != "":
		FileSystem = http.Dir(prefixPath)
	default:
		if picked := pickContent(); picked != nil {
			FileSystem = http.FS(picked)
			return
		}
		FileSystem = emptyFS{}
	}
}

// Register 由 web/frpc、web/frps 的 init 调用。按调用方记到不同的 FS，避免后注册的覆盖先注册的。
func Register(fileSystem fs.FS) {
	subFs, err := fs.Sub(fileSystem, "dist")
	if err != nil {
		return
	}
	switch callerKind() {
	case "frpc":
		contentFRPC = subFs
	case "frps":
		contentFRPS = subFs
	default:
		content = subFs
	}
}

func pickContent() fs.FS {
	switch callerKind() {
	case "frpc":
		if contentFRPC != nil {
			return contentFRPC
		}
	case "frps":
		if contentFRPS != nil {
			return contentFRPS
		}
	}
	switch {
	case content != nil:
		return content
	case contentFRPC != nil && contentFRPS == nil:
		return contentFRPC
	case contentFRPS != nil && contentFRPC == nil:
		return contentFRPS
	default:
		return nil
	}
}

func callerKind() string {
	var pcs [48]uintptr
	n := runtime.Callers(2, pcs[:])
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if kind := kindFromFunction(frame.Function); kind != "" {
			return kind
		}
		if !more {
			break
		}
	}
	return ""
}

func kindFromFunction(name string) string {
	switch {
	case strings.Contains(name, "github.com/fatedier/frp/web/frpc."),
		strings.HasPrefix(name, "github.com/fatedier/frp/client."):
		return "frpc"
	case strings.Contains(name, "github.com/fatedier/frp/web/frps."),
		strings.HasPrefix(name, "github.com/fatedier/frp/server."):
		return "frps"
	default:
		return ""
	}
}

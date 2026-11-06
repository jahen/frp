//go:build android

package assets

import "testing"

func TestKindFromFunction(t *testing.T) {
	cases := []struct {
		name string
		want string
	}{
		{name: "github.com/fatedier/frp/web/frpc.init", want: "frpc"},
		{name: "github.com/fatedier/frp/client.NewService", want: "frpc"},
		{name: "github.com/fatedier/frp/client.(*Service).Run", want: "frpc"},
		{name: "github.com/fatedier/frp/web/frps.init", want: "frps"},
		{name: "github.com/fatedier/frp/server.NewService", want: "frps"},
		{name: "github.com/fatedier/frp/pkg/plugin/server.New", want: ""},
		{name: "github.com/fatedier/frp/pkg/util/http.NewServer", want: ""},
	}
	for _, tc := range cases {
		if got := kindFromFunction(tc.name); got != tc.want {
			t.Errorf("kindFromFunction(%q) = %q, want %q", tc.name, got, tc.want)
		}
	}
}

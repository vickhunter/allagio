package version

import "testing"

func TestResolveCommitOrder(t *testing.T) {
	cases := []struct {
		name, build, render, railway, want string
	}{
		{name: "render wins", build: "buildsha", render: "rendersha", railway: "railsha", want: "rendersha"},
		{name: "railway when render empty", build: "buildsha", railway: "railsha", want: "railsha"},
		{name: "build when hosts empty", build: "buildsha", want: "buildsha"},
		{name: "unknown when all empty", want: "unknown"},
		{name: "blank build is unknown", build: "  ", render: " ", railway: "\t", want: "unknown"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := Resolve(tc.build, tc.render, tc.railway); got != tc.want {
				t.Fatalf("Resolve() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestCommitReadsHostEnv(t *testing.T) {
	t.Setenv("RENDER_GIT_COMMIT", "from-render")
	t.Setenv("RAILWAY_GIT_COMMIT_SHA", "from-railway")
	if got := Commit(); got != "from-render" {
		t.Fatalf("Commit() = %q", got)
	}
	t.Setenv("RENDER_GIT_COMMIT", "")
	if got := Commit(); got != "from-railway" {
		t.Fatalf("railway Commit() = %q", got)
	}
	t.Setenv("RAILWAY_GIT_COMMIT_SHA", "")
	if got := Commit(); got != "unknown" {
		t.Fatalf("default Commit() = %q", got)
	}
}

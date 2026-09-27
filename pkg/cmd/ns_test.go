package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

func writeKubeconfig(t *testing.T, cfg *api.Config) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "config")
	if err := clientcmd.WriteToFile(*cfg, path); err != nil {
		t.Fatal(err)
	}
	return path
}

func baseConfig() *api.Config {
	cfg := api.NewConfig()
	cfg.Clusters["c1"] = &api.Cluster{Server: "https://c1.invalid"}
	cfg.Clusters["c2"] = &api.Cluster{Server: "https://c2.invalid"}
	cfg.AuthInfos["admin/c1"] = &api.AuthInfo{Token: "x"}
	cfg.AuthInfos["admin/c2"] = &api.AuthInfo{Token: "y"}
	cfg.Contexts["dev"] = &api.Context{Cluster: "c1", AuthInfo: "admin/c1", Namespace: "zeta"}
	cfg.Contexts["ops"] = &api.Context{Cluster: "c1", AuthInfo: "admin/c1", Namespace: "alpha"}
	cfg.CurrentContext = "dev"
	return cfg
}

func run(t *testing.T, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	cmd := NewCmdNamespace("ns", genericiooptions.IOStreams{In: os.Stdin, Out: &out, ErrOut: &out})
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func load(t *testing.T, path string) *api.Config {
	t.Helper()
	cfg, err := clientcmd.LoadFromFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestNamespace(t *testing.T) {
	t.Setenv("KUBECONFIG", "")
	path := writeKubeconfig(t, baseConfig())

	if out, err := run(t, "--kubeconfig", path); err != nil || out != "zeta\n" {
		t.Fatalf("current: got %q, %v", out, err)
	}
	if out, err := run(t, "--kubeconfig", path, "--list"); err != nil || out != "alpha\nzeta\n" {
		t.Fatalf("list: got %q, %v", out, err)
	}
	if _, err := run(t, "--kubeconfig", path, "a", "b"); err == nil {
		t.Fatal("expected error for two args")
	}
	if _, err := run(t, "--kubeconfig", path, "-n", "a", "b"); err == nil {
		t.Fatal("expected error for both --namespace and arg")
	}

	// switching must write to the --kubeconfig file and leave existing contexts alone
	if _, err := run(t, "--kubeconfig", path, "beta"); err != nil {
		t.Fatal(err)
	}
	cfg := load(t, path)
	if cfg.CurrentContext != "beta/c1/admin" {
		t.Fatalf("current-context = %q", cfg.CurrentContext)
	}
	if got := cfg.Contexts["beta/c1/admin"]; got == nil || got.Namespace != "beta" || got.Cluster != "c1" || got.AuthInfo != "admin/c1" {
		t.Fatalf("new context = %+v", got)
	}
	if cfg.Contexts["dev"].Namespace != "zeta" {
		t.Fatal("existing context was modified")
	}
	if out, err := run(t, "--kubeconfig", path); err != nil || out != "beta\n" {
		t.Fatalf("after switch: got %q, %v", out, err)
	}
}

func TestNamespaceFlags(t *testing.T) {
	t.Setenv("KUBECONFIG", "")
	path := writeKubeconfig(t, baseConfig())

	// --cluster/--user override the current context's values
	if _, err := run(t, "--kubeconfig", path, "--cluster", "c2", "--user", "admin/c2", "beta"); err != nil {
		t.Fatal(err)
	}
	cfg := load(t, path)
	if got := cfg.Contexts[cfg.CurrentContext]; cfg.CurrentContext != "beta/c2/admin" || got.Cluster != "c2" || got.AuthInfo != "admin/c2" {
		t.Fatalf("current-context = %q, %+v", cfg.CurrentContext, got)
	}

	// --context updates the named context in place
	if _, err := run(t, "--kubeconfig", path, "--context", "ops", "gamma"); err != nil {
		t.Fatal(err)
	}
	cfg = load(t, path)
	if cfg.CurrentContext != "ops" || cfg.Contexts["ops"].Namespace != "gamma" || cfg.Contexts["ops"].Cluster != "c1" {
		t.Fatalf("current-context = %q, ops = %+v", cfg.CurrentContext, cfg.Contexts["ops"])
	}
}

func TestNamespaceMultiFileKubeconfig(t *testing.T) {
	cfg := baseConfig()
	first := writeKubeconfig(t, &api.Config{CurrentContext: "dev", Contexts: map[string]*api.Context{"dev": cfg.Contexts["dev"]}})
	second := writeKubeconfig(t, &api.Config{Clusters: cfg.Clusters, AuthInfos: cfg.AuthInfos})
	secondBefore, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBECONFIG", first+string(filepath.ListSeparator)+second)

	if _, err := run(t, "beta"); err != nil {
		t.Fatal(err)
	}
	if got := load(t, first); got.CurrentContext != "beta/c1/admin" || got.Contexts["beta/c1/admin"] == nil {
		t.Fatalf("first file: current-context = %q, contexts = %v", got.CurrentContext, got.Contexts)
	}
	if secondAfter, _ := os.ReadFile(second); !bytes.Equal(secondBefore, secondAfter) {
		t.Fatal("second kubeconfig file was modified")
	}
	if out, err := run(t); err != nil || out != "beta\n" {
		t.Fatalf("after switch: got %q, %v", out, err)
	}
}

func TestGenerateContextNameCollision(t *testing.T) {
	contexts := map[string]*api.Context{}
	a := &api.Context{Namespace: "ns", Cluster: "c", AuthInfo: "admin/a"}
	b := &api.Context{Namespace: "ns", Cluster: "c", AuthInfo: "admin/b"}

	nameA := generateContextName(a, contexts)
	if nameA != "ns/c/admin" {
		t.Fatalf("got %q", nameA)
	}
	contexts[nameA] = a
	if got := generateContextName(a, contexts); got != nameA {
		t.Fatalf("same context must reuse its name, got %q", got)
	}
	if got := generateContextName(b, contexts); got != "ns/c/admin/b" {
		t.Fatalf("colliding context must get the full user name, got %q", got)
	}
}

func TestHelpUsesPluginName(t *testing.T) {
	var out bytes.Buffer
	cmd := NewCmdNamespace("change-ns", genericiooptions.IOStreams{Out: &out, ErrOut: &out})
	cmd.SetOut(&out)
	cmd.SetArgs([]string{"--help"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "kubectl change-ns [new-namespace]") || !strings.Contains(out.String(), "kubectl change-ns --list") {
		t.Fatalf("help:\n%s", out.String())
	}
}

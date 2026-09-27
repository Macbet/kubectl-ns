/*
Copyright 2018 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package main

import (
	"os"
	"path/filepath"
	"strings"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/Macbet/kubectl-ns/pkg/cmd"
)

// version is set at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	root := cmd.NewCmdNamespace(pluginName(), genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr})
	root.Version = version
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// pluginName derives the kubectl subcommand from the binary name, so help text is
// right both for "kubectl-ns" and for krew's "kubectl-change_ns" symlink.
func pluginName() string {
	name := strings.TrimSuffix(filepath.Base(os.Args[0]), ".exe")
	name, ok := strings.CutPrefix(name, "kubectl-")
	if !ok {
		return "ns"
	}
	return strings.ReplaceAll(name, "_", "-")
}

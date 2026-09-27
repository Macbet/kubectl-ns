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

package cmd

import (
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
)

const namespaceExample = `
	# view the current namespace in your KUBECONFIG
	%[1]s

	# view all of the namespaces in use by contexts in your KUBECONFIG
	%[1]s --list

	# switch your current-context to one that contains the desired namespace
	%[1]s foo
`

var errNoContext = fmt.Errorf("no context is currently set, use %q to select a new one", "kubectl config use-context <context>")

// NamespaceOptions provides information required to update
// the current context on a user's KUBECONFIG
type NamespaceOptions struct {
	configFlags *genericclioptions.ConfigFlags

	resultingContext     *api.Context
	resultingContextName string

	rawConfig      api.Config
	listNamespaces bool

	genericiooptions.IOStreams
}

// NewNamespaceOptions provides an instance of NamespaceOptions with default values
func NewNamespaceOptions(streams genericiooptions.IOStreams) *NamespaceOptions {
	return &NamespaceOptions{
		configFlags: genericclioptions.NewConfigFlags(true),
		IOStreams:   streams,
	}
}

// NewCmdNamespace provides a cobra command wrapping NamespaceOptions.
// name is the plugin name as typed after "kubectl", e.g. "ns" or "change-ns".
func NewCmdNamespace(name string, streams genericiooptions.IOStreams) *cobra.Command {
	o := NewNamespaceOptions(streams)

	cmd := &cobra.Command{
		Use:          name + " [new-namespace] [flags]",
		Short:        "View or set the current namespace",
		Example:      fmt.Sprintf(namespaceExample, "kubectl "+name),
		Annotations:  map[string]string{cobra.CommandDisplayNameAnnotation: "kubectl " + name},
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		RunE: func(_ *cobra.Command, args []string) error {
			if err := o.Complete(args); err != nil {
				return err
			}
			if err := o.Validate(); err != nil {
				return err
			}
			return o.Run()
		},
	}

	cmd.Flags().BoolVar(&o.listNamespaces, "list", o.listNamespaces, "if true, print the list of all namespaces in the current KUBECONFIG")
	o.configFlags.AddFlags(cmd.Flags())

	return cmd
}

// Complete sets all information required for updating the current context
func (o *NamespaceOptions) Complete(args []string) error {
	var err error
	o.rawConfig, err = o.configFlags.ToRawKubeConfigLoader().RawConfig()
	if err != nil {
		return err
	}

	namespace := *o.configFlags.Namespace
	if len(args) > 0 {
		if namespace != "" {
			return errors.New("cannot specify both a --namespace value and a new namespace argument")
		}
		namespace = args[0]
	}

	// if no namespace argument or flag value was specified, then there
	// is no need to generate a resulting context
	if namespace == "" {
		return nil
	}

	currentContext, exists := o.rawConfig.Contexts[o.rawConfig.CurrentContext]
	if !exists {
		return errNoContext
	}

	o.resultingContext = api.NewContext()
	o.resultingContext.Cluster = currentContext.Cluster
	o.resultingContext.AuthInfo = currentContext.AuthInfo

	// if a target context is explicitly provided by the user,
	// use that as our reference for the final, resulting context
	if userContext := *o.configFlags.Context; userContext != "" {
		o.resultingContextName = userContext
		if userCtx, exists := o.rawConfig.Contexts[userContext]; exists {
			o.resultingContext = userCtx.DeepCopy()
		}
	}

	// override context info with user provided values
	o.resultingContext.Namespace = namespace
	if cluster := *o.configFlags.ClusterName; cluster != "" {
		o.resultingContext.Cluster = cluster
	}
	if authInfo := *o.configFlags.AuthInfoName; authInfo != "" {
		o.resultingContext.AuthInfo = authInfo
	}

	// generate a unique context name based on its new values if
	// user did not explicitly request a context by name
	if o.resultingContextName == "" {
		o.resultingContextName = generateContextName(o.resultingContext, o.rawConfig.Contexts)
	}

	return nil
}

// generateContextName returns "<namespace>/<cluster>/<user>", where user is cut at
// the first "/" (OpenShift-style "user/cluster" names). A name already used by a
// different context is never reused: the full user name is tried next, then "-2", "-3"...
func generateContextName(ctx *api.Context, contexts map[string]*api.Context) string {
	free := func(name string) bool {
		existing, ok := contexts[name]
		return !ok || isContextEqual(ctx, existing)
	}

	shortAuthInfo, _, _ := strings.Cut(ctx.AuthInfo, "/")
	base := joinNonEmpty(ctx.Namespace, ctx.Cluster, shortAuthInfo)
	if free(base) {
		return base
	}
	if full := joinNonEmpty(ctx.Namespace, ctx.Cluster, ctx.AuthInfo); free(full) {
		return full
	}
	for i := 2; ; i++ {
		if name := fmt.Sprintf("%s-%d", base, i); free(name) {
			return name
		}
	}
}

func joinNonEmpty(parts ...string) string {
	return strings.Join(slices.DeleteFunc(parts, func(s string) bool { return s == "" }), "/")
}

// Validate ensures that all required arguments and flag values are provided
func (o *NamespaceOptions) Validate() error {
	if o.rawConfig.CurrentContext == "" && *o.configFlags.Context == "" {
		return errNoContext
	}
	return nil
}

// Run lists all available namespaces on a user's KUBECONFIG or updates the
// current context based on a provided namespace.
func (o *NamespaceOptions) Run() error {
	if o.resultingContext != nil {
		return o.setNamespace(o.resultingContext, o.resultingContextName)
	}

	if !o.listNamespaces {
		name := o.rawConfig.CurrentContext
		if *o.configFlags.Context != "" {
			name = *o.configFlags.Context
		}
		c, exists := o.rawConfig.Contexts[name]
		if !exists {
			return fmt.Errorf("context %q not found in your configuration", name)
		}
		if c.Namespace == "" {
			return fmt.Errorf("no namespace is set for context %q", name)
		}
		_, err := fmt.Fprintln(o.Out, c.Namespace)
		return err
	}

	namespaces := map[string]struct{}{}
	for _, c := range o.rawConfig.Contexts {
		if c.Namespace != "" {
			namespaces[c.Namespace] = struct{}{}
		}
	}
	for _, n := range slices.Sorted(maps.Keys(namespaces)) {
		if _, err := fmt.Fprintln(o.Out, n); err != nil {
			return err
		}
	}
	return nil
}

func isContextEqual(ctxA, ctxB *api.Context) bool {
	if ctxA == nil || ctxB == nil {
		return false
	}
	return ctxA.Cluster == ctxB.Cluster &&
		ctxA.Namespace == ctxB.Namespace &&
		ctxA.AuthInfo == ctxB.AuthInfo
}

// setNamespace receives a "desired" context state and determines if a similar context
// is already present in a user's KUBECONFIG. If one is not, then a new context is added
// to the user's config under the provided destination name.
// The current context field is updated to point to the new context.
func (o *NamespaceOptions) setNamespace(fromContext *api.Context, withContextName string) error {
	if fromContext.Namespace == "" {
		return errors.New("a non-empty namespace must be provided")
	}

	// determine if we have already saved this context to the user's KUBECONFIG before
	// if so, simply switch the current context to the existing one.
	if existing, exists := o.rawConfig.Contexts[withContextName]; !exists || !isContextEqual(fromContext, existing) {
		o.rawConfig.Contexts[withContextName] = fromContext
	}
	o.rawConfig.CurrentContext = withContextName

	// honor --kubeconfig and $KUBECONFIG when writing the result back
	configAccess := o.configFlags.ToRawKubeConfigLoader().ConfigAccess()
	if err := clientcmd.ModifyConfig(configAccess, o.rawConfig, true); err != nil {
		return err
	}

	_, err := fmt.Fprintf(o.Out, "namespace changed to %q\n", fromContext.Namespace)
	return err
}

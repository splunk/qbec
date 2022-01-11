// Copyright 2021 Splunk Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build integration
// +build integration

package commands

import (
	"context"
	"fmt"
	"os"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	corev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
)

var (
	once        sync.Once
	contextName string
	restConfig  *rest.Config
	coreClient  *corev1.CoreV1Client
)

func initialize(t *testing.T) {
	contextName = os.Getenv("QBEC_CONTEXT")
	if contextName == "" {
		contextName = "kind-kind"
	}
	c, err := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		clientcmd.NewDefaultClientConfigLoadingRules(),
		&clientcmd.ConfigOverrides{CurrentContext: contextName},
	).ClientConfig()
	require.NoError(t, err)
	restConfig = c
	coreClient, err = corev1.NewForConfig(restConfig)
	require.NoError(t, err)
}

type integrationScaffold struct {
	baseScaffold
	ns string
}

func newNamespace(t *testing.T) (name string, reset func()) {
	once.Do(func() { initialize(t) })
	ns, err := coreClient.Namespaces().Create(context.TODO(), &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "qbec-test-",
		},
		Spec: v1.NamespaceSpec{},
	}, metav1.CreateOptions{})
	require.NoError(t, err)
	return ns.GetName(), func() {
		pp := metav1.DeletePropagationForeground
		err := coreClient.Namespaces().Delete(context.TODO(), ns.GetName(), metav1.DeleteOptions{PropagationPolicy: &pp})
		if err != nil {
			fmt.Println("Error deleting namespace", ns.GetName(), err)
		}
	}
}

func newIntegrationScaffold(t *testing.T, ns string, dir string) *integrationScaffold {
	once.Do(func() { initialize(t) })
	b := newBaseScaffold(t, dir, nil)
	return &integrationScaffold{
		baseScaffold: b,
		ns:           ns,
	}
}

func (s *integrationScaffold) executeCommand(testArgs ...string) error {
	args := append(testArgs,
		"--force:k8s-context="+contextName,
		"--force:k8s-namespace="+s.ns,
	)
	return s.baseScaffold.executeCommand(args...)
}

func (s *integrationScaffold) sub() *integrationScaffold {
	return &integrationScaffold{
		baseScaffold: s.baseScaffold.sub(),
		ns:           s.ns,
	}
}

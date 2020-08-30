// +build integration

package commands

import (
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
	ns, err := coreClient.Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			GenerateName: "qbec-test-",
		},
		Spec: v1.NamespaceSpec{},
	})
	require.NoError(t, err)
	return ns.GetName(), func() {
		pp := metav1.DeletePropagationForeground
		err := coreClient.Namespaces().Delete(ns.GetName(), &metav1.DeleteOptions{PropagationPolicy: &pp})
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

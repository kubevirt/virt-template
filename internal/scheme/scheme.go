package scheme

import (
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"

	templatev1alpha1 "kubevirt.io/virt-template/api/v1alpha1"
)

func New() *runtime.Scheme {
	scheme := runtime.NewScheme()

	utilruntime.Must(clientgoscheme.AddToScheme(scheme))

	utilruntime.Must(templatev1alpha1.AddToScheme(scheme))
	// +kubebuilder:scaffold:scheme

	return scheme
}

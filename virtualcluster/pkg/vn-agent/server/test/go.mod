// This is defined as a seperate module as the tests here depend on k8s.io/kubernetes,
// which cause issues when consumers of virtualcluster's APIs attempt to import the module.
module sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/vn-agent/server/test

go 1.13

require (
	github.com/google/cadvisor v0.35.0
	github.com/pkg/errors v0.8.1
	github.com/stretchr/testify v1.4.0
	k8s.io/api v0.18.6
	k8s.io/apimachinery v0.18.6
	k8s.io/apiserver v0.18.6
	k8s.io/client-go v0.18.6
	k8s.io/cri-api v0.18.6
	k8s.io/kubernetes v1.18.6
	k8s.io/utils v0.0.0-20200619165400-6e3d28b6ed19

	// v0.0.0 does not really exist - the `replace` directive below pins it to
	// the local copy stored in the parent directories.
	sigs.k8s.io/cluster-api-provider-nested/virtualcluster v0.0.0
)

// We use the replace directive to pin k8s.io dependencies that we don't directly
// depend on here to the same version as the k8s.io/kubernetes dependency above.
// This is neccessary as k8s.io/kubernetes depends on v0.0.0 of each staging,
// and itself makes use of replace directives (that will not work here) to make
// use of the 'staging' versions of dependencies within the repository.
// TODO: ideally, we should not depend on k8s.io/kubernetes at all, which in turn
//  will avoid us needing to pin dependencies here that we don't actually directly
//  depend on. This is a product of Kubernetes' staging hacks in the main repo,
//  and it is not advised that external projects depend upon k8s.io/kubernetes for
//  this exact reason.
replace (
	k8s.io/api => k8s.io/api v0.18.6
	k8s.io/apiextensions-apiserver => k8s.io/apiextensions-apiserver v0.18.6
	k8s.io/apimachinery => k8s.io/apimachinery v0.18.6
	k8s.io/apiserver => k8s.io/apiserver v0.18.6
	k8s.io/cli-runtime => k8s.io/cli-runtime v0.18.6
	k8s.io/client-go => k8s.io/client-go v0.18.6
	k8s.io/cloud-provider => k8s.io/cloud-provider v0.18.6
	k8s.io/cluster-bootstrap => k8s.io/cluster-bootstrap v0.18.6
	k8s.io/code-generator => k8s.io/code-generator v0.18.6
	k8s.io/component-base => k8s.io/component-base v0.18.6
	k8s.io/cri-api => k8s.io/cri-api v0.18.6
	k8s.io/csi-translation-lib => k8s.io/csi-translation-lib v0.18.6
	k8s.io/kube-aggregator => k8s.io/kube-aggregator v0.18.6
	k8s.io/kube-controller-manager => k8s.io/kube-controller-manager v0.18.6
	k8s.io/kube-proxy => k8s.io/kube-proxy v0.18.6
	k8s.io/kube-scheduler => k8s.io/kube-scheduler v0.18.6
	k8s.io/kubectl => k8s.io/kubectl v0.18.6
	k8s.io/kubelet => k8s.io/kubelet v0.18.6
	k8s.io/legacy-cloud-providers => k8s.io/legacy-cloud-providers v0.18.6
	k8s.io/metrics => k8s.io/metrics v0.18.6
	k8s.io/sample-apiserver => k8s.io/sample-apiserver v0.18.6

	// Force Go to use the module as defined on disk in the parent module else
	// we'll have to bump a revision/version every time anything changes in
	// virtual-cluster.
	sigs.k8s.io/cluster-api-provider-nested/virtualcluster => ../../../../
)

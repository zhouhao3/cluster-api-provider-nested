/*
Copyright 2019 The Kubernetes Authors.

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

package conversion

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"strings"

	"github.com/pkg/errors"
	v1 "k8s.io/api/core/v1"
	v1scheduling "k8s.io/api/scheduling/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation"
	v1core "k8s.io/client-go/kubernetes/typed/core/v1"
	listersv1 "k8s.io/client-go/listers/core/v1"

	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/apis/tenancy/v1alpha1"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/constants"
	"sigs.k8s.io/cluster-api-provider-nested/virtualcluster/pkg/syncer/util/featuregate"
)

// ToClusterKey makes a unique key which is used to create the root namespace in super master for a virtual cluster.
// To avoid name conflict, the key uses the format <namespace>-<hash>-<name>
func ToClusterKey(vc *v1alpha1.VirtualCluster) string {
	digest := sha256.Sum256([]byte(vc.GetUID()))
	return vc.GetNamespace() + "-" + hex.EncodeToString(digest[0:])[0:6] + "-" + vc.GetName()
}

func ToSuperMasterNamespace(cluster, ns string) string {
	targetNamespace := strings.Join([]string{cluster, ns}, "-")
	if len(targetNamespace) > validation.DNS1123SubdomainMaxLength {
		digest := sha256.Sum256([]byte(targetNamespace))
		return targetNamespace[0:57] + "-" + hex.EncodeToString(digest[0:])[0:5]
	}
	return targetNamespace
}

// GetVirtualNamespace is used to find the corresponding namespace in tenant master for objects created in super master originally, e.g., events.
func GetVirtualNamespace(nsLister listersv1.NamespaceLister, pNamespace string) (cluster, namespace string, err error) {
	vcInfo, err := nsLister.Get(pNamespace)
	if err != nil {
		return
	}

	if v, ok := vcInfo.GetAnnotations()[constants.LabelCluster]; ok {
		cluster = v
	}
	if v, ok := vcInfo.GetAnnotations()[constants.LabelNamespace]; ok {
		namespace = v
	}
	return
}

func GetVirtualOwner(meta metav1.Object) (cluster, namespace string) {
	cluster = meta.GetAnnotations()[constants.LabelCluster]
	namespace = meta.GetAnnotations()[constants.LabelNamespace]
	return cluster, namespace
}

func GetKubeConfigOfVC(c v1core.CoreV1Interface, vc *v1alpha1.VirtualCluster) ([]byte, error) {
	if adminKubeConfig, exists := vc.GetAnnotations()[constants.LabelAdminKubeConfig]; exists {
		decoded, err := base64.StdEncoding.DecodeString(adminKubeConfig)
		if err != nil {
			return nil, fmt.Errorf("failed to decode kubeconfig from annotations %s: %v", constants.LabelAdminKubeConfig, err)
		}
		return decoded, nil
	}

	clusterName := ToClusterKey(vc)
	adminKubeConfigSecret, err := c.Secrets(clusterName).Get(context.TODO(), constants.KubeconfigAdminSecretName, metav1.GetOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to get secret (%s) for virtual cluster in root namespace %s: %v", constants.KubeconfigAdminSecretName, clusterName, err)
	}
	return adminKubeConfigSecret.Data[constants.KubeconfigAdminSecretName], nil
}

func BuildMetadata(cluster, vcns, vcname, targetNamespace string, obj runtime.Object) (runtime.Object, error) {
	target := obj.DeepCopyObject()
	m, err := meta.Accessor(target)
	if err != nil {
		return nil, err
	}

	ownerReferencesStr, err := json.Marshal(m.GetOwnerReferences())
	if err != nil {
		return nil, errors.Wrap(err, "failed to marshal owner references")
	}

	var tenantScopeMetaInAnnotation = map[string]string{
		constants.LabelCluster:         cluster,
		constants.LabelUID:             string(m.GetUID()),
		constants.LabelOwnerReferences: string(ownerReferencesStr),
		constants.LabelNamespace:       m.GetNamespace(),
		constants.LabelVCName:          vcname,
		constants.LabelVCNamespace:     vcns,
	}

	ResetMetadata(m)
	if len(targetNamespace) > 0 {
		m.SetNamespace(targetNamespace)
	}

	anno := m.GetAnnotations()
	if anno == nil {
		anno = make(map[string]string)
	}
	for k, v := range tenantScopeMetaInAnnotation {
		anno[k] = v
	}
	m.SetAnnotations(anno)

	labels := m.GetLabels()
	if labels == nil {
		labels = make(map[string]string)
	}
	var tenantScopeMetaInLabel = map[string]string{
		constants.LabelVCName:      vcname,
		constants.LabelVCNamespace: vcns,
	}
	for k, v := range tenantScopeMetaInLabel {
		labels[k] = v
	}
	m.SetLabels(labels)

	return target, nil
}

func BuildSuperMasterNamespace(cluster, vcName, vcNamespace, vcUID string, obj runtime.Object) (runtime.Object, error) {
	target := obj.DeepCopyObject()
	m, err := meta.Accessor(target)
	if err != nil {
		return nil, err
	}

	anno := m.GetAnnotations()
	if anno == nil {
		anno = make(map[string]string)
	}
	anno[constants.LabelCluster] = cluster
	anno[constants.LabelUID] = string(m.GetUID())
	anno[constants.LabelNamespace] = m.GetName()
	// We put owner information in annotation instead of  metav1.OwnerReference because vc is a namespace scope resource
	// and metav1.OwnerReference does not provide namespace field. The owner information is needed for super master ns gc.
	anno[constants.LabelVCName] = vcName
	anno[constants.LabelVCNamespace] = vcNamespace
	anno[constants.LabelVCUID] = vcUID
	m.SetAnnotations(anno)

	ResetMetadata(m)

	targetName := ToSuperMasterNamespace(cluster, m.GetName())
	m.SetName(targetName)
	return target, nil
}

func ResetMetadata(obj metav1.Object) {
	obj.SetSelfLink("")
	obj.SetUID("")
	obj.SetResourceVersion("")
	obj.SetGeneration(0)
	obj.SetDeletionTimestamp(nil)
	obj.SetDeletionGracePeriodSeconds(nil)
	obj.SetOwnerReferences(nil)
	obj.SetFinalizers(nil)
	obj.SetClusterName("")
}

func BuildVirtualEvent(cluster string, pEvent *v1.Event, vObj metav1.Object) *v1.Event {
	vEvent := pEvent.DeepCopy()
	ResetMetadata(vEvent)
	vEvent.SetNamespace(vObj.GetNamespace())
	vEvent.InvolvedObject.Namespace = vObj.GetNamespace()
	vEvent.InvolvedObject.UID = vObj.GetUID()
	vEvent.InvolvedObject.ResourceVersion = ""

	vEvent.Message = strings.ReplaceAll(vEvent.Message, cluster+"-", "")
	vEvent.Message = strings.ReplaceAll(vEvent.Message, cluster, "")

	return vEvent
}

func BuildVirtualStorageClass(cluster string, pStorageClass *storagev1.StorageClass) *storagev1.StorageClass {
	vStorageClass := pStorageClass.DeepCopy()
	ResetMetadata(vStorageClass)
	return vStorageClass
}

func BuildVirtualPriorityClass(cluster string, pPriorityClass *v1scheduling.PriorityClass) *v1scheduling.PriorityClass {
	vPriorityClass := pPriorityClass.DeepCopy()
	ResetMetadata(vPriorityClass)
	return vPriorityClass
}

func BuildVirtualCRD(cluster string, pCRD *v1beta1.CustomResourceDefinition) *v1beta1.CustomResourceDefinition {
	vCRD := pCRD.DeepCopy()
	ResetMetadata(vCRD)
	return vCRD
}

func BuildVirtualPersistentVolume(cluster, vcNS, vcName string, pPV *v1.PersistentVolume, vPVC *v1.PersistentVolumeClaim) *v1.PersistentVolume {
	vPVobj, _ := BuildMetadata(cluster, vcNS, vcName, "", pPV)
	vPV := vPVobj.(*v1.PersistentVolume)
	// The pv needs to bind with the vPVC
	vPV.Spec.ClaimRef.Namespace = vPVC.Namespace
	vPV.Spec.ClaimRef.UID = vPVC.UID
	return vPV
}

// IsControlPlaneService will return if the namespacedName matches the proper
// NamespacedName in the tenant control plane
func IsControlPlaneService(service *v1.Service, cluster string) bool {
	kubernetesNamespace := ToSuperMasterNamespace(cluster, metav1.NamespaceDefault)
	kubernetesService := "kubernetes"

	// If the super cluster service networking is enabled this supports allowing
	// the "real" apiserver-svc to propagate to the tenant default/kubernetes service
	if featuregate.DefaultFeatureGate.Enabled(featuregate.SuperClusterServiceNetwork) {
		kubernetesNamespace = cluster
		kubernetesService = "apiserver-svc"
	}

	// If the super cluster pooling is enabled, the service in tenant default namepsace
	// is used.
	if featuregate.DefaultFeatureGate.Enabled(featuregate.SuperClusterPooling) {
		kubernetesNamespace = metav1.NamespaceDefault
		kubernetesService = "kubernetes"
	}
	return service.Namespace == kubernetesNamespace && service.Name == kubernetesService
}

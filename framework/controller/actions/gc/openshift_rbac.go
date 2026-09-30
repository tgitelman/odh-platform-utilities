package gc

import (
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/opendatahub-io/odh-platform-utilities/framework/resources"
)

const (
	kubernetesRBACGroup         = "rbac.authorization.k8s.io"
	openShiftAuthorizationGroup = "authorization.openshift.io"

	kindRole               = "Role"
	kindRoleBinding        = "RoleBinding"
	kindClusterRole        = "ClusterRole"
	kindClusterRoleBinding = "ClusterRoleBinding"
)

// openShiftRBACAliasKinds: kinds shared by rbac.authorization.k8s.io and
// authorization.openshift.io (same etcd object). RoleBindingRestriction is not an alias.
//
//nolint:gochecknoglobals // Immutable kind set.
var openShiftRBACAliasKinds = map[string]struct{}{
	kindRole:               {},
	kindRoleBinding:        {},
	kindClusterRole:        {},
	kindClusterRoleBinding: {},
}

func isOpenShiftRBACAlias(gvk schema.GroupVersionKind) bool {
	if gvk.Group != openShiftAuthorizationGroup {
		return false
	}

	_, ok := openShiftRBACAliasKinds[gvk.Kind]

	return ok
}

func kubernetesRBACAliasKinds(items []resources.Resource) map[string]struct{} {
	kinds := make(map[string]struct{})

	for i := range items {
		gvk := items[i].GroupVersionKind()
		if gvk.Group != kubernetesRBACGroup {
			continue
		}

		if _, ok := openShiftRBACAliasKinds[gvk.Kind]; ok {
			kinds[gvk.Kind] = struct{}{}
		}
	}

	return kinds
}

// preferKubernetesRBACResources drops OpenShift RBAC aliases from deletable when
// the Kubernetes twin is in both deletable and listable. See pkg/controller/gc.
func preferKubernetesRBACResources(deletable, listable []resources.Resource) []resources.Resource {
	deletableKubernetes := kubernetesRBACAliasKinds(deletable)
	if len(deletableKubernetes) == 0 {
		return deletable
	}

	listableKubernetes := kubernetesRBACAliasKinds(listable)
	preferKubernetes := make(map[string]struct{})

	for kind := range deletableKubernetes {
		if _, ok := listableKubernetes[kind]; ok {
			preferKubernetes[kind] = struct{}{}
		}
	}

	if len(preferKubernetes) == 0 {
		return deletable
	}

	filtered := make([]resources.Resource, 0, len(deletable))

	for i := range deletable {
		gvk := deletable[i].GroupVersionKind()
		if isOpenShiftRBACAlias(gvk) {
			if _, drop := preferKubernetes[gvk.Kind]; drop {
				continue
			}
		}

		filtered = append(filtered, deletable[i])
	}

	return filtered
}

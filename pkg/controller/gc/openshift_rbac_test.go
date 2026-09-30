package gc

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"github.com/opendatahub-io/odh-platform-utilities/pkg/resources"
)

func TestIsOpenShiftRBACAlias(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		gvk  schema.GroupVersionKind
		want bool
	}{
		{
			name: "openshift clusterrolebinding",
			gvk:  schema.GroupVersionKind{Group: openShiftAuthorizationGroup, Version: "v1", Kind: kindClusterRoleBinding},
			want: true,
		},
		{
			name: "openshift role",
			gvk:  schema.GroupVersionKind{Group: openShiftAuthorizationGroup, Version: "v1", Kind: kindRole},
			want: true,
		},
		{
			name: "kubernetes clusterrolebinding",
			gvk:  schema.GroupVersionKind{Group: kubernetesRBACGroup, Version: "v1", Kind: kindClusterRoleBinding},
			want: false,
		},
		{
			name: "openshift-only rolebindingrestriction",
			gvk:  schema.GroupVersionKind{Group: openShiftAuthorizationGroup, Version: "v1", Kind: "RoleBindingRestriction"},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := isOpenShiftRBACAlias(tt.gvk); got != tt.want {
				t.Errorf("isOpenShiftRBACAlias(%s) = %t, want %t", tt.gvk, got, tt.want)
			}
		})
	}
}

func TestPreferKubernetesRBACResources_DropsOpenShiftAliasesWhenKubernetesListableAndDeletable(t *testing.T) {
	t.Parallel()

	deletable := []resources.Resource{
		resourceWithGVK("apps", "deployments", "Deployment", true),
		resourceWithGVK(kubernetesRBACGroup, "clusterrolebindings", kindClusterRoleBinding, false),
		resourceWithGVK(openShiftAuthorizationGroup, "clusterrolebindings", kindClusterRoleBinding, false),
		resourceWithGVK(kubernetesRBACGroup, "roles", kindRole, true),
		resourceWithGVK(openShiftAuthorizationGroup, "roles", kindRole, true),
		resourceWithGVK(openShiftAuthorizationGroup, "rolebindingrestrictions", "RoleBindingRestriction", true),
	}
	listable := deletable

	got := preferKubernetesRBACResources(deletable, listable)
	if len(got) != 4 {
		t.Fatalf("expected 4 resources after filter, got %d", len(got))
	}

	found := map[string]bool{}

	for i := range got {
		gvk := got[i].GroupVersionKind()
		found[gvk.Group+"/"+gvk.Kind] = true

		if isOpenShiftRBACAlias(gvk) {
			t.Errorf("OpenShift RBAC alias %s should have been filtered", gvk)
		}
	}

	for _, key := range []string{
		"apps/Deployment",
		kubernetesRBACGroup + "/" + kindClusterRoleBinding,
		kubernetesRBACGroup + "/" + kindRole,
		openShiftAuthorizationGroup + "/RoleBindingRestriction",
	} {
		if !found[key] {
			t.Errorf("expected resource %s in filtered result", key)
		}
	}
}

func TestPreferKubernetesRBACResources_KeepsOpenShiftWhenKubernetesMissing(t *testing.T) {
	t.Parallel()

	deletable := []resources.Resource{
		resourceWithGVK(openShiftAuthorizationGroup, "clusterrolebindings", kindClusterRoleBinding, false),
		resourceWithGVK(openShiftAuthorizationGroup, "roles", kindRole, true),
		resourceWithGVK("apps", "deployments", "Deployment", true),
	}

	got := preferKubernetesRBACResources(deletable, deletable)
	if len(got) != 3 {
		t.Errorf("expected OpenShift RBAC types to be kept when Kubernetes twins are absent, got %d", len(got))
	}

	foundOpenShiftClusterRoleBinding := false
	foundOpenShiftRole := false

	for i := range got {
		gvk := got[i].GroupVersionKind()
		if gvk.Group == openShiftAuthorizationGroup && gvk.Kind == kindClusterRoleBinding {
			foundOpenShiftClusterRoleBinding = true
		}

		if gvk.Group == openShiftAuthorizationGroup && gvk.Kind == kindRole {
			foundOpenShiftRole = true
		}
	}

	if !foundOpenShiftClusterRoleBinding {
		t.Error("expected authorization.openshift.io/ClusterRoleBinding to be kept")
	}

	if !foundOpenShiftRole {
		t.Error("expected authorization.openshift.io/Role to be kept")
	}
}

func TestPreferKubernetesRBACResources_KeepsOpenShiftWhenKubernetesNotListable(t *testing.T) {
	t.Parallel()

	// Kubernetes twin is authorized for delete but not list; OpenShift has both.
	deletable := []resources.Resource{
		resourceWithGVK(kubernetesRBACGroup, "clusterrolebindings", kindClusterRoleBinding, false),
		resourceWithGVK(openShiftAuthorizationGroup, "clusterrolebindings", kindClusterRoleBinding, false),
		resourceWithGVK("apps", "deployments", "Deployment", true),
	}
	listable := []resources.Resource{
		resourceWithGVK(openShiftAuthorizationGroup, "clusterrolebindings", kindClusterRoleBinding, false),
		resourceWithGVK("apps", "deployments", "Deployment", true),
	}

	got := preferKubernetesRBACResources(deletable, listable)
	if len(got) != 3 {
		t.Fatalf("expected OpenShift alias to be kept when Kubernetes is not listable, got %d", len(got))
	}

	foundOpenShift := false
	foundKubernetes := false

	for i := range got {
		gvk := got[i].GroupVersionKind()
		if gvk.Group == openShiftAuthorizationGroup && gvk.Kind == kindClusterRoleBinding {
			foundOpenShift = true
		}

		if gvk.Group == kubernetesRBACGroup && gvk.Kind == kindClusterRoleBinding {
			foundKubernetes = true
		}
	}

	if !foundOpenShift {
		t.Error("expected authorization.openshift.io/ClusterRoleBinding to be kept")
	}

	if !foundKubernetes {
		t.Error("expected rbac.authorization.k8s.io/ClusterRoleBinding to remain in deletable set")
	}
}

func TestPreferKubernetesRBACResources_Empty(t *testing.T) {
	t.Parallel()

	if got := preferKubernetesRBACResources(nil, nil); got != nil {
		t.Errorf("expected nil input to return nil, got %#v", got)
	}

	if got := preferKubernetesRBACResources([]resources.Resource{}, nil); len(got) != 0 {
		t.Errorf("expected empty input to stay empty, got %d", len(got))
	}
}

func resourceWithGVK(group, resource, kind string, namespaced bool) resources.Resource {
	const version = "v1"

	scope := meta.RESTScopeRoot
	if namespaced {
		scope = meta.RESTScopeNamespace
	}

	return resources.Resource{
		RESTMapping: meta.RESTMapping{
			Resource: schema.GroupVersionResource{
				Group:    group,
				Version:  version,
				Resource: resource,
			},
			GroupVersionKind: schema.GroupVersionKind{
				Group:   group,
				Version: version,
				Kind:    kind,
			},
			Scope: scope,
		},
	}
}

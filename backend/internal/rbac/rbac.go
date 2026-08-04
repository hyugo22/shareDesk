// Package rbac fournit les vérifications de permissions basées sur les rôles.
// Le modèle est extensible : de nouveaux rôles/permissions peuvent être créés
// depuis l'interface d'administration sans changement de code (voir
// internal/repository.RoleRepo).
package rbac

func HasPermission(userPermissions []string, required string) bool {
	for _, p := range userPermissions {
		if p == required {
			return true
		}
	}
	return false
}

func HasAnyPermission(userPermissions []string, required ...string) bool {
	for _, r := range required {
		if HasPermission(userPermissions, r) {
			return true
		}
	}
	return false
}

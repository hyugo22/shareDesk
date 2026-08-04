// Package migrations embarque les fichiers de migration SQL dans le binaire
// backend, pour que l'image Docker n'ait pas besoin de les copier séparément.
package migrations

import "embed"

//go:embed *.sql
var FS embed.FS

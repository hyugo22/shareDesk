// Package trayicon génère à la volée de petites icônes .ico (un disque plein
// coloré sur fond transparent) pour l'icône de zone de notification, plutôt
// que de dépendre de fichiers d'assets graphiques externes.
package trayicon

import (
	"bytes"
	"encoding/binary"
	"image"
	"image/color"
	"image/png"
	"math"
)

const size = 32

// Circle rend un disque plein de la couleur donnée, encodé en .ico (conteneur
// ICO autour d'un PNG — supporté depuis Windows Vista, plus simple qu'un DIB brut).
func Circle(c color.RGBA) []byte {
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	center := float64(size) / 2
	radius := center - 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx, dy := float64(x)+0.5-center, float64(y)+0.5-center
			if math.Hypot(dx, dy) <= radius {
				img.Set(x, y, c)
			}
		}
	}

	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		panic(err) // ne peut pas échouer sur une image en mémoire valide
	}
	pngData := pngBuf.Bytes()

	var ico bytes.Buffer
	// ICONDIR : reserved(2)=0, type(2)=1 (icone), count(2)=1
	binary.Write(&ico, binary.LittleEndian, uint16(0))
	binary.Write(&ico, binary.LittleEndian, uint16(1))
	binary.Write(&ico, binary.LittleEndian, uint16(1))
	// ICONDIRENTRY (16 octets)
	ico.WriteByte(size)                                          // largeur
	ico.WriteByte(size)                                          // hauteur
	ico.WriteByte(0)                                              // palette (0 = pas de palette, vraies couleurs)
	ico.WriteByte(0)                                              // réservé
	binary.Write(&ico, binary.LittleEndian, uint16(1))             // plans couleur
	binary.Write(&ico, binary.LittleEndian, uint16(32))            // bits par pixel
	binary.Write(&ico, binary.LittleEndian, uint32(len(pngData)))  // taille des données image
	binary.Write(&ico, binary.LittleEndian, uint32(6+16))          // offset des données (après header+entry)
	ico.Write(pngData)

	return ico.Bytes()
}

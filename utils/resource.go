package utils

import (
	"github.com/sandertv/gophertunnel/minecraft/resource"
	"os"
	"path/filepath"
	"strings"
)

func GetResourcePacks() []*resource.Pack {
	var packs []*resource.Pack
	if _, err := os.Stat("resources"); !os.IsNotExist(err) {
		filepath.Walk("resources", func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}

			if strings.HasSuffix(info.Name(), ".zip") || strings.HasSuffix(info.Name(), ".mcpack") {
				pack, err := resource.Compile("resources/" + info.Name())
				if err != nil {
					return err
				}
				packs = append(packs, pack)
			}
			return nil
		})
	}
	return packs
}
